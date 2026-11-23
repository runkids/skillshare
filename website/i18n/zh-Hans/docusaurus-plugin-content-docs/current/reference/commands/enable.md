---
sidebar_position: 5
---

# enable / disable

临时启用或禁用 skills，而不将其删除。

```bash
skillshare disable draft-*          # 按模式禁用
skillshare enable draft-*           # 重新启用
skillshare disable "frontend/**"    # 禁用某个文件夹中的所有 skill
skillshare disable my-skill -p      # Project mode
```

## 何时使用

- 临时将某个 skill 从同步中隐藏，而不卸载它
- 在所有 targets 中静音某个草稿或实验性的 skill
- 从 list TUI 中用 `t` 键切换 skills 的启用/禁用状态

## 工作原理

`disable` 会向 `.skillignore` 添加一个模式；`enable` 则将其移除。被禁用的 skills 仍保留在 source 目录中，但会被排除在 `sync` 和 `collect` 之外。

skill 名称指向所有位于同名文件夹中的 skill，因此 `enable inner` 会同时作用于 `group/inner` 和 `other/inner`。像 `group/inner` 这样的确切路径，或像 `group__inner` 这样的 flat name，只指向该 skill。对于名称指向的每个 skill，`enable` 也会移除其路径那一行（list TUI 和 dashboard 写入的行），因此 `enable inner` 也会移除 `group/inner`；每个 `✓` 行显示实际写入或移除的那一行。如果输入的名称作为一行能匹配该 skill，`disable` 会按输入写入；flat name 无法被任何一行匹配，因此会改为写入该 skill 的路径。

```mermaid
flowchart LR
    DIS["skillshare disable my-skill"]
    IGN[".skillignore += my-skill"]
    SYNC["sync skips my-skill"]
    DIS --> IGN --> SYNC
```

```mermaid
flowchart LR
    EN["skillshare enable my-skill"]
    IGN[".skillignore -= my-skill"]
    SYNC["sync includes my-skill"]
    EN --> IGN --> SYNC
```

:::tip
启用或禁用之后，运行 `skillshare sync` 将更改应用到 targets。
:::

## 选项

| 标志 | 描述 |
|------|-------------|
| `<name\|pattern>` | 一个或多个 skill 名称或 glob 模式（例如 `draft-*`、`frontend/**`） |
| `--project, -p` | 使用 project 的 `.skillignore`（`.skillshare/skills/.skillignore`） |
| `--global, -g` | 使用 global 的 `.skillignore`（`~/.config/skillshare/.skillignore`） |
| `--dry-run, -n` | 预览而不写入 |
| `--help, -h` | 显示帮助 |

当既未指定 `-p` 也未指定 `-g` 时会自动检测模式（与其他命令相同）。

## 示例

```bash
# 禁用单个 skill
$ skillshare disable my-draft
✓ my-draft  added to .skillignore

Next
  skillshare sync  apply the change

# 按 glob 模式禁用
$ skillshare disable "experimental-*"
✓ experimental-*  added to .skillignore

Next
  skillshare sync  apply the change

# 重新启用
$ skillshare enable my-draft
✓ my-draft  removed from .skillignore

Next
  skillshare sync  apply the change

# 预览而不写入
$ skillshare disable my-skill --dry-run
  my-skill  would be added to ~/.config/skillshare/skills/.skillignore

Dry run — nothing was written

# 预览会列出每一行，包括 enable 会移除的路径行
$ skillshare enable inner --dry-run
  inner        would be removed from ~/.config/skillshare/skills/.skillignore
  group/inner  would be removed from ~/.config/skillshare/skills/.skillignore

Dry run — nothing was written

# 已经被禁用
$ skillshare disable my-draft
! my-draft  already disabled
```

## 由其他规则决定时

