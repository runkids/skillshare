---
sidebar_position: 1
---

# Appendix

skillshare 的技术参考附录。

## What are you looking for?

| Topic | Read |
|-------|------|
| 影响 skillshare 的环境变量 | [Environment Variables](/docs/reference/appendix/environment-variables) |
| skillshare 存放 config、skills、logs、cache 的位置 | [File Structure](/docs/reference/appendix/file-structure) |
| 支持的 git URL 格式 | [URL Formats](/docs/reference/appendix/url-formats) |
| 配置文件格式与选项 | [Configuration](/docs/reference/targets/configuration) |
| 所有 CLI 命令 | [Commands](/docs/reference/commands) |

## Quick Reference

### Key Paths (Unix)

| Path | Purpose |
|------|---------|
| `~/.config/skillshare/config.yaml` | 配置文件 |
| `~/.config/skillshare/skills/` | Source 目录（你的 skills） |
| `~/.config/skillshare/skills/.metadata.json` | 已安装 skill 的元数据（自动管理） |
| `~/.local/share/skillshare/backups/` | Backup 目录 |
| `~/.local/share/skillshare/trash/` | 被软删除的 skills |
| `~/.local/state/skillshare/logs/` | 操作与审计日志 |
| `~/.local/state/skillshare/projects/` | 本机项目注册及锁 |
| `~/.cache/skillshare/ui/` | 下载的 Web dashboard |

### Environment Variables

| Variable | Purpose |
|----------|---------|
| `SKILLSHARE_CONFIG` | 覆盖配置文件路径 |
| `GITHUB_TOKEN` | GitHub API 认证 |

## See Also

- [Configuration](/docs/reference/targets/configuration) — 配置文件详情
- [Commands](/docs/reference/commands) — 所有命令
