---
sidebar_position: 3
---

# File Structure

skillshare 的目录布局与文件位置。

## Overview

```
~/.config/skillshare/        # XDG_CONFIG_HOME
├── config.yaml              # 配置文件
├── audit-rules.yaml         # 自定义 audit 规则（可选）
├── mcp.yaml                 # MCP servers，若 sources.mcp 指向此处（可选）
├── skills/                  # Skills 来源（skills 及元数据）
│   ├── .metadata.json       # 已安装 skill 的元数据（自动管理）
│   ├── .skillignore         # 可选：从 sync 中排除 skill
│   ├── my-skill/            # 普通 skill
│   │   ├── SKILL.md         # Skill 定义（必需）
│   ├── code-review/         # 另一个 skill
│   │   └── SKILL.md
│   └── _team-skills/        # 被追踪的仓库
│       ├── .git/            # 保留的 Git 历史
│       ├── frontend/
│       │   └── ui/
│       │       └── SKILL.md
│       └── backend/
│           └── api/
│               └── SKILL.md
├── agents/                  # Agents 来源（单一 .md 文件）
│   ├── .agentignore         # 可选：从 sync 中排除 agent
│   ├── reviewer.md          # Agent 文件
│   └── auditor.md           # 另一个 agent
├── rules/                   # Extras 来源（若已配置）
│   ├── coding.md
│   └── testing.md
└── commands/                # Extras 来源（若已配置）
    └── deploy.md

~/.local/share/skillshare/   # XDG_DATA_HOME
├── backups/                 # Backup 目录
│   ├── 2026-01-20_15-30-00/
│   │   ├── claude/          # claude 的 skills 备份
│   │   ├── claude-agents/   # claude 的 agents 备份
│   │   └── cursor/
│   └── 2026-01-19_10-00-00/
│       └── claude/
└── trash/                   # 已卸载的 skills/agents（保留 7 天）
    ├── my-skill_2026-01-20_15-30-00/
    │   └── SKILL.md
    └── old-skill_2026-01-19_10-00-00/
        └── SKILL.md

~/.local/state/skillshare/   # XDG_STATE_HOME
├── logs/                    # 操作日志（JSONL）
│   ├── operations.log       # install、sync、update 等
│   └── audit.log            # 安全审计扫描
├── mcp/                     # MCP 同步状态（自动管理）
│   ├── state.json           # 记录 Skillshare 拥有哪些原生条目
│   └── backups/             # 每次写入前的 agent 文件备份（每个文件保留最新 20 份）
└── plugins/                 # 已审阅的插件来源本地副本

~/.cache/skillshare/         # XDG_CACHE_HOME      
├── version-check.json       # 版本检查缓存（24 小时 TTL）
└── ui/                      # Web UI dist 缓存
    └── 0.13.0/              # 按版本缓存的资源
        ├── index.html
        └── assets/
```

---

## Configuration File

### Location

```
~/.config/skillshare/config.yaml
```

**通过 XDG 覆盖：**
```
XDG_CONFIG_HOME=/custom/path → /custom/path/skillshare/config.yaml
```

**Windows 默认值：**
```
%AppData%\skillshare\config.yaml
```

### Contents

```yaml
# yaml-language-server: $schema=https://raw.githubusercontent.com/runkids/skillshare/main/schemas/config.schema.json
source: ~/.config/skillshare/skills
agents_source: ~/.config/skillshare/agents  # 可选；默认为 <source parent>/agents
mode: merge
targets:
  claude:
    path: ~/.claude/skills
    agents:                                 # 可选；为该 target 启用 agent 同步
      path: ~/.claude/agents
  cursor:
    path: ~/.cursor/skills
ignore:
  - "**/.DS_Store"
  - "**/.git/**"
```

完整参考请参见 [Configuration](/docs/reference/targets/configuration)。

---

## Metadata File

### Location

```
~/.config/skillshare/skills/.metadata.json
```

存储已安装及被追踪 skill 的元数据。它位于来源目录内，方便通过 git 同步以支持多机使用。由 `install`、`uninstall`、`update` **自动管理** — 请勿手动编辑。

### Contents

```json
{
  "skills": [
    {
      "name": "pdf",
      "source": "anthropics/skills/skills/pdf"
    },
    {
      "name": "_team-skills",
      "source": "github.com/team/skills",
      "tracked": true
    }
  ]
}
```

