---
sidebar_position: 1
---

# 在 Claude Code 中使用 skillshare

> 從安裝到第一次 sync — 只要 5 分鐘。

## 先決條件

- 已安裝並可正常運作的 [Claude Code](https://docs.anthropic.com/en/docs/claude-code/overview)
- macOS、Linux 或 Windows（WSL）

## 步驟 1：安裝 skillshare

```bash
curl -fsSL https://raw.githubusercontent.com/runkids/skillshare/main/install.sh | sh
```

只有安裝器顯示 PATH 設定提示時，才需要依提示設定後再執行下方指令。沒有 PATH 警告就不需要額外設定。

## 步驟 2：初始化

```bash
skillshare init
```

這會偵測 Claude Code 的 skill 目錄（`~/.claude/skills/`），並自動將它加入為 target。

## 步驟 3：安裝你的第一個 Skill

```bash
skillshare install anthropics/courses/prompt-eng
```

該 skill 會被下載、進行安全性稽核，並加入到你的 source 目錄中。

## 步驟 4：Sync

```bash
skillshare sync
```

這會從你的 source 建立 symlinks 到 `~/.claude/skills/`。Claude Code 會立即讀取這些 skills — 不需要重新啟動。

## 步驟 5：驗證

```bash
ls ~/.claude/skills/
```

你應該會看到已安裝的 skill 以 symlink 的形式出現。

## Claude Code 整合細節

- **Skill 路徑**：`~/.claude/skills/`（global）或 `.claude/skills/`（project）
- **CLAUDE.md**：skillshare 的 skills 使用 `SKILL.md` 格式，Claude Code 可以原生讀取
- **Project mode**：在某個 repo 中執行 `skillshare init -p` 以管理該專案的 `.claude/skills/`

## 接下來呢？

- [管理多個 skills →](/docs/how-to/daily-tasks/organizing-skills)
- [與團隊分享 →](/docs/how-to/sharing/organization-sharing)
- [探索更多 skills →](/docs/reference/commands/search)
