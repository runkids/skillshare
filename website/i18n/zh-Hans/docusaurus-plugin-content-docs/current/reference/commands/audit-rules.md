---
sidebar_position: 4
---

# audit rules

浏览、启用、禁用和自定义 audit 规则。

```bash
skillshare audit rules                          # 交互式 TUI 规则浏览器
skillshare audit rules --no-tui                 # 纯文本表格
skillshare audit rules --pattern credential-access  # 按 pattern 筛选
skillshare audit rules --severity high          # 按 severity 筛选
skillshare audit rules --disabled               # 仅显示已禁用的规则
skillshare audit rules --format json            # JSON 输出

skillshare audit rules disable prompt-injection-0           # 禁用单条规则
skillshare audit rules disable --pattern credential-access  # 禁用整个 group
skillshare audit rules enable prompt-injection-0            # 重新启用规则
skillshare audit rules enable --pattern credential-access   # 重新启用整个 group

skillshare audit rules severity destructive-commands-2 medium      # 降级单条规则
skillshare audit rules severity --pattern destructive-commands low  # 降级整个 group
skillshare audit rules reset                    # 移除所有自定义规则，恢复默认值

skillshare audit rules init                     # 创建初始 audit-rules.yaml
skillshare audit rules init -p                  # 创建 project 级别的规则文件
```

## Pattern 级规则

你可以在 `audit-rules.yaml` 中禁用或覆盖整个 pattern group：

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

Pattern 级条目只使用 `pattern` 而不使用 `id`。合并顺序：pattern 级规则先生效，之后 id 级规则可以覆盖某个被禁用 group 内的个别条目。

## 自定义规则 {#custom-rules}

你可以使用 YAML 文件添加、覆盖或禁用 audit 规则。规则按以下顺序合并：**内置 → global 用户 → project 用户**。

使用 `--init-rules`（或 `audit rules init`）创建一个带注释示例的初始文件：

```bash
skillshare audit --init-rules         # Create global rules file
skillshare audit -p --init-rules      # Create project rules file
```

### 文件位置

| 作用域 | 路径 |
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

### 字段

| 字段 | 是否必填 | 说明 |
|-------|----------|--------------|
| `id` | 是 | 稳定标识符。匹配的 ID 会覆盖内置规则。 |
| `severity` | 是* | `CRITICAL`、`HIGH`、`MEDIUM`、`LOW` 或 `INFO` |
| `pattern` | 是* | 规则类别名称（例如 `prompt-injection`） |
| `message` | 是* | 在发现结果中展示的可读描述 |
| `regex` | 是* | 用于匹配每一行的正则表达式 |
| `exclude` | 否 | 如果某一行同时匹配 `regex` 和 `exclude`，该发现结果会被抑制 |
| `enabled` | 否 | 设为 `false` 以禁用某条规则。禁用时只需提供 `id`。 |

*除非 `enabled: false`，否则为必填。

### 合并语义

每一层（先 global，再 project）都在上一层的基础上叠加应用：

- **相同 `id`** + `enabled: false` → 禁用该规则
- **相同 `id`** + 其他字段 → 替换整条规则
- **新的 `id`** → 作为自定义规则追加
- **仅 `pattern`**（无 `id`）+ `enabled: false` → 禁用所有匹配该 pattern 的规则
- **仅 `pattern`** + `severity` → 覆盖所有匹配规则的 severity
- **先 pattern 后 id** → id 级条目可以在被禁用的 pattern group 内重新启用个别规则

### 实用模板

可以此为起点，进行贴近实际场景的策略调优：

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

### 使用 `init` 快速上手

`audit rules init`（或 `audit --init-rules`）会创建一个带注释示例的初始 `audit-rules.yaml`，你可以取消注释并进行调整：

```bash
skillshare audit rules init          # → ~/.config/skillshare/audit-rules.yaml
skillshare audit rules init -p       # → .skillshare/audit-rules.yaml
```

生成的文件内容如下：

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

如果文件已存在，`init` 会以错误退出——它绝不会覆盖已有规则。

## 工作流程：修复误报

自定义规则的常见原因是某个合法 skill 触发了内置规则。以下是一个分步示例：