每个条目记录 skill 名称及其安装来源。被追踪的仓库（以 `_` 为前缀）会包含完整仓库 URL，供 `update` 与 `check` 操作使用。

---

## Source Directory

### Location

```
~/.config/skillshare/skills/
```

**Windows：**
```
%AppData%\skillshare\skills\
```

### Structure

```
skills/
├── .metadata.json                # 集中式 skill 元数据（自动管理）
├── skill-name/                   # Skill 目录
│   ├── SKILL.md                  # 必需：skill 定义
│   ├── examples/                 # 可选：示例文件
│   └── templates/                # 可选：代码模板
├── frontend/                     # 分类文件夹（通过 --into 或手动创建）
│   └── react-skill/              # 位于子目录中的 skill
│       └── SKILL.md              # 同步为 frontend__react-skill
└── _tracked-repo/                # 被追踪的仓库
    ├── .git/                     # Git 历史
    └── ...                       # Skill 子目录
```

---

## Skill Files

### SKILL.md (Required)

Skill 定义文件：

```markdown
---
name: skill-name
description: Brief description
---

# Skill Name

Instructions for the AI...
```

详情参见 [Skill Format](/docs/understand/skill-format)。

### .skillfollow / .skillfollow.local {#skillfollow-optional}

位于**配置的 skills source 根目录**，声明直接子链接名称（如 `_team-skills`）。Local 文件加入本机名称，不使用 ignore patterns，也不是嵌套 repo 配置。搭配 source `.gitignore` 的 `/_team-skills`、`/.skillfollow.local`（无末尾斜杠），每台机器自行创建链接；`skillshare follow`/`unfollow` 会一并写入或移除链接、声明和 ignore 行。`_` 前缀且含 `.git` 为 tracked repo；根目录含 `SKILL.md` 尚不支持。见[设置、严格名称与安全](../skillfollow.md)。

### .skillignore (Optional) {#skillignore-optional}

从发现（discovery）中排除 skill。支持两种位置：

**仓库级别** — 位于被追踪 skill 仓库的根目录。影响安装发现以及所有安装后命令（`doctor`、`status`、`list`、`sync` 等）：

```text title="_team-skills/.skillignore"
# 从发现中隐藏 vendored 包
.venv
node_modules

# 排除内部工具
validation-scripts
prompt-eval-*
```

**来源根目录级别** — 位于你的来源目录根部（`~/.config/skillshare/skills/.skillignore`）。全局适用于所有 skill（被追踪与非追踪均适用）：

```text title="~/.config/skillshare/skills/.skillignore"
# 临时屏蔽某个实验性 skill
my-experimental-skill

# 排除所有草稿
draft-*
```

