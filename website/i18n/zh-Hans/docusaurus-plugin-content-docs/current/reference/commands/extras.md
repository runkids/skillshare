---
sidebar_position: 2
---

# extras

管理与 Skill 一起同步的非 Skill 资源（rules、commands、prompts 等）。

## 概述

Extras 是 skillshare 管理的额外资源类型 —— 可以把它们理解为"给非 Skill 内容用的 Skill"。常见用例包括在各工具间同步 AI rules、编辑器 commands 或 prompt 模板。

每个 extra 都有：
- 一个**名称**（例如 `rules`、`prompts`、`commands`）
- 一个**source 目录** —— 可通过 `extras_source` 或每个 extra 各自的 `source` 配置，默认为 `~/.config/skillshare/extras/<name>/`（global）或 `.skillshare/extras/<name>/`（project）
- 一个或多个用于同步文件的 **target**

在仪表板中，**Extras → Folders & files** 会列出每个 extra 及其 targets 和模式：

![Extras › Folders & files：rules 和 commands 同步到各自的 targets](/img/extras-folders.png)

## 命令

### `extras memory` {#extras-memory}

管理 `memory` extra 中的共享 Markdown 笔记，可用任意文本编辑器编辑。
[记忆共享教程（英文截图）](../../how-to/daily-tasks/sharing-memory)演示创建、目录浏览与跨 Agent 使用。

| 子命令 | 行为 |
|---|---|
| `init` | 创建没有 targets 的 memory extra，补上缺少的 `INDEX.md`、`LEARNED.md`，保留现有文件和设置 |
| `list` | 列出笔记；`--search <text>` 忽略大小写搜索路径和内容，包含子目录 |
| `show <note.md>` | 读取笔记；`--json` 包含 `version` hash |
| `write <note.md> --from <file\|->` | 从文件或 stdin 读取内容；创建时不指定 `--version`，更新须使用最近读取的 version |
| `delete <note.md> --version <hash>` | 备份后删除指定版本，拒绝过期或缺少的 version |
| `instructions` | 输出指向实际 source 目录的读取指引 |

各子命令支持 `--json`、`-g` / `--global`、`-p` / `--project` 和 `--help`。
未指定时自动判断 scope。默认 global 路径为 `~/.config/skillshare/extras/memory/`，
project 为 `.skillshare/extras/memory/`；沿用现有 extras source 覆盖设置。

笔记须为相对 `.md` 路径、UTF-8，最多 1 MiB；排除隐藏文件、隐藏目录和内部符号链接。过大或非 UTF-8 文件仍列出并标为不支持，其他正常笔记仍可使用。`wiki/architecture.md` 会自动创建目录。Dashboard 提供目录树、**Preview** / **Source**、**Copy path**、**Edit**、**Delete note** 和 **History**。**Move or rename** 可输入新的相对 `.md` 路径，创建缺少的文件夹，保留内容和权限，并拒绝同名目标或过期版本。移动前会在旧路径备份；Markdown 链接需自行修复。请保留来源根目录的 `INDEX.md`，供 agent 指引读取。

保存检查最近读取的 version。冲突会保留草稿，显示最新保存内容供比较。**Save my draft** 须确认，使用更新后的 version，备份已保存内容后再替换。删除也须确认、检查版本并备份。**History** 和删除后的恢复链接会打开 **Backup Files**，以笔记的绝对路径筛选。CLI 可用 `backup files show <absolute-path>` 和 `backup files restore <absolute-path> <id>`。

**New note** 的 **Link from INDEX.md** 在索引可读取时显示并默认勾选，于文件末尾附加链接，检查 version 并备份。失败仍保留新笔记。**Add to INDEX** 可添加未索引的笔记。失效链接会显示警告，不会自动移除。CLI 写入不会新增索引链接。

使用 **Connect to agents** 选择工具，再 **Review changes** → **Apply changes**。此流程将 scope/hash 标记块添加或更新至现有 instructions 或共享来源，保留其他内容与分配。可检查更改、其他读取工具与已知字符上限。现有文件会备份，过期预览会被拒绝。完整但过期的块可经检查后更新；手动修改或格式错误的块会保留。未同步或无法读取的 instructions 文件会跳过。

