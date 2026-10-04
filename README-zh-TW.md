<p align="center" style="margin-bottom: 0;">
  <img src=".github/assets/skillshare-logo-card.png" alt="skillshare" width="280">
</p>

<h1 align="center" style="margin-top: 0.5rem; margin-bottom: 0.5rem;">skillshare</h1>

<p align="center">
  <a href="README.md">English</a> · <a href="README-ja.md">日本語</a> · <a href="README-ko.md">한국어</a> · <a href="README-zh-CN.md">简体中文</a> · <a href="README-zh-TW.md">繁體中文</a>
</p>

<p align="center">
  <a href="https://skillshare.runkids.cc"><img src="https://img.shields.io/badge/Website-skillshare.runkids.cc-blue?logo=docusaurus" alt="Website"></a>
  <a href="LICENSE"><img src="https://img.shields.io/badge/License-MIT-yellow.svg" alt="License: MIT"></a>
  <a href="https://github.com/runkids/skillshare/releases"><img src="https://img.shields.io/github/v/release/runkids/skillshare" alt="Release"></a>
  <a href="https://github.com/runkids/skillshare/releases"><img src="https://img.shields.io/github/downloads/runkids/skillshare/total" alt="Downloads"></a>
  <img src="https://img.shields.io/badge/platform-macOS%20%7C%20Linux%20%7C%20Windows-blue" alt="Platform">
  <a href="https://deepwiki.com/runkids/skillshare"><img src="https://deepwiki.com/badge.svg" alt="Ask DeepWiki"></a>
</p>

<p align="center">
  <a href="https://github.com/runkids/skillshare/stargazers"><img src="https://img.shields.io/github/stars/runkids/skillshare?style=social" alt="Star on GitHub"></a>
</p>

<p align="center">
  <a href="https://trendshift.io/repositories/21835" target="_blank"><img src="https://trendshift.io/api/badge/repositories/21835" alt="runkids%2Fskillshare | Trendshift" style="width: 250px; height: 55px;" width="250" height="55"/></a>
</p>

<p align="center">
  <strong>你的 AI coding 環境，隨處可用。</strong><br>
  在同一個地方管理 skills、agents、rules、MCP 連線與 hooks。<br>
  適用於 Claude Code、Codex、Pi、OpenCode 等工具。
</p>

<p align="center">
  <a href="https://skillshare.runkids.cc">官方網站</a> •
  <a href="#安裝">安裝</a> •
  <a href="#快速開始">快速開始</a> •
  <a href="#功能重點">功能重點</a> •
  <a href="#cli-與介面預覽">畫面截圖</a> •
  <a href="#desktop-app">桌面 App</a> •
  <a href="https://skillshare.runkids.cc/docs">文件</a>
</p>

<p align="center">
  <img src=".github/assets/demo.gif" alt="skillshare demo" width="960">
</p>

