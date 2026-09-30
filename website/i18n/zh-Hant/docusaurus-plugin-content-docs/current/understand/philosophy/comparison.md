---
sidebar_position: 9
---

# 比較 Skill 管理方式

本頁比較 AI CLI skill 管理的兩種主要架構方式：**命令式**（每次操作各自安裝）與**宣告式**（config + sync）。

如果你正在評估工具或考慮換用其他方案，這篇分析能幫助你理解兩者在設計上的根本差異。

## 架構總覽

### 命令式（每次操作各自安裝）

命令式工具採用每次操作各自安裝的模式 — 每次安裝都是獨立的操作：

```
tool add owner/repo → select agents → choose method → done
tool add owner/repo → select agents → choose method → done
tool add owner/repo → select agents → choose method → done
```

每個操作都需要使用者輸入。沒有任何持久化狀態能描述「什麼東西該裝在哪裡」。

### 宣告式（Config + Sync）

skillshare 採用宣告式模式 — 你只需定義一次期望狀態，然後執行 sync：

```yaml
# config.yaml — define once
source: ~/.config/skillshare/skills
targets:
  claude: ~/.claude
  cursor: ~/.cursor/skills
  codex: ~/.codex/skills
```

```bash
skillshare sync  # reconcile actual state to desired state
```

一個命令，沒有提示，每次結果都一致可預期。

## 功能比較

| 能力 | 命令式（每次操作各自安裝） | 宣告式（skillshare） |
|------------|------------------------|--------------------------|
| **設定** | 沒有設定檔；每次執行都會提示 | `config.yaml` — 設定一次，永久沿用 |
| **Agent 選擇** | 每次都要互動式提示 | 定義在設定中；`sync` 一次處理全部 |
| **安裝方式** | 每次操作各自選擇 copy/symlink | 設定中的 `sync_mode`（merge、copy 或 symlink） |
| **單一事實來源** | Skills 各自獨立複製到每個 agent | Source 目錄 → symlink 到所有 targets |
| **從單一 agent 移除 skill** | 可能刪除 source 檔案，波及其他 agent | 只影響該 target 的 symlink |
| **可重現的設定** | 沒有內建方式能在新機器上還原 | `config.yaml` + source 目錄 = 完整還原 |
| **Project-scoped skills** | Lock 檔案只追蹤全域範圍 | `skillshare init -p` 提供每個 repo 各自的 skills |
| **跨機器同步** | 手動（透過 dotfiles 同步 lock 檔案） | 內建 `push` / `pull` 搭配 git |
| **雙向流動** | 單向（僅安裝） | `collect` 能把 targets 上的改進拉回來 |
| **區分自製與已安裝 skills** | 混在同一個目錄裡 | Tracked repos 使用 `_` 前綴 |
| **離線操作** | CLI 本身就需要 npx + 網路 | 單一執行檔，安裝後可離線運作 |
| **Web 儀表板** | 無 | `skillshare ui` — 視覺化管理 |
| **備份／還原** | 無 | `skillshare backup` / `skillshare restore` |
| **Git 平台支援** | update/check 僅限 GitHub（寫死使用 GitHub Trees API） | 任何 Git remote — GitHub、GitLab、Bitbucket、Azure DevOps、Gitea、AtomGit、Gitee、自架伺服器 |
| **執行環境依賴** | Node.js + npm | 無（單一 Go 執行檔） |

## 常見痛點的解法

### 「我每次安裝都要重新選一次 agent」

用 skillshare，你只需要設定一次 targets：

```yaml
targets:
  claude: ~/.claude
  cursor: ~/.cursor/skills
```

之後每次 `sync`、`install` 或 `collect` 都知道要送去哪裡。不會再有提示。

### 「從一個 agent 移除 skill 會弄壞其他 agent」

在命令式工具中，從一個 agent 移除 skill 可能會刪掉共用的 source 檔案，讓其他 agent 留下損毀的 symlink。

skillshare 的架構完全避免了這個問題 — source 目錄是唯一的事實來源。Target 的 symlink 都指**向** source。移除一個 target 只會移除該 target 自己的 symlink；source 檔案不受影響。

