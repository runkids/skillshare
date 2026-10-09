---
sidebar_position: 1
---

# target

管理 sync targets（AI CLI skill 目錄）。

```bash
skillshare target add <name> <path>    # 新增 target
skillshare target remove <name>        # 移除 target
skillshare target list                 # 列出所有 targets
skillshare target <name>               # 顯示 target 資訊
skillshare target <name> --mode merge  # 變更 sync 模式
skillshare target <name> --target-naming standard  # 變更命名方式
skillshare target <name> --skills=false    # 停止同步 skills
```

## 使用時機

- 安裝新的 AI CLI 工具後新增 target
- 移除不再使用的 target
- 變更某個 target 的 sync 模式（merge、copy 或 symlink）
- 變更某個 target 的命名方式（flat、standard 或 prefixed）
- 逐一調整各 target 的相容性，而非強制套用單一全域模式
- 設定 include/exclude filters 以選擇性同步 skills
- 某個工具已經會讀取另一個 target 的資料夾時，停止同步 skills 給它，但仍繼續管理它的 agents、MCP servers 與 instructions

## 子指令

### target add

新增一個新的 skill 同步 target。

```bash
skillshare target add windsurf ~/.windsurf/skills
```

此指令會驗證：
- 路徑存在，或父目錄存在
- 路徑看起來像是一個 skills 目錄
- target 名稱是唯一的

加上 `--no-skills` 可新增一個不同步 skills 的 target。它的 agents、MCP servers 與 instructions 仍會受到管理，skills 資料夾也不必事先存在：

```bash
skillshare target add gemini ~/.gemini/skills --no-skills
# Added target: gemini -> ~/.gemini/skills (skills off)
```