> [!NOTE]
> **最新版本**：v0.24.0 — 重新設計的 **`init`** 先提問、確認後才寫入；CLI 與所有 **TUI** 使用一致的輸出風格和按鍵；用 **`push --pull`** 一個指令雙向同步；在多個 agent 之間共享 **Markdown 記憶**；並能依 target 開關 **Pi extension**、加入 **pi.dev 上的 npm 套件**。完整的新功能與修正見 [Releases](https://github.com/runkids/skillshare/releases) 和[更新紀錄](https://skillshare.runkids.cc/changelog)。

## 為什麼用 skillshare

切換 AI 工具，不該每次都重新設定環境。
skillshare 把 skills 與其他 AI 資源集中到由你掌控的地方。

- **換工具，繼續用你的 skills** — 修改一次，再同步到 Claude Code、Codex、Pi 和你使用的其他工具。
- **換電腦，帶著環境走** — 用 Git 管理資源來源，在另一台電腦上 pull。
- **和團隊共用** — 專案資源與程式碼一起管理，共用 skills 透過 tracked repo 發布。

一位同事用 Claude Code，另一位用 Codex。把共用的程式碼審查清單放在專案的 `.skillshare/` 中一起管理。新成員安裝專案宣告的 skills，再同步到設定好的工具，就不用從聊天紀錄複製指令。[團隊 onboarding →](https://skillshare.runkids.cc/docs/how-to/recipes/team-onboarding-recipe)

透過桌面 App 或 CLI 在本機管理，[使用前稽核 skills](https://skillshare.runkids.cc/docs/reference/commands/audit)，並[選擇各工具接收哪些內容](https://skillshare.runkids.cc/docs/how-to/daily-tasks/filtering-skills)。

> 從其他工具換過來？ [遷移指南](https://skillshare.runkids.cc/docs/how-to/advanced/migration) · [比較](https://skillshare.runkids.cc/docs/understand/philosophy/comparison)

## CLI 與介面預覽

| Skill 詳細資訊 | 安全稽核 |
|---|---|
| <img src=".github/assets/skill-detail-tui.png" alt="Skill 詳細資訊" width="480" height="300"> | <img src=".github/assets/audit-tui.png" alt="安全稽核" width="480" height="300"> |

| 網頁儀表板 | 網頁 Skills 頁面 |
|---|---|
| <img src=".github/assets/ui/web-dashboard-demo.png" alt="網頁儀表板" width="480"> | <img src=".github/assets/ui/web-skills-demo.png" alt="網頁 Skills 頁面" width="480"> |

## 安裝

> [!TIP]
> **用桌面 App 管理 skillshare。** [下載 Skillshare App](https://github.com/runkids/skillshare-app/releases/latest)，支援 macOS（Apple Silicon）、Windows 與 Linux。首次啟動會引導你安裝或選取 CLI、選擇 AI 工具，並完成第一次同步。[安裝指南](https://skillshare.runkids.cc/zh-Hant/docs/getting-started/desktop-app)。

<a id="desktop-app"></a>

### 桌面 App — 圖形化設定與日常管理

[Skillshare App](https://github.com/runkids/skillshare-app) 把技能、代理、MCP 與 hooks 整合在桌面視窗中。安裝並開啟 App，再依首次啟動的引導完成設定。

macOS（Apple Silicon），使用 Homebrew：

```bash
brew tap runkids/tap
brew install --cask skillshare-app
```

**Windows／Linux，或手動安裝 macOS 版本：**[下載最新 App 安裝包](https://github.com/runkids/skillshare-app/releases/latest)。各平台的詳細說明請見[桌面 App 安裝指南](https://skillshare.runkids.cc/zh-Hant/docs/getting-started/desktop-app)。

### CLI：macOS / Linux

```bash
curl -fsSL https://raw.githubusercontent.com/runkids/skillshare/main/install.sh | sh
```

腳本預設安裝到 `~/.local/bin`，正常安裝與更新不需要 `sudo`。只有安裝器顯示 PATH 設定提示時，才需要依提示設定後再執行 `skillshare`。可把提示的指令加入 shell 設定檔（例如 `~/.zshrc` 或 `~/.bashrc`），讓之後開啟的終端機也能使用。可用 `INSTALL_DIR` 指定其他安裝位置。

### Windows PowerShell

```powershell
irm https://raw.githubusercontent.com/runkids/skillshare/main/install.ps1 | iex
```

### CLI：Homebrew

```bash
brew install skillshare
```

> **提示：** 執行 `skillshare upgrade` 更新到最新版。它會自動判斷你的安裝方式並處理後續步驟。

### GitHub Actions

```yaml
- uses: runkids/setup-skillshare@v1
  with:
    source: ./skills
- run: skillshare sync
```

所有選項（audit、project 模式、鎖定版本）請見 [`setup-skillshare`](https://github.com/marketplace/actions/setup-skillshare)。

### 簡寫（選用）

在 shell 設定檔（`~/.zshrc` 或 `~/.bashrc`）加上 alias：

```bash
alias ss='skillshare'
```

## 快速開始

```bash
skillshare init            # 建立設定檔、來源目錄，並偵測已安裝的 target
skillshare sync            # 把 skills 同步到所有 target
```

## 運作方式

- macOS / Linux: `~/.config/skillshare/`
- Windows: `%AppData%\skillshare\`

```
┌─────────────────────────────────────────────────────────────┐
│                    Source Directory                         │
│   ~/.config/skillshare/skills/    ← skills (SKILL.md)       │
│   ~/.config/skillshare/agents/    ← agents                  │
│   ~/.config/skillshare/extras/    ← rules, commands, etc.   │
└─────────────────────────────────────────────────────────────┘
                              │ sync
              ┌───────────────┼───────────────┐
              ▼               ▼               ▼
       ┌───────────┐   ┌───────────┐   ┌───────────┐
       │  Claude   │   │  OpenCode │   │ OpenClaw  │   ...
       └───────────┘   └───────────┘   └───────────┘
```

| 平台 | Skills 來源 | Agents 來源 | Extras 來源 | 連結方式 |
|----------|---------------|---------------|---------------|-----------|
| macOS/Linux | `~/.config/skillshare/skills/` | `~/.config/skillshare/agents/` | `~/.config/skillshare/extras/` | Symlinks |
| Windows | `%AppData%\skillshare\skills\` | `%AppData%\skillshare\agents\` | `%AppData%\skillshare\extras\` | 資料夾用 NTFS Junction（不需要管理員權限）；檔案 symlink 需要開發人員模式，否則改為複製 |

| | 命令式（每次個別安裝） | 宣告式（skillshare） |
|---|---|---|
| **單一來源** | skills 各自複製，互不相關 | 一份來源，以 symlink（或複製）發布 |
| **新電腦的設定** | 手動重跑每一次安裝 | `git clone` 設定檔，再 `sync` |
| **安全稽核** | 無 | 內建 `audit`，install 和 update 時自動掃描 |
| **網頁儀表板** | 無 | `skillshare ui` |
| **執行環境相依** | Node.js + npm | 無（單一 Go 執行檔） |

> [完整比較 →](https://skillshare.runkids.cc/docs/understand/philosophy/comparison)

## 功能重點

**安裝與更新 skills** — 來源可以是 GitHub、GitLab 或任何 Git 主機

```bash
skillshare install github.com/reponame/skills
skillshare update --all
skillshare target claude --mode copy  # symlink 不能用的時候
```

**Symlink 有問題？** — 個別 target 可以改用 copy 模式

```bash
skillshare target <name> --mode copy
skillshare sync
```

**安全稽核** — 在 skills 進到 agent 之前先掃描

```bash
skillshare audit
```

**專案 skills** — 跟著 repo 走，和程式碼一起 commit

```bash
skillshare init -p && skillshare sync
```

**Agents** — 把自訂 agent 同步到支援 agent 的 target

```bash
skillshare sync agents            # 只同步 agents
skillshare sync --all             # skills、agents、extras、MCP、hooks 一起同步
```

**Extras** — 管理 rules、commands、prompts 等資源

```bash
skillshare extras init rules          # 建立名為 "rules" 的 extra
skillshare sync --all                 # skills、agents、extras、MCP、hooks 一起同步
skillshare extras collect rules       # 把本機檔案收回來源
```

**MCP 連線** — 設定一次，Claude Code、Codex、Pi、VS Code、OpenCode 等工具都能用

```bash
skillshare mcp add                    # 引導式設定，輸入 URL 或貼上 JSON
skillshare sync mcp --dry-run         # 預覽各工具設定檔會有的變更
skillshare sync mcp                   # 套用連線設定
```

定義可以放在 `config.yaml`，或另外引用一個 `mcp.yaml`。
範例、環境變數引用，以及匯入既有連線的方式，請見 [MCP 設定](https://skillshare.runkids.cc/docs/how-to/daily-tasks/sharing-mcp)。

管理原生 hooks，管理操作不執行 hook：

```bash
skillshare hooks add check --file ./check.yaml
skillshare hooks sync --dry-run
skillshare hooks sync
```

**Plugins** — 安裝完整的 plugin，並選擇哪些工具要安裝

```bash
skillshare plugin add                 # 引導式：來源、plugin、target、確認
skillshare plugin add owner/repo --target claude --target codex --no-tui
skillshare sync plugins --dry-run     # plugin 的同步獨立於 sync --all
```

工具裡已經裝好的 plugin 可以用 `plugin import` 納入管理。
請見[跨工具管理 plugin](https://skillshare.runkids.cc/docs/how-to/daily-tasks/sharing-plugins)。

**Shell 自動補全** — 用 Tab 補全指令、flag 和子指令

```bash
skillshare completion bash --install   # 也支援 zsh、fish、powershell、nushell
```

**本機檢查點** — commit 來源目錄的變更，但不 push

```bash
skillshare commit -m "Update review skill"
skillshare commit --dry-run
```

**網頁儀表板** — 視覺化的控制面板

```bash
skillshare ui
```

[所有指令與指南 →](https://skillshare.runkids.cc/docs/reference/commands)

## 參與貢獻

歡迎貢獻！請先開 issue，再送出附測試的 draft PR。
環境設定請見 [CONTRIBUTING.md](CONTRIBUTING.md)。

```bash
git clone https://github.com/runkids/skillshare.git && cd skillshare
make check  # format + lint + test
```

> [!TIP]
> 不知道從哪裡開始？看看 [open issues](https://github.com/runkids/skillshare/issues)，或試試 [Playground](https://skillshare.runkids.cc/docs/learn/with-playground)，不需要任何設定就有開發環境。

## 貢獻者

感謝每一位讓 skillshare 變得更好的人。完整名單在[英文版 README](README.md#contributors)。

---

如果 skillshare 對你有幫助，歡迎給個 ⭐

## Star History

[![Star History Chart](https://star-history.dera.page/svg?repos=runkids/skillshare&type=date&legend=top-left)](https://star-history.dera.page/#runkids/skillshare&type=date&legend=top-left)

---

## 授權

MIT
