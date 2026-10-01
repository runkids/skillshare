---
sidebar_position: 4
---

# Configuration

skillshare 的配置文件参考。

## 概览

```text
~/.config/skillshare/
├── config.yaml          ← 配置文件
├── skills/              ← Source 目录（你的 Skill）
│   ├── .metadata.json   ← Skill 元数据（自动管理）
│   ├── my-skill/
│   ├── another/
│   └── _team-repo/      ← 已 track 的仓库
├── extras/              ← Extras Source 根目录
│   └── rules/           ← Extra 资源（例如 rules）

~/.local/share/skillshare/
└── backups/             ← 自动备份
    └── 2026-01-20.../
```

---

## IDE 支持（JSON Schema） {#ide-support}

配置文件包含一个 YAML Language Server 指令，可在支持的编辑器中启用**自动补全**、**校验**和**悬浮文档**。

`skillshare init` 创建的新配置会自动包含此项：

```yaml
# yaml-language-server: $schema=https://raw.githubusercontent.com/runkids/skillshare/main/schemas/config.schema.json
source: ~/.config/skillshare/skills
targets:
  claude:
    path: ~/.claude/skills
```

### 添加到现有配置

如果你的配置是在此功能出现之前创建的，将该注释添加为**第一行**：

**全局配置**（`~/.config/skillshare/config.yaml`）：
```yaml
# yaml-language-server: $schema=https://raw.githubusercontent.com/runkids/skillshare/main/schemas/config.schema.json
```

**项目配置**（`.skillshare/config.yaml`）：
```yaml
# yaml-language-server: $schema=https://raw.githubusercontent.com/runkids/skillshare/main/schemas/project-config.schema.json
```

或者直接重新运行 `skillshare init --force`（全局）或 `skillshare init -p --force`（项目），以带上 schema 注释重新生成配置。

### 支持的编辑器

