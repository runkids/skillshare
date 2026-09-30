---
sidebar_position: 6
---

# Recipe：團隊導入

> 在 5 分鐘內設定好新團隊成員的 AI skill 環境。

## 情境

一位新開發者加入你的團隊。他們需要：
- 組織範圍的 skills（程式碼規範、審查準則）
- Project 專屬的 skills（領域知識、架構規則）
- 在他們所有的 AI 工具（Claude Code、Cursor 等）中都能正常運作

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
skillshare init

echo "正在安裝組織 skills..."
skillshare install your-org/org-skills

echo "正在執行安全 audit..."
skillshare audit

echo "正在同步到所有 AI 工具..."
skillshare sync

echo "完成！執行 'skillshare list' 以查看已安裝的 skills。"
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

當新成員 clone 一個 project 時：

```bash
cd your-project
skillshare sync -p
```

這會自動取得 project 範圍的 skills。

### 步驟 4：驗證一切正常運作

```bash
# 檢查 global skills
skillshare list

# 檢查 project skills
skillshare list -p

# 檢查 sync 狀態
skillshare status
```

## 驗證

- `skillshare list` 顯示組織 skills
- `skillshare status` 顯示所有 targets 皆已同步
- 開啟 Claude Code / Cursor 可看到 skills 已載入

## 變化

- **Dev container 導入**：若你的團隊使用 dev containers，將 skillshare 加入 `.devcontainer/Dockerfile` 與 `postCreateCommand` — 容器啟動時 skills 就已就緒
- **基於 Homebrew 的安裝**：對於 macOS/Linux 團隊，將 `curl | sh` 替換為 `brew install skillshare`
- **Hub 探索**：引導新成員到你的 hub：`skillshare search --hub https://your-org.github.io/skillshare-hub.json`

## 相關

- [入門指南](/docs/getting-started)
- [組織分享](/docs/how-to/sharing/organization-sharing)
- [Project 設定](/docs/how-to/sharing/project-setup)
- [Dev container 指南](/docs/learn/with-devcontainer)
