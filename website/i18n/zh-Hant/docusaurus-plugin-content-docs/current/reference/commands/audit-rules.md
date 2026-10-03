---
sidebar_position: 4
---

# audit rules

瀏覽、啟用、停用並自訂稽核規則。

```bash
skillshare audit rules                          # 互動式 TUI 規則瀏覽器
skillshare audit rules --no-tui                 # 純文字表格
skillshare audit rules --pattern credential-access  # 依 pattern 篩選
skillshare audit rules --severity high          # 依嚴重程度篩選
skillshare audit rules --disabled               # 只顯示已停用的規則
skillshare audit rules --format json            # JSON 輸出

skillshare audit rules disable prompt-injection-0           # 停用單一規則
skillshare audit rules disable --pattern credential-access  # 停用整個群組
skillshare audit rules enable prompt-injection-0            # 重新啟用規則
skillshare audit rules enable --pattern credential-access   # 重新啟用群組

skillshare audit rules severity destructive-commands-2 medium      # 降級單一規則
skillshare audit rules severity --pattern destructive-commands low  # 降級整個群組
skillshare audit rules reset                    # 移除所有自訂規則，還原預設值

skillshare audit rules init                     # 建立起始 audit-rules.yaml
skillshare audit rules init -p                  # 建立 project 層級的 rules 檔案
```

## Pattern 層級規則

你可以在 `audit-rules.yaml` 中停用或覆寫整個 pattern 群組：

```yaml
rules:
  # Disable all credential-access rules
  - pattern: credential-access
    enabled: false

  # But keep .env detection
  - id: credential-access-env-file
    enabled: true

  # Downgrade all destructive-commands to MEDIUM
  - pattern: destructive-commands
    severity: MEDIUM
```

Pattern 層級的項目使用 `pattern` 而不使用 `id`。合併順序：pattern 層級規則會先套用，然後 id 層級規則可以覆寫已停用群組中的個別項目。

## Custom Rules {#custom-rules}

你可以使用 YAML 檔案新增、覆寫或停用稽核規則。規則會依序合併：**built-in → global 使用者 → project 使用者**。

使用 `--init-rules`（或 `audit rules init`）建立一個帶有註解範例的起始檔案：

```bash
skillshare audit --init-rules         # Create global rules file
skillshare audit -p --init-rules      # Create project rules file
```

### 檔案位置

| Scope | 路徑 |
|-------|------|
| Global | `~/.config/skillshare/audit-rules.yaml` |
| Project | `.skillshare/audit-rules.yaml` |

### 格式

```yaml
rules:
  # Add a new rule
  - id: my-custom-rule
    severity: HIGH
    pattern: custom-check
    message: "Custom pattern detected"
    regex: 'DANGEROUS_PATTERN'

  # Add a rule with an exclude (suppress matches on certain lines)
  - id: url-check
    severity: MEDIUM
    pattern: url-usage
    message: "External URL detected"
    regex: 'https?://\S+'
    exclude: 'https?://(localhost|127\.0\.0\.1)'

  # Override an existing built-in rule (match by id)
  - id: destructive-commands-2
    severity: MEDIUM
    pattern: destructive-commands
    message: "Sudo usage (downgraded to MEDIUM)"
    regex: '(?i)\bsudo\s+'

  # Disable a built-in rule
  - id: insecure-http-0
    enabled: false

  # Disable the dangling-link structural check
  - id: dangling-link
    enabled: false
```

### 欄位

| 欄位 | 必要 | 說明 |
|-------|----------|------|
| `id` | 是 | 穩定的識別碼。相符的 ID 會覆寫內建規則。 |
| `severity` | 是* | `CRITICAL`、`HIGH`、`MEDIUM`、`LOW` 或 `INFO` |
| `pattern` | 是* | 規則分類名稱（例如 `prompt-injection`） |
| `message` | 是* | 顯示於發現項目中的人類可讀說明 |
| `regex` | 是* | 用於比對每一行的正規表示式 |
| `exclude` | 否 | 若一行同時符合 `regex` 與 `exclude`，該發現項目會被抑制 |
| `enabled` | 否 | 設為 `false` 以停用規則。停用時只需要 `id`。 |

*除非 `enabled: false`，否則為必要欄位。

### 合併語意

每一層（global，再來是 project）都會疊加在前一層之上：

- **相同 `id`** + `enabled: false` → 停用該規則
- **相同 `id`** + 其他欄位 → 取代整條規則
- **新的 `id`** → 附加為自訂規則
- **只有 `pattern`**（沒有 `id`）+ `enabled: false` → 停用所有符合該 pattern 的規則
- **只有 `pattern`** + `severity` → 覆寫所有符合規則的嚴重程度
- **Pattern 後接 id** → id 層級的項目可以在已停用的 pattern 群組中重新啟用個別規則

