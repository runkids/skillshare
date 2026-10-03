---
sidebar_position: 3
---

# audit

扫描已安装的 skills，检测安全威胁和恶意模式。

```bash
skillshare audit                        # 扫描所有已安装的 skills
skillshare audit <name>                 # 扫描指定的已安装 skill
skillshare audit a b c                  # 扫描多个 skills
skillshare audit --group frontend       # 扫描某个 group 中的所有 skills
skillshare audit <path>                 # 扫描文件/目录路径
skillshare audit --threshold high       # 在 HIGH 及以上级别时阻断
skillshare audit -T h                   # 等同于 --threshold high
skillshare audit --format json           # JSON 输出
skillshare audit --format sarif         # SARIF 2.1.0 输出（GitHub Code Scanning）
skillshare audit --format markdown      # Markdown 报告（用于 GitHub Issues/PR）
skillshare audit --json                 # 等同于 --format json（已废弃）
skillshare audit -p                     # 扫描 project skills
skillshare audit --quiet                # 仅显示有发现结果的 skills
skillshare audit --yes                  # 跳过大规模扫描确认
skillshare audit --no-tui               # 纯文本输出（非交互式 TUI）
skillshare audit --profile strict       # 使用 strict profile（在 HIGH 及以上级别时阻断）
skillshare audit --dedupe global        # 完整的复合键去重
skillshare audit --analyzer static      # 仅运行静态分析器
skillshare audit --analyzer static --analyzer dataflow  # 使用多个分析器
```

## 何时使用

- 在安装新 skill 后查看安全性发现结果
- 扫描所有 skills，检测提示注入（prompt injection）、数据外泄或凭证访问模式
- 为你所在组织的安全策略自定义 audit 规则
- 生成用于合规审计（`--format json`）、静态分析工具（`--format sarif`）或文档（`--format markdown`）的 audit 报告
- 集成到 CI/CD 流水线，作为 skill 部署的准入门槛
- 将 SARIF 结果上传到 GitHub Code Scanning，实现 PR 级别的注释

## 检测内容

audit 引擎会依据 100 多条内置规则（正则表达式模式、基于表格的凭证检测、结构性检查、内容完整性校验，以及供应链信任分析），扫描每个 skill 目录中的所有文本类文件，结果分为 5 个严重级别：**CRITICAL**、**HIGH**、**MEDIUM**、**LOW** 和 **INFO**。

完整的检测目录、威胁类别详解、风险评分算法、命令安全分级以及跨 skill 交互分析，请参见 [Audit Engine](/docs/understand/audit-engine)。

## 示例输出

```
skillshare audit
Audit  ~/.config/skillshare/skills
  global · blocks at CRITICAL · policy DEFAULT / dedupe:GLOBAL / analyzers:ALL

! ci-release-helper  HIGH · risk 25/100
✗ suspicious-skill   CRITICAL · risk 35/100

Summary
  Scanned      12 skills
✓ Passed       9
! Warning      2
✗ Failed       1
  Severity     1 critical, 2 high, 1 medium
  Threats      credential:1 exfiltration:1 injection:1 privilege:1
  Risk         HIGH 35/100 · 100% auditable

✗ Blocked 1 of 12 skills: findings at CRITICAL or above · 2.1s
  The risk score is informational; only the severity blocks

Next
  skillshare audit suspicious-skill  see its findings
```

`Failed` 统计的是发现结果达到或超过当前阈值（`--threshold` 或配置项 `audit.block_threshold`；默认 `CRITICAL`）的 skills 数量。

`Threats` 用简称展示所有发现结果的类别分布：`inj`（注入）、`exfil`（外泄）、`cred`（凭证）、`obfusc`（混淆）、`priv`（权限提升）、`integ`（完整性）、`struct`（结构）、`risk`（风险）。当没有任何发现结果时，此行会被省略。在终端输出中，每个类别都有对应的颜色编码。

`audit.block_threshold` 只控制阻断阈值，**不会**禁用扫描本身。

### 交互式 TUI 模式

在交互式终端中扫描多个 skills 时，`audit` 会打开全屏界面，而不是逐行输出结果：左侧是 skills，有发现问题的排在前面；右侧是所选 skill 的风险摘要和发现项。打开 skill 会显示它的文件，逐一跳到每个发现项，并标出被标记的那一行与行号。按键列在界面底部。

只有在终端是交互式、输出不是 JSON、且结果多于一条时才会打开。使用 `--no-tui` 强制输出纯文本。

### 大规模扫描确认

