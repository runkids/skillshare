---
sidebar_position: 6
---

# Recipe：團隊導入

> 讓新成員取得團隊共享的 skills 與專案情境。

## 情境

一位新開發者加入你的團隊。他們需要：
- 組織範圍的 skills（程式碼規範、審查準則）
- Project 專屬的 skills（領域知識、架構規則）
- 在他們所有的 AI 工具（Claude Code、Pi 等）中都能正常運作

一位同事用 Claude Code，另一位用 Codex，新成員則用 Pi。三人都需要一份說明哪些舊版 API 應避免使用的審查清單。把清單複製到各個工具，會產生多個版本，隨著專案變動逐漸分歧。

將專案的本機 skills、`.skillshare/config.yaml` 與 `.skillshare/skills.lock.json` 放在專案 repository。Config 宣告遠端 skills 與 targets；lockfile 記錄遠端 skill 的 commit。每位成員在本機套用這些檔案。共享指令提供共同情境，各工具仍保有自己的權限與行為。

## 解決方案

### 步驟 1：建立導入腳本

儲存為 `scripts/setup-skills.sh`，放在你的團隊 wiki 或 repo 中：

```bash
#!/bin/bash
set -e

echo "正在安裝 skillshare..."
curl -fsSL https://raw.githubusercontent.com/runkids/skillshare/main/install.sh | sh
export PATH="$HOME/.local/bin:$PATH"

echo "正在初始化..."
skillshare init -g

echo "正在安裝組織 skills..."
skillshare install github.com/your-org/org-skills --track -g

echo "正在執行安全 audit..."
skillshare audit -g --threshold high

echo "正在同步到所有 AI 工具..."
skillshare sync -g

echo "完成！執行 'skillshare list -g' 以查看已安裝的 skills。"
```

### 步驟 2：新成員執行腳本

```bash
curl -fsSL https://your-org.github.io/setup-skills.sh | sh
```

或者若腳本存放在團隊 repo 中：

```bash
git clone your-org/team-tools
./team-tools/scripts/setup-skills.sh
```

### 步驟 3：Project 專屬設定

維護者先依[專案設定](/docs/how-to/sharing/project-setup)完成設定，並提交專案 config、本機 skills 與 lockfile。新成員 clone 該專案後：

```bash
cd your-project
skillshare install -p
skillshare audit -p --threshold high
skillshare sync -p
```

`install -p` 安裝 `.skillshare/config.yaml` 宣告的遠端 skills；若有鎖定的 commit，就使用該版本。`audit -p` 檢查專案 skills，包含已提交的本機 skills。`sync -p` 將它們分發到設定的 targets。請依序執行；任何一步失敗就停止。被 audit 封鎖的內容需要先審查，再同步。

Pull 專案更新後，重複這個流程。Git 傳輸 config 與 lockfile，不會自行安裝缺少的遠端 skills 或更新 target 副本。刻意更新 skill 時，透過 PR 審查，並提交產生的 lockfile 變更。

### 步驟 4：驗證一切正常運作

```bash
# 檢查 global skills
skillshare list -g

# 檢查 project skills
skillshare list -p

# 檢查 sync 狀態
skillshare status -p
```

## 驗證

- `skillshare list -g` 顯示組織 skills
- `skillshare list -p` 顯示專案的本機與已安裝的遠端 skills
- `skillshare status -p` 顯示設定的 project targets 已同步
- 開啟設定的 AI 工具，確認預期的 skills 已載入；用已知的舊版 API 範例試跑審查清單

## 變化

- **Dev container 導入**：若你的團隊使用 dev containers，將 skillshare 加入 `.devcontainer/Dockerfile` 與 `postCreateCommand` — 容器啟動時 skills 就已就緒
- **基於 Homebrew 的安裝**：對於 macOS/Linux 團隊，將 `curl | sh` 替換為 `brew install skillshare`
- **Hub 探索**：引導新成員到你的 hub：`skillshare search --hub https://your-org.github.io/skillshare-hub.json`

## 相關

- [入門指南](/docs/getting-started)
- [組織分享](/docs/how-to/sharing/organization-sharing)
- [Project 設定](/docs/how-to/sharing/project-setup)
- [Dev container 指南](/docs/learn/with-devcontainer)
