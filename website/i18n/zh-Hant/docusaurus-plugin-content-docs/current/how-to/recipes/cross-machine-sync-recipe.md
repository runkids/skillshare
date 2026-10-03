---
sidebar_position: 5
---

# Recipe：跨機器同步

> 使用 git push/pull 在多台機器間保持 skills 同步。

## 情境

你同時使用桌上型電腦與筆電（或是家中與辦公室的機器）。你希望在每台機器上都能使用相同的 skill 庫，而不需要在每台機器上重新執行安裝指令。

## 解決方案

### 初始設定（機器 A）

```bash
# 初始化 skillshare
skillshare init

# 安裝你的 skills
skillshare install your-org/team-skills
skillshare install another/repo --into tools

# 將 source 推送到 git remote
skillshare push
```

`skillshare push` 會將你的 source 目錄 commit 到受 git 追蹤的 branch，並推送到已設定的 remote。

### 在新機器上設定（機器 B）

```bash
# 安裝 skillshare
curl -fsSL https://raw.githubusercontent.com/runkids/skillshare/main/install.sh | sh
```

只有安裝器顯示 PATH 設定提示時，才需要依提示設定後再執行下方指令。沒有 PATH 警告就不需要額外設定。

```bash

# 初始化
skillshare init

# 從 remote pull
skillshare pull

# 同步到本機 targets
skillshare sync
```

### 每日同步工作流程

在任一台機器上：

```bash
# 從其他機器 pull 最新變更
skillshare pull

# 同步到本機 AI 工具
skillshare sync

# 在本機做出變更之後
skillshare push

# 或一步完成雙向同步：合併、push，然後 sync
skillshare push --pull
```

## 驗證

- `skillshare push` 以 0 結束並回報已 commit 的變更
- 在另一台機器上執行 `skillshare pull` 會顯示收到的變更
- `skillshare list` 在兩台機器上顯示相同的 skills
- `skillshare sync` 會在該 target 機器上建立 symlinks

## 變化

- **登入時自動同步**：在你的 shell profile（`.bashrc` / `.zshrc`）加入 `skillshare pull && skillshare sync`
- **衝突解決**：`pull` 會合併兩台機器的 commits，並自動解決 `.metadata.json` 的衝突。若兩台機器都編輯了同一個 skill 檔案，`pull` 會停止、復原 merge 並列出該檔案 — 請在 source 目錄中用 git 解決
- **選擇性同步**：在 `config.yaml` 中使用各 target 的 `include` / `exclude` 篩選器，控制哪些 skills 同步到哪台機器
- **Plugins、MCP 與 hooks**：這些都存在 `config.yaml` 裡，`push` / `pull` 永遠不會帶走。請參考 [Plugins、MCP 與 Hooks](/docs/how-to/sharing/cross-machine-sync#plugins-mcp-hooks)

## 相關

- [跨機器同步指南](/docs/how-to/sharing/cross-machine-sync)
- [`push` 指令參考](/docs/reference/commands/push)
- [`pull` 指令參考](/docs/reference/commands/pull)