**1. 运行 audit，看到误报：**

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

**2. 从[内置规则表](#built-in-rule-ids)中找到规则 ID：**

pattern `destructive-commands` 中带有 `sudo` 的匹配对应规则 `destructive-commands-2`。

**3. 创建自定义规则文件（如果还没有的话）：**

```bash
skillshare audit rules init
```

**4. 添加规则覆盖，将其抑制或降级：**

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

或直接完全禁用它：

```yaml
rules:
  - id: destructive-commands-2
    enabled: false
```

**5. 重新运行 audit 以确认：**

```bash
$ skillshare audit ci-helper
Audit  ~/.config/skillshare/skills
  global · blocks at CRITICAL · policy DEFAULT / dedupe:GLOBAL / analyzers:ALL

✓ No issues found in ci-helper · 0.1s   # Now passes (or shows MEDIUM instead of HIGH)
```

### 验证更改

编辑规则后，重新运行 audit 以验证：

```bash
skillshare audit                     # Check all skills
skillshare audit <name>              # Check a specific skill
skillshare audit --json | jq '.skills[].findings'  # Inspect findings programmatically
```

摘要解读：

- `Failed` 统计的是发现结果达到或超过当前阈值的 skills 数量。
- `Warning` 统计的是发现结果低于阈值但高于"干净"状态的 skills 数量（例如当阈值为 `CRITICAL` 时，统计 `HIGH/MEDIUM/LOW/INFO`）。

## 内置规则 ID {#built-in-rule-ids}

使用 `id` 值来覆盖或禁用特定的内置规则：

基于正则表达式的规则的权威来源：
[`internal/audit/rules.yaml`](https://github.com/runkids/skillshare/blob/main/internal/audit/rules.yaml)

:::note 结构性、分级和跨 skill 检查

`dangling-link`、`content-tampered`、`content-oversize`、`content-missing` 和 `content-unexpected` 属于**结构性检查**（文件系统查找和哈希比较，而非正则表达式）。`low-analyzability` 是由 [Analyzability Score](/docs/understand/audit-engine#analyzability-score) 生成的**可分析性发现结果**。`tier-stealth`、`tier-destructive-network`、`tier-network-heavy`、`tier-interpreter` 和 `tier-interpreter-network` 是由 [Command Safety Tiering](/docs/understand/audit-engine#command-safety-tiering) profile 生成的**分级组合发现结果**。`cross-skill-*` 发现结果由 [Cross-Skill Interaction Detection](/docs/understand/audit-engine#cross-skill-interaction-detection) 生成。以上这些都出现在下面的表格中，但并未定义在 `rules.yaml` 里。

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

> **说明：** 上面每个凭证条目还会为不同访问方式生成变体 ID：`-copy`、`-redirect`、`-dd`、`-exfil`（例如 `credential-access-ssh-private-key-copy`）。要禁用某个特定变体，请在 `audit-rules.yaml` 中使用其完整 ID。

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

## 子命令

| 子命令 | 说明 |
|-----------|-------------|
| `rules` | 浏览、启用和禁用 audit 规则 |
| `rules disable <id>` | 按 ID 禁用单条规则 |
| `rules disable --pattern <p>` | 禁用所有匹配某个 pattern 的规则 |
| `rules enable <id>` | 按 ID 重新启用单条规则 |
| `rules enable --pattern <p>` | 重新启用所有匹配某个 pattern 的规则 |
| `rules severity <id> <level>` | 覆盖单条规则的 severity |
| `rules severity --pattern <p> <level>` | 覆盖某个 pattern group 中所有规则的 severity |
| `rules reset` | 移除所有自定义规则（恢复内置默认值） |
| `rules init` | 创建初始 `audit-rules.yaml`（等同于 `audit --init-rules`） |

## 另请参阅

- [`audit`](/docs/reference/commands/audit) — 主要的 audit 命令参考
- [Audit Engine](/docs/understand/audit-engine) — 引擎工作原理（威胁模型、风险评分、分级）
- [Securing Your Skills](/docs/how-to/advanced/security) — 面向团队的安全指南
- [CI/CD Skill Validation](/docs/how-to/recipes/ci-cd-skill-validation) — 流水线自动化 recipe
