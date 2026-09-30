<p align="center" style="margin-bottom: 0;">
  <img src=".github/assets/logo.png" alt="skillshare" width="280">
</p>

<h1 align="center" style="margin-top: 0.5rem; margin-bottom: 0.5rem;">skillshare</h1>

<p align="center">
  <a href="README.md">English</a> · <a href="README-ja.md">日本語</a> · <a href="README-ko.md">한국어</a> · <a href="README-zh-CN.md">简体中文</a> · <a href="README-zh-TW.md">繁體中文</a>
</p>

<p align="center">
  <a href="https://skillshare.runkids.cc"><img src="https://img.shields.io/badge/Website-skillshare.runkids.cc-blue?logo=docusaurus" alt="Website"></a>
  <a href="LICENSE"><img src="https://img.shields.io/badge/License-MIT-yellow.svg" alt="License: MIT"></a>
  <a href="https://github.com/runkids/skillshare/releases"><img src="https://img.shields.io/github/v/release/runkids/skillshare" alt="Release"></a>
  <img src="https://img.shields.io/badge/platform-macOS%20%7C%20Linux%20%7C%20Windows-blue" alt="Platform">
  <a href="https://goreportcard.com/report/github.com/runkids/skillshare"><img src="https://goreportcard.com/badge/github.com/runkids/skillshare" alt="Go Report Card"></a>
  <a href="https://deepwiki.com/runkids/skillshare"><img src="https://deepwiki.com/badge.svg" alt="Ask DeepWiki"></a>
</p>

<p align="center">
  <a href="https://github.com/runkids/skillshare/stargazers"><img src="https://img.shields.io/github/stars/runkids/skillshare?style=social" alt="Star on GitHub"></a>
</p>

<p align="center">
  <a href="https://trendshift.io/repositories/21835" target="_blank"><img src="https://trendshift.io/api/badge/repositories/21835" alt="runkids%2Fskillshare | Trendshift" style="width: 250px; height: 55px;" width="250" height="55"/></a>
</p>

<p align="center">
  <strong>AI CLI 的 skills、agents、rules、commands 等資源，只需要一份來源。一個指令同步到所有工具，個人到整個組織都適用。</strong><br>
  支援 Codex、Claude Code、OpenClaw、OpenCode 等 60 多種工具。
</p>

<p align="center">
  <img src=".github/assets/demo.gif" alt="skillshare demo" width="960">
</p>

<p align="center">
  <a href="https://skillshare.runkids.cc">官方網站</a> •
  <a href="#安裝">安裝</a> •
  <a href="#快速開始">快速開始</a> •
  <a href="#功能重點">功能重點</a> •
  <a href="#cli-與介面預覽">畫面截圖</a> •
  <a href="https://github.com/runkids/skillshare-app">桌面 App</a> •
  <a href="https://skillshare.runkids.cc/docs">文件</a>
</p>

> [!NOTE]
> **最新版本**：v0.21.* — 每一版的新功能與修正都列在 [Releases](https://github.com/runkids/skillshare/releases) 和[更新紀錄](https://skillshare.runkids.cc/changelog)。近期重點是用 `projects:` 從一份 global 設定管理**多個專案資料夾**、用**專案 lockfile** 讓每位隊友裝到相同的 commit、**MCP 連線**只定義一次就同步進每個 Agent 自己的設定格式，以及把**完整 plugin** 安裝與同步到 Claude、Codex、Cursor 等工具。

## 為什麼用 skillshare

每個 AI CLI 都有自己的 skills 目錄。
你在其中一個改了內容，忘了複製到另一個，最後搞不清楚哪份才是最新的。

skillshare 解決這個問題：

- **一份來源，所有 agent** — 用 `skillshare sync` 同步到 Claude、Cursor、Codex 等 60 多種工具
- **Agent 管理** — 自訂 agent 和 skills 一起同步到支援 agent 的 target
- **不只是 skills** — 用 [extras](https://skillshare.runkids.cc/docs/reference/targets/configuration#extras) 管理 rules、commands、prompts 和任何以檔案為主的資源
- **MCP 連線** — 伺服器只定義一次，用 [`sync mcp`](https://skillshare.runkids.cc/docs/how-to/daily-tasks/sharing-mcp) 寫進每個 Agent 自己的設定格式
- **完整 plugin** — plugin 的 skills、hooks 和 MCP 設定保持完整，用 [`plugin`](https://skillshare.runkids.cc/docs/how-to/daily-tasks/sharing-plugins) 選擇哪些工具要安裝
- **原生 hooks** — 管理各 Agent 的 event 設定或 extension 程式碼，支援預覽、啟用／停用與復原。 [hooks](https://skillshare.runkids.cc/docs/reference/commands/hooks)
- **從任何地方安裝** — GitHub、GitLab、Bitbucket、Azure DevOps、Gitea、CNB，或任何自架的 Git
- **內建安全檢查** — 使用前先稽核 skills 是否含有 prompt injection 或資料外洩的內容
- **適合團隊** — 專案的 skills 放在 `.skillshare/`，組織共用的 skills 透過 tracked repo 發布
- **本機、輕量** — 單一執行檔，沒有 registry，沒有遙測，可完全離線使用
- **細緻的篩選** — 用 [`.skillignore`](https://skillshare.runkids.cc/docs/how-to/daily-tasks/filtering-skills)、SKILL.md 的 `targets`，以及各 target 的 include/exclude，控制哪些 skills 到哪些 target

> 從其他工具換過來？ [遷移指南](https://skillshare.runkids.cc/docs/how-to/advanced/migration) · [比較](https://skillshare.runkids.cc/docs/understand/philosophy/comparison)

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

## CLI 與介面預覽

| Skill 詳細資訊 | 安全稽核 |
|---|---|
| <img src=".github/assets/skill-detail-tui.png" alt="Skill 詳細資訊" width="480" height="300"> | <img src=".github/assets/audit-tui.png" alt="安全稽核" width="480" height="300"> |

| 網頁儀表板 | 網頁 Skills 頁面 |
|---|---|
| <img src=".github/assets/ui/web-dashboard-demo.png" alt="網頁儀表板" width="480"> | <img src=".github/assets/ui/web-skills-demo.png" alt="網頁 Skills 頁面" width="480"> |

## 安裝

### macOS / Linux

```bash
curl -fsSL https://raw.githubusercontent.com/runkids/skillshare/main/install.sh | sh
```

### Windows PowerShell

```powershell
irm https://raw.githubusercontent.com/runkids/skillshare/main/install.ps1 | iex
```

### Homebrew

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

**MCP 連線** — 設定一次，Claude Code、Codex、Cursor、VS Code、OpenCode 等工具都能用

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
