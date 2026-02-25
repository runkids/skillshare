---
sidebar_position: 1
---

# Appendix

Technical reference appendix for skillshare.

## What are you looking for?

| Topic | Read |
|-------|------|
| Environment variables that affect skillshare | [Environment Variables](/docs/reference/appendix/environment-variables) |
| Where skillshare stores config, skills, logs, cache | [File Structure](/docs/reference/appendix/file-structure) |
| Supported git URL formats | [URL Formats](/docs/reference/appendix/url-formats) |
| Config file format and options | [Configuration](/docs/reference/targets/configuration) |
| All CLI commands | [Commands](/docs/reference/commands) |

## Quick Reference

### Key Paths (Unix)

| Path | Purpose |
|------|---------|
| `~/.config/skillshare/config.yaml` | Configuration file |
| `~/.config/skillshare/registry.yaml` | Installed skill registry (auto-managed) |
| `~/.config/skillshare/skills/` | Source directory (your skills) |
| `~/.local/share/skillshare/backups/` | Backup directory |
| `~/.local/share/skillshare/trash/` | Soft-deleted skills |
| `~/.local/state/skillshare/logs/` | Operation and audit logs |
| `~/.cache/skillshare/ui/` | Downloaded web dashboard |

### Environment Variables

| Variable | Purpose |
|----------|---------|
| `SKILLSHARE_CONFIG` | Override config path |
| `GITHUB_TOKEN` | GitHub API authentication |

## See Also

- [Configuration](/docs/reference/targets/configuration) — Config file details
- [Commands](/docs/reference/commands) — All commands