### 實務範本

以此作為實際政策調整的起點：

```yaml
rules:
  # Downgrade hardcoded-secret to MEDIUM for educational/reference skills
  - pattern: hardcoded-secret
    severity: MEDIUM

  # Override built-in suspicious-fetch with internal allowlist
  - id: suspicious-fetch-0
    severity: MEDIUM
    pattern: suspicious-fetch
    message: "External URL used in command context"
    regex: '(?i)(curl|wget|invoke-webrequest|iwr)\s+https?://'
    exclude: '(?i)https?://(localhost|127\.0\.0\.1|artifacts\.company\.internal|registry\.company\.internal)'

  # Governance exception: disable noisy insecure-http signal
  - id: insecure-http-0
    enabled: false
```

### 使用 `init` 快速開始

`audit rules init`（或 `audit --init-rules`）會建立一個帶有註解範例的起始 `audit-rules.yaml`，你可以取消註解並自行調整：

```bash
skillshare audit rules init          # → ~/.config/skillshare/audit-rules.yaml
skillshare audit rules init -p       # → .skillshare/audit-rules.yaml
```

產生的檔案內容如下：

```yaml
# Custom audit rules for skillshare.
# Rules are merged on top of built-in rules in order:
#   built-in → global (~/.config/skillshare/audit-rules.yaml)
#            → project (.skillshare/audit-rules.yaml)
#
# Each rule needs: id, severity, pattern, message, regex.
# Optional: exclude (suppress match), enabled (false to disable).

rules:
  # Example: flag TODO comments as informational
  # - id: flag-todo
  #   severity: MEDIUM
  #   pattern: todo-comment
  #   message: "TODO comment found"
  #   regex: '(?i)\bTODO\b'

  # Example: disable a built-in rule by id
  # - id: insecure-http-0
  #   enabled: false

  # Example: disable the dangling-link structural check
  # - id: dangling-link
  #   enabled: false

  # Example: override a built-in rule (match by id, change severity)
  # - id: destructive-commands-2
  #   severity: MEDIUM
  #   pattern: destructive-commands
  #   message: "Sudo usage (downgraded)"
  #   regex: '(?i)\bsudo\s+'
```

如果檔案已存在，`init` 會以錯誤結束——它絕不會覆寫現有規則。

## 工作流程：修正誤判

自訂規則的常見原因，是一個合法的 skill 觸發了內建規則。以下是逐步範例：

**1. 執行 audit 並看到誤判：**

```bash
$ skillshare audit ci-helper
Audit  ~/.config/skillshare/skills
  global · blocks at CRITICAL · policy DEFAULT / dedupe:GLOBAL / analyzers:ALL

HIGH      Sudo escalation  SKILL.md:42
  destructive-commands-2 / static
  "sudo apt-get install -y jq"

  Risk       HIGH 15/100
  Auditable  100%

! ci-helper has findings below CRITICAL · 0.2s
```

