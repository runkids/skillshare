---
sidebar_position: 3
---

# 在 Codex 中使用 skillshare

> 從安裝到第一次 sync — 只要 5 分鐘。

## 先決條件

- 已安裝並可正常運作的 [OpenAI Codex CLI](https://github.com/openai/codex)
- macOS、Linux 或 Windows

## 步驟 1：安裝 skillshare

```bash
curl -fsSL https://raw.githubusercontent.com/runkids/skillshare/main/install.sh | sh
```

只有安裝器顯示 PATH 設定提示時，才需要依提示設定後再執行下方指令。沒有 PATH 警告就不需要額外設定。

## 步驟 2：初始化

```bash
skillshare init
```

Codex 會讀取 `~/.agents/skills/`，也就是它文件中記載的 user 層級 skill 共用目錄。`init` 會透過 Codex 的設定目錄（`~/.codex/`）偵測到它，並自動為它設定共用的 `universal` target。

## 步驟 3：安裝你的第一個 Skill

```bash
skillshare install runkids/my-skills
```

## 步驟 4：Sync

```bash
skillshare sync
```

Skills 會被 symlink 到 `~/.agents/skills/`。

## 步驟 5：驗證

```bash
ls ~/.agents/skills/
```

你應該會看到已安裝的 skill 以 symlink 的形式出現。

## Codex 專屬注意事項

- **Skill 路徑**：`~/.agents/skills/`（global）或 `.agents/skills/`（project）
- **既有設定**：如果你的設定中 `codex` 仍指向 `~/.codex/skills`，Codex 依然會讀取它。但若同時啟用了 `universal`，每個 skill 都會出現兩次——請移除 `codex` target（可先用 `skillshare target remove codex --dry-run` 預覽）
- **描述長度限制**：Codex 對 skill 描述有 1024 字元的限制。請將 `SKILL.md` frontmatter 中的 `description` 欄位維持簡潔
- **Project mode**：執行 `skillshare init -p` 以管理 project 層級的 Codex skills

## 接下來呢？

- [管理多個 skills →](/docs/how-to/daily-tasks/organizing-skills)
- [與團隊分享 →](/docs/how-to/sharing/organization-sharing)
- [探索更多 skills →](/docs/reference/commands/search)