詳見 [Skills 開啟或關閉](#skills-off)。

#### 某個 Agent 的另一個帳號 {#another-account}

如果你用獨立的 config 目錄執行某個 Agent 的第二個帳號，例如以 `CLAUDE_CONFIG_DIR=~/.claude-work` 啟動的 Claude Code、以 `CODEX_HOME` 啟動的 Codex，或以 `PI_CODING_AGENT_DIR` 啟動的 Pi，請把該目錄新增為 target。Skillshare 會從它推導出 skills 與 agents 的路徑：

```bash
skillshare target add claude-work --agent claude --config-dir ~/.claude-work
# Added target: claude-work -> ~/.claude-work/skills
```

你有幾個帳號就新增幾個，每個各用自己的名稱。這個名稱也可以當作 [MCP target](./mcp.md#accounts) 使用，因此一次 sync 就能涵蓋每個帳號的 skills、agents 與 MCP servers。

如果這個帳號使用相容的 CLI，例如 Pi 的 omo，`--cli` 會讓它的 [plugin 指令](./plugin.md#accounts)改用那個 CLI：

```bash
skillshare target add omo --agent pi --config-dir ~/.omo/agent --cli omo
```

`--agent` 接受 `claude`（`CLAUDE_CONFIG_DIR`）、`codex`（`CODEX_HOME`）、`pi` 與 `omp`（兩者都使用 `PI_CODING_AGENT_DIR`）。Codex、Pi 或 OMP 帳號會把 skills 同步到 `<config_dir>/skills`；只有 Claude 另外有 agents 目錄。OMP 帳號支援 skills、instructions、files、MCP 與原生程式碼 hooks，但不支援 plugin 指令。該目錄必須是絕對路徑或以 `~` 開頭，不能是該 Agent 的預設目錄，也不能由兩個 targets 共用。

移除這類 target 不會因為 MCP 而失敗：即使 `mcp.targets` 或某個 server 的 `targets` 仍然指名它，`skillshare target remove` 還是會移除該 target，並提醒你把那邊的名稱也一併移除。

### target remove

移除某個 target，並將其 skills 還原為一般目錄。

```bash
skillshare target remove cursor           # 移除單一 target
skillshare target remove --all            # 移除所有 targets
skillshare target remove cursor --dry-run # 預覽
```

**執行內容：**
1. 建立該 target 的備份
2. 偵測 sync 模式：
   - **Symlink 模式：** 移除目錄 symlink，將 source 內容以真實目錄的形式複製回去
   - **Merge 模式：** 只移除指向 source 的 symlinks（依路徑前綴判斷），將每個 skill 以真實檔案複製回去。本機（非 symlink）的 skills 會被保留。
   - **Copy 模式：** 移除 `.skillshare-manifest.json`。受管理的複本與本機 skills 會保留為一般目錄。
3. 從設定中移除該 target

如果還有其他 target 寫入同一個 skills 資料夾（例如 `codex` 和 `universal` 都用 `~/.agents/skills`），會略過第 2 步：skills 仍為那個 target 保持連結，只有被移除的 target 從設定中拿掉。

[關閉 skills](#skills-off) 的 target 沒有任何同步內容，因此同樣會略過第 2 步，其資料夾維持原樣。

### target list

列出所有已設定的 targets。

```bash
skillshare target list                 # 互動式 TUI（TTY 下預設）
skillshare target list --no-tui        # 純文字輸出
skillshare target list --json          # 供 CI/scripts 使用的 JSON 輸出
```

#### 互動式 TUI

在 TTY 中，`target list` 會開啟互動式畫面：左側是 targets，右側是選取 target 的路徑、模式與篩選規則。在這裡可以更改 target 的同步模式、命名方式與 include/exclude 篩選，或移除 target（會先備份再解除連結，與 `target remove` 相同）。按鍵列在畫面底部。

變更會立即寫入 config。執行 `skillshare sync` 套用。

使用 `--no-tui` 跳過 TUI，改為輸出純文字：

```
claude
  Skills    ~/.claude/skills  merge · flat · merged · 43 shared
  Agents    ~/.claude/agents  merge · 2/2 linked

cursor
  Skills    ~/.cursor/skills  merge · flat · merged · 43 shared, 1 local
  Agents    ~/.cursor/agents  merge · 2/2 linked

codex
  Skills    ~/.openai-codex/skills  symlink · flat · linked

3 targets
```

#### JSON 輸出

```bash
skillshare target list --json
```

```json
{
  "targets": [
    {
      "name": "claude",
      "path": "~/.claude/skills",
      "mode": "merge",
      "targetNaming": "flat",
      "include": [],
      "exclude": [],
      "skillsEnabled": true
    },
    {
      "name": "cursor",
      "path": "~/.cursor/skills",
      "mode": "merge",
      "targetNaming": "standard",
      "include": [],
      "exclude": [],
      "skillsEnabled": true
    }
  ]
}
```

### target info / settings

顯示 target 詳細資訊或變更設定。

```bash
# 顯示資訊
skillshare target claude

# 變更模式
skillshare target claude --mode symlink
skillshare target claude --mode merge

# 變更命名方式
skillshare target claude --target-naming standard
skillshare target claude --target-naming flat

skillshare sync  # 套用變更
```

## Sync 模式

| 模式 | 行為 |
|------|----------|
| `merge` | 每個 skill 個別以 symlink 連結。保留本機 skills。**預設。** |
| `copy` | 每個 skill 以真實檔案複製。適合無法跟隨 symlink 的 AI CLIs。 |
| `symlink` | 整個目錄以單一 symlink 連結。各處皆為完全一致的複本。 |

`target --mode` 是主要的相容性控制手段。讓你的全域預設保持簡單，只在需要時才覆寫。

## Target 命名

| 命名方式 | 行為 |
|--------|----------|
| `flat` | 巢狀 skills 以 `__` 分隔符扁平化（例如 `frontend__dev`）。**預設。** |
| `standard` | 直接使用 SKILL.md 的 `name` 欄位（例如 `dev`）。遵循 [Agent Skills spec](https://agentskills.io/specification)。 |
| `prefixed` | 僅限 copy mode。與 `standard` 類似，但 tracked repo 內的 skill 會命名為 `<repo>-<name>`，資料夾名稱與複本的 `name:` 都是如此（例如 `mattpocock-skills-prototype`）。 |

`target --target-naming` 控制 target 中 skill 目錄的命名方式。在 `standard` 和 `prefixed` 模式下，名稱無效或衝突的 skills 會被警告並跳過。`flat` 和 `standard` 在 symlink 模式下會被忽略。除非 target 以 copy mode 同步 skills，否則 `--target-naming prefixed` 會被拒絕；當 target 使用 `prefixed` 時，`--mode` 也會拒絕離開 copy mode。要同時切換兩者，可在同一行指令一起指定：`skillshare target cursor --mode copy --target-naming prefixed`。`--mode`、`--agent-mode` 和 `--target-naming` 可以這樣組合：它們會一起檢查並只儲存一次，target 已有的值會顯示為未變更。它們不能與 `--skills` 或 include/exclude 旗標組合，請分開執行。見 [Target Naming](/docs/understand/sync-modes#target-naming)。

```bash
# 將 target 設為 copy 模式（適合 Cursor、Copilot CLI 等）
skillshare target cursor --mode copy
skillshare sync  # 套用變更
```

### 混合策略範例

```bash
# 對大多數 targets 保持預設的 merge 行為
skillshare target claude --mode merge

# 對某個 target 優先考慮相容性
skillshare target cursor --mode copy

# 對另一個 target 做完全鏡像
skillshare target codex --mode symlink

skillshare sync
```

## Target Filters（include/exclude）{#target-filters-includeexclude}

從 CLI 管理 skills 與 agents 的每個 target include/exclude filters：

```bash
# Skills
skillshare target claude --add-include "team-*"
skillshare target claude --add-exclude "_legacy*"
skillshare target claude --remove-include "team-*"
skillshare target claude --remove-exclude "_legacy*"

# Agents
skillshare target claude --add-agent-include "team-*"
skillshare target claude --add-agent-exclude "draft-*"
skillshare target claude --remove-agent-include "team-*"
skillshare target claude --remove-agent-exclude "draft-*"
```

變更 filters 後，執行 `skillshare sync` 以套用。

Filters 在 **merge 與 copy 模式**下才有作用。模式使用 Go 的 `filepath.Match` 語法（`*`、`?`、`[...]`）。在 symlink 模式下，filters 會被忽略。

Agent filters 僅適用於有 agents 路徑的 targets，無論是來自內建的 target 定義，或設定中明確的 `agents.path` 覆寫。

詳見 [Configuration](/docs/reference/targets/configuration#include--exclude-target-filters) 了解 pattern 速查表與情境範例。

:::tip
Target filters 是三層過濾機制之一。詳見 [Filtering Reference](/docs/reference/filtering) 了解它與 `.skillignore` 及 SKILL.md `targets` 的互動方式。
:::

## Skills 開啟或關閉 {#skills-off}

有些工具除了自己的資料夾，也會從另一個 target 的資料夾讀取 skills。例如 Pi 會讀取 `~/.pi/agent/skills`，也會讀取 `universal` target 的資料夾 `~/.agents/skills`。兩邊都同步 skills 的話，Pi 會找到每個 skill 兩次：Pi 會保留先找到的那個，並對另一個發出警告，有些工具則會兩個都列出來。為該 target 關閉 skills 後，skillshare 會繼續管理它的 agents、MCP servers 與 instructions，但不再動它的 skills 資料夾：

```bash
skillshare target pi --skills=false --dry-run   # 預覽
skillshare target pi --skills=false
```

```
✓ Removed   2 links  alpha, beta
  Kept      1 local skill  my-notes

✓ Skills off for pi
  Agents, MCP servers and instructions are still managed
```

關閉 skills 會在設定中儲存 `skills.enabled: false`，接著清理資料夾：

- **Merge 模式：** 移除指向 source 的連結。你自己的 skills 會保留。
- **Symlink 模式：** 移除資料夾指向 source 的連結，絕不會動到它所指向的內容。
- **Copy 模式：** 保留複本（它們是真實資料夾，你可能改過），並另外列出。工具仍會載入這些複本，所以如果它也從別的資料夾讀取同樣的 skills，請自行刪除這些複本：

  ```
  ! Kept      2 copied skills  alpha, beta

  ✓ Skills off for pi
    The tool still loads these copies; delete them if it reads the same skills elsewhere
    Agents, MCP servers and instructions are still managed
  ```

- **共用資料夾：** 如果有已啟用的 target 寫入同一個資料夾，就不會移除任何東西。

此後，`sync`、`diff`、`status` 與 `doctor` 都會略過該 target 的 skills；`status` 與 `sync` 會將它顯示為 `skills off`。用 `--skills=true` 重新開啟 skills，下次執行 `skillshare sync` 就會再次同步。

`--skills` 不能與 include/exclude flags 在同一個指令中一起使用，請分開執行。在 project 模式（`-p`）下的運作方式相同。

在 web dashboard 中，請在該 target 的 Skills 分頁使用 **停止同步 Skills**。移除任何東西之前，它會列出哪些會被移除、哪些會保留，並在其他工具也讀取同一個資料夾時提出警告。

## 選項

### target add

| Flag | 說明 |
|------|-------------|
| `--agent <agent>` | 新增這個 Agent 的[另一個帳號](#another-account)，而不是指定路徑。需搭配 `--config-dir` |
| `--config-dir <dir>` | 該帳號使用的 config 目錄 |
| `--cli <executable>` | 用這個相容的 CLI 取代 Agent 本身來執行該帳號的 plugin 指令。填 `PATH` 上的名稱或絕對路徑 |
| `--no-skills` | 以[關閉 skills](#skills-off) 的狀態新增 target |

### target remove

| Flag | 說明 |
|------|-------------|
| `--all, -a` | 移除所有 targets |
| `--dry-run, -n` | 預覽而不做任何變更 |

### target list

| Flag | 說明 |
|------|-------------|
| `--json` | 以 JSON 輸出 |
| `--no-tui` | 停用互動式 TUI，改用純文字輸出 |

### target info / settings

| Flag | 說明 |
|------|-------------|
| `--mode, -m <mode>` | 設定 sync 模式（merge、copy 或 symlink） |
| `--agent-mode <mode>` | 設定 agents 的 sync 模式（merge、copy 或 symlink） |
| `--target-naming <naming>` | 設定 target 命名方式（flat、standard 或 prefixed；prefixed 需要 copy mode） |
| `--skills <true\|false>` | [開啟或關閉](#skills-off) skills 同步；也可寫成 `--skills=false` |
| `--dry-run, -n` | 搭配 `--skills=false`，預覽會被移除的內容 |
| `--add-include <pattern>` | 新增一個 include filter pattern |
| `--add-exclude <pattern>` | 新增一個 exclude filter pattern |
| `--remove-include <pattern>` | 移除一個 include filter pattern |
| `--remove-exclude <pattern>` | 移除一個 exclude filter pattern |
| `--add-agent-include <pattern>` | 新增一個 agent include filter pattern |
| `--add-agent-exclude <pattern>` | 新增一個 agent exclude filter pattern |
| `--remove-agent-include <pattern>` | 移除一個 agent include filter pattern |
| `--remove-agent-exclude <pattern>` | 移除一個 agent exclude filter pattern |

## 支援的 AI CLIs

skillshare 在 `init` 期間會自動偵測這些：

| CLI | 預設路徑 |
|-----|-------------|
| Claude Code | `~/.claude/skills` |
| Cursor | `~/.cursor/skills` |
| OpenCode | `~/.opencode/skills` |
| Windsurf | `~/.windsurf/skills` |
| Codex | `~/.openai-codex/skills` |
| Antigravity（app） | `~/.gemini/config/skills` |
| Antigravity CLI | `~/.gemini/antigravity-cli/skills` |
| Gemini CLI | `~/.gemini/skills` |
| Amp | `~/.amp/skills` |
| ... 以及其他 45+ 個 | 詳見 [supported targets](/docs/reference/targets/supported-targets) |

## 範例

```bash
# 新增自訂 target
skillshare target add my-tool ~/my-tool/skills

# 檢查 target 狀態
skillshare target claude

# 切換為 copy 模式（適合無法讀取 symlink 的 AI CLIs）
skillshare target cursor --mode copy
skillshare sync

# 切換為 symlink 模式
skillshare target claude --mode symlink
skillshare sync

# 設定 agent sync 模式
skillshare target claude --agent-mode copy
skillshare sync

# 新增/移除 skill filters
skillshare target claude --add-include "team-*"
skillshare target claude --add-exclude "_legacy*"
skillshare target claude --remove-include "team-*"
skillshare sync

# 新增/移除 agent filters
skillshare target claude --add-agent-include "team-*"
skillshare target claude --add-agent-exclude "draft-*"
skillshare sync

# 移除 target（還原 skills）
skillshare target remove cursor
```

## Project 模式

管理目前 project 的 targets：

```bash
skillshare target add windsurf -p                                # 新增已知的 target
skillshare target add custom ./tools/ai/skills -p                # 新增自訂路徑
skillshare target remove cursor -p                                # 移除 target
skillshare target list -p                                         # 列出 project targets
skillshare target claude -p                                  # 顯示 target 資訊
skillshare target claude --add-include "team-*" -p          # 新增 filter
skillshare target claude --add-agent-include "team-*" -p    # 新增 agent filter
```

### 差異之處

| | Global | Project（`-p`） |
|---|---|---|
| 設定 | `~/.config/skillshare/config.yaml` | `.skillshare/config.yaml` |
| 路徑 | 絕對路徑（例如 `~/.claude/skills`） | 相對或絕對路徑（例如 `.claude/skills`） |
| Sync 模式 | Merge、copy 或 symlink | Merge、copy 或 symlink（預設 merge） |
| 模式變更 | `--mode` flag | `--mode` flag |

### Project Target List 範例

```
claude
  Skills    .claude/skills  merge · flat · merged · 3 shared

cursor
  Skills    .cursor/skills  merge · flat · merged · 3 shared

custom-tool
  Skills    ./tools/ai/skills  merge · flat · merged · 3 shared

3 targets
```

Project 模式下的 targets 支援：
- **已知的 target 名稱**（例如 `claude`、`cursor`） — 解析為 project 本機路徑
- **自訂路徑** — 相對於 project 根目錄，或以 `~` 展開的絕對路徑

## 另請參閱

- [sync](/docs/reference/commands/sync) — 將 skills 同步到 targets
- [status](/docs/reference/commands/status) — 顯示 target 狀態
- [Targets](/docs/reference/targets) — Target 管理指南
- [Project Skills](/docs/understand/project-skills) — Project 模式概念