```
Source: ~/.config/skillshare/skills/my-skill/SKILL.md  (always preserved)
  ├── ~/.claude/skills/my-skill → symlink to source  ✓
  ├── ~/.cursor/skills/my-skill  → symlink to source  ✓  (unaffected)
  └── ~/.codex/skills/my-skill  → symlink to source  ✓  (unaffected)
```

### 「我沒辦法在新機器上還原我的設定」

用 skillshare，你的整套設定都是可攜的：

1. 把 `~/.config/skillshare/`（source + config）納入版本控制
2. 在新機器上：`git clone` 你的設定 repo
3. 執行 `skillshare sync`

所有 targets 會立即被重新建立。

### 「Update 和 check 在 GitLab / Bitbucket / Azure DevOps 上不能用」

命令式工具通常依賴 GitHub Trees API 來做更新檢查，這代表 `update` 和 `check` 會默默略過來自非 GitHub 來源的 skills。

skillshare 使用**本機 git 操作**（`git fetch` + tree hash 比對）— 可以搭配任何 Git remote 運作，包括 GitLab、Bitbucket、Azure DevOps、Gitea、AtomGit、Gitee，以及任何自架的伺服器。不需要平台專屬的 API。

```bash
# All of these support install, update, and check:
skillshare install https://gitlab.com/team/skills
skillshare install git@bitbucket.org:company/private-skills.git
skillshare install https://git.mycompany.com/org/repo
skillshare update   # checks all sources, regardless of host
```

### 「大型 repository clone 起來要花超久」

skillshare 預設對非 tracked 的安裝使用淺層 clone（`--depth 1`），大幅縮短下載時間。若 tracked repo 需要完整歷史紀錄，可使用 `--track`。

### 「我的 skills 散落在各個 agent 目錄裡」

skillshare 把所有東西集中在一個地方：

```
~/.config/skillshare/skills/
├── my-custom-skill/          # Your own skills
├── react-best-practices/     # Installed skills
├── _team-repo/               # Tracked repos (prefixed with _)
│   ├── frontend-guidelines/
│   └── code-review/
└── _another-org-repo/
```

`_` 前綴清楚區分了 tracked（團隊／組織）repo 與你個人的 skills。

## 遷移至 skillshare

如果你已經在使用其他的 skill 管理工具：

### 步驟 1：安裝 skillshare

```bash
# macOS / Linux
curl -fsSL https://raw.githubusercontent.com/runkids/skillshare/main/install.sh | sh
```

只有安裝器顯示 PATH 設定提示時，才需要依提示設定後再執行下方指令。沒有 PATH 警告就不需要額外設定。

```bash

# Homebrew
brew install skillshare
```

### 步驟 2：初始化並收集既有 skills

```bash
skillshare init              # Creates config and detects targets
skillshare collect --all     # Imports existing skills from all detected targets
```

### 步驟 3：Sync

```bash
skillshare sync              # Symlinks source skills to all targets
```

你現有的 skills 現在都集中在同一個地方管理了。詳細步驟請見 [遷移指南](/docs/how-to/advanced/migration)。

## 選擇合適的工具

**在以下情況選擇命令式工具：**
- 你很少安裝 skill，也不介意互動式提示
- 你只使用一種 AI CLI
- 你不需要跨機器或團隊協作的工作流程

**在以下情況選擇 skillshare：**
- 你同時使用多種 AI CLI，並希望它們保持同步
- 你想要一次設定、之後就不用再管
- 你在多台機器上工作
- 你會與團隊或組織分享 skills
- 你需要 skills 的備份、還原與版本控制
- 你把 skills 放在 GitLab、Bitbucket、Azure DevOps 或自架 Git 上
- 你偏好單一執行檔、不想有執行環境依賴
- 你不希望安裝／下載活動被追蹤到你本機工作流程之外

---

## 延伸閱讀

- [Migration](/docs/how-to/advanced/migration) — 遷移指南
- [Core Concepts](/docs/understand) — skillshare 如何運作
