---
sidebar_position: 2
---

# Recipe：CI/CD Skill 驗證

> 在你的 CI pipeline 中自動 audit 並 sync skills。

## 情境

你有一個團隊 skill repository，希望確保每個 PR 都：
- 通過安全 audit（無 prompt injection、憑證竊取等）
- 驗證 SKILL.md 格式
- 無錯誤地完成 sync

## 解決方案

### GitHub Actions（搭配 setup-skillshare）

[`setup-skillshare`](https://github.com/marketplace/actions/setup-skillshare) action 會在單一步驟中處理安裝、初始化，以及可選的安全 audit。

```yaml
name: Skill Validation
on:
  pull_request:
    paths:
      - 'skills/**'

jobs:
  validate:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: runkids/setup-skillshare@v1
        with:
          source: ./skills
          audit: true
          audit-threshold: high
      - run: skillshare sync --dry-run
```

### 搭配 SARIF 上傳的 GitHub Actions

若要透過 [GitHub Code Scanning](https://docs.github.com/en/code-security/code-scanning) 取得內嵌的 PR 註解，可使用 SARIF 輸出。設定 `audit-output` 後，Action 會儲存結果並記錄 exit code，而不會直接失敗，以便繼續上傳 SARIF 檔案。請在最後加上一步，在有稽核發現時讓 job 失敗：

```yaml
name: Skill Security Scan
on:
  pull_request:
    paths: ['skills/**']
  push:
    branches: [main]

jobs:
  validate:
    runs-on: ubuntu-latest
    permissions:
      security-events: write
    steps:
      - uses: actions/checkout@v4
      - uses: runkids/setup-skillshare@v1
        id: skillshare
        with:
          source: ./skills
          audit: true
          audit-threshold: high
          audit-format: sarif
          audit-output: results.sarif

      - name: Upload SARIF to Code Scanning
        if: always()
        uses: github/codeql-action/upload-sarif@v3
        with:
          sarif_file: results.sarif
          category: skillshare-audit

      - run: skillshare sync --dry-run

      - name: Fail on audit findings
        if: steps.skillshare.outputs.audit-exit-code != '0'
        run: exit 1
```

### 不使用 action（手動設定）

如果你不想使用該 action，可以直接安裝 skillshare：

```yaml
jobs:
  validate:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - run: |
          curl -fsSL https://raw.githubusercontent.com/runkids/skillshare/main/install.sh | sh
          echo "$HOME/.local/bin" >> "$GITHUB_PATH"
      - run: skillshare init --no-copy --all-targets --no-git --no-skill --source ./skills
      - run: skillshare audit --threshold high --format json
      - run: skillshare sync --dry-run
```

### GitLab CI

建立 `.gitlab-ci.yml`：

```yaml
skill-validation:
  image: ghcr.io/runkids/skillshare-ci:latest
  stage: test
  script:
    - skillshare init
    - skillshare install . --into ci-check
    - skillshare audit --threshold high --format json
    - skillshare sync --dry-run
  rules:
    - changes:
        - skills/**/*
```

### 使用 CI Docker Image

若要加快 pipeline 啟動速度，可使用預先建置的 CI image：

```yaml
# GitHub Actions
jobs:
  validate:
    runs-on: ubuntu-latest
    container:
      image: ghcr.io/runkids/skillshare-ci:latest
    steps:
      - uses: actions/checkout@v4
      - run: skillshare init && skillshare audit --format json
```

## 輸出格式

`audit` 指令支援多種輸出格式，適用於不同的 CI/CD 整合需求。

### 結束代碼（Exit Codes）

```bash
# 若任何 skill 的發現達到或超過門檻，則封鎖部署
skillshare audit --threshold high
echo $?  # 0 = 乾淨，1 = 有發現
```

### SARIF 輸出

[SARIF（Static Analysis Results Interchange Format）](https://docs.oasis-open.org/sarif/sarif/v2.1.0/sarif-v2.1.0.html) 是一個 OASIS 標準，被 GitHub Code Scanning、VS Code SARIF Viewer、Azure DevOps、SonarQube 等靜態分析工具所使用。

```bash
skillshare audit --format sarif              # 輸出到 stdout
skillshare audit --format sarif > results.sarif  # 儲存到檔案
```

SARIF 輸出包含：
- **工具中繼資料** — 工具名稱（`skillshare`）、版本，以及資訊 URI
- **規則** — 去重複後的規則描述，附帶 `security-severity` 分數
- **結果** — 每個發現對應到一個 SARIF result，包含檔案位置與嚴重程度

嚴重程度對應到 SARIF 等級：

| skillshare 嚴重程度 | SARIF 等級 | security-severity |
|---------------------|-------------|-------------------|
| CRITICAL | `error` | 9.0 |
| HIGH | `error` | 7.0 |
| MEDIUM | `warning` | 4.0 |
| LOW | `note` | 2.0 |
| INFO | `note` | 0.5 |

### Markdown 報告

產生一份適合貼進 GitHub Issues、Pull Requests 或文件的自足式 Markdown 報告：

```bash
skillshare audit --format markdown               # 印到 stdout
skillshare audit --format markdown > report.md   # 儲存到檔案
skillshare audit -p --format markdown > report.md  # Project mode
```

報告內容包含：
- **標頭** — 掃描數量、模式與門檻
- **摘要表** — 通過／警告／失敗數量、嚴重程度分布、風險分數、可分析性
- **發現** — 每個 skill 的表格，包含嚴重程度、模式、訊息與位置；可折疊片段
- **乾淨的 Skills** — 以逗號分隔、無任何發現的 skill 清單

### 搭配 jq 的 JSON 輸出

```bash
# 列出所有具有 CRITICAL 發現的 skills
skillshare audit --json | jq '[.skills[] | select(.findings[] | .severity == "CRITICAL")]'

# 擷取所有 skills 的風險分數
skillshare audit --json | jq '.skills[] | {name: .skillName, score: .riskScore, label: .riskLabel}'

# 依嚴重程度統計發現數量
skillshare audit --json | jq '[.skills[].findings[].severity] | group_by(.) | map({(.[0]): length}) | add'
```

## 驗證

- PR check 通過：audit 以 0 結束（無達到／超過門檻的發現）
- Audit 的 JSON 輸出可被下游工具解析
- SARIF 上傳會將發現顯示為 PR diff 上的內嵌註解
- Sync dry-run 顯示預期的 symlink 操作

## 變化

- **在 HIGH 嚴重程度時封鎖**：在 `audit` 加上 `--threshold HIGH`（或 `-T HIGH`）— 任何 HIGH 以上的發現都會以非 0 結束
- **用於 Code Scanning 的 SARIF**：搭配 `github/codeql-action/upload-sarif@v3` 使用 `--format sarif`，取得內嵌 PR 註解
- **平行驗證**：在不同的 CI job 中分別執行 audit 與 sync，取得更快的回饋
- **排程 audit**：每晚執行，以捕捉既有 skills 中新偵測到的模式

## 相關

- [安全 audit 指南](/docs/how-to/advanced/security)
- [`audit` 指令參考](/docs/reference/commands/audit)
- [`audit rules` 參考](/docs/reference/commands/audit-rules)
- [Audit 引擎](/docs/understand/audit-engine) — 引擎運作方式
- [Docker sandbox 指南](/docs/how-to/advanced/docker-sandbox)
