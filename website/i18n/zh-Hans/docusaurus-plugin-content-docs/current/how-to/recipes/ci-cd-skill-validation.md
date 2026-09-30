---
sidebar_position: 2
---

# Recipe: CI/CD Skill Validation

> 在 CI 流水线中自动审计并 Sync Skill。

## Scenario

你有一个团队 Skill 仓库，希望确保每个 PR：
- 通过安全审计（无 prompt injection、凭证窃取等）
- 验证 SKILL.md 格式
- Sync 不出错

## Solution

### GitHub Actions（配合 setup-skillshare）

[`setup-skillshare`](https://github.com/marketplace/actions/setup-skillshare) action 在一个步骤中完成安装、初始化和可选的安全审计。

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

### 带 SARIF 上传的 GitHub Actions

要通过 [GitHub Code Scanning](https://docs.github.com/en/code-security/code-scanning) 获得内联 PR 注释，使用 SARIF 输出：

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
```

### 不使用该 action（手动设置）

如果你不想使用该 action，可以直接安装 skillshare：

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

创建 `.gitlab-ci.yml`：

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

### 使用 CI Docker 镜像

为了加快流水线启动速度，使用预构建的 CI 镜像：

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

## Output Formats

`audit` 命令支持多种输出格式，以满足不同的 CI/CD 集成需求。

### Exit Codes

```bash
# 若任何 Skill 的 findings 达到或超过阈值，阻止部署
skillshare audit --threshold high
echo $?  # 0 = 干净, 1 = 存在 findings
```

### SARIF Output

[SARIF（Static Analysis Results Interchange Format）](https://docs.oasis-open.org/sarif/sarif/v2.1.0/sarif-v2.1.0.html) 是一种 OASIS 标准，被 GitHub Code Scanning、VS Code SARIF Viewer、Azure DevOps、SonarQube 等静态分析工具所使用。

```bash
skillshare audit --format sarif              # 输出到 stdout
skillshare audit --format sarif > results.sarif  # 保存到文件
```

SARIF 输出包含：
- **Tool metadata** — 工具名称（`skillshare`）、版本和信息 URI
- **Rules** — 带有 `security-severity` 分数的去重规则描述符
- **Results** — 每个 finding 映射为一个带有文件位置和严重程度等级的 SARIF result

严重程度到 SARIF 等级的映射：

| skillshare Severity | SARIF Level | security-severity |
|---------------------|-------------|-------------------|
| CRITICAL | `error` | 9.0 |
| HIGH | `error` | 7.0 |
| MEDIUM | `warning` | 4.0 |
| LOW | `note` | 2.0 |
| INFO | `note` | 0.5 |

### Markdown Report

生成一份自包含的 Markdown 报告，适合粘贴到 GitHub Issues、Pull Requests 或文档中：

```bash
skillshare audit --format markdown               # 打印到 stdout
skillshare audit --format markdown > report.md   # 保存到文件
skillshare audit -p --format markdown > report.md  # Project mode
```

该报告包含：
- **Header** — 扫描数量、模式和阈值
- **Summary table** — 通过/警告/失败数量、严重程度分布、风险分数、可分析性
- **Findings** — 按 Skill 分组的表格，包含严重程度、模式、消息和位置；可折叠的代码片段
- **Clean Skills** — 无 findings 的 Skill 的逗号分隔列表

### JSON Output with jq

```bash
# 列出所有存在 CRITICAL findings 的 Skill
skillshare audit --json | jq '[.skills[] | select(.findings[] | .severity == "CRITICAL")]'

# 提取所有 Skill 的风险分数
skillshare audit --json | jq '.skills[] | {name: .skillName, score: .riskScore, label: .riskLabel}'

# 按严重程度统计 findings 数量
skillshare audit --json | jq '[.skills[].findings[].severity] | group_by(.) | map({(.[0]): length}) | add'
```

## Verification

- PR 检查通过：audit 以 0 退出（无达到/超过阈值的 findings）
- Audit JSON 输出可被下游工具解析
- SARIF 上传在 PR diff 上显示为内联注释
- Sync dry-run 显示预期的 symlink 操作

## Variations

- **在 HIGH 严重程度时阻塞**：为 `audit` 添加 `--threshold HIGH`（或 `-T HIGH`）— 任何 HIGH 及以上的 finding 都会以非零状态退出
- **用于 Code Scanning 的 SARIF**：配合 `github/codeql-action/upload-sarif@v3` 使用 `--format sarif`，实现内联 PR 注释
- **并行验证**：在不同的 CI job 中分别运行 audit 和 sync，以获得更快的反馈
- **定期审计**：每晚运行以捕获现有 Skill 中新检测到的模式

## Related

- [Security audit guide](/docs/how-to/advanced/security)
- [`audit` command reference](/docs/reference/commands/audit)
- [`audit rules` reference](/docs/reference/commands/audit-rules)
- [Audit Engine](/docs/understand/audit-engine) — 引擎工作原理
- [Docker sandbox guide](/docs/how-to/advanced/docker-sandbox)