在交互式终端中扫描超过 1,000 个 skills 时，命令会在继续之前提示确认。使用 `--yes` 可在 TTY 环境（例如本地自动化脚本）中跳过此提示。在 CI/CD 流水线（非 TTY）中，该提示会自动跳过。

## 策略与 Profile

audit 命令支持通过 profile、去重模式和分析器选择实现**策略驱动**的配置。这些均可通过 CLI 标志、project 配置或 global 配置进行设置。

### Profile

Profile 是为阈值和去重预设的合理默认值组合：

| Profile | 阈值 | 去重 | 使用场景 |
|---------|-----------|--------|----------|
| `default` | `CRITICAL` | `global` | 标准行为 —— 仅阻断 critical 级别威胁 |
| `strict` | `HIGH` | `global` | 注重安全的团队 —— 阻断 high 及以上级别威胁 |
| `permissive` | `CRITICAL` | `legacy` | 仅提示，不阻断 —— 最小化阻断，不进行 global 去重 |

```bash
skillshare audit --profile strict       # 在 HIGH 及以上级别阻断，global 去重
skillshare audit --profile permissive   # 仅提示模式
```

显式指定的标志始终会覆盖 profile 默认值：

```bash
skillshare audit --profile strict --threshold medium  # 使用 strict profile，但在 MEDIUM 及以上级别阻断
```

### 去重

当同一条发现结果被多个分析器检测到时（例如同时被 static 和 dataflow 检测到），去重功能会移除重复条目：

| 模式 | 行为 |
|------|------|
| `global` | 跨所有发现结果的完整复合键去重（默认） |
| `legacy` | 仅按分析器去重（pre-v0.16.9 行为） |

### 分析器选择

默认情况下会运行所有分析器。使用 `--analyzer` 可仅运行指定的分析器：

```bash
skillshare audit --analyzer static                    # 仅静态模式匹配
skillshare audit --analyzer static --analyzer dataflow # 使用多个分析器
```

| 分析器 | 作用范围 | 说明 |
|----------|-------|-------------|
| `static` | 逐文件 | 基于正则表达式，依据 audit 规则进行模式匹配 |
| `dataflow` | 逐文件 | 对 shell 脚本和 markdown 代码块进行污点追踪 |
| `tier` | 逐 skill | 能力分级组合风险分析 |
| `integrity` | 逐 skill | 内容哈希校验（SKILL.md 中的 `file_hashes`） |
| `metadata` | 逐 skill | 供应链信任校验（发布者不匹配、权威性声明） |
| `structure` | 逐 skill | 悬空 markdown 链接检测 |
| `cross-skill` | 整批 | 跨 skill 的外泄与权限提升分析 |

你也可以在配置中设置：

```yaml
audit:
  enabled_analyzers: [static, dataflow]
```

### 优先级 {#precedence}

设置按以下顺序解析（第一个非空值生效）：

1. CLI 标志（`--profile`、`--threshold`、`--dedupe`、`--analyzer`）
2. Project 配置（`.skillshare/config.yaml`）
3. Global 配置（`~/.config/skillshare/config.yaml`）
4. Profile 默认值

## 自动扫描

### 安装时

Skills 会在安装过程中自动扫描。达到或超过 `audit.block_threshold`（默认：`CRITICAL`）的发现结果会阻止安装：

```bash
skillshare install /path/to/evil-skill
# Error: security audit failed: critical threats detected in skill

skillshare install /path/to/evil-skill --force
# Installs with warnings (use with caution)

skillshare install /path/to/skill --audit-threshold high
# Per-command block threshold override

skillshare install /path/to/skill -T h
# Same as --audit-threshold high

skillshare install /path/to/skill --skip-audit
# Bypasses scanning (use with caution)
```

`--force` 会覆盖阻断决策。`--skip-audit` 会为该 install 命令禁用扫描。

没有可以全局禁用安装时 audit 的配置项。请仅在你确实需要绕过扫描的命令中使用 `--skip-audit`。

差异对比：

| Install 标志 | 是否运行 audit？ | 是否提供发现结果？ |
|--------------|-------------|---------------------|
| `--force` | 是 | 是（安装仍会继续） |
| `--skip-audit` | 否 | 否（扫描被跳过） |

如果两者同时提供，`--skip-audit` 实际生效，因为 audit 根本不会执行。

### 更新时

