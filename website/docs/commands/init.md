---
sidebar_position: 1
---

# init

First-time setup. Auto-detects installed AI CLIs and configures targets.

```bash
skillshare init              # Interactive setup
skillshare init --dry-run    # Preview without changes
```

## What Happens

```
┌─────────────────────────────────────────────────────────────────┐
│ skillshare init                                                 │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│ 1. Create source directory                                      │
│    → ~/.config/skillshare/skills/                               │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│ 2. Auto-detect AI CLIs                                          │
│    → Found: claude, cursor, codex                               │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│ 3. Initialize git (optional)                                    │
│    → Ready for cross-machine sync                               │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│ 4. Create config.yaml                                           │
│    → ~/.config/skillshare/config.yaml                           │
└─────────────────────────────────────────────────────────────────┘
```

## Options

| Flag | Description |
|------|-------------|
| `--source <path>` | Custom source directory |
| `--remote <url>` | Set git remote (implies git init) |
| `--dry-run` | Preview without changes |

## Common Scenarios

```bash
# Standard setup (auto-detect everything)
skillshare init

# Setup with git remote for cross-machine sync
skillshare init --remote git@github.com:you/my-skills.git

# Use existing skills directory
skillshare init --source ~/.config/skillshare/skills
```