**2. 從[內建規則表](#built-in-rule-ids)中找出規則 ID：**

pattern `destructive-commands` 搭配 `sudo` 符合規則 `destructive-commands-2`。

**3. 建立自訂規則檔案（若尚未建立）：**

```bash
skillshare audit rules init
```

**4. 加入規則覆寫以抑制或降級：**

```yaml
# ~/.config/skillshare/audit-rules.yaml
rules:
  # Downgrade sudo to MEDIUM for CI automation skills
  - id: destructive-commands-2
    severity: MEDIUM
    pattern: destructive-commands
    message: "Sudo usage (downgraded for CI automation)"
    regex: '(?i)\bsudo\s+'
```

或是完全停用它：

```yaml
rules:
  - id: destructive-commands-2
    enabled: false
```

**5. 重新執行 audit 以確認：**

```bash
$ skillshare audit ci-helper
Audit  ~/.config/skillshare/skills
  global · blocks at CRITICAL · policy DEFAULT / dedupe:GLOBAL / analyzers:ALL

✓ No issues found in ci-helper · 0.1s   # Now passes (or shows MEDIUM instead of HIGH)
```

### 驗證變更

編輯規則後，重新執行 audit 以驗證：

```bash
skillshare audit                     # Check all skills
skillshare audit <name>              # Check a specific skill
skillshare audit --json | jq '.skills[].findings'  # Inspect findings programmatically
```

摘要解讀：

- `Failed` 計算的是發現項目達到或超過目前門檻的 skills。
- `Warning` 計算的是發現項目低於門檻但高於 clean 的 skills（例如門檻為 `CRITICAL` 時的 `HIGH/MEDIUM/LOW/INFO`）。

## Built-in Rule IDs {#built-in-rule-ids}

使用 `id` 值來覆寫或停用特定的內建規則：

以正規表示式為基礎的規則，其真實來源為：
[`internal/audit/rules.yaml`](https://github.com/runkids/skillshare/blob/main/internal/audit/rules.yaml)

:::note 結構性、分層與跨 skill 檢查

`dangling-link`、`content-tampered`、`content-oversize`、`content-missing` 與 `content-unexpected` 是**結構性檢查**（檔案系統查找與雜湊比對，並非正規表示式）。`low-analyzability` 是由 [Analyzability Score](/docs/understand/audit-engine#analyzability-score) 產生的**可分析性發現項目**。`tier-stealth`、`tier-destructive-network`、`tier-network-heavy`、`tier-interpreter` 與 `tier-interpreter-network` 是由 [Command Safety Tiering](/docs/understand/audit-engine#command-safety-tiering) 設定檔產生的**分層組合發現項目**。`cross-skill-*` 發現項目是由 [Cross-Skill Interaction Detection](/docs/understand/audit-engine#cross-skill-interaction-detection) 產生。以上這些都會出現在下方表格中，但並未定義於 `rules.yaml` 中。

:::

| ID | Pattern | Severity |
|----|---------|----------|
| `prompt-injection-0` | prompt-injection | CRITICAL |
| `prompt-injection-1` | prompt-injection | CRITICAL |
| `prompt-injection-2` | prompt-injection | HIGH |
| `prompt-injection-3` | prompt-injection | CRITICAL |
| `prompt-injection-4` | prompt-injection | CRITICAL |
| `hidden-unicode-1` | invisible-payload | CRITICAL |
| `data-exfiltration-0` | data-exfiltration | CRITICAL |
| `data-exfiltration-1` | data-exfiltration | CRITICAL |
| `data-exfiltration-2` | data-exfiltration | MEDIUM |
| `data-exfiltration-3` | data-exfiltration | HIGH |
| `credential-access-ssh-private-key` | credential-access | CRITICAL |
| `credential-access-env-file` | credential-access | CRITICAL |
| `credential-access-aws-credentials` | credential-access | CRITICAL |
| `credential-access-etc-shadow` | credential-access | CRITICAL |
| `credential-access-git-credentials` | credential-access | CRITICAL |
| `credential-access-netrc` | credential-access | CRITICAL |
| `credential-access-gnupg` | credential-access | CRITICAL |
| `credential-access-kube-config` | credential-access | CRITICAL |
| `credential-access-vault-token` | credential-access | CRITICAL |
| `credential-access-terraform-creds` | credential-access | CRITICAL |
| `credential-access-gnome-keyring` | credential-access | CRITICAL |
| `credential-access-npmrc` | credential-access | CRITICAL |
| `credential-access-pypirc` | credential-access | CRITICAL |
| `credential-access-gem-credentials` | credential-access | CRITICAL |
| `credential-access-ssl-private` | credential-access | CRITICAL |
| `credential-access-ssh-host-key` | credential-access | CRITICAL |
| `credential-access-pgpass` | credential-access | CRITICAL |
| `credential-access-mysql-cnf` | credential-access | CRITICAL |
| `credential-access-etc-passwd` | credential-access | MEDIUM |
| `credential-access-azure-creds` | credential-access | HIGH |
| `credential-access-gcloud-creds` | credential-access | HIGH |
| `credential-access-docker-config` | credential-access | HIGH |
| `credential-access-gh-cli-token` | credential-access | HIGH |
| `credential-access-password-store` | credential-access | HIGH |
| `credential-access-macos-keychain-user` | credential-access | HIGH |
| `credential-access-macos-keychain-sys` | credential-access | HIGH |
| `credential-access-terraformrc` | credential-access | HIGH |
| `credential-access-cargo-credentials` | credential-access | HIGH |
| `credential-access-op-cli` | credential-access | HIGH |
| `credential-access-age-keys` | credential-access | HIGH |
| `credential-access-shell-history` | credential-access | LOW |
| `credential-access-openvpn` | credential-access | LOW |
| `credential-access-auth-log` | credential-access | INFO |
| `credential-access-unknown-dotdir` | credential-access | INFO |

> **注意：** 上方每一個 credential 項目也會依存取方式產生額外的變體 ID：`-copy`、`-redirect`、`-dd`、`-exfil`（例如 `credential-access-ssh-private-key-copy`）。若要停用特定變體，請在你的 `audit-rules.yaml` 中使用其完整 ID。

| ID | Pattern | Severity |
|----|---------|----------|
| `hidden-unicode-0` | hidden-unicode | HIGH |
| `hidden-unicode-2` | hidden-unicode | HIGH |
| `config-manipulation-0` | config-manipulation | HIGH |
| `hidden-comment-injection-1` | hidden-comment-injection | HIGH |
| `self-propagation-0` | self-propagation | HIGH |
| `destructive-commands-0` | destructive-commands | HIGH |
| `destructive-commands-1` | destructive-commands | HIGH |
| `destructive-commands-2` | destructive-commands | HIGH |
| `destructive-commands-3` | destructive-commands | HIGH |
| `destructive-commands-4` | destructive-commands | HIGH |
| `dynamic-code-exec-0` | dynamic-code-exec | HIGH |
| `dynamic-code-exec-1` | dynamic-code-exec | HIGH |
| `shell-execution-0` | shell-execution | HIGH |
| `hidden-comment-injection-0` | hidden-comment-injection | HIGH |
| `obfuscation-0` | obfuscation | HIGH |
| `fetch-with-pipe-0` | fetch-with-pipe | HIGH |
| `fetch-with-pipe-1` | fetch-with-pipe | HIGH |
| `fetch-with-pipe-2` | fetch-with-pipe | HIGH |
| `hardcoded-secret-0` | hardcoded-secret | HIGH |
| `hardcoded-secret-1` | hardcoded-secret | HIGH |
| `hardcoded-secret-2` | hardcoded-secret | HIGH |
| `hardcoded-secret-3` | hardcoded-secret | HIGH |
| `hardcoded-secret-4` | hardcoded-secret | HIGH |
| `hardcoded-secret-5` | hardcoded-secret | HIGH |
| `hardcoded-secret-6` | hardcoded-secret | HIGH |
| `hardcoded-secret-7` | hardcoded-secret | HIGH |
| `hardcoded-secret-8` | hardcoded-secret | HIGH |
| `hardcoded-secret-9` | hardcoded-secret | HIGH |
| `data-uri-0` | data-uri | MEDIUM |
| `escape-obfuscation-0` | escape-obfuscation | MEDIUM |
| `suspicious-fetch-0` | suspicious-fetch | MEDIUM |
| `ip-address-url-0` | ip-address-url | MEDIUM |
| `hidden-unicode-3` | hidden-unicode | MEDIUM |
| `untrusted-install-0` | untrusted-install | MEDIUM |
| `untrusted-install-1` | untrusted-install | MEDIUM |
| `insecure-http-0` | insecure-http | LOW |
| `external-link-0` | external-link | LOW |
| `dangling-link` | dangling-link | LOW |
| `content-tampered` | content-tampered | MEDIUM |
| `content-oversize` | content-oversize | MEDIUM |
| `content-missing` | content-missing | LOW |
| `content-unexpected` | content-unexpected | LOW |
| `shell-chain-0` | shell-chain | INFO |
| `low-analyzability` | low-analyzability | INFO |
| `tier-stealth` | tier-stealth | CRITICAL |
| `tier-destructive-network` | tier-destructive-network | HIGH |
| `tier-network-heavy` | tier-network-heavy | MEDIUM |
| `tier-interpreter` | tier-interpreter | INFO |
| `tier-interpreter-network` | tier-interpreter-network | MEDIUM |
| `cross-skill-exfiltration` | cross-skill-exfiltration | HIGH |
| `cross-skill-privilege-network` | cross-skill-privilege-network | MEDIUM |
| `cross-skill-stealth` | cross-skill-stealth | HIGH |
| `cross-skill-cred-interpreter` | cross-skill-cred-interpreter | MEDIUM |

## Subcommands

| Subcommand | 說明 |
|-----------|------|
| `rules` | 瀏覽、啟用並停用稽核規則 |
| `rules disable <id>` | 依 ID 停用單一規則 |
| `rules disable --pattern <p>` | 停用所有符合 pattern 的規則 |
| `rules enable <id>` | 依 ID 重新啟用單一規則 |
| `rules enable --pattern <p>` | 重新啟用所有符合 pattern 的規則 |
| `rules severity <id> <level>` | 覆寫單一規則的嚴重程度 |
| `rules severity --pattern <p> <level>` | 覆寫某個 pattern 群組中所有規則的嚴重程度 |
| `rules reset` | 移除所有自訂規則（還原內建預設值） |
| `rules init` | 建立起始 `audit-rules.yaml`（與 `audit --init-rules` 相同） |

## 另請參閱

- [`audit`](/docs/reference/commands/audit) — 主要 audit 指令參考
- [Audit Engine](/docs/understand/audit-engine) — 引擎運作原理（威脅模型、風險評分、分層）
- [Securing Your Skills](/docs/how-to/advanced/security) — 給團隊的安全指南
- [CI/CD Skill Validation](/docs/how-to/recipes/ci-cd-skill-validation) — Pipeline 自動化操作手冊
