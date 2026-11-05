---
sidebar_position: 1
---

# 附錄

skillshare 的技術參考附錄。

## 你在找什麼？

| Topic | Read |
|-------|------|
| 影響 skillshare 的環境變數 | [Environment Variables](/docs/reference/appendix/environment-variables) |
| skillshare 儲存 config、skills、logs、cache 的位置 | [File Structure](/docs/reference/appendix/file-structure) |
| 支援的 git URL 格式 | [URL Formats](/docs/reference/appendix/url-formats) |
| 設定檔格式與選項 | [Configuration](/docs/reference/targets/configuration) |
| 所有 CLI 指令 | [Commands](/docs/reference/commands) |

## 快速參考

### 主要路徑（Unix）

| Path | Purpose |
|------|---------|
| `~/.config/skillshare/config.yaml` | 設定檔 |
| `~/.config/skillshare/skills/` | Source 目錄（你的 skills） |
| `~/.config/skillshare/skills/.metadata.json` | 已安裝 skill 的 metadata（自動管理） |
| `~/.local/share/skillshare/backups/` | 備份目錄 |
| `~/.local/share/skillshare/trash/` | 軟刪除的 skills |
| `~/.local/state/skillshare/logs/` | 操作與 audit 記錄 |
| `~/.local/state/skillshare/projects/` | 本機專案註冊及鎖定 |
| `~/.cache/skillshare/ui/` | 已下載的 web dashboard |

### 環境變數

| Variable | Purpose |
|----------|---------|
| `SKILLSHARE_CONFIG` | 覆寫設定檔路徑 |
| `GITHUB_TOKEN` | GitHub API 驗證 |

## 另請參閱

- [Configuration](/docs/reference/targets/configuration) — 設定檔細節
- [Commands](/docs/reference/commands) — 所有指令
