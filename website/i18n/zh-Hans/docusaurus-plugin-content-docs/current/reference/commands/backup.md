---
sidebar_position: 2
---

# backup

创建、列出并管理 target 目录的 Backup。

```bash
skillshare backup              # 备份所有 skill target
skillshare backup claude       # 备份指定 target
skillshare backup agents       # 备份所有 agent target
skillshare backup --all        # 备份 skills + agents
skillshare backup --list       # 列出所有 Backup
skillshare backup --cleanup    # 移除旧的 Backup
skillshare backup --delete 2026-01-19_10-00-00  # 删除一个 Backup
skillshare backup files        # skillshare 改写过的单个文件的各个版本
```

## 何时使用

- 在有风险的变更之前创建手动 Backup
- 列出现有 Backup 以检查恢复选项
- 清理旧的 Backup，或删除某个不再需要的 Backup
- 找回 `AGENTS.md` 或 `CLAUDE.md` 等文件的早期版本

## 自动 Backup

Backup 会在以下操作之前**自动**创建：
- `skillshare sync`（skill target 和 agent target）
- `skillshare sync agents`（仅 agent target）
- `skillshare target remove`

位置：`~/.local/share/skillshare/backups/<timestamp>/`（global）、`.skillshare/backups/`（project mode，仅 agents）

每次自动 Backup 之后都会自动应用 Retention 策略，使用与 `--cleanup` 相同的规则。你不需要手动清理快照。

## Commands

### 创建 Backup

```bash
skillshare backup              # 所有 target
skillshare backup claude       # 指定 target
skillshare backup --dry-run    # 预览
```

### 列出 Backup

```bash
skillshare backup --list
```

```
All backups (15.3 MB total)
  2026-01-20_15-30-00  claude, cursor     4.2 MB  ~/.local/share/.../2026-01-20_15-30-00
  2026-01-19_10-00-00  claude             2.1 MB  ~/.local/share/.../2026-01-19_10-00-00
  2026-01-18_09-00-00  claude, cursor     4.0 MB  ~/.local/share/.../2026-01-18_09-00-00
```

### 清理旧的 Backup

```bash
skillshare backup --cleanup           # 移除旧的 Backup
skillshare backup --cleanup --dry-run # 预览清理效果
```

默认清理策略：
- 保留最近 10 个 Backup
- 移除超过 30 天的 Backup
- 总大小上限为 500 MB

份数和大小上限可以在全局 config 中调整，也可以在 dashboard **目标文件夹** 标签页的保留摘要中修改。`0` 表示不限。30 天的上限是固定的，project 的快照沿用默认值。

```yaml
backup:
  max_count: 20      # 默认 10
  max_size_mb: 1000  # 默认 500
```

即使最新的快照单独就超过了大小上限，它也始终会被保留——你永远不会失去恢复点。

这个策略在每次 `sync` 之后都会自动运行，因此 `--cleanup` 只在你想按需清理时才需要用到。

### 删除 Backup

```bash
skillshare backup --delete 2026-01-19_10-00-00            # 删除一个快照
skillshare backup --delete 2026-01-19_10-00-00 --dry-run  # 显示将被删除的内容
skillshare backup --delete 2026-01-19_10-00-00 -p         # 从 project 的 .skillshare/backups/ 中删除
```

timestamp 就是 `--list` 显示的文件夹名称。整个快照都会被删除，包括其中的每个 target。

### 文件历史 {#file-history}