**Configured** 仅表示读取链已有当前指引，不代表已读取。**Copy verification prompt** 用于新会话，要求 Agent 读取 `INDEX.md` 与相关笔记、报告完整路径及用户加入的临时验证值。请手动检查实际 read tool event；没有保证可用的读取 telemetry。

**Copy guidance** 是手动粘贴的替代方式，**Open AGENTS.md** 可编辑 instructions。Project 内的来源路径相对于 **project root**，不依 instructions 文件位置；外部或 global 来源用绝对路径，移动后须重新生成。CLI `instructions` 也输出相同的 scope/hash 块，未新增 CLI flags。不启用 native automatic memory、自动学习或 Obsidian 集成。

### `extras init`

创建一个新的 extra 资源类型。

```bash
# 交互式向导
skillshare extras init

# CLI flags
skillshare extras init <name> --target <path> [--target <path2>] [--mode <mode>]

# 单文件 extra
skillshare extras init <name> --file <filename> [--as <filename>] --target <path> [--source <dir>] [--mode <mode>]
```

向导会在输入名称后询问 **What do you want to sync?**：**Folder** 或 **Single file**。

**Options:**

| Flag | Description |
|------|-------------|
| `--target <path>` | Target 目录路径（可重复） |
| `--file <filename>` | 只同步 source 目录中的这个文件，创建一个[单文件 extra](#single-file-extras)。必须是单纯的文件名，不能包含 `/` 或 `\` |
| `--as <filename>` | 写入每个 target 的文件名（默认为 `--file` 的名称）。需要搭配 `--file` |
| `--mode <mode>` | Sync mode：`merge`（默认）、`copy` 或 `symlink`；`import` 仅可搭配 `--file` |
| `--flatten` | 将子目录中的文件直接同步到 target 根目录（不能与 `symlink` mode 或 `--file` 一起使用） |
| `--source <path>` | 该 extra 的自定义 source 目录（覆盖 `extras_source` 和默认值；project mode 下为相对于项目根目录的路径） |
| `--force` | 如果 extra 已存在则覆盖 |
| `--no-tui` | 跳过交互式向导，仅使用 CLI flags |
| `--project, -p` | 在 project 配置（`.skillshare/`）中创建 |
| `--global, -g` | 在 global 配置中创建 |

:::note
`--source` 仅在 global mode 下支持。Project mode 始终使用 `.skillshare/extras/<name>/` 作为 source 目录。
:::

**Examples:**

```bash
# 将 rules 同步到 Claude 和 Cursor
skillshare extras init rules --target ~/.claude/rules --target ~/.cursor/rules

# 使用自定义 source 目录
skillshare extras init rules --target ~/.claude/rules --source ~/company-shared/rules

# 用新的 targets 覆盖一个已存在的 extra
skillshare extras init rules --target ~/.cursor/rules --force

# project 范围的 extra，使用 copy mode
skillshare extras init prompts --target .claude/prompts --mode copy -p

# 以扁平方式同步 agents（像 Claude Code 这样的工具只能发现扁平文件）
skillshare extras init agents --target ~/.claude/agents --flatten

# 同步一个文件，并在 target 中重命名
skillshare extras init pi-prompt --file system.md --as APPEND_SYSTEM.md \
  --source ~/dotfiles/prompts --target ~/.pi/agent
```

`extras init` 只写入配置，不会创建 source 文件，也不会执行同步。对于单文件 extra，它会打印完整的 source 与 target 文件路径：

```
  Source    ~/dotfiles/prompts/system.md
  Target    ~/.pi/agent/APPEND_SYSTEM.md · merge

✓ Created extra pi-prompt (single file)

Next
  skillshare sync extras  sync it
```

如果 source 文件尚不存在，source 行末尾会显示 `(not found)`，最后一行则为 `Create the source file, then run 'skillshare sync extras'.`

### `extras list`

列出所有已配置的 extras 及其同步状态。默认启动交互式 TUI。

```bash
skillshare extras list [--json] [--no-tui] [-p|-g]
```

**Options:**

| Flag | Description |
|------|-------------|
| `--json` | JSON 输出（包含 `source_type`：`per-extra` / `extras_source` / `default`，以及设置时的每个 target 的 `extension` 字段） |
| `--no-tui` | 禁用交互式 TUI，使用纯文本输出 |
| `--project, -p` | 使用 project-mode extras（`.skillshare/`） |
| `--global, -g` | 使用 global extras（`~/.config/skillshare/`） |

#### 交互式 TUI

在 TTY 上，`extras list` 会打开交互式界面：左侧是 extras，右侧是所选 extra 的 targets 和文件。在这里可以新建、移除、同步和收回（collect）extras，也可以更改某个 target 的模式或 flatten 设置。按键列在界面底部。

可以用 `skillshare tui off` 永久关闭 TUI。

#### 纯文本输出

当 TUI 被禁用时（通过 `--no-tui`、`skillshare tui off`，或输出被管道传递）：

```
$ skillshare extras list --no-tui
rules  ~/.config/skillshare/extras/rules · 2 files
✓ ~/.claude/rules  merge
✓ ~/.cursor/rules  copy

codex-agents  ~/.config/skillshare/agents · 3 files
✓ ~/.codex/agents  extension: codex-agents

2 extras
```

对于[单文件 extra](#single-file-extras)，source 和每个 target 显示的是完整文件路径，而不是目录。

已同步的行只显示图标、路径和 mode；未同步的行会附加一个状态词（`drift`、`modified`、`not synced`、`no source`）。带有 transform extension 的 target 会显示为 `extension: <name>`，取代 sync mode 的位置（其底层 mode 始终是 `copy`）。

### `extras source`

显示或设置全局 `extras_source` 目录。这是存放 extras source 文件的默认父目录。

```bash
skillshare extras source            # 显示当前值
skillshare extras source <path>     # 设置新值
```

不带参数时，显示当前的 `extras_source` 路径（若为自动检测则显示 `(default)`）。带路径参数时，更新 global 配置中的 `extras_source`。

:::note
此命令仅限 global 使用。Project mode 始终使用 `.skillshare/extras/`，不支持 `extras_source`。
:::

**Examples:**

```bash
# 显示当前的 extras_source
skillshare extras source

# 设置为共享目录
skillshare extras source ~/company-shared/extras
```

### 操作一个已存在的 extra

通过 `extras <name>` 更改 target 的 sync mode、flatten 设置，或添加／移除 target。更改 mode、flatten 或添加 target 后，请运行 `skillshare sync extras` 应用。`--remove-target --prune` 也会立即还原或移除受管理的文件。

```bash
skillshare extras <name> --mode <mode> [--target <path>] [-p|-g]
skillshare extras <name> --flatten | --no-flatten [--target <path>]
skillshare extras <name> --add-target <path> [--as <filename>] [--mode <mode>] [--flatten] [-p|-g]
skillshare extras <name> --remove-target <path> [--prune] [-p|-g]
skillshare extras <name> --help
```

**Options:**

| Flag | Description |
|------|-------------|
| `--mode <mode>` | 新的 sync mode：`merge`、`copy` 或 `symlink`；`import` 仅用于[单文件 extras](#single-file-extras) |
| `--flatten` | 启用 flatten（将子目录文件同步到 target 根目录） |
| `--no-flatten` | 禁用 flatten |
| `--add-target <path>` | 为该 extra 添加一个新 target |
| `--as <filename>` | `--add-target` 的 target 文件名（仅限单文件 extra；默认为 `file`） |
| `--remove-target <path>` | 从该 extra 中移除一个 target（默认仅修改配置） |
| `--prune` | 与 `--remove-target` 一起使用：同时删除该 target 下由 skillshare 管理的文件。对单文件 extra 则改为还原 target 文件 |
| `--target <path>` | Target 目录路径（对多 target 的 extra 使用 `--mode` 时必填；省略时 `--flatten`/`--no-flatten` 会应用于所有 target） |
| `--project, -p` | 使用 project-mode extras（`.skillshare/`） |
| `--global, -g` | 使用 global extras（`~/.config/skillshare/`） |

**Examples:**

```bash
# 更改 rules 的 mode（单个 target — 自动解析）
skillshare extras rules --mode copy

# 明确指定 target（多 target 的 extra 必填）
skillshare extras rules --mode copy --target ~/.claude/rules

# 一次性对所有 target 启用 / 禁用 flatten
skillshare extras agents --flatten
skillshare extras agents --no-flatten

# 为已存在的 extra 添加一个新 target（然后 sync）
skillshare extras rules --add-target ~/.cursor/rules
skillshare extras commands --add-target ~/.config/opencode/commands --mode copy
skillshare extras personal --add-target ~/.claude --as CLAUDE.md --mode import

# 移除一个 target（保留已同步的文件）
skillshare extras rules --remove-target ~/.cursor/rules

# 移除一个 target 并删除其已同步的文件
skillshare extras rules --remove-target ~/.cursor/rules --prune
```

也可以通过 TUI（`e` 键）和 Web UI（每个 target 上的 mode 下拉菜单和 flatten 复选框）进行操作。

### `extras remove`

从配置中移除一个 extra。

```bash
skillshare extras remove <name> [--force] [-p|-g]
```

Source 文件会保留。目录 extra 会保留已同步的 target。[单文件 extra](#single-file-extras) 会先还原 target，再移除配置条目；还原失败时会保留配置，便于重试。

### `extras collect`

将 target 上的本地文件收集回 extras source 目录。文件会被复制到 source，并替换为 symlink。copy mode 的 target 则会保留普通副本形式的文件。[单文件 extras](#single-file-extras) 不支持 collect。

source 中已存在的文件会被跳过。使用 `--force` 可用 target 版本覆盖它们——例如把直接在 copy mode target 中做的修改拉回来。内容已与 source 一致的文件仍会被跳过。

```bash
skillshare extras collect <name> [--from <path>] [--force] [--dry-run] [-p|-g]
```

**Options:**

| Flag | Description |
|------|-------------|
| `--from <path>` | 要从中收集的 target 目录（有多个 target 时必填） |
| `--force`, `-f` | 覆盖 source 中已存在的文件 |
| `--dry-run` | 显示将会收集的内容而不做任何更改 |

**Example:**

```bash
# 从 Claude 收集 rules 回 source
skillshare extras collect rules --from ~/.claude/rules

# 预览将会收集的内容
skillshare extras collect rules --from ~/.claude/rules --dry-run

# 把 target 的修改拉回来，覆盖已有的 source 文件
skillshare extras collect rules --force
```

---

## Sync Mode

| Mode | Behavior |
|------|----------|
| `merge`（默认） | 从 target 到 source 的按文件 symlink |
| `copy` | 按文件复制 |
| `symlink` | 整个目录的 symlink |
| `import` | 仅限[单文件 extras](#single-file-extras)：在 target 文件中加入一行 `@<source file>` |

在未开启 Developer Mode 的 Windows 上，`merge` 会复制每个文件而不是链接它，`sync` 会打印 `file links need Windows Developer Mode; copying instead`。之后 `extras list` 和 `status` 会把该 target 显示为 `copy`。这些副本会被追踪，因此之后的 sync 会更新和清理它们、保留你自己的文件，并在文件链接可用后把它们替换为链接。参见 [Windows 疑难解答](/docs/troubleshooting/windows#file-links-need-windows-developer-mode-copying-instead)。

内容相同的本地文件会显示为 `local preserved`；`sync extras` 不会为它们建议使用 `--force`。它们仍是本地文件，不是受管理的链接。

切换 mode 时（例如从 `merge` 切换到 `copy`），下一次 `sync` 会自动将已有的 symlink 替换为新 mode 的格式。无需 `--force` —— symlink 始终可以安全替换。本地创建的普通文件需要 `--force` 才能被覆盖。

---

## Flatten

某些 AI 工具（例如 Claude Code 的 `/agents`）只在其配置目录的**顶层**发现文件 —— 它们不会递归进入子目录。如果你的 extras source 使用子目录来组织内容，同步后的文件对该工具而言将不可见。

`flatten` 选项通过将所有文件直接同步到 target 根目录来解决这个问题，无论它们在 source 中位于哪一层子目录：

```yaml
extras:
  - name: agents
    targets:
      - path: ~/.claude/agents
        flatten: true
```

**Behavior:**
- `flatten: true`：`source/curriculum/tactician.md` → `target/tactician.md`
- `flatten: false`（默认）：`source/curriculum/tactician.md` → `target/curriculum/tactician.md`

**文件名冲突：** 当两个位于不同子目录的文件同名时（例如 `team-a/agent.md` 和 `team-b/agent.md`），第一个文件生效（按路径字母顺序排序）。后续的冲突会被跳过并给出警告。

**Constraints:**
- 仅适用于 `merge` 和 `copy` mode —— 不能与 `symlink` mode 一起使用
- `collect` 会将新收集的文件放在 source 根目录（新文件不做子目录映射）

---

## Extension 转换 {#extension-transforms}

有些工具不读取 markdown。Gemini CLI 需要 TOML commands；Codex CLI 需要 TOML agents。target 上的 `extension` 字段会在 sync 期间运行一个外部脚本，将每个 source 文件转换为该 target 的原生格式。

```yaml
extras:
  - name: commands
    targets:
      - path: .claude/commands        # no extension — synced as-is
      - path: .gemini/commands
        extension: gemini-commands           # transform during sync
```

**Resolution** —— 裸名称会解析到 extensions 目录下（global 为 `~/.config/skillshare/extensions/<name>`，project 为 `.skillshare/extensions/<name>`）；路径（`./x.sh`、`/abs/x`）会被直接使用。

**Copy semantics** —— `extension` 隐含 `copy` mode。在带有 `extension` 的 target 上设置 `mode: merge` 或 `mode: symlink` 会报错。

**One-way** —— transform 只从 source 运行到 target。`extras collect` 会跳过带 extension 的 target。

**Overwrite safety** —— 生成的输出遵循与 `copy` mode 相同的冲突规则。输出路径上遗留的 symlink 会被自动替换；你在本地创建的现有普通文件或目录会被保留并跳过，除非传入 `--force`（使用 `--force` 时，冲突的目录会被生成的文件整体替换）。

### Extension 目录结构

单个可执行文件，或带 manifest 的目录：

```
.skillshare/extensions/gemini-commands/
├── extension.yaml
├── convert.js        # mapping rules you edit
└── md-toml.js        # helper for markdown/frontmatter/TOML
```

`extension.yaml`：

```yaml
run: ["node", "convert.js"]      # explicit command (argv), execed directly
output_ext: toml                  # .md → .toml; omit to keep the source extension
description: "Markdown command → Gemini CLI TOML"
```

没有 manifest 的裸单文件可执行程序会被直接执行（在 Unix 上依赖 shebang），并保留 source 的扩展名。需要改变扩展名的 transform 必须使用目录形式。

### 执行约定

- Source 文件内容通过 **stdin** 传入，脚本将转换后的内容写到 **stdout**。
- 环境变量：`SS_SRC_PATH`、`SS_REL_PATH`（相对于 source 根目录的路径 —— 对 Gemini 的 `/namespace:command` 命名很有用）、`SS_TARGET_DIR`、`SS_MODE`。
- 非零退出码会将该文件标记为失败；其他文件继续处理。

### 跨平台

该机制本身是跨平台的；某个 extension 能否运行取决于它的解释器。由于 `run` 是一个显式命令，用 `node` 或 `python3` 编写的 extension 可以在 Windows、macOS 和 Linux 上运行。纯 `bash` 脚本只能在有 shell 的环境中运行（Unix，或带 Git Bash 的 Windows）。Node 是参考 extensions 首选的解释器，因为它在各平台上都能统一提供。

### 参考 Extension

skillshare 仓库在 `extensions/` 下提供了示例 extensions（`gemini-commands`、`codex-agents`、`opencode-agents`）。将其中一个复制到你的 extensions 目录并按需调整 —— 它们是参考实现，不会自动安装。每个参考 extension 都让 `convert.js` 保持简短，你只需编辑字段映射；`md-toml.js` 负责读取 markdown、解析简单的 frontmatter 并写出 TOML。

### Recipe: Codex agents

Codex CLI 需要 TOML agents 而非 markdown。由于 `source` 可以指向任意目录，你可以把你的 agents source 复用为 extras source，并用 `codex-agents` 进行转换：

```yaml
extras:
  - name: codex-agents
    source: ~/.config/skillshare/agents   # reuse the agents source
    targets:
      - path: ~/.codex/agents
        extension: codex-agents
```

`skillshare sync extras` 会将每个 `<agent>.md` 转换为 `~/.codex/agents/<agent>.toml`，映射 frontmatter 中的 `name`、`description` 和 `model`，并将 markdown 正文折叠进 `developer_instructions`（其他 frontmatter 字段会被丢弃）。[Codex custom agent schema](https://developers.openai.com/codex/subagents#custom-agent-file-schema) 要求 `name`、`description` 和 `developer_instructions`，因此当解析出的 name、description 或 Markdown 正文为空时，参考 transform 会报出明确的错误。不需要单独再复制一份 agents。

agent target 也可以直接设置 `extension`，不需要 extra。参见[使用 extension 转换 agent](/docs/understand/agents#extensions)。

---

## Recipe：在多个 Agent 之间共享指令

:::tip Dashboard
网页控制台可以帮你完成这些设置，并提供预览、备份和还原按钮：
参见[让多个工具共用一份 AGENTS.md](../../how-to/daily-tasks/sharing-instructions.md)。
它使用的是[单文件 extras](#single-file-extras)，而不是目录。
:::

现在大多数 coding agent 都会读取一个用于常驻指令的 `AGENTS.md`，但每个 agent 都把自己的用户级副本保存在不同的目录中。一个带有多个 target 的 extra，就能把同一个 source 文件分发到所有这些目录：

```bash
skillshare extras init instructions \
  --target ~/.codex \
  --target ~/.config/opencode \
  --target ~/.claude \
  --target ~/.gemini \
  --no-tui
```

把你的 `AGENTS.md` 放到解析出的 source 目录中
（默认是 `~/.config/skillshare/extras/instructions/`），然后运行
`skillshare sync extras`。

| Agent | Global path | Reads `AGENTS.md` |
|-------|-------------|-------------------|
| Codex CLI | `~/.codex/AGENTS.md` | 直接读取 |
| opencode | `~/.config/opencode/AGENTS.md` | 直接读取 |
| Claude Code | `~/.claude/AGENTS.md` | 通过 `CLAUDE.md` import |
| Antigravity | `~/.gemini/AGENTS.md` | 通过 `GEMINI.md` import |

有两个 agent 在用户级别读取各自固定的文件名，因此每个都需要在被同步的文件旁边放一个一行文件。这些文件只需写一次；skillshare 之后不会再碰它们：

```markdown title="~/.claude/CLAUDE.md"
@AGENTS.md
```

```markdown title="~/.gemini/GEMINI.md"
@AGENTS.md
```

Claude Code 读取的是 `CLAUDE.md` 而不是 `AGENTS.md`，而 import 正是它的[记忆文档](https://code.claude.com/docs/en/memory)推荐的与其他 agent 共享同一个文件的方式。Antigravity 将其全局 rules 保存在
`~/.gemini/GEMINI.md` 中，并相对于该 rules 文件自身所在目录解析相对的 `@filename`，因此同样的一行代码就能读取到同步过来的 `AGENTS.md`。`~/.gemini` 这个 target 同时也覆盖了同样读取该全局文件的 Antigravity CLI。

请将 source 文件保持命名为 `AGENTS.md`。一个中立的名字如 `memory.md` 同样可以正常同步，但会不再被读取：Codex 是按文件名拼接 `AGENTS.md` 文件的，且没有 import 语法，因此只有这个文件名才能被它识别。

由于 target 都是目录，每个 target 都会以 source 中的原始文件名接收每个文件。请让 source 目录只保留你想在所有地方都出现的文件 —— 多余的文件会同时出现在全部四个 target 中。

:::note
此 recipe 分享的是你自己写的指令，而不是 agent 自身写入的记忆。各 agent 会以私有格式存储自己的学习内容 —— Claude Code 是一个 Markdown 目录，Codex 是一个数据库，Cursor 是非文件存储 —— 这些内容无法通过在 target 之间复制文件来迁移。
:::

---

## 单文件 extras {#single-file-extras}

设置了 `file` 的 extra 只同步其 source 目录中的一个文件，而不是整个目录。每个
target 会得到 `<path>/<as>`，其中 `as` 默认为 `file` 的名称。任何从固定路径读取单个文件的工具都可以使用它。例如，Pi
会把 `~/.pi/agent/APPEND_SYSTEM.md` 附加到它的 system prompt；你可以把这段文字以
`system.md` 保存在 dotfiles 中，再链接过去：

```yaml
extras:
  - name: pi-prompt
    source: ~/dotfiles/prompts     # project mode：相对于项目根目录
    file: system.md                # ~/dotfiles/prompts/system.md
    targets:
      - path: ~/.pi/agent
        as: APPEND_SYSTEM.md       # ~/.pi/agent/APPEND_SYSTEM.md 变成链接
```

用 CLI 创建同一个 extra：

```bash
skillshare extras init pi-prompt --file system.md --as APPEND_SYSTEM.md \
  --source ~/dotfiles/prompts --target ~/.pi/agent
skillshare sync extras
```

`extras init` 的 `--as` 会作用于每个 target。要在某一个 target 使用不同的文件名，
请单独添加该 target：

```bash
skillshare extras pi-prompt --add-target ~/Documents/prompts --as pi-system.md
```

控制台中的[共享 AGENTS.md](../../how-to/daily-tasks/sharing-instructions.md)
也是单文件 extras，并且可以混用普通链接、重命名和 import：

```yaml
extras:
  - name: personal
    file: AGENTS.md                # ~/.config/skillshare/extras/personal/AGENTS.md
    targets:
      - path: ~/.codex             # ~/.codex/AGENTS.md 变成链接
      - path: ~/.gemini
        as: GEMINI.md              # ~/.gemini/GEMINI.md 变成链接
      - path: ~/.claude
        as: CLAUDE.md
        mode: import               # ~/.claude/CLAUDE.md 保留原内容并导入该文件
```

| Mode | Target 文件 |
|------|-------------|
| `merge`（默认）或 `symlink` | 指向 source 文件的 symlink（在未开启 Developer Mode 的 Windows 上为副本） |
| `copy` | source 文件的副本 |
| `import` | 你自己的文件，顶部的受管区块中有一行 `@<source file>` |

`import` 把 `@` 行放在 `<!-- skillshare:instructions:begin -->` 与
`<!-- skillshare:instructions:end -->` 之间，从不改动文件的其余部分。只对会展开
`@` import 的工具使用它，例如 Claude Code。

规则：

- 使用链接或 `copy` 的 target 文件只能属于一份共享文件，不能同时 import 另一份。
- `file` 和 `as` 必须是单纯的文件名，不能包含 `/` 或 `\`。
- `as` 和 `import` 需要搭配 `file`。单文件 extra 不能使用 `flatten` 和 `extension`。
- 当 target 已经有一个不同的普通文件或 symlink 时，sync 会先保存它再替换，无需
  `--force`。挡在路径上的目录会被跳过。
- 链接后又变成 `modified` 的 target 也会被替换；编辑过的文件会作为 drift 备份保留，
  而不是作为还原点。
- 当链接被替换为内容不同的普通文件，或受管理的副本被修改时，`extras list` 会显示 `modified`。
- 把 target 从 `merge`、`symlink` 或 `copy` 切换为 `import` 时，会放回上次在 `import` mode
  的自有内容（包括空内容）；若未用过则使用连接前的内容，再加上 import 区块。编辑过的副本会先作为 drift 备份保留。
- `extras remove` 和 `--remove-target --prune` 会还原每个 target 文件：移除链接、副本或
  import 行，并放回第一次 sync 之前的文件或 symlink（原本没有文件则不留文件）。
  `modified` 的 target 会先作为 drift 备份保留。不带 `--prune` 的 `--remove-target` 会保留单文件 target，停止管理并忘记还原点。之后的 sync 不会清理它；重新连接才会记录新的还原点。
- 不支持 `extras collect`。要保留在 target 中做的修改，请把它复制回 source 文件。
  对于共享的 `AGENTS.md`，控制台 **AGENTS.md** 标签页中的 **收进** 会替你完成这一步。

在控制台中，`file` 为 `AGENTS.md` 的单文件 extras 会显示在 **AGENTS.md** 标签页；其他单文件
extras 则显示在 **Folders & files**。在那里，**添加 Extra** 提供 **Folder** 或 **Single file** 两种选择，
每个 target 都有 **文件名**，单个文件可以使用 `merge`、`copy` 或 `import`。单个文件的 **名称** 会跟随
文件名去掉扩展名自动填入（`APPEND_SYSTEM.md` 会填成 `APPEND_SYSTEM`），直到你自己输入名称为止。控制台不会编辑
文件内容；请直接编辑 source 文件。

### 一个文件夹，多个文件

多个单文件 extras 可以共用同一个 `source` 目录。每个文件创建
一个 extra；文件夹中没有被任何 extra 指定的文件不会被同步：

```yaml
extras:
  - name: pi-system
    source: ~/dotfiles/pi
    file: system.md
    targets:
      - path: ~/.pi/agent
        as: APPEND_SYSTEM.md
  - name: pi-agents
    source: ~/dotfiles/pi
    file: agents.md
    targets:
      - path: ~/.pi/agent
        as: AGENTS.md
```

```bash
skillshare extras init pi-system --source ~/dotfiles/pi --file system.md \
  --as APPEND_SYSTEM.md --target ~/.pi/agent
skillshare extras init pi-agents --source ~/dotfiles/pi --file agents.md \
  --as AGENTS.md --target ~/.pi/agent
```

在 project mode 中，`source` 是相对于项目根目录的路径，且必须位于项目内；绝对路径会被拒绝：

```bash
skillshare extras init review -p --source .skillshare/extras/prompts \
  --file review.md --target .claude/commands
skillshare extras init plan -p --source .skillshare/extras/prompts \
  --file plan.md --target .claude/commands
```

在 Dashboard 中，共享 extras 文件夹里的单文件有一个 **Source folder** 字段。默认是 extra 的名称；
填入另一个 extra 的文件夹，即可把两个文件放在同一个文件夹中。

备份保存在 skillshare 的 state 目录中（macOS 和 Linux 上为
`~/.local/state/skillshare/extras/backups/`），每个文件保留最近 10 份。drift 备份位于该处的
`extras/backups/<id>/drift/`，其中 `<id>` 由 target 文件的路径推导而来；还原时从不使用它们。要列出或还原任何已保存的
版本，请使用 [`backup files`](./backup.md#file-history)。

---

## 目录结构

```
~/.config/skillshare/
├── config.yaml          # extras config lives here
├── skills/              # skill source
└── extras/              # extras source root
    ├── rules/           # extras/rules/ source files
    │   ├── coding.md
    │   └── testing.md
    └── prompts/
        └── review.md
```

---

## 配置

在 `config.yaml` 中：

```yaml
# Optional: set a global default extras source directory
extras_source: ~/my-extras

extras:
  - name: rules
    source: ~/company-shared/rules    # optional per-extra override
    targets:
      - path: ~/.claude/rules
      - path: ~/.cursor/rules
        mode: copy
  - name: agents
    targets:
      - path: ~/.claude/agents
        flatten: true                  # sync subdirectory files flat
  - name: prompts
    targets:
      - path: ~/.claude/prompts
```

### Source 解析优先级

每个 extra 的 source 目录按三级优先顺序解析：

1. **每个 extra 各自的 `source`**（最高优先级）—— 精确路径，按原样使用
2. **`extras_source`** —— `<extras_source>/<name>/`
3. **默认值** —— `~/.config/skillshare/extras/<name>/`（global）或 `.skillshare/extras/<name>/`（project）

`extras list --json` 的输出包含一个 `source_type` 字段（`per-extra`、`extras_source` 或 `default`），指明路径是由哪一级解析出来的。

:::tip Auto-populated
当你运行 `skillshare init` 或用 `extras init` 创建你的第一个 extra 时，`extras_source` 会自动设置为默认路径（`~/.config/skillshare/extras/`）。之后如需更改，使用 `skillshare extras source <path>`。
:::

---

## 同步

Extras 通过以下方式同步：

```bash
skillshare sync extras        # sync extras only
skillshare sync --all         # sync skills + extras together
```

完整的同步文档（包括 `--json`、`--dry-run` 和 `--force` 选项）参见 [sync extras](/docs/reference/commands/sync#sync-extras)。

---

## 工作流程

```bash
# 1. 创建一个新的 extra
skillshare extras init rules --target ~/.claude/rules --target ~/.cursor/rules

# 1b. 或者使用自定义 source 目录
skillshare extras init rules --target ~/.claude/rules --source ~/my-rules

# 1c. 重新配置一个已存在的 extra（覆盖）
skillshare extras init rules --target ~/.cursor/rules --force

# 2. 向 source 目录添加文件
# (编辑解析出的 source 目录 — 可通过以下命令查看: skillshare extras list --json)

# 3. 同步到 target
skillshare sync extras

# 4. 列出状态（source_type 显示每个 extra 的 source 是从哪一级解析出来的）
skillshare extras list

# 5. 将在 target 中编辑过的文件收集回 source
skillshare extras collect rules --from ~/.claude/rules

# 6. 更改全局 extras source 目录
skillshare extras source ~/company-shared/extras
```

---

## 另请参阅

- [sync](/docs/reference/commands/sync#sync-extras) — 将 extras 同步到 target
- [status](/docs/reference/commands/status) — 显示 extras 的文件和 target 数量
- [Configuration](/docs/reference/targets/configuration#extras) — Extras 配置参考
