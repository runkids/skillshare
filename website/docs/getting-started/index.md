---
sidebar_position: 1
---

# Getting Started

Get skillshare running in minutes. Choose your starting point:

## What's your situation?

| I want to... | Start here |
|--------------|-----------|
| Set up skillshare from scratch | [First Sync](./first-sync.md) — install, init, and sync in 5 minutes |
| I already have skills in Claude/Cursor/etc. | [From Existing Skills](./from-existing-skills.md) — consolidate and unify |
| I know skillshare, just need command syntax | [Quick Reference](./quick-reference.md) — cheat sheet |
| Try without installing anything | [Docker Playground](/docs/how-to/advanced/docker-sandbox#playground) — `make playground` in cloned repo |

## The 3-Step Pattern

No matter which path you choose, skillshare follows a simple pattern:

```bash
# 1. Install skillshare
curl -fsSL https://raw.githubusercontent.com/runkids/skillshare/main/install.sh | sh

# 2. Initialize (auto-detects your AI CLIs)
skillshare init

# 3. Sync skills to all targets
skillshare sync
```

After setup, behavior depends on mode:
- `merge`/`symlink`: source edits reflect immediately
- `copy`: changes apply on next `sync`

## What's Next?

After you're set up:

- [Core Concepts](/docs/understand) — Understand source, targets, and sync modes
- [Daily Workflow](/docs/how-to/daily-tasks/daily-workflow) — How to use skillshare day-to-day
- [Commands Reference](/docs/reference/commands) — Full command reference
