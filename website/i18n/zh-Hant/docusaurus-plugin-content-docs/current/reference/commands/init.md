---
sidebar_position: 1
---

# init

第一次設定。偵測已安裝的 AI CLI、匯入它們已有的 skills，並完成同步。

```bash
skillshare init              # 互動式設定
skillshare init --dry-run    # 預覽而不變更
```

## 何時使用

- 第一次在某台機器上設定 skillshare
- 遷移到新電腦（搭配 `--remote` 連接既有 repo）
- 把 skillshare 加入專案（搭配 `--project`）
- 發現新安裝的 AI CLI（搭配 `--discover`）

## 發生了什麼

`init` 先問、最後才寫入。確認摘要之前不會建立任何東西；在任何一題按 <kbd>Esc</kbd> 都會取消，不寫入任何東西。

```mermaid
flowchart TD
    TITLE["skillshare init"]
    START{"How do you want to start?"}
    NEW["New setup: tools → import → git → remote (optional)"]
    CONNECT["Connect my existing repo: URL → keep local skills → tools"]
    SUMMARY["Summary: Yes / Change settings / Cancel"]
    APPLY["Write config, copy skills, install built-in skill, commit"]
    SYNC["Sync now?"]
    TITLE --> START
    START --> NEW --> SUMMARY
    START --> CONNECT --> SUMMARY
    SUMMARY --> APPLY --> SYNC
```

每一題都有預設值，一路按 <kbd>Enter</kbd> 就能得到可用的設定：

| 問題 | 預設值 |
|------|--------|
| Targets | 偵測到的所有 AI CLI |
| Import | 這些工具裡已有的所有 skills |
| Git | 開啟。只版控 skills；綁定 remote 時版控 skills、agents、extras。Plugins、MCP servers 和 hooks 留在各台機器的 `config.yaml` |
| 內建 skill | 安裝 |
| Sync | 是 |

摘要裡的 **Change settings** 可以修改 source 路徑、sync mode，以及 git 要版控哪些東西。

**Connect my existing repo** 用於第二台機器。它會先檢查 repo（不寫入任何東西）並判斷結構：用 `--git-root root` 推上去的 repo 會成為整個 skillshare 資料夾；skills 放在 `skills/` 資料夾的 repo 會以該資料夾作為 source。這台機器和 repo 都有的同名 skill 使用 repo 版本。只存在於這台機器的 skills 會詢問是否保留，並在下次 `skillshare push` 時加入 repo。

第一次 sync 時，工具裡與 source 逐位元組相同的 skill 資料夾會換成連結。內容不同的資料夾會保留並列出；執行 `skillshare sync --force` 可以把它們換掉。

### 沒有終端機時

stdin 或 stdout 不是終端機時（CI、腳本、AI agent），`init` 不會詢問任何問題，直接使用上面的預設值。每個決定印一行，並附上可以改變它的 flag：

```text
✓ Source   ~/.config/skillshare/skills (--source, --subdir)
✓ Targets  claude, cursor, universal (--targets, --no-targets)
✓ Import   all 2 (--copy-from, --no-copy)
✓ Git      skills only (--no-git, --git-root)
✓ Remote   none (--remote <url>)
✓ Skill    install skillshare (--skill, --no-skill)
✓ Sync     merge (--mode)
```

搭配 `--remote` 時，已有 skills 的 repo 會被 pull 下來，同名 skill 使用 repo 版本，並列出名稱。沒有終端機時的輸出不含顏色或控制碼。

`init` 會一次建立 skills source 目錄**與** `agents/` 同層目錄，讓兩種資源類型立即可用。agents 目錄是靜默建立的 — 沒有額外的提示或旗標。檔案格式請見 [Agents](/docs/understand/agents)。

