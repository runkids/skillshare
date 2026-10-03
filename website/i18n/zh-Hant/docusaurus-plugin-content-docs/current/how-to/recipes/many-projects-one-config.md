---
sidebar_position: 8
---

# Recipe：多個 Projects，一份 Config

> 從 global config 把 skills 與 MCP servers 送進多個 project 資料夾，只需同步一次。

## 情境 {#scenario}

像 `~/.claude/skills` 這樣的 global targets 會被每個 project 讀取，所以每個 project 看到的都是同一組 skills。如果這就是你要的效果，你不需要這份 recipe。

這份 recipe 適用於 projects 之間應該拿到**不同**內容的情況：

- 你安裝了很多 skills，而某個 frontend project 只需要 `frontend-*`。一個 session 中的 skills 越少，花在描述上的 context 就越少，選錯的機率也越低。
- 某個 project 不該載入在其他地方都沒問題的 MCP server，例如客戶的 repo。
- 某個 project 應該保留 skills 的實體副本以便 commit，但不 commit 任何 Skillshare config。
- 你使用的某個工具只會讀取 project 內的某個資料夾。

[Project mode](/docs/how-to/recipes/skill-per-project-workflow) 同樣能讓每個 project 擁有自己的一組。它會在每個 project 中保留一份 `.skillshare/config.yaml`，並且你要在每個資料夾內各自執行 sync。global config 中的 `projects` 則能用你機器上的一份檔案達到相同效果：

| | Global targets | Global `projects` | Project mode |
|---|---|---|---|
| **誰拿到 skills** | 每個 project，同一組 | 你列出的資料夾，各自一組 | 那一個 project |
| **設定放在哪裡** | 你的機器 | 你的機器 | project 的 repo |
| **隊友拿得到嗎** | 否 | 否 | 是，clone 就有 |
| **加進 project 的檔案** | 無 | 只有同步的 skills 與 agents | `.skillshare/` 加上同步的檔案 |
| **Sync** | 在任何地方執行一次 `sync` | 在任何地方執行一次 `sync` | 在每個 project 內執行 `sync` |

當設定應該跟著 repo 走時，選擇 project mode。當這些是你自己的 projects、是無法加入 `.skillshare/` 的客戶或開源 repo、或你想用一次 `sync` 更新全部時，選擇 `projects`。

## 解決方案

### Skills 與 agents：`projects`

```yaml
# ~/.config/skillshare/config.yaml
projects:
  ~/work/project01:
    targets: [claude, codex]
    skills:
      mode: copy
      include:
        - myskill-*
    agents: {}
```

```bash
skillshare sync --dry-run   # 預覽
skillshare sync
```

- `targets` 指定你在這個 project 中使用的工具。Skillshare 會寫入每個工具的 project 路徑，這裡是 `.claude/skills` 與 `.agents/skills`，所以不需要輸入路徑。
- `skills` 與 `agents` 用來開啟該部分的同步。留空代表同步全部；用 `include` 與 `exclude` 縮小範圍。參見[篩選 skills](/docs/how-to/daily-tasks/filtering-skills)。
- `copy` 會寫入實際檔案，所以 project 可以 commit 它們。若 symlinks 就夠用，保留預設的 `merge` 即可。

在 dashboard 中，**Projects** 頁面做的是同一件事：**Add project**，選擇 targets，並選擇要同步的內容。每個欄位請參見 [`projects`](/docs/reference/targets/configuration#projects)。

### MCP servers：`mcp.projects`

MCP servers 會寫入各個 Agent 自己的設定檔，所以它們是依 project 資料夾列出，而不是依路徑：

```yaml
# ~/.config/skillshare/config.yaml
mcp:
  servers:
    context7:
      command: npx
      args: ["-y", "@upstash/context7-mcp"]
      targets: [opencode]
  projects:
    ~/work/project01:
      servers:
        context7:            # 其他地方都會載入，在這裡關閉
          disabled: true
          targets: [opencode]
```

```bash
skillshare sync mcp --dry-run   # 預覽每個檔案
skillshare sync mcp
```

欄位與限制請參閱 [`mcp`：管理多個 projects](/docs/reference/commands/mcp#manage-several-projects-from-the-global-config)。

## 驗證

- `skillshare sync` 會回報該 project 的 targets，例如 `project01@claude  1 copied`
- `~/work/project01/.claude/skills/` 只包含符合 `include` 的 skills
- `skillshare sync mcp --dry-run` 會為每個 project 檔案列出一行
- 再執行一次 `skillshare sync mcp` 會把每個項目回報為 `unchanged`

## 變化

- **工具路徑之外的資料夾**：一個 target 就只是一個名稱加一個路徑，所以對於任何工具的 project 路徑都涵蓋不到的資料夾，`skillshare target add project01 ~/work/project01/some/folder` 仍然可用。當這樣的 target 剛好指向某個工具的 project 路徑時，dashboard 的 **Projects** 頁面會提供轉換選項。
- **Commit 或忽略**：在 `copy` mode 下，Skillshare 也會在 target 資料夾中寫入 `.skillshare-manifest.json`，用來追蹤它複製了哪些內容。可以把它和 skills 一起 commit，或加入 `.gitignore`。
- **路徑重疊警告**：若 project 資料夾是另一個 target 已在使用的資料夾，`sync` 會印出路徑重疊警告。執行 `skillshare doctor` 可查看哪些 targets 共用該路徑。
- **多個 projects 使用同一個 server**：在其中一個 project 下用 YAML anchor 定義一次（`docs: &docs`），再於其他 project 中重複使用（`docs: *docs`）。請參閱 [`mcp` 參考](/docs/reference/commands/mcp#manage-several-projects-from-the-global-config)。
- **共用的 projects**：clone 該 project 的隊友不會拿到你的 global config。當設定必須跟著 repo 走時，請使用 [project mode](/docs/how-to/recipes/skill-per-project-workflow)。

## 相關

- [`target` 指令參考](/docs/reference/commands/target)
- [`mcp` 指令參考](/docs/reference/commands/mcp)
- [共用 MCP servers](/docs/how-to/daily-tasks/sharing-mcp)