`disable` 和 `enable` 只写入或删除一行，但其他规则仍可能决定 skill 的状态：像 `draft-*` 这样的 glob、像 `frontend/` 这样的文件夹规则、`.skillignore.local` 中的 `!` 规则，或 tracked repo 自己的 `.skillignore`。写入后，skillshare 会按 `skillshare list` 显示的状态检查你指定的每个 skill 或 agent。如果仍处于原来的状态，会输出一行 `✗` 并指出要编辑的文件，然后以状态码 1 退出：

```bash
$ skillshare enable draft-one

✗ could not enable draft-one: still disabled: another rule in .skillignore or .skillignore.local matches it — remove that rule to enable it
```

若有多个被阻挡，每个都会有自己的 `✗` 行。已生效的更改会保留并以 `✓` 显示，操作日志会将这次运行记为 `partial`。

像 `"draft-*"` 这样的 glob 参数不对应单一 skill，因此会直接写入或删除，不做这项检查。

## 禁用整个文件夹

`disable`/`enable` 使用与 `.skillignore` 相同的 glob 语法，因此没有单独的“group”标志——将模式指向文件夹，其中的所有 skill 就会一并被切换。

```bash
# 禁用 frontend/ 下的所有 skill（任意深度）
$ skillshare disable "frontend/**"
✓ frontend/**  added to .skillignore

Next
  skillshare sync  apply the change

# 重新启用整个文件夹
$ skillshare enable "frontend/**"
✓ frontend/**  removed from .skillignore

Next
  skillshare sync  apply the change
```

:::tip 给模式加上引号
始终用引号包裹文件夹模式（`"frontend/**"`），这样你的 shell 就不会在 skillshare 看到之前展开 `*`。
:::

`frontend/**` 会向 `.skillignore` 写入一行，并持续覆盖你之后添加到该文件夹中的任何内容。使用**相同**的模式执行 `enable` 会移除那一行。如果想改为禁用单个 skill，请按名称列出它们（`skillshare disable a b c`）。完整的 glob 参考（`*`、`**`、`?`、`[abc]`、`!negation`、锚定的 `/`、仅目录的 `pattern/`）参见 [.skillignore pattern syntax](/docs/reference/filtering#skillignore)。

## TUI 切换

你也可以在 `skillshare list` TUI 中启用或禁用 skills。更改会立即写入 `.skillignore`。如果其他规则让 skill 保持原状态，该行不会改变，按键行会显示与 CLI 相同的 `✗` 信息。

## .skillignore 在哪里？

| 模式 | 路径 |
|------|------|
| Global | `~/.config/skillshare/skills/.skillignore` |
| Project | `.skillshare/skills/.skillignore` |

该文件会在首次执行 `disable` 时自动创建。

## Agent 支持

使用 `--kind agent` 来启用或禁用 agents。这会写入 `.agentignore` 而不是 `.skillignore`：

```bash
skillshare disable --kind agent draft-reviewer     # 禁用一个 agent
skillshare enable --kind agent draft-reviewer      # 重新启用
skillshare disable --kind agent "experimental-*"   # 按模式禁用
```

`.agentignore` 的每一行匹配的是文件名（包含 `.md`）。不带 `.md` 的名称如果对应到同名文件的 agent，写入时会自动补上 `.md`，因此 `draft-reviewer` 会写入 `draft-reviewer.md`。使用 agent 的 frontmatter `name` 时则写入其路径，因此对 `rev.md` 执行 `disable --kind agent critic` 会写入 `rev.md`。

| 模式 | `.agentignore` 路径 |
|------|---------------------|
| Global | `~/.config/skillshare/agents/.agentignore` |
| Project | `.skillshare/agents/.agentignore` |

关于 agent 管理的背景信息，参见 [Agents](/docs/understand/agents)。

## 另请参阅

- [list](./list.md) — 查看被禁用的 skills，并用 `t` 键切换
- [Filtering Skills](/docs/how-to/daily-tasks/filtering-skills) — 所有过滤层
- [.skillignore](/docs/reference/filtering#skillignore) — 模式语法
- [sync](./sync.md) — 在 enable/disable 之后应用更改
- [Agents](/docs/understand/agents) — Agent 概念
