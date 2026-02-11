---
sidebar_position: 1
---

# push

Commit and push source to git remote.

```bash
skillshare push                  # Auto-generated message
skillshare push -m "Add pdf"     # Custom message
skillshare push --dry-run        # Preview
```

## What Happens

```
┌─────────────────────────────────────────────────────────────────┐
│ skillshare push -m "Add pdf skill"                              │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│ 1. Check repository status                                      │
│    → Source is a git repo: ✓                                    │
│    → Remote configured: ✓                                       │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│ 2. Stage all changes                                            │
│    → git add -A                                                 │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│ 3. Commit                                                       │
│    → git commit -m "Add pdf skill"                              │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│ 4. Push to remote                                               │
│    → git push                                                   │
└─────────────────────────────────────────────────────────────────┘
```

## Options

| Flag | Description |
|------|-------------|
| `-m, --message <msg>` | Commit message (default: "Update skills") |
| `--dry-run, -n` | Preview without making changes |

## Prerequisites

Your source directory must be a git repository with a remote:

```bash
# Set up during init (recommended):
skillshare init --remote git@github.com:you/my-skills.git

# Or add remote to existing setup:
skillshare init --remote git@github.com:you/my-skills.git
```

Init automatically creates the initial commit, so `push` works immediately after setup.

## Examples

```bash
# Quick push with auto message
skillshare push

# Custom commit message
skillshare push -m "Add commit-commands skill"

# Preview what would be pushed
skillshare push --dry-run
```

## Conflict Handling

If the remote has newer commits:

```bash
$ skillshare push
Push failed
  Remote may have newer changes
  Run: skillshare pull
  Then: skillshare push
```

Solution:
```bash
skillshare pull    # Get remote changes
skillshare push    # Push your changes
```

## Workflow

Typical workflow for sharing skills:

```bash
# 1. Make changes to skills
# 2. Push to remote
skillshare push -m "Update my-skill"

# On another machine:
skillshare pull    # Gets changes and syncs
```

## Related

- [pull](/docs/commands/pull) — Pull from remote
- [sync](/docs/commands/sync) — Sync to local targets
- [Cross-Machine Sync](/docs/guides/cross-machine-sync) — Full setup
