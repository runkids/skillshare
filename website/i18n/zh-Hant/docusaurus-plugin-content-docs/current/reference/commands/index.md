---
sidebar_position: 1
---

# Commands

skillshare 所有指令的完整參考。

## 你想做什麼？

| 我想要... | 指令 |
|--------------|---------|
| 第一次設定 skillshare | [`init`](./init.md) |
| 從 GitHub 安裝一個 skill | [`install`](./install.md) |
| 建立自己的 skill | [`new`](./new.md) |
| 把 skills 同步到所有 AI CLI | [`sync`](./sync.md) |
| 檢查哪裡不同步 | [`status`](./status.md) / [`diff`](./diff.md) |
| 搜尋社群 skills | [`search`](./search.md) |
| 更新已安裝的 skills | [`check`](./check.md) 然後 [`update`](./update.md) |
| 暫時隱藏某個 skill 而不移除它 | [`enable` / `disable`](./enable.md) |
| 從在別處編輯的連結 repository 找到 skill | [`follow`](./follow.md) / [`unfollow`](./unfollow.md) |
| 用 git 儲存或同步變更 | [`commit`](./commit.md) / [`push`](./push.md) / [`pull`](./pull.md) |
| 為每個工具一次設定好 MCP server | [`mcp`](./mcp.md) |
| 在支援的工具間管理完整的 plugins | [`plugin`](./plugin.md) |
| 管理非 skill 資源（rules、commands） | [`extras`](./extras.md) |
| 管理單一檔案的 `.md` agents | 大多數指令都接受 `agents` 或 `--kind agent` — 見 [Agents](/docs/understand/agents) |
| 查看哪些 skills 佔用最多 context tokens | [`analyze`](./analyze.md) |
| 修復壞掉的東西 | [`doctor`](./doctor.md) |
| 在我的 shell 啟用 tab 自動完成 | [`completion`](./completion.md) |
| 開啟網頁儀表板 | [`ui`](./ui.md) |

---

## 總覽

| 分類 | 指令 |
|----------|----------|
| **核心** | `init`, `install`, `uninstall`, `list`, `search`, `sync`, `status` |
| **Skill 管理** | `new`, `check`, `update`, `upgrade`, `enable`, `disable`, `follow`, `unfollow` |
| **MCP 連線** | `mcp` (`add`, `edit`, `import`, `list`, `remove`, `restore`), `sync mcp` |
| **Plugin 管理** | `plugin` (`list`, `discover`, `add`, `import`, `inspect`, `sync`, `check`, `update`, `enable`, `disable`, `remove`) |
| **Target 管理** | `target`, `diff` |
| **Extras 管理** | `extras` (`init`, `list`, `remove`, `collect`) |
| **同步操作** | `collect`, `backup`, `restore`, `trash`, `commit`, `push`, `pull` |
| **安全與工具** | `analyze`, `audit`, `hub`, `log`, `doctor`, `tui`, `ui`, `completion`, `version` |

---

## 核心指令

| 指令 | 說明 |
|---------|-------------|
| [init](./init.md) | 第一次設定 |
| [install](./install.md) | 從 repo 或路徑新增一個 skill |
| [uninstall](./uninstall.md) | 移除一個 skill |
| [list](./list.md) | 列出所有 skills |
| [search](./search.md) | 搜尋 skills |
| [sync](./sync.md) | 把 skills 推送到所有 targets |
| [status](./status.md) | 顯示同步狀態 |

## Skill 管理

| 指令 | 說明 |
|---------|-------------|
| [new](./new.md) | 建立新的 skill |
| [check](./check.md) | 檢查是否有可用更新 |
| [update](./update.md) | 更新一個 skill 或 tracked repo |
| [upgrade](./upgrade.md) | 升級 CLI 或內建 skill |
| [enable / disable](./enable.md) | 暫時啟用或停用 skills |
| [follow](./follow.md) | 跟隨 skills source 中的第一層連結 |
| [unfollow](./unfollow.md) | 停止跟隨第一層連結 |

## Target 管理

| 指令 | 說明 |
|---------|-------------|
| [target](./target.md) | 管理 targets |
| [diff](./diff.md) | 顯示 source 與 targets 之間的差異 |

## Extras 管理

| 指令 | 說明 |
|---------|-------------|
| [extras](./extras.md) | 管理非 skill 資源（rules、commands、prompts） |

## MCP 與 Plugins

| 指令 | 說明 |
|---------|-------------|
| [mcp](./mcp.md) | 定義一次 MCP servers 並同步到每個工具的原生設定中 |
| [hooks](./hooks.md) | 管理原生 hooks |
| [plugin](./plugin.md) | 安裝完整的 plugins 並選擇哪些工具接收它們 |

## 同步操作

| 指令 | 說明 |
|---------|-------------|
| [collect](./collect.md) | 從 target 收集 skills 回 source |
| [backup](./backup.md) | 建立 targets 的備份 |
| [restore](./restore.md) | 從備份還原 targets |
| [trash](./trash.md) | 管理已移除的 skills（trash） |
| [commit](./commit.md) | 建立本地 git commit 而不推送 |
| [push](./push.md) | Commit 並推送到 git remote |
| [pull](./pull.md) | 從 git remote 拉取並同步 |

## 安全與工具

| 指令 | 說明 |
|---------|-------------|
| [analyze](./analyze.md) | 分析 context window 用量 |
| [audit](./audit.md) | 掃描 skills 是否有安全威脅 |
| [log](./log.md) | 查看操作與稽核日誌 |
| [doctor](./doctor.md) | 診斷問題 |
| [tui](./tui.md) | 切換互動式 TUI 模式 |
| [ui](./ui.md) | 啟動網頁儀表板 |
| [hub](./hub.md) | 管理 skill hub 來源 |
| [completion](./completion.md) | 產生 shell 自動完成腳本 |
| [version](./version.md) | 顯示 CLI 版本 |

---

## 常用旗標

大多數指令都支援：

| 旗標 | 說明 |
|------|-------------|
| `--dry-run`, `-n` | 預覽而不實際變更 |
| `--help`, `-h` | 顯示說明 |

---

## 快速參考

```bash
# 設定
skillshare init
skillshare init --remote git@github.com:you/skills.git

# 安裝 skills
skillshare install anthropics/skills/skills/pdf
skillshare install github.com/team/skills --track

# 建立 skill
skillshare new my-skill

# 同步
skillshare sync
skillshare sync --dry-run

# Git 檢查點／跨機器
skillshare commit -m "Update skill"
skillshare push -m "Add skill"
skillshare pull

# 狀態
skillshare status
skillshare list
skillshare diff

# 啟用／停用 skills
skillshare disable draft-*
skillshare enable draft-*

# 維護
skillshare update --all
skillshare analyze
skillshare audit
skillshare log
skillshare doctor
skillshare backup

# TUI 偏好設定
skillshare tui            # 顯示目前狀態
skillshare tui off        # 停用互動式 TUI
skillshare tui on         # 重新啟用 TUI

# 網頁 UI
skillshare ui
skillshare ui -p          # Project mode

# Hub
skillshare hub list
skillshare hub add https://hub.example.com/index.json

# 檢查更新
skillshare check

# Trash 管理
skillshare trash list
skillshare trash restore my-skill

# Shell 自動完成
skillshare completion bash --install
skillshare completion zsh --install

# 版本
skillshare version
```

---

## 相關資源

- [Quick Reference](/docs/getting-started/quick-reference) — 指令速查表
- [Workflows](/docs/how-to/daily-tasks) — 常見使用模式