| 编辑器 | 所需扩展 |
|--------|-------------------|
| VS Code | Red Hat 的 [YAML](https://marketplace.visualstudio.com/items?itemName=redhat.vscode-yaml) 扩展 |
| JetBrains IDEs | 内置 YAML 支持 |
| Neovim | 通过 LSP 使用 [yaml-language-server](https://github.com/redhat-developer/yaml-language-server) |

---

## 配置文件

**位置：** `~/.config/skillshare/config.yaml`

### 完整示例

```yaml
# yaml-language-server: $schema=https://raw.githubusercontent.com/runkids/skillshare/main/schemas/config.schema.json
# Source 目录（你编辑 Skill 的地方）
source: ~/.config/skillshare/skills

# 新 Target 的默认 Sync 模式
mode: merge

# 默认 Target 命名方式（flat 或 standard）
# target_naming: flat

# Targets（AI CLI Skill 目录）
targets:
  claude:
    path: ~/.claude/skills
    # mode: merge（继承自默认值）

  codex:
    path: ~/.codex/skills
    mode: symlink  # 覆盖默认模式
    include: [codex-*] # 仅 merge/copy 模式

  cursor:
    path: ~/.cursor/skills
    mode: copy  # 为 Cursor 提供真实文件
    exclude: [experimental-*] # 仅 merge/copy 模式

  # 自定义 Target
  myapp:
    path: ~/apps/myapp/skills

# 远程 Skill —— 由 install/uninstall 自动管理
skills:
  - name: pdf
    source: anthropics/skills/skills/pdf
  - name: _team-skills
    source: github.com/team/skills
    tracked: true

# 保存时将 $HOME 折叠为 ~（对 dotfiles 友好）
# preserve_tilde_on_save: true

# commit/push/pull 所使用的目录（skills 为默认值，此外还有 agents、extras、root）
# git_root: skills

# 自定义 agents Source（可选，覆盖默认位置）
agents_source: ~/my-agents

# 自定义 extras Source（可选，覆盖默认位置）
extras_source: ~/my-extras

# 需要 sync 的非 Skill 资源
extras:
  - name: rules
    source: ~/company-shared/rules   # 可选，针对该 extra 的覆盖设置
    targets:
      - path: ~/.claude/rules
      - path: ~/.cursor/rules
        mode: copy

# Sync 时要忽略的文件
ignore:
  - "**/.DS_Store"
  - "**/.git/**"
  - "**/node_modules/**"
  - "**/*.log"
```

---

## 字段

### `source`

指向你的 Skill 目录的路径（唯一的 Source）。

```yaml
source: ~/.config/skillshare/skills
```

**默认值：** `~/.config/skillshare/skills`

### `mode`

所有 Target 的默认 Sync 模式。

```yaml
mode: merge
```

| 值 | 行为 |
|-------|----------|
| `merge` | 每个 Skill 单独建立符号链接。本地 Skill 会被保留。**（默认）** |
| `copy` | 每个 Skill 以真实文件形式复制。适用于无法跟随符号链接的 AI CLI。 |
| `symlink` | 整个 Target 目录作为一个符号链接。 |

### `target_naming`

Merge/copy Sync 的默认 Target 命名策略。

```yaml
target_naming: flat
```

| 值 | 行为 |
|-------|----------|
| `flat` | 嵌套 Skill 用 `__` 分隔符扁平化（例如 `frontend__dev`）。**（默认）** |
| `standard` | 直接使用 SKILL.md 的 `name` 字段（例如 `dev`）。遵循 [Agent Skills 规范](https://agentskills.io/specification)。 |

### `targets`

需要 sync 到的 AI CLI Skill 目录。

```yaml
targets:
  <name>:
    path: <path>
    mode: <mode>  # 可选，覆盖默认值
    include: [<glob>, ...]  # 可选，仅 merge/copy 模式
    exclude: [<glob>, ...]  # 可选，仅 merge/copy 模式
```

**示例：**
```yaml
targets:
  claude:
    path: ~/.claude/skills

  codex:
    path: ~/.codex/skills
    mode: symlink

  custom:
    path: ~/my-app/skills
```

#### 某个 Agent 的另一个账号 {#agent-config-dir}

一个 Target 可以是某个内置 Agent 的第二个配置目录：通过 `CLAUDE_CONFIG_DIR` 启动的 Claude Code、通过 `CODEX_HOME` 启动的 Codex，或通过 `PI_CODING_AGENT_DIR` 启动的 Pi。指定 Agent 和该目录即可；skills 和 agents 路径会随之确定。

```yaml
targets:
  claude-work:
    agent: claude
    config_dir: ~/.claude-work   # skills 写入 ~/.claude-work/skills，agents 写入 ~/.claude-work/agents
  codex-work:
    agent: codex
    config_dir: ~/.codex-work    # skills 写入 ~/.codex-work/skills
```

Codex 也会读取共享的 `~/.agents/skills`，但一个账号只拥有它自己的目录，因此它的 skills 会写入 `<config_dir>/skills`。Pi 的行为相同。只有 Claude 有 agents 目录。

| 字段 | 说明 |
|-------|-------------|
| `agent` | 内置 Agent：`claude`（`CLAUDE_CONFIG_DIR`）、`codex`（`CODEX_HOME`）或 `pi`（`PI_CODING_AGENT_DIR`） |
| `config_dir` | 该账号的配置目录。绝对路径或以 `~` 开头，不能是该 Agent 的默认目录，且只能被一个 Target 使用 |
| `cli` | 可选。用兼容的 CLI 代替 Agent 本身来运行这个账号的 [plugin 命令](/docs/reference/commands/plugin#accounts)，例如 Pi 用 `omo`。填写 `PATH` 中的名称，或绝对路径（可以以 `~` 开头）。只能是一个可执行文件、不带参数；shell alias 不会生效 |

沿用 Agent 命令的兼容 CLI 可以运行这个账号的 plugin。例如 omo 基于 Pi：

```yaml
targets:
  omo:
    agent: pi
    config_dir: ~/.omo/agent
    cli: omo
```

`cli` 只改变由哪个程序安装和移除 plugin。Skills、agents 和 MCP server 仍照常写入 `config_dir`。

`mode`、`include`、`exclude` 以及其他 Target 设置的用法与任何 Target 相同。你自己写的 `skills.path` 或 `agents.path` 优先于推导出的路径。Target 名称也可以用作 [MCP target](/docs/reference/commands/mcp#accounts) 和 [plugin target](/docs/reference/commands/plugin#accounts)。 也可以作为 [hooks target](/docs/reference/commands/hooks#accounts) 使用。

#### 指示文件 {#target-instructions}

skillshare 知道许多内置 target 的指示文件（`CLAUDE.md`、`AGENTS.md`、`GEMINI.md` 等）。
对于其他工具，`instructions` 告诉 skillshare 该工具读取哪个文件，这样控制台就能显示和编辑它，
并接上[共享 AGENTS.md](../../how-to/daily-tasks/sharing-instructions.md)。这里设置的值会取代内置的文件。

```yaml
targets:
  myagent:
    path: ~/.myagent/skills
    instructions:
      path: ~/.myagent/AGENTS.md
      import: true        # 该工具会展开 @path 行
```

| 字段 | 说明 |
|-------|-------------|
| `instructions.path` | 该工具读取的文件。在全局配置中，是绝对路径或以 `~/` 开头的路径。在项目配置中，是相对于项目根目录的路径，例如 `.myagent/AGENTS.md`。必须指向文件，不能是目录 |
| `instructions.import` | 该工具会展开 `@path` 行时设为 `true`。此时它可以同时使用多份共享文件，每份以一行 import 加入。默认为 `false`：该工具只使用一份共享文件，以链接取代它自己的文件 |

控制台会在添加 target 时从 **自定义目标** 对话框写入这个字段，之后也可以在 target 的文件标签页中修改。当 target 正在使用共享文件时，控制台会拒绝更改或移除它。
移除它不会删除文件。

#### 其他文件 {#target-files}

除了指示文件，工具还可能读取其他普通文件，例如 Pi 的 `APPEND_SYSTEM.md`。控制台会在 target 页面上
为每个文件显示一个标签页。skillshare 会为 `pi` 和 `omp` 加上 `APPEND_SYSTEM.md`；`files` 列出你自己添加的文件。

```yaml
targets:
  pi:
    files:
      - SYSTEM.md
      - prompts/review.md
```

每个条目都相对于该工具的文件夹：在全局配置中，pi 是 `~/.pi/agent`；在项目中是 `.pi`。条目可以指定子文件夹，
但不能离开该文件夹：绝对路径、`..` 以及链接到外部的文件夹都会被拒绝。这个文件夹是该工具自己的配置文件夹，
例如 codex 的 `~/.codex`，或某个账号的 [`config_dir`](#agent-config-dir)；当 skillshare 不知道时，
则是 skills 文件夹的上一层。如果 target 的文件夹会是你的主目录或项目根目录，就不会有 **+** 按钮。

控制台会在你添加或移除标签页时写入这个字段。移除标签页不会删除文件。

#### 关闭 Skills {#skills-enabled}

`skills.enabled: false` 会停止向某个 target 同步 skills，同时 skillshare 仍继续管理它的 agents、MCP server 和指示文件。适用于已经会读取另一个 target 的 skills 文件夹的工具，避免它找到每个 skill 两次。

```yaml
targets:
  pi:
    skills:
      path: ~/.pi/agent/skills
      enabled: false
```

路径、模式和过滤器都会保留在配置中，以便之后重新开启 skills。可以用 `skillshare target <name> --skills=false` 设置它（这也会移除该文件夹中指向 source 的链接），或在添加 target 时使用 `--no-skills`。参见 [开启或关闭 Skills](/docs/reference/commands/target#skills-off)。

### `include` / `exclude`（Target 过滤器） {#include--exclude-target-filters}

使用逐 Target 的过滤器，控制在 **merge 和 copy 模式**下哪些 Skill 会被 sync。

```yaml
targets:
  codex:
    path: ~/.codex/skills
    include: [codex-*]
  claude:
    path: ~/.claude/skills
    exclude: [codex-*]
```

规则：
- 匹配对象是 Target 的扁平化名称（例如 `team__frontend__ui`）
- `include` 先被应用
- `exclude` 在 include 之后应用
- Pattern 语法使用 Go `filepath.Match`（`*`、`?`、`[...]`）
- 在 `symlink` 模式下，include/exclude 会被忽略
- 如果一个先前已 sync 的 Source 链接变为被排除状态，`sync` 会移除该 Target 条目
- Target 中原本已存在的本地非符号链接文件夹会被保留

#### Pattern 速查表

| Pattern | 匹配 | 典型用途 |
|---------|---------|-------------|
| `codex-*` | `codex-agent`、`codex-rag` | 基于前缀分组 |
| `team__*` | `team__frontend__ui` | 仓库 / 组命名空间 |
| `*-experimental` | `rag-experimental` | 基于后缀清理 |
| `core-?` | `core-a`、`core-1` | 单字符变体 |
| `[ab]-tool` | `a-tool`、`b-tool` | 少量明确指定的集合 |

#### 场景 A：仅 include

当某个 Target 应只接收一个专注的子集时使用 `include`。

```yaml
targets:
  codex:
    path: ~/.codex/skills
    include: [codex-*, shared-*]
```

用例：
- 让 Codex 只专注于编码相关的工作流
- 避免向该 Target 发送仅用于写作 / 研究的 Skill

#### 场景 B：仅 exclude

当某个 Target 应接收几乎所有内容、只排除已知子集时使用 `exclude`。

```yaml
targets:
  claude:
    path: ~/.claude/skills
    exclude: [*-experimental, codex-*]
```

用例：
- 让主要 Target 保持宽泛
- 隐藏不稳定或特定于其他 Target 的 Skill

#### 场景 C：include + exclude

当你想要先大范围 include，再挖出例外情况时，两者一起使用。

```yaml
targets:
  cursor:
    path: ~/.cursor/skills
    include: [core-*, team__*]
    exclude: [*-deprecated, team__legacy__*]
```

评估顺序：
1. 只保留匹配 `include` 的名称
2. 从中移除匹配 `exclude` 的项

给定以下 Source Skill：
- `core-auth`
- `core-deprecated`
- `team__frontend__ui`
- `team__legacy__docs`
- `misc-tool`

`cursor` 的结果：
- 被 sync：`core-auth`、`team__frontend__ui`
- 未被 sync：`core-deprecated`、`team__legacy__docs`、`misc-tool`

#### 通过 CLI 管理过滤器

除了手动编辑 YAML，也可以使用 `target` 命令：

```bash
# Skill
skillshare target claude --add-include "team-*"
skillshare target claude --add-exclude "_legacy*"
skillshare target claude --remove-include "team-*"

# Agent（仅适用于拥有 agents 路径的 Target）
skillshare target claude --add-agent-include "team-*"
skillshare target claude --add-agent-exclude "draft-*"
skillshare target claude --remove-agent-include "team-*"

skillshare sync  # 应用更改
```

重复的 pattern 会被静默忽略。无效的 glob pattern 会返回错误。Agent 过滤器使用与 Skill 过滤器相同的 glob 语法，但只在 `merge` 和 `copy` 模式下生效。在 `symlink` 模式下，Agent 过滤器会被忽略，因为整个 agents 目录会作为一个整体被链接。

完整参考参见 [target 命令](/docs/reference/commands/target#target-filters-includeexclude)。

#### Skill 级别的 targets {#skill-level-targets}

Skill 可以在 SKILL.md 中通过 `metadata.targets` 声明它们兼容哪些 Target。为兼容旧版 Skill，仍支持顶层的 `targets` 字段作为回退，但当两者同时存在时，`metadata.targets` 优先：

```yaml
---
name: claude-prompts
metadata:
  targets: [claude]
---
```

这是与配置层面 include/exclude 并行工作的**第二层**过滤：

```
Source Skills
  │
  ├─ Config include/exclude    ← 按 Target，由使用者设置
  │
  └─ Skill targets 字段        ← 按 Skill，由作者设置
      │
      ▼
  Sync 到 Target 的 Skill
```

**评估顺序：**
1. `include` —— 只保留匹配的名称
2. `exclude` —— 移除匹配的名称
3. `targets` 字段 —— 移除 targets 列表中不包含该 Target 的 Skill

两层都必须通过（AND 关系）。配置过滤器始终优先——即使某个 Skill 声明了 `targets: [claude]`，配置中的 `exclude: [claude-*]` 仍会将其排除。

**跨模式匹配：** `targets: [claude]` 会同时匹配全局 Target `claude` 和项目 Target `claude`，因为它们指的是同一个 AI CLI。参见 [supported targets](/docs/reference/targets/supported-targets)。

:::tip
当**使用者**想要控制什么内容去到哪里时，使用配置过滤器（`include`/`exclude`）。当**作者**知道某个 Skill 只适用于特定 AI CLI 时，使用 Skill 级别的 `targets`。
:::

#### 过滤器变更时已有的 Target 条目

当你添加或更改过滤器，然后运行 `skillshare sync` 时：

| Target 中已有的项目 | 会发生什么 |
|-------------------------|--------------|
| 现在被过滤掉的 Source 链接符号链接 / junction | 被移除（取消链接） |
| 现在被过滤掉的受管副本（copy 模式） | 被移除 |
| 在 Target 中创建的本地非符号链接目录 | 被保留 |
| 无关的本地内容 | 被保留 |

### `skills`

追踪远程安装的 Skill。由 `skillshare install` 和 `skillshare uninstall` 自动管理。

```yaml
skills:
  - name: pdf
    source: anthropics/skills/skills/pdf
  - name: _team-skills
    source: github.com/team/skills
    tracked: true
```

| 字段 | 是否必填 | 说明 |
|-------|----------|-------------|
| `name` | 是 | Skill 目录名称 |
| `source` | 是 | GitHub URL 或本地路径 |
| `tracked` | 否 | 若以 `--track` 安装则为 `true`（默认：`false`） |

当你不带任何参数运行 `skillshare install` 时，所有列出但尚未安装的 Skill 都会被安装。这使得 `config.yaml` 成为一份可移植的 Skill 清单——将其复制到另一台机器上，运行 `skillshare install && skillshare sync` 即可重现相同设置。

`skills:` 列表会在每次 `install` 和 `uninstall` 操作后自动更新。你不需要手动编辑它。

:::note 已迁移到 .metadata.json
从 v0.16.2 开始，已安装的 Skill 条目从 `config.yaml` 移动到了一个独立文件。在当前版本中，所有安装元数据都集中存储在 `skills/` 目录内的 `.metadata.json` 中。从旧格式（`registry.yaml`、逐 Skill 的 `.skillshare-meta.json`）迁移是首次运行时自动完成的。
:::

### `agents_source` {#agents-source}

Agent 的自定义 Source 目录。覆盖默认的 `~/.config/skillshare/agents/`。

```yaml
agents_source: ~/my-agents
```

设置后，所有 Agent 都会从此目录读取，而不是默认位置。支持 `~` 展开。

默认值：`~/.config/skillshare/agents/`（自动检测，除非需要自定义位置，否则无需显式设置）。

:::note 仅限 Global mode
Project mode 始终使用 `.skillshare/agents/`，不支持 `agents_source`。
:::

关于 Agent 文件格式、Sync 行为和支持的 Target 的详情，参见 [Agents](/docs/understand/agents)。

### `projects` {#projects}

从这份 global 配置获取 Skill 和 Agent 的项目文件夹。这些文件夹不需要自己的 `.skillshare/`，在任意目录下运行一次 `skillshare sync` 就能写入所有项目。

当项目需要拿到不同的 Skill 时使用它。全局 Target 已经能让每个项目看到相同的一套内容，而 [Project mode](/docs/understand/project-skills) 把设置保留在项目仓库中，方便队友使用。[Many Projects, One Config](/docs/how-to/recipes/many-projects-one-config#scenario) 比较了这三种方式。

```yaml
projects:
  <folder>:                  # 绝对路径，或以 ~ 开头
    name: <name>             # 可选，默认为文件夹名
    targets: [<target>, ...] # 该项目所用的工具
    skills:                  # 存在即 sync Skill；留空则同步全部
      mode: <mode>
      target_naming: <flat|standard>
      include: [<glob>, ...]
      exclude: [<glob>, ...]
    agents:                  # 存在即 sync Agent；留空则同步全部
      mode: <mode>
      include: [<glob>, ...]
      exclude: [<glob>, ...]
```

**示例：**
```yaml
projects:
  ~/work/shop-web:
    targets: [claude, cursor, codex]
    skills:
      mode: copy
      include: ["frontend-*"]
    agents: {}
  ~/work/api-server:
    targets: [claude]
    skills: {}
```

`targets` 中的每一项都是一个[受支持的 Target](./supported-targets.md)名称。Skillshare 会写入该工具在这个文件夹内的 project 路径，例如 `claude` 对应 `.claude/skills` 和 `.claude/agents`。不需要设置 `path`。

- **共享的文件夹只会被写入一次。** 多个工具会读取同一个 project 文件夹（`cursor` 和 `codex` 都读取 `.agents/skills`），它们会合并成同一个 Sync Target。
- **输出中的名称。** `sync`、`status`、`diff`、`doctor` 和 `backup` 会把一个项目的 targets 显示为 `<name>@<target>`，例如 `shop-web@claude`。`name` 不能包含 `@`、`/` 或 `\`，且两个项目不能共用同一个名称。
- **Agent** 只会写入拥有 project Agent 目录的工具。只设置了 `agents`、没有设置 `skills` 的项目只会 sync Agent。
- **缺失的文件夹会被跳过。** `sync` 会打印 `project <folder>: folder not found, skipped`，且不会重新创建你已经移动或删除的项目。
- **`target` 和 `collect` 不会改动 projects。** `skillshare target` 只列出并编辑 `targets` 部分，`collect` 也不会把项目自己的 Skill 拉回 Source。请在 `config.yaml` 中编辑 `projects`，或在仪表盘的 **Projects** 页面编辑。
- 带有 `targets` frontmatter 字段的 Skill 会与工具进行匹配，因此 `targets: [claude]` 能到达 `shop-web@claude`。

同样，这些文件夹下的 MCP server 列在 [`mcp.projects`](/docs/reference/commands/mcp#manage-several-projects-from-the-global-config) 中，以相同的文件夹作为 key。完整的分步设置参见 [Many Projects, One Config](/docs/how-to/recipes/many-projects-one-config)。

### `extras` {#extras}

将非 Skill 资源（rules、commands、prompts 等）sync 到任意目录。

```yaml
extras_source: ~/my-extras            # 可选，全局默认 Source
extras:
  - name: rules
    source: ~/company-shared/rules    # 可选，针对该 extra 的覆盖设置
    targets:
      - path: ~/.claude/rules
      - path: ~/.cursor/rules
        mode: copy
  - name: agents
    targets:
      - path: ~/.claude/agents
        flatten: true                 # 将子目录文件扁平化 sync
  - name: commands
    targets:
      - path: ~/.claude/commands
```

| 字段 | 是否必填 | 说明 |
|-------|----------|-------------|
| `name` | 是 | Extra 标识符 |
| `source` | 否 | 该 extra 的自定义 Source 目录（覆盖 `extras_source` 和默认值） |
| `file` | 否 | 只同步 source 目录中的这个文件：单纯的文件名，例如 `system.md` 或 `AGENTS.md`。参见[单文件 extras](../commands/extras.md#single-file-extras) |
| `targets` | 是 | Target 路径列表 |
| `targets[].path` | 是 | 目标目录 |
| `targets[].mode` | 否 | `merge`（默认）、`copy` 或 `symlink`；`import` 仅用于单文件 extras |
| `targets[].as` | 否 | 单文件 extra 在 target 中的文件名（默认：`file` 的名称） |
| `targets[].flatten` | 否 | 为 `true` 时，将子目录文件直接 sync 到 Target 根目录（不能与 `symlink` 或 `file` 同时使用） |

`extras_source` 会在 `skillshare init` 或首次 `extras init` 时自动填充为默认路径（`~/.config/skillshare/extras/`）。可覆盖它以对所有 extras 使用自定义位置。

**Source 解析**（三级优先级）：
1. 逐 extra 的 `source` → 精确路径（例如 `~/company-shared/rules`）
2. `extras_source` → `<extras_source>/<name>/`（例如 `~/my-extras/rules/`）
3. 默认值 → `~/.config/skillshare/extras/<name>/`

**Sync 模式：**
- `merge`（默认）—— 逐文件符号链接
- `copy` —— 逐文件复制
- `symlink` —— 整个目录符号链接

运行 `skillshare sync extras` 进行 sync，或运行 `skillshare sync --all` 一起 sync Skill 和 extras。

:::info 两种模式均支持
Extras 在 Global mode 和 Project mode 下都可用。在 Project mode 下，Source 为 `.skillshare/extras/<name>/`。
:::

用法详情参见 [sync extras](/docs/reference/commands/sync#sync-extras)。

### `ignore`

Sync 时要跳过的文件 glob pattern。

```yaml
ignore:
  - "**/.DS_Store"
  - "**/.git/**"
  - "**/node_modules/**"
```

**默认 pattern：**
- `**/.DS_Store`
- `**/.git/**`

### `gitlab_hosts`

使用嵌套子群组的自建 GitLab 实例的主机名。名称中包含 `gitlab` 或 `jihulab` 的主机会被自动检测——此字段仅在使用其他自定义域名时才需要。

```yaml
gitlab_hosts:
  - git.company.com
  - code.internal.io
```

当某个主机名列在这里时，`skillshare install` 会将完整的 URL 路径视为仓库（支持最多 20 层的嵌套子群组），而不是假定标准的 `owner/repo` 两段式拆分。

**不使用 `gitlab_hosts` 时：**
```bash
# git.company.com/team/frontend/ui → 克隆 "team/frontend"，子目录 "ui"
skillshare install git.company.com/team/frontend/ui
```

**使用 `gitlab_hosts: [git.company.com]` 时：**
```bash
# git.company.com/team/frontend/ui → 克隆 "team/frontend/ui"（完整路径）
skillshare install git.company.com/team/frontend/ui
```

**无需配置的变通方法：** 在末尾附加 `.git` 以标记仓库路径的结束：
```bash
skillshare install git.company.com/team/frontend/ui.git
```

条目必须是裸主机名（不含 scheme、路径或端口）。它们会被标准化为小写。

#### 环境变量

对于没有配置文件的 CI/CD 流水线，使用 `SKILLSHARE_GITLAB_HOSTS`（逗号分隔）：

```bash
SKILLSHARE_GITLAB_HOSTS=git.company.com,code.internal.io skillshare install git.company.com/team/frontend/ui
```

当配置文件和环境变量都设置了值时，两者会被**合并**（去重）。环境变量中的无效条目会被静默跳过。

### `azure_hosts`

自建 Azure DevOps Server 实例的主机名。针对 `dev.azure.com` 和 `*.visualstudio.com` 的内置 pattern 始终生效——此字段仅在使用自定义域名的本地部署 Azure DevOps Server 时才需要。

```yaml
azure_hosts:
  - azuredevops.mycompany.com
```

当某个主机名列在这里时，包含 `/_git/` 的 URL 会通过 Azure DevOps 解析逻辑处理，正确提取 org、project 和 repo，而不会在克隆 URL 末尾附加 `.git`。

**不使用 `azure_hosts` 时：**

```bash
# 回退到通用 HTTPS 解析——克隆 URL 会变成
# https://azuredevops.mycompany.com/Org/Project.git（错误）
skillshare install https://azuredevops.mycompany.com/Org/Project/_git/Repo
```

**使用 `azure_hosts: [azuredevops.mycompany.com]` 时：**

```bash
# 正确解析——克隆 URL 为
# https://azuredevops.mycompany.com/Org/Project/_git/Repo
skillshare install https://azuredevops.mycompany.com/Org/Project/_git/Repo
```

条目必须是裸主机名（不含 scheme、路径或端口）。它们会被标准化为小写。

#### 环境变量

对于 CI/CD 流水线，使用 `SKILLSHARE_AZURE_HOSTS`（逗号分隔）：

```bash
SKILLSHARE_AZURE_HOSTS=azuredevops.mycompany.com skillshare install \
  https://azuredevops.mycompany.com/Org/Project/_git/Repo
```

### `gitea_hosts`

自建 Gitea 实例的主机名。名称中包含 `gitea` 的主机，例如 `gitea.com` 或 `gitea.company.com`，会被自动检测。此字段仅在使用其他自定义域名时才需要。

```yaml
gitea_hosts:
  - git.company.com
```

当某个主机名列在这里时：

- `install` 和 `update` 会在该主机上使用 [`GITEA_TOKEN`](/docs/reference/appendix/environment-variables#gitea_token) 进行 HTTPS 身份验证
- 当 sparse checkout 不可用或失败时，`install` 会通过 Gitea Contents API 下载子目录，而不是克隆整个仓库。如果该 API 调用也失败，则回退到完整克隆。

条目必须是裸主机名（不含 scheme、路径或端口）。它们会被标准化为小写。

#### 环境变量

对于 CI/CD 流水线，使用 `SKILLSHARE_GITEA_HOSTS`（逗号分隔）：

```bash
SKILLSHARE_GITEA_HOSTS=git.company.com skillshare install https://git.company.com/team/skills/review
```

当配置文件和环境变量都设置了值时，两者会被**合并**（去重）。

### `cnb_hosts`

自建 [CNB](https://cnb.cool) 实例的主机名。`cnb.cool` 会被自动检测。此字段仅在其他域名上的私有部署时才需要。

```yaml
cnb_hosts:
  - cnb.company.com
```

列出的主机会使用 [`CNB_TOKEN`](/docs/reference/appendix/environment-variables#cnb_token) 进行 HTTPS 身份验证，子目录安装也可以通过 CNB contents API，并采用相同的回退到完整克隆的机制。

条目必须是裸主机名（不含 scheme、路径或端口）。它们会被标准化为小写。

#### 环境变量

```bash
SKILLSHARE_CNB_HOSTS=cnb.company.com skillshare install https://cnb.company.com/team/skills/review
```

### `audit`

安全审计配置。

```yaml
audit:
  block_threshold: CRITICAL
  profile: default
  dedupe_mode: global
  enabled_analyzers: [static, dataflow, tier, integrity]
```

| 字段 | 取值 | 默认值 | 说明 |
|-------|--------|---------|-------------|
| `block_threshold` | `CRITICAL`、`HIGH`、`MEDIUM`、`LOW`、`INFO` | `CRITICAL` | 阻止 `skillshare install` 的最低严重级别 |
| `profile` | `default`、`strict`、`permissive` | `default` | 审计 profile 预设（设置 threshold 和 dedupe 的默认值） |
| `dedupe_mode` | `legacy`、`global` | `global` | 结果去重模式 |
| `enabled_analyzers` | Analyzer ID 数组 | *(全部)* | 允许运行的 Analyzer 白名单（省略表示全部运行） |

**Profile** 设置合理的默认值，可被显式字段值覆盖：

| Profile | Threshold | Dedupe | 说明 |
|---------|-----------|--------|-------------|
| `default` | `CRITICAL` | `global` | 与当前行为相同 |
| `strict` | `HIGH` | `global` | 为注重安全的团队提供更严格的阻止 |
| `permissive` | `CRITICAL` | `legacy` | 仅提示，最小化阻止 |

**Analyzer ID：** `static`、`dataflow`、`tier`、`integrity`、`structure`、`cross-skill`

**优先级：** CLI flag → 项目配置 → 全局配置 → profile 默认值。

- `block_threshold` 只控制何时**阻止** install——扫描始终会运行
- 使用 `--skip-audit` 可为单次 install 跳过扫描
- 使用 `--force` 可覆盖阻止（结果仍会被显示）

### `context_budget`

Token 预算警告阈值。当 token 数量超过预算时，会在 `sync` 和 `analyze` 之后出现警告。

```yaml
context_budget:
  warn_always_loaded_tokens: 10000
  warn_on_demand_tokens: 100000
```

| 字段 | 类型 | 默认值 | 说明 |
|-------|------|---------|-------------|
| `warn_always_loaded_tokens` | integer | `10000` | 当始终加载的 token 超过此值时警告。`0` 表示禁用 |
| `warn_on_demand_tokens` | integer | `100000` | 当按需加载的 token 超过此值时警告。`0` 表示禁用 |

省略时，采用默认值（10K / 100K）。使用 `skillshare sync --quiet` 可抑制警告。输出格式参见 [sync — Context Cost](/docs/reference/commands/sync#context-cost)。

### `preserve_tilde_on_save`

为 `true` 时，会在写入 `config.yaml` 之前将 `$HOME` 前缀折叠回 `~`。这使磁盘上的配置在多台机器之间保持可移植——当配置通过 dotfiles（stow、chezmoi、yadm、裸 git 仓库）共享时非常有用。

```yaml
preserve_tilde_on_save: true
```

**默认值：** `false`（现有行为不变——路径以绝对形式保存）。

如果没有此 flag，每次保存都会将 `~/...` 路径重写为 `/home/alice/...`（展开形式）。当配置被版本控制并跨机器共享时，这会产生噪声很大的 diff 并破坏可移植性。

启用该 flag 后，序列化的 YAML 会对 `$HOME` 下的任何路径使用 `~`：

```yaml
# 之前（默认）：绝对路径，与机器绑定
source: /home/alice/.config/skillshare/skills
targets:
  claude:
    skills:
      path: /home/alice/.claude/skills

# 之后（preserve_tilde_on_save: true）：可移植
source: ~/.config/skillshare/skills
targets:
  claude:
    skills:
      path: ~/.claude/skills
```

内存中的配置不受影响——`Load()` 仍会照常展开 `~`。非 home 的绝对路径（例如 `/opt/shared/skills`）会原样保留。

:::note 仅限 Global mode
此选项仅适用于全局 `config.yaml`。项目配置（`.skillshare/config.yaml`）通常使用相对路径，不需要 tilde 折叠。
:::

### `git_root` {#git-root}

选择 `skillshare commit`、`push` 和 `pull` 所操作的目录。

```yaml
git_root: skills
```

| 值 | 被版本控制的目录 |
|-------|---------------------|
| `skills`（默认） | Skill Source（`~/.config/skillshare/skills/`） |
| `agents` | Agent Source（`~/.config/skillshare/agents/`） |
| `extras` | Extras Source（`~/.config/skillshare/extras/`） |
| `root` | 配置根目录（`~/.config/skillshare/`）—— Skill + Agent + Extras 在同一个仓库中；`config.yaml` 会被自动忽略 |

**默认值：** `skills`

在 init 期间通过 `skillshare init --git-root <scope>` 设置，或在 init 向导中交互式设置。

#### init 之后更改 scope

在已初始化的设置上，以非交互方式切换 scope：

```bash
skillshare init --git-root <scope>   # global mode；如果你的 cwd 是一个项目，加上 -g
```

这会在新的 scope 目录下初始化一个 git 仓库（如果那里已经有一个则复用），将 `git_root` 持久化到配置中，且不会提示或要求 `--remote`。不过它**不会**移动现有仓库——切换 scope 意味着"开始对一个不同的目录进行版本控制"，而不是"迁移历史记录"：

- **全新历史** —— `skillshare init --git-root <scope>` 会在新 scope 处初始化一个空仓库。
- **保留历史** —— 先执行 `mv <old-scope>/.git <new-scope>/.git`，然后运行 `skillshare init --git-root <scope>` 以记录该 scope。

你也可以直接在 `config.yaml` 中编辑 `git_root`。如果 `git_root` 指向一个没有仓库的目录，而另一个 scope 目录有仓库，`commit`/`push`/`pull` 会打印一条 "Git root mismatch" 错误，其中包含用于解决问题的确切 `skillshare init` / `mv` 命令。

:::note 仅限 Global mode
`git_root` 仅适用于 Global mode。Project mode 使用 `.skillshare/` 目录，不支持此字段。
:::

---

## 项目配置

**位置：** `.skillshare/config.yaml`（位于项目根目录）

项目配置使用与全局配置不同的格式。

```yaml
# yaml-language-server: $schema=https://raw.githubusercontent.com/runkids/skillshare/main/schemas/project-config.schema.json
# Targets —— 字符串或对象形式
targets:
  - claude                    # 字符串：带默认值的已知 Target
  - cursor
  - name: custom-ide               # 对象：自定义路径和模式
    path: ./tools/ide/skills
    mode: symlink
  - name: codex                    # 带过滤器的对象
    include: [codex-*]
    exclude: [codex-experimental-*]

# 远程 Skill —— 由 install/uninstall 自动管理
skills:
  - name: pdf
    source: anthropic/skills/pdf
  - name: _team-skills
    source: github.com/team/skills
    tracked: true                  # 带 git 历史克隆

# Audit —— 字段与全局配置相同
audit:
  block_threshold: HIGH
  profile: strict
```

### `targets`（项目）

支持两种 YAML 形式：

| 形式 | 示例 | 适用场景 |
|------|---------|-------------|
| **字符串** | `- claude` | 已知 Target，使用默认路径和 merge 模式 |
| **对象** | `- name: x, path: ..., mode: ..., include: [...], exclude: [...]` | 自定义路径、模式覆盖，或逐 Target 过滤器 |

对象形式的条目也可以设置 [`instructions`](#target-instructions)，路径相对于项目根目录。

### `skills`（项目）

与[全局 `skills` 字段](#skills)相同的 schema。由 `skillshare install -p` 和 `skillshare uninstall -p` 自动管理。

:::tip 可移植清单
`config.yaml` 在 Global mode 和 Project mode 下都是一份可移植的 Skill 清单——在新机器上运行 `skillshare install && skillshare sync`（在项目中运行 `skillshare install -p`）即可重现相同设置。
:::

---

## 管理配置

### 查看当前配置

```bash
skillshare status
# 显示 source、targets、模式
```

### 直接编辑配置

```bash
# 在编辑器中打开
$EDITOR ~/.config/skillshare/config.yaml

# 然后 sync 以应用更改
skillshare sync
```

### 重置配置

```bash
rm ~/.config/skillshare/config.yaml
skillshare init
```

---

## 自定义 Audit 规则

**位置：**

| 模式 | 路径 |
|------|------|
| Global | `~/.config/skillshare/audit-rules.yaml` |
| Project | `.skillshare/audit-rules.yaml` |

规则按以下顺序合并：**内置 → 全局 → 项目**。你可以添加新规则、禁用内置规则，或覆盖严重级别。

```yaml
rules:
  # 添加自定义规则
  - id: flag-todo
    severity: MEDIUM
    pattern: todo-comment
    message: "TODO comment found"
    regex: '(?i)\bTODO\b'

  # 禁用一条内置规则
  - id: insecure-http-0
    enabled: false
```

| 字段 | 是否必填 | 说明 |
|-------|----------|-------------|
| `id` | 是 | 唯一规则标识符 |
| `severity` | 是 | `CRITICAL`、`HIGH`、`MEDIUM`、`LOW`、`INFO` |
| `pattern` | 是 | Pattern 分类名称 |
| `message` | 是 | 可读的结果描述 |
| `regex` | 是 | 用于匹配的正则表达式 |
| `exclude` | 否 | 当该行也匹配此正则表达式时抑制匹配 |
| `enabled` | 否 | 设为 `false` 可禁用一条内置规则 |

生成一个初始文件：

```bash
skillshare audit --init-rules       # 全局
skillshare audit --init-rules -p    # 项目
```

完整详情参见 [audit 命令](/docs/reference/commands/audit)。

---

## 环境变量

| 变量 | 说明 |
|----------|-------------|
| `SKILLSHARE_CONFIG` | 覆盖配置文件路径 |
| `GITHUB_TOKEN` | 用于 API 速率限制问题 |

**示例：**
```bash
SKILLSHARE_CONFIG=~/custom-config.yaml skillshare status
```

---

## Skill 元数据

当你安装一个 Skill 时，skillshare 会将其元数据记录在集中式的 `.metadata.json` 文件中：

```json
{
  "skills": [
    {
      "name": "pdf",
      "source": "anthropics/skills/skills/pdf",
      "type": "github",
      "installed_at": "2026-01-20T15:30:00Z",
      "repo_url": "https://github.com/anthropics/skills.git",
      "subdir": "skills/pdf",
      "version": "abc1234"
    }
  ]
}
```

每个 Skill 条目包含：

| 字段 | 说明 |
|-------|-------------|
| `name` | Skill 目录名称 |
| `source` | 原始安装来源输入 |
| `type` | Source 类型（`github`、`local` 等） |
| `installed_at` | 安装时间戳 |
| `repo_url` | Git 克隆 URL（仅 git Source） |
| `subdir` | 子目录路径（仅 monorepo Source） |
| `version` | 安装时的 git commit hash |

`skillshare update` 和 `skillshare check` 使用它来判断从何处获取更新。

**请勿手动编辑此文件。**

---

## 平台差异

### macOS / Linux

```yaml
source: ~/.config/skillshare/skills
targets:
  claude:
    path: ~/.claude/skills
```

使用符号链接。

### Windows

```yaml
source: %AppData%\skillshare\skills
targets:
  claude:
    path: %USERPROFILE%\.claude\skills
```

文件夹通过 NTFS junction 链接（无需管理员权限）。单个文件——`merge` mode 下的 agents 和目录类 extras，以及单文件 extras——需要文件 symlink，而这需要 Developer Mode；没有开启时会改为复制。参见 [Windows 疑难解答](/docs/troubleshooting/windows#file-links-need-windows-developer-mode-copying-instead)。

---

## 相关内容

- [Source & Targets](/docs/understand/source-and-targets) — 核心概念
- [Sync Modes](/docs/understand/sync-modes) — Merge vs copy vs symlink
- [Environment Variables](/docs/reference/appendix/environment-variables) — 所有变量
