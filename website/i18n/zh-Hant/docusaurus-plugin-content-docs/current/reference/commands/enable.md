---
sidebar_position: 5
---

# enable / disable

暫時啟用或停用 skill，而不需要移除它們。

```bash
skillshare disable draft-*          # 依 pattern 停用
skillshare enable draft-*           # 重新啟用
skillshare disable "frontend/**"    # 停用某個資料夾中的所有 skill
skillshare disable my-skill -p      # Project mode
```

## When to Use

- 暫時讓某個 skill 不參與 sync，而不需要解除安裝
- 在所有 target 上靜音某個草稿或實驗性的 skill
- 在 list TUI 中用 `t` 鍵切換 skill 的啟用/停用狀態

## How It Works

`disable` 會把 pattern 加進 `.skillignore`；`enable` 則會移除它。被停用的 skill 仍留在 source 目錄中，但會被排除在 `sync` 與 `collect` 之外。

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
啟用或停用之後，請執行 `skillshare sync` 才能把變更套用到 target。
:::

## Options

| Flag | Description |
|------|-------------|
| `<name\|pattern>` | 一個或多個 skill 名稱或 glob pattern（例如 `draft-*`、`frontend/**`） |
| `--project, -p` | 使用 project 的 `.skillignore`（`.skillshare/skills/.skillignore`） |
| `--global, -g` | 使用 global 的 `.skillignore`（`~/.config/skillshare/.skillignore`） |
| `--dry-run, -n` | 預覽而不寫入 |
| `--help, -h` | 顯示說明 |

當未指定 `-p` 或 `-g` 時，模式會自動偵測（與其他指令相同）。

## Examples

```bash
# 停用單一 skill
$ skillshare disable my-draft
✓ my-draft  added to .skillignore

Next
  skillshare sync  apply the change

# 依 glob pattern 停用
$ skillshare disable "experimental-*"
✓ experimental-*  added to .skillignore

Next
  skillshare sync  apply the change

# 重新啟用
$ skillshare enable my-draft
✓ my-draft  removed from .skillignore

Next
  skillshare sync  apply the change

# 預覽而不寫入
$ skillshare disable my-skill --dry-run
  my-skill  would be added to ~/.config/skillshare/skills/.skillignore

Dry run — nothing was written

# 已經被停用
$ skillshare disable my-draft
! my-draft  already disabled
```

## Disable a Whole Folder

`disable`/`enable` 接受與 `.skillignore` 相同的 glob 語法，因此沒有另外的「group」旗標——只要把 pattern 指向該資料夾，裡面的所有 skill 就會一起被切換。

```bash
# 停用 frontend/ 底下所有深度的 skill
$ skillshare disable "frontend/**"
✓ frontend/**  added to .skillignore

Next
  skillshare sync  apply the change

# 重新啟用整個資料夾
$ skillshare enable "frontend/**"
✓ frontend/**  removed from .skillignore

Next
  skillshare sync  apply the change
```

:::tip Quote the pattern
請務必用引號包住資料夾 pattern（`"frontend/**"`），這樣你的 shell 才不會在 skillshare 看到之前就展開 `*`。
:::

`frontend/**` 只會在 `.skillignore` 中寫入一行，並持續涵蓋你之後加進該資料夾的任何內容。用**相同**的 pattern 執行 `enable` 會移除那一行。若要改成停用個別的 skill，請依名稱逐一列出（`skillshare disable a b c`）。完整的 glob 參考（`*`、`**`、`?`、`[abc]`、`!negation`、錨定的 `/`、僅限目錄的 `pattern/`）請參閱 [.skillignore pattern syntax](/docs/reference/filtering#skillignore)。

## TUI Toggle

你也可以在 `skillshare list` TUI 中開關 skills。變更會立即寫入 `.skillignore`。

## Where is the .skillignore?

| 模式 | 路徑 |
|------|------|
| Global | `~/.config/skillshare/skills/.skillignore` |
| Project | `.skillshare/skills/.skillignore` |

此檔案會在第一次執行 `disable` 時自動建立。

## Agent Support

使用 `--kind agent` 可以啟用或停用 agent。這會寫入 `.agentignore`，而不是 `.skillignore`：

```bash
skillshare disable --kind agent draft-reviewer     # 停用一個 agent
skillshare enable --kind agent draft-reviewer      # 重新啟用一個 agent
skillshare disable --kind agent "experimental-*"   # 依 pattern 停用
```

| 模式 | `.agentignore` 路徑 |
|------|---------------------|
| Global | `~/.config/skillshare/agents/.agentignore` |
| Project | `.skillshare/agents/.agentignore` |

關於 agent 管理的背景說明，請參閱 [Agents](/docs/understand/agents)。

## See Also

- [list](./list.md) — 查看被停用的 skill，並用 `t` 鍵切換
- [Filtering Skills](/docs/how-to/daily-tasks/filtering-skills) — 所有的篩選層級
- [.skillignore](/docs/reference/filtering#skillignore) — Pattern 語法
- [sync](./sync.md) — 在 enable/disable 之後套用變更
- [Agents](/docs/understand/agents) — Agent 概念