:::info Universal target
偵測到任何 AI CLI 時，`init` 會自動推薦 **universal** target（`~/.agents/skills`）。這是 [vercel-labs/skills](https://github.com/vercel-labs/skills)（`npx skills list`）用來一次為所有相容 agents 提供 skills 的共用目錄。
:::

:::tip Agents source path
agents source 預設為 `<source parent>/agents`（預設安裝下即 `~/.config/skillshare/agents/`）。可在 `config.yaml` 中設定 `agents_source:` 來覆寫位置。Project mode 一律使用專案目錄內的 `agents/`，不會採用 `agents_source`。支援 agent 的 targets（Claude、Cursor、Augment、OpenCode）在你執行 `skillshare sync` 後會自動取得 agents。
:::

## Project Mode

以 `-p` 初始化專案層級的 skills：

```bash
skillshare init -p                              # 互動式（沒有終端機時：所有偵測到的工具）
skillshare init -p --targets claude,cursor  # 指定工具
skillshare init -p --visible                    # 使用可見的 skillshare/ 目錄
```

### 發生了什麼

```mermaid
flowchart TD
    TITLE["skillshare init -p"]
    S1["1. Create .skillshare/skills + .skillshare/agents"]
    S2["2. Detect AI CLI directories"]
    S3["3. Create target skill directories"]
    S4["4. Write config.yaml"]
    TITLE --> S1 --> S2 --> S3 --> S4
```

init 完成後，把專案目錄提交到 git（`skills/` 與 `agents/` 都要）。用 `--visible` 建立可見的 `skillshare/` 而非 `.skillshare/`。完整指南請見 [Project Setup](/docs/how-to/sharing/project-setup)。

## 探索模式

在既有設定上重新執行 init，以偵測並新增新的 AI CLI targets：

### Global

```bash
skillshare init --discover              # 互動式選擇
skillshare init --discover --select codex,opencode  # 非互動式
```

掃描尚未加入設定的新安裝 AI CLI，並詢問要新增哪些；預設全部勾選。沒有終端機時會新增所有新工具。只要偵測到任何 CLI，就會自動推薦 `universal` target（`~/.agents/skills`）。

### Project

```bash
skillshare init -p --discover           # 互動式選擇
skillshare init -p --discover --select antigravity  # 非互動式
```

掃描專案目錄中新的 AI CLI 目錄（例如 `.agents/`）並將其新增為 targets。 沒有終端機時會新增找到的所有新工具。

### Discover + Mode 行為

當你把 `--discover` 與 `--mode` 一起使用時，該 mode **只**套用到這次 discover 執行所新增的 targets。
設定檔中既有的 targets 不會被變更。

```bash
# 以 mode=copy 新增 cursor，不會變更既有的 targets
skillshare init --discover --select cursor --mode copy

# Project mode 變體（規則相同）
skillshare init -p --discover --select cursor --mode copy
```

:::tip
如果你在已初始化的設定上執行 `skillshare init`（未加 `--discover`），錯誤訊息會提示你使用它。
:::

## 選項

| 旗標 | 說明 |
|------|-------------|
| `--source, -s <path>` | 自訂 source 目錄（也可在摘要的 **Change settings** 修改） |
| `--remote <url>` | 設定 git remote（隱含 `--git`）。已有 skills 的 repo 會被 pull 下來；同名 skill 使用 repo 版本 |
| `--project, -p` | 在目前目錄初始化專案層級的 skills |
| `--copy-from, -c <name\|path>` | 從特定 CLI 或路徑複製 skills |
| `--no-copy` | 以空的 source 開始（跳過複製提示） |
| `--targets, -t <list>` | 以逗號分隔的 target 名稱 |
| `--all-targets` | 新增所有偵測到的 targets |
| `--no-targets` | 跳過 target 選擇 |
| `--mode, -m <mode>` | 為新設定的 targets 設定預設 mode（`merge`、`copy`、`symlink`）。搭配 `--discover` 時，只影響新增的 targets。 |
| `--git` | 初始化 git 而不提示（預設） |
| `--no-git` | 跳過 git 初始化 |
| `--skill` | 安裝內建的 skillshare skill（預設；會把 `/skillshare` 加入 AI CLI） |
| `--no-skill` | 跳過內建 skill 安裝 |
| `--discover, -d` | 偵測並將新的 AI CLI targets 加入既有設定 |
| `--select <list>` | 以逗號分隔要新增的 targets（需搭配 `--discover`） |
| `--config local` | 將 `config.yaml` 加入 gitignore，讓每位開發者自行管理自己的 targets（僅限 project mode）。見 [Centralized Skills Repo](/docs/how-to/recipes/centralized-skills-repo) recipe。 |
| `--visible` | 建立可見的 `skillshare/` 專案目錄，而非 `.skillshare/`（僅限 project mode）。見 [Project Skills](/docs/understand/project-skills#visible-project-directory)。 |
| `--git-root <scope>` | `commit`/`push`/`pull` 操作的目錄範圍（預設 `skills`，另有 `agents`、`extras`、`root`）。`root` 會把 skills、agents、extras 一起放進同一個 repo 版控，並自動忽略 `config.yaml`。綁定 remote 時預設為 `root`，否則為 `skills`；也可在摘要的 **Change settings** 修改。之後可重新執行 `skillshare init --git-root <scope>` 以非互動方式切換範圍 — 它會在新範圍初始化 repo 並保存設定，但不會搬移既有歷史。 |
| `--subdir <name>` | 使用子目錄作為 source 路徑（例如 `skills`）；連接 repo 時會自動偵測 |
| `--dry-run, -n` | 預覽而不實際變更 |

`init` 會設定你的初始 mode 策略。之後隨時可以針對個別 target 微調：

```bash
skillshare target cursor --mode copy
skillshare sync
```

## Source 子目錄

預設情況下，`init --remote` 會把整個 git repo 根目錄視為 skills source。如果你的 repo 也包含非 skill 檔案（README、CI 設定、dotfiles 等），可以改把 skills 存在子目錄中：

```
# 不加 --subdir：repo root = source（所有檔案都是 skills）
~/.config/skillshare/skills/          ← git repo root = source
  ├── my-skill/
  └── another-skill/

# 加 --subdir skills：source 指向一個子目錄
~/.config/skillshare/skills/          ← git repo root
  ├── README.md
  ├── .github/
  └── skills/                         ← source points here
      ├── my-skill/
      └── another-skill/
```

典型使用情境：把 skills 嵌入既有的 dotfiles 或 monorepo，而非用專屬的 skills-only repo。

連接的 repo 最上層沒有 skills、但 `skills/` 資料夾裡有時，`init` 會自動使用該資料夾。要用其他名稱就傳入 `--subdir`：

```bash
skillshare init --remote git@github.com:you/dotfiles.git --subdir skills
```

## 常見情境

### Remote 設定（擇一）

互動式：執行 `skillshare init` 並選擇 **Connect my existing skillshare repo**。

非互動式（無提示，自動偵測已安裝的 targets）：

```bash
skillshare init --remote git@github.com:you/my-skills.git --no-copy --all-targets --no-skill
```

非互動式（無提示，並立即匯入既有的 Claude skills）：

```bash
skillshare init --remote git@github.com:you/my-skills.git --copy-from claude --all-targets --no-skill
```

### 集中式 skills repo

```bash
# 建立者：以本地設定建立共用 repo
skillshare init -p --config local --targets claude

# 團隊成員：clone 並自動偵測共用 repo
git clone <repo> && cd <repo>
skillshare init -p
skillshare target add myproject ~/DEV/myproject/.claude/skills -p
```

### 其他情境

```bash
# 標準設定（自動偵測所有東西）
skillshare init

# 使用既有的 skills 目錄
skillshare init --source ~/.config/skillshare/skills

# 專案層級設定
skillshare init -p
skillshare init -p --targets claude,cursor

# 不詢問、直接使用預設值（沒有終端機時也是這樣）
skillshare init --no-copy --all-targets --git --skill

# 以 copy mode 作為新增 targets 的預設值
skillshare init --mode copy

# 把新安裝的 CLI 加入既有設定
skillshare init --discover
skillshare init -p --discover

# 新增一個新發現的 target，並只對該 target 強制使用 copy mode
skillshare init --discover --select cursor --mode copy
```
