---
sidebar_position: 4
---

# 在多個 AI 工具中使用 skillshare

> 一個 source of truth，同步到你使用的每一個 AI CLI。

## 問題

你在工作中使用 Claude Code，side project 用 Cursor，實驗性專案用 Codex。每一個都有自己的 skill 目錄。手動讓它們保持同步既繁瑣又容易出錯。

## 解決方案

skillshare 維護單一的 source 目錄，並用一個指令將它 sync 到你所有的 targets。

## 步驟 1：安裝並初始化

```bash
curl -fsSL https://raw.githubusercontent.com/runkids/skillshare/main/install.sh | sh
```

只有安裝器顯示 PATH 設定提示時，才需要依提示設定後再執行下方指令。沒有 PATH 警告就不需要額外設定。

```bash
skillshare init
```

`init` 會自動偵測所有已安裝的 AI 工具，並將它們加入為 targets。

## 步驟 2：檢查你的 Targets

```bash
skillshare target list
```

輸出範例：

```
  claude       ~/.claude/skills (merge)
  cursor       ~/.cursor/skills (merge)
  opencode     ~/.config/opencode/skills (merge)
  universal    ~/.agents/skills (merge)
```

Codex 不會有自己的一行：它讀取的是共用的 `~/.agents/skills` 目錄，所以由 `universal` target 涵蓋。

## 步驟 3：安裝 Skills

```bash
skillshare install runkids/my-skills
skillshare install anthropics/courses/prompt-eng
```

## 步驟 4：Sync 所有內容

```bash
skillshare sync
```

一個指令就能將所有 skills 推送到每一個 target。每個 target 都會取得指回你單一 source 的 symlinks。

## 步驟 5：驗證

```bash
skillshare status
```

顯示所有 targets 的 sync 狀態 — 哪些 skills 已同步、遺失，或已過時。

## 每個 Target 個別的 Mode 控制

不同工具有不同的需求。你可以針對每個 target 分別設定 sync mode：

```bash
# Cursor 能正常跟隨 symlinks（預設）
skillshare target cursor --mode merge

# 有些工具需要真實檔案
skillshare target opencode --mode copy
```

## 接下來呢？

- [了解 sync modes →](/docs/understand/sync-modes)
- [跨機器同步 →](/docs/how-to/sharing/cross-machine-sync)
- [團隊分享 →](/docs/how-to/sharing/organization-sharing)