`skillshare update` 会在拉取 tracked 仓库后运行一次安全 audit。达到或超过当前阈值（默认为 `audit.block_threshold`，或通过 `--audit-threshold` / `--threshold` / `-T` 覆盖）的发现结果会触发回滚。详情参见 [`update --skip-audit`](/docs/reference/commands/update#security-audit-gate)。

你用 `--force` 接受的发现结果会针对该 skill 被记住，因此后续更新在同一条规则匹配到同一段文本时不会再次阻断。新的发现结果，或同一条规则匹配到不同的文本，仍会再次阻断。参见 [Accepted Findings](/docs/reference/commands/update#accepted-findings)。

当通过 install 更新 tracked 仓库时（`skillshare install <repo> --track --update`），会使用相同的阈值策略（`audit.block_threshold` 或 `--audit-threshold` / `--threshold` / `-T`）。

## CI/CD 集成

`audit` 命令专为流水线自动化设计。在非 TTY 环境（CI runner、管道输出）中，交互式 TUI 和确认提示会自动禁用 —— 无需 `--yes` 或 `--no-tui`。

完整的 CI/CD 工作流（GitHub Actions、GitLab CI、SARIF 上传、输出格式），请参见 [CI/CD Skill Validation recipe](/docs/how-to/recipes/ci-cd-skill-validation)。

### Pre-commit Hook

使用 [pre-commit](https://pre-commit.com/) 框架，在每次提交时自动运行 `skillshare audit`。该 hook 会扫描匹配 `.skillshare/` 或 `skills/` 目录的文件，如果发现结果超过你配置的阈值，则阻止提交。

```yaml
# .pre-commit-config.yaml
repos:
  - repo: https://github.com/runkids/skillshare
    rev: v0.16.11  # use latest release tag
    hooks:
      - id: skillshare-audit
```

完整设置说明参见 [Pre-commit Hook recipe](/docs/how-to/recipes/pre-commit-hook)。

## 最佳实践

### 面向个人开发者

- **先审计再信任** —— 从不受信任的来源安装 skills 后，务必运行一次 `skillshare audit`
- **查看发现结果，而不只看通过/未通过** —— "通过"的 skill 仍可能存在值得排查的 LOW/MEDIUM 发现结果
- **阅读 skill 文件** —— 自动化扫描能捕获已知模式，但新型攻击仍需人工审查

### 面向团队和组织

- **设置 `audit.block_threshold: HIGH`** —— 比默认的 `CRITICAL` 更严格，可捕获混淆和破坏性命令
- **创建面向组织的自定义规则** —— 为内部密钥格式添加模式（例如 `corp-api-key-*`）
- **使用 project 模式的规则做覆盖** —— 针对单个 project 降级预期模式，而不是全局降级

### 推荐的 Audit 工作流

1. **安装**：Skills 会被自动扫描 —— 超过阈值时会被阻断
2. **定期扫描**：定期运行 `skillshare audit`，以捕获安装后更新的规则
3. **Pre-commit hook**：借助 [pre-commit framework](/docs/how-to/recipes/pre-commit-hook)，在问题被提交前发现它们
4. **CI 准入门槛**：为共享的 skill 仓库在 CI 流水线中加入 audit
5. **自定义规则**：根据你所在组织的威胁模型定制检测逻辑
6. **查看报告**：使用 `--format json` 用于合规审计，`--format sarif` 用于 GitHub Code Scanning，或 `--format markdown` 用于 GitHub Issues/PR

### 阈值配置

在配置文件中设置阻断阈值：

```yaml
# ~/.config/skillshare/config.yaml
audit:
  block_threshold: HIGH  # Block on HIGH or above (stricter than default CRITICAL)
```

或按命令指定：

```bash
skillshare audit --threshold medium  # 在 MEDIUM 及以上级别阻断
```

### 完整的 Audit 配置

所有 audit 设置都可以持久化保存在 `config.yaml` 中：

```yaml
# ~/.config/skillshare/config.yaml (or .skillshare/config.yaml for project)
audit:
  block_threshold: HIGH                         # Blocking severity gate
  profile: strict                               # Profile preset (default/strict/permissive)
  dedupe_mode: global                           # Dedup mode (global/legacy)
  enabled_analyzers: [static, dataflow, tier]   # Limit to specific analyzers
```

CLI 标志会覆盖配置值。完整的解析顺序参见[优先级](#precedence)。

`skillshare status` 命令会展示解析后的 audit 策略，显示应用所有优先级层之后生效的 profile、阈值、去重模式和分析器列表。

## Web UI

audit 功能同样可在 `/audit` 的 web dashboard 中使用：

```bash
skillshare ui
# Navigate to Audit page → Click "Run Audit"
```

![Security Audit page in web dashboard](/img/web-audit-demo.png)

Dashboard 页面包含一个 Security Audit 区块，提供快速扫描摘要。

### 自定义规则编辑器

web dashboard 在 `/audit/rules` 提供了专门的 **Audit Rules** 页面，可直接在浏览器中创建和编辑自定义规则：

- **创建**：如果不存在 `audit-rules.yaml`，点击 "Create Rules File" 生成一个初始文件
- **编辑**：带语法高亮和校验的 YAML 编辑器
- **保存**：保存前会校验 YAML 格式和正则表达式模式

可从 Audit 页面通过 "Custom Rules" 按钮进入。

## 退出码

| 代码 | 含义 |
|------|---------|
| `0` | 没有达到或超过当前阈值的发现结果 |
| `1` | 存在一个或多个达到或超过当前阈值的发现结果 |

## 扫描的文件

audit 会扫描 skill 目录中的文本类文件：

- `.md`、`.txt`、`.yaml`、`.yml`、`.json`、`.toml`
- `.sh`、`.bash`、`.zsh`、`.fish`
- `.py`、`.js`、`.ts`、`.rb`、`.go`、`.rs`
- 没有扩展名的文件（例如 `Makefile`、`Dockerfile`）

扫描在每个 skill 目录内是递归的，因此 `SKILL.md`、嵌套的 `references/*.md` 以及 `scripts/*.sh` 只要匹配受支持的文本文件类型，都会被检查。

二进制文件（图片、`.wasm` 等）和隐藏目录（`.git`）会被跳过。

## 选项

| 标志 | 说明 |
|------|------------|
| `-G`, `--group` `<name>` | 扫描某个 group 中的所有 skills（可重复） |
| `-p`, `--project` | 扫描 project 级别的 skills |
| `-g`, `--global` | 扫描 global skills |
| `--threshold` `<t>`, `-T` `<t>` | 阻断阈值：`critical`\|`high`\|`medium`\|`low`\|`info`（简写：`c`\|`h`\|`m`\|`l`\|`i`，另加 `crit`、`med`） |
| `--profile` `<p>` | audit profile 预设：`default`、`strict`、`permissive` |
| `--dedupe` `<mode>` | 去重模式：`legacy`、`global`（默认） |
| `--analyzer` `<id>` | 仅运行指定的分析器（可重复）。可选 ID：`static`、`dataflow`、`tier`、`integrity`、`metadata`、`structure`、`cross-skill` |
| `--format` `<f>` | 输出格式：`text`（默认）、`json`、`sarif`、`markdown` |
| `--json` | 输出 JSON（**已废弃**：请改用 `--format json`） |
| `--yes`, `-y` | 跳过大规模扫描确认提示（自动确认） |
| `--quiet`, `-q` | 仅显示有发现结果的 skills 及摘要（隐藏干净的 ✓ 行） |
| `--no-tui` | 禁用交互式 TUI，打印纯文本输出 |
| `--init-rules` | 创建一个初始 `audit-rules.yaml`（遵循 `-p`/`-g`） |
| `-h`, `--help` | 显示帮助 |

### 子命令

| 子命令 | 说明 |
|-----------|-------------|
| `rules` | 浏览、启用和禁用 audit 规则（参见 [`audit rules`](/docs/reference/commands/audit-rules)） |

## Agent 支持

`skillshare audit agents` 会将安全扫描的范围限定为仅 agents，扫描 agents source 目录中的 `.md` 文件：

```bash
skillshare audit agents                    # 扫描所有 agents
skillshare audit agents --threshold high   # 对 agents 在 HIGH 及以上级别阻断
skillshare audit agents --format sarif     # 为 agents 生成 SARIF 输出
skillshare audit agents -p                 # 扫描 project agents
```

Agents 遵循与 skills 相同的 audit 规则、严重级别和阈值门槛。不带 `agents` 参数时，`audit` 仅扫描 skills（默认行为）。背景信息参见 [Agents](/docs/understand/agents)。

## 另请参阅

- [Audit Engine](/docs/understand/audit-engine) — 引擎工作原理（威胁模型、风险评分、命令分级）
- [`audit rules`](/docs/reference/commands/audit-rules) — 规则管理与自定义
- [install](/docs/reference/commands/install) — 安装 skills（含自动扫描）
- [check](/docs/reference/commands/check) — 校验 skill 完整性与同步状态
- [doctor](/docs/reference/commands/doctor) — 诊断配置问题
- [list](/docs/reference/commands/list) — 列出已安装的 skills
- [Securing Your Skills](/docs/how-to/advanced/security) — 面向团队和组织的安全指南
- [CI/CD Skill Validation](/docs/how-to/recipes/ci-cd-skill-validation) — 流水线自动化 recipe
- [Pre-commit Hook](/docs/how-to/recipes/pre-commit-hook) — 每次提交时自动运行 audit
- [Agents](/docs/understand/agents) — Agent 概念