使用 [gitignore 语法](https://git-scm.com/docs/gitignore) — 每行一个模式。支持 `*`（单层）、`**`（任意深度）、`?`、`[abc]`（字符类）、`!pattern`（取反）、`/pattern`（锚定）、`pattern/`（仅目录）以及 `\#`/`\!`（转义字面量）。以 `#` 开头的行是注释。像 `internal-tools` 这样的分组名会排除该目录下的所有 skill；`internal-tools/helper` 则只排除特定 skill。两层规则都会生效 — 只要任一匹配，该 skill 就会被排除。

:::tip
`.skillignore` 是三层过滤机制之一。所有场景（包括按 target 过滤及 SKILL.md 的 `targets`）参见 [Filtering Skills](/docs/how-to/daily-tasks/filtering-skills)。
:::

### .skillignore.local (Optional) {#skillignorelocal-optional}

与 `.skillignore` 配合使用的本地专属覆盖文件。放置在与 `.skillignore` 相同的目录中（来源根目录或被追踪仓库根目录）。`.skillignore.local` 中的模式会追加在 `.skillignore` 之后，因此取反模式（`!pattern`）可以覆盖基础文件的设置：

```text title="_team-skills/.skillignore.local"
# 仓库的 .skillignore 屏蔽了 private-*，但我需要保留自己的
!private-mine
```

此文件**不应**提交到版本控制 — 请将其加入 `.gitignore`。它的存在是为了让仓库使用者可以在本地覆盖仓库维护者的 `.skillignore`，而无需修改它。

启用后，`sync -v`、`status` 和 `doctor` 会显示 `.local active` 提示。


---

## Agent Files

Agent 是与 skill 不同的一种资源类型。它们存放在同级的来源目录中，并同步到支持 agent 的 target（Claude、Cursor、Augment、OpenCode）。

### Agent source directory

```
~/.config/skillshare/agents/      # 全局模式
.skillshare/agents/               # 项目模式
```

Agent 来源目录会由 `skillshare init` 与 `skills/` 一起自动创建。你可以通过 `agents_source` 配置字段覆盖全局位置；项目模式始终使用 `.skillshare/agents/`。

### Agent file format

每个 agent 都是一个带 frontmatter 的独立 Markdown 文件：

```markdown title="~/.config/skillshare/agents/reviewer.md"
---
name: reviewer
description: Reviews pull requests for security and style issues.
---

# Reviewer

Instructions for the AI agent...
```

Agent 文件名只能使用 `a-z`、`0-9`、`_`、`-`、`.`。与 skill 不同，agent 是**单一文件** — 它们不包含子目录。

完整的文件格式与发现规则参见 [Agents](/docs/understand/agents)。

### .agentignore (Optional)

从同步中排除 agent。位于 agent 来源根目录：

```text title="~/.config/skillshare/agents/.agentignore"
# 隐藏草稿
draft-*

# 禁用特定 agent
experimental-reviewer
```

使用 [gitignore 语法](https://git-scm.com/docs/gitignore)。与 `.skillignore` 相同的模式规则适用（`*`、`**`、`!negation`、`#` 注释等）。被禁用的 agent 仍保留在来源目录中，但会从同步中排除。

`skillshare disable <agent>` 与 `skillshare enable <agent>` 会自动添加/移除相应条目。

### .agentignore.local (Optional)

本地专属覆盖文件（与 `.skillignore.local` 模式相同）。放置在 `.agentignore` 旁边。模式会追加在 `.agentignore` 之后，因此 `!negation` 模式可以重新启用基础文件禁用的 agent。不应提交到版本控制。

---

## Backup Directory

### Location

```
~/.local/share/skillshare/backups/
```

### Structure

```
backups/
└── <timestamp>/             # YYYY-MM-DD_HH-MM-SS
    ├── claude/              # target 的备份
    │   ├── skill-a/
    │   └── skill-b/
    └── cursor/
        └── ...
```

备份的创建时机：
- 在 `sync` 与 `target remove` 之前自动创建
- 通过 `skillshare backup` 手动创建

---

## Trash Directory

### Location

```
~/.local/share/skillshare/trash/
```

**项目模式：**
```
<project>/.skillshare/trash/
```

### Structure

```
trash/
└── <skill-name>_<timestamp>/    # skill-name_YYYY-MM-DD_HH-MM-SS
    ├── SKILL.md
    └── ...                      # 保留所有原始文件
```

被移入 trash 的 skill：
- 由 `skillshare uninstall` 创建
- 保留 7 天后自动清理
- 以原 skill 名称加时间戳命名

---

## Log Directory

### Location

```
~/.local/state/skillshare/logs/
```

**项目模式：**
```
<project>/.skillshare/logs/
```

---

## Target Directories

Target 是 AI CLI 的 skill 目录。同步后，其中包含指向 source 的符号链接（或副本）。

### Merge mode

每个 skill 分别被符号链接。清单（manifest）会追踪受管理的 skill 以便清理孤立项：
```
~/.claude/skills/
├── my-skill -> ~/.config/skillshare/skills/my-skill
├── code-review -> ~/.config/skillshare/skills/code-review
├── local-only/              # 未被符号链接（用户自建，予以保留）
└── .skillshare-manifest.json  # 追踪受管理的 skill
```

### Copy mode

每个 skill 以真实文件形式复制。清单会追踪校验和以支持增量同步：
```
~/.cursor/skills/
├── my-skill/                  # 真实文件（从 source 复制而来）
├── code-review/               # 真实文件
├── local-only/                # 用户自建，予以保留
└── .skillshare-manifest.json  # 追踪受管理的 skill 及校验和
```

### Symlink mode

整个目录被符号链接：
```
~/.claude/skills -> ~/.config/skillshare/skills/
```

### 用副本代替文件链接（Windows）

在未开启 Developer Mode 的 Windows 上，agent targets 以及 `merge` mode 下的目录类 extras 会得到副本，而不是文件链接。此时 target 目录中会有一个 `.skillshare-manifest.json`，为每个副本记录 checksum，这样之后的 sync 就能更新或清理它们，而不会动到你自己的文件：
```
~/.claude/agents/
├── reviewer.md                # Copy of the source agent
├── local-agent.md             # User-created, preserved
└── .skillshare-manifest.json  # Tracks copied agents + checksums
```

单文件 extra（例如共享 `AGENTS.md`）则把它的副本记录在 skillshare 的 [extras 备份文件夹](../commands/extras.md#single-file-extras)中，而不是 target 目录中。

---

## Tracked Repositories

被追踪的仓库（以 `--track` 安装）会保留 git 历史：

```
_team-skills/
├── .git/                    # 保留的 Git
├── frontend/
│   └── ui/
│       └── SKILL.md
└── backend/
    └── api/
        └── SKILL.md
```

### Naming conventions

- `_` 前缀：被追踪的仓库
- 扁平化名称中的 `__`：路径分隔符

**在 source 中：**
```
_team-skills/frontend/ui/SKILL.md
```

**在 target 中（扁平化）：**
```
_team-skills__frontend__ui/SKILL.md
```

---

## Platform Differences

:::tip XDG Base Directory
skillshare 遵循 XDG Base Directory Specification。可通过 `XDG_CONFIG_HOME`、`XDG_DATA_HOME`、`XDG_STATE_HOME` 和 `XDG_CACHE_HOME` 覆盖基础目录。

详情参见 [Environment Variables](./environment-variables.md#xdg_config_home)。
:::

### macOS / Linux

| Item | Path |
|------|------|
| Config | `~/.config/skillshare/config.yaml` |
| Metadata | `~/.config/skillshare/skills/.metadata.json` |
| Skills source | `~/.config/skillshare/skills/` |
| Agents source | `~/.config/skillshare/agents/` |
| Backups | `~/.local/share/skillshare/backups/` |
| Trash | `~/.local/share/skillshare/trash/` |
| Logs | `~/.local/state/skillshare/logs/` |
| Version cache | `~/.cache/skillshare/version-check.json` |
| UI cache | `~/.cache/skillshare/ui/{version}/` |
| Link type | 符号链接（Symlinks） |

### Windows

| Item | Path |
|------|------|
| Config | `%AppData%\skillshare\config.yaml` |
| Metadata | `%AppData%\skillshare\skills\.metadata.json` |
| Skills source | `%AppData%\skillshare\skills\` |
| Agents source | `%AppData%\skillshare\agents\` |
| Backups | `%AppData%\skillshare\backups\` |
| Trash | `%AppData%\skillshare\trash\` |
| Logs | `%AppData%\skillshare\logs\` |
| Version cache | `%AppData%\skillshare\version-check.json` |
| UI cache | `%AppData%\skillshare\ui\{version}\` |
| Link type | 文件夹用 NTFS Junctions；单个文件用 symlinks（需要 Developer Mode，否则复制） |

## XDG Base Directory Layout

skillshare 在 Unix 系统上遵循 [XDG Base Directory Specification](https://specifications.freedesktop.org/basedir/latest/)：

| XDG Variable | Default Path | skillshare Uses For |
|-------------|-------------|---------------------|
| `XDG_CONFIG_HOME` | `~/.config` | `skillshare/config.yaml`、`skillshare/skills/`（含 `.metadata.json`）、`skillshare/agents/` |
| `XDG_DATA_HOME` | `~/.local/share` | `skillshare/backups/`、`skillshare/trash/` |
| `XDG_STATE_HOME` | `~/.local/state` | `skillshare/logs/` |
| `XDG_CACHE_HOME` | `~/.cache` | `skillshare/ui/`（下载的 Web dashboard） |

### Windows Paths

| Purpose | Path |
|---------|------|
| Config + Skills | `%AppData%\skillshare\` |
| Data (backups, trash) | `%AppData%\skillshare\` |
| State (logs) | `%AppData%\skillshare\` |
| Cache (UI) | `%AppData%\skillshare\` |

### Migration Note

如果从 XDG 拆分之前的版本升级，skillshare 会在首次运行时自动将数据从旧位置（`~/.config/skillshare/`）迁移到正确的 XDG 目录。

---

## Related

- [Configuration](/docs/reference/targets/configuration) — 配置文件详情
- [Skill Format](/docs/understand/skill-format) — SKILL.md 格式
- [Agents](/docs/understand/agents) — Agent 文件格式与发现规则
- [Tracked Repositories](/docs/understand/tracked-repositories) — 被追踪的仓库