skillshare 在改写或替换单个文件之前——例如 `AGENTS.md`、`CLAUDE.md` 这类指令文件，或[共享文件](/docs/how-to/daily-tasks/sharing-instructions#backups)的某个位置——会先保存旧内容。`backup files` 用于列出并还原这些版本。

```bash
skillshare backup files                                   # 有已保存版本的文件
skillshare backup files show ~/.claude/CLAUDE.md          # 某个文件的各个版本，最新的在前
skillshare backup files restore ~/.claude/CLAUDE.md origin
skillshare backup files restore ./CLAUDE.md 1769000000000000000.shim --dry-run
```

```
Versions of /Users/me/.claude/CLAUDE.md
  1769000000000000000.edit          2026-01-21 12:53:20  history/edit          2.1 KB  # Team rules
  drift:1768900000000000000.mode    2026-01-20 09:06:40  drift/mode            1.9 KB  # Team rules
  origin                            2026-01-10 08:00:00  origin                1.2 KB  # My notes
```

每个版本都有一个 ID：

| ID | 类型 | 含义 |
|----|------|---------|
| `<time>[.<reason>]` | `history` | skillshare 写入该文件之前保存 |
| `drift:<time>[.<reason>]` | `drift` | 你自己的修改，被 skillshare 替换掉了 |
| `origin` | `origin` | 共享文件第一次接上时该文件原本的内容；移除该位置时会自动还原它。如果当时没有文件，还原它会删除当前文件 |

reason 说明 skillshare 当时要做什么：

| 类型 | Reason | 保存时机 |
|------|--------|--------------|
| `history` | `convert` | 把文件转换为 `AGENTS.md`，或重命名为 `AGENTS.md` 之前 |
| `history` | `shim` | 在项目文件中加入 `@AGENTS.md` 之前 |
| `history` | `edit` | 在 dashboard 中编辑之前 |
| `history` | `collect` | 把某个 target 的修改收进共享文件之前 |
| `history` | `attach` | 共享文件第一次接上并替换它之前 |
| `history` | `restore` | 还原某个较早版本之前 |
| `history` | `migrate` | 同步保存不含 0.23.0 已停用 MCP 设置的配置之前。参见[从 0.22 升级 Pi](/docs/reference/commands/mcp#pi-migration) |
| `drift` | `overwrite` | 你直接编辑了该文件，然后选择了 **覆盖** |
| `drift` | `mode` | 切换该位置的模式之前 |
| `drift` | `restore` | 还原该位置之前 |

旧版本保存的版本没有 reason。每个文件的每种类型保留最近 10 个版本。

`restore` 会先把当前内容保存为一个 reason 为 `restore` 的新版本，然后写入所选版本。如果路径是 symlink，除非加上 `--unlink`（把链接替换为常规文件），否则会拒绝执行。

`backup files` 跟随当前模式：在项目内（或使用 `-p`）时只列出该项目中的文件，`show` / `restore` 会拒绝项目之外的路径；`-g` 涵盖所有文件。由于 `files` 是一个子命令，要备份名称恰好为 `files` 的 target，请使用 `skillshare backup -t files`。

## Dashboard {#dashboard}

[`skillshare ui`](/docs/reference/commands/ui) 中的 **设置 › 备份** 有三个标签页：

- **目标文件夹**——上文的快照，按日期分组。可以按 target 或 **只看 agents** 过滤。展开某个快照可以看到每个文件夹的文件数和大小，并可 **恢复** 其中任意一个（skill 和 agent 条目都可以）、**复制路径** 或 **删除这份备份**。**立即备份** 和 **清理旧备份** 分别对应 `backup` 和 `--cleanup`。旁边的保留摘要可以打开面板调整份数和大小上限，**全部删除** 会在你确认后删除所有快照，文件、MCP 和 Hooks 的备份不受影响。
- **文件**——上文的文件历史。选择一个文件即可查看它的各个版本及其 reason，然后 **预览并还原** 会显示与当前文件的差异，或完整的版本内容。链接形式的位置只有在你确认 **还原并断开链接** 之后，才会被替换为常规文件。
- **MCP**——每次写入 MCP 配置之前做的备份，按 Agent 配置分组，并列出每份备份新增、修改或移除的 server。**预览并还原** 会打开与 **MCP** 页面相同的还原对话框（命令行中则使用 [`mcp restore`](/docs/reference/commands/mcp)）。

![设置 › 备份 › 文件：恢复前预览较早的 CLAUDE.md 版本](/img/backup-files-preview.png)

在 project mode 下，此页面只涵盖该项目：`.skillshare/backups/` 中的 agent 快照、项目内的文件，以及其 MCP 配置的备份。已删除的 skills 和 agents 不在这里；它们会进入 **Skill** 和 **Agent** 页面的 **回收站** 标签页。

## Options

| Flag | 说明 |
|------|------|
| `--all` | 同时备份 skills 和 agents |
| `--project, -p` | 使用 project mode（`.skillshare/backups/`）；**仅限 agents** |
| `--global, -g` | 使用 global mode（skills 的默认值） |
| `--list, -l` | 列出所有 Backup；加 `-p` 时列出项目的 |
| `--cleanup, -c` | 移除旧的 Backup；加 `-p` 时处理项目的 |
| `--delete <timestamp>` | 删除一个 Backup；搭配 `-p` 时从 `.skillshare/backups/` 中删除 |
| `--target, -t <name>` | 针对指定 target 备份（作为位置参数的替代方式） |
| `--dry-run, -n` | 预览而不做任何变更 |

`backup files` 有自己的选项：`--project, -p`、`--global, -g`，以及 `restore` 专用的 `--unlink` 和 `--dry-run, -n`。参见[文件历史](#file-history)。

`backup` 还接受一个位置形式的 kind 参数：`skillshare backup agents` 会把备份范围限定为仅 agent target。

## Backup 结构

```
~/.local/share/skillshare/backups/
├── 2026-01-20_15-30-00/
│   ├── claude/
│   │   ├── skill-a/
│   │   └── skill-b/
│   └── cursor/
│       ├── skill-a/
│       └── skill-b/
└── 2026-01-19_10-00-00/
    └── claude/
        └── ...
```

存在的 skill 目录取决于该 target 的模式——参见 [What Gets Backed Up](#what-gets-backed-up)。

## What Gets Backed Up {#what-gets-backed-up}

一次 Backup 只保护 `sync` 可能会破坏的内容：**存在于 target、但不存在于你的 source 中的本地内容。**

- target 中的常规文件和目录会被备份
- merge 模式 target 中的逐个 skill symlink 会被**跳过**——它们指向你的 source（唯一的真实来源），本身已经是安全的。`skillshare sync` 会重新创建它们

这意味着：
- 在 merge 模式下：只有本地（非 symlink）的 skills 会被备份。被同步的 skills 存放在 source 中
- 在 copy 模式下：所有受管理的 skill 目录都会被备份（它们是真实文件）
- 在 symlink 模式下：不会备份任何内容（整个目录是一个 symlink）

如果一个 target 中只包含 symlink，则不会创建 Backup，`backup` 会报告没有需要做的事——一个空的恢复点没有意义。

## Backups & Disk Space {#backups--disk-space}

Backup 从不复制你的 source，因此它们一直很小。以下三种机制很容易混淆：

| 机制 | 范围 | 控制的内容 |
|-----------|-------|------------------|
| source 中的 `.gitignore` | 仅限 Git | Git 追踪的内容。被忽略的文件仍然存在于磁盘上 |
| `config.yaml` 中的 `ignore:` | `sync` | `sync` 复制到 target 的文件（主要是 copy 模式）。参见 [sync](/docs/reference/commands/sync) |
| Backup | 快照 | 仅限本地 target 内容——symlink 以及因此对应的 source 内容会被排除 |

因为 symlink 的 skills 不会被跟随，存放在 source skill 内部的重量级产物（模型权重、`.venv`、浏览器 profile、媒体文件）**永远不会**被复制进快照，无论 `.gitignore` 或 `ignore:` 是否提到它们。

Retention 会在每次 `sync` 之后自动运行，使用下方的默认策略。要手动查看用量：

```bash
du -sh ~/.local/share/skillshare/backups   # 磁盘上的总大小
skillshare backup --list                   # 每个快照的大小
skillshare backup --cleanup --dry-run      # 预览 Retention 会移除什么
```

Copy 模式的 target 是快照仍可能变大的唯一情形：它们是真实文件，因此某个 skill 目录下的任何内容都会被复制。请把运行时缓存和大型产物放在 skill 目录之外，或者用 `ignore:` 排除它们，避免它们一开始就到达 target。

## Agent Backup {#agent-backup}

Agents 有自己的 Backup 流程，与 skill Backup 并行运行，有两点值得注意的区别：

**条目命名。** Agent Backup 存储在每个 timestamp 目录内的 `<target>-agents/` 下，与 skill Backup 并列。例如，运行 `skillshare backup --all` 之后目录结构如下：

```
~/.local/share/skillshare/backups/2026-01-20_15-30-00/
├── claude/          # claude 的 skills 备份
├── claude-agents/   # claude 的 agents 备份
└── cursor/
```

**Project mode 与 skills 相反。** 在 project mode（`-p`）下，`backup` 会拒绝备份 skill target，但**会**备份 agent target。如果你忘了加 `agents` 过滤条件，会看到这样的错误：

```
backup is not supported in project mode (except for agents)
```

因此在 project mode 下，你必须使用 `skillshare backup -p agents` 或 `skillshare backup -p --all`。`--list -p` 和 `--cleanup -p` 不需要指定，会直接处理 `.skillshare/backups/`。

```bash
skillshare backup agents                  # 所有 agent target（global）
skillshare backup agents claude           # 只备份 claude 的 agents
skillshare backup agents -p               # project 的 agent target
skillshare backup --all                   # 一次性备份 skills + agents
```

参见 [Agents](/docs/understand/agents) 了解 agent 资源模型，以及 [restore](/docs/reference/commands/restore) 了解恢复方式。

## 另请参阅

- [restore](/docs/reference/commands/restore) —— 从 Backup 恢复
- [sync](/docs/reference/commands/sync) —— 自动创建 Backup
- [target remove](/docs/reference/commands/target) —— 自动创建 Backup
