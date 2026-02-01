---
sidebar_position: 3
---

# Cross-Machine Sync

Sync your skills across multiple computers using git.

## Overview

```
┌──────────────────────────────────────────────────────────────┐
│                    CROSS-MACHINE SYNC                        │
│                                                              │
│  ┌─────────────────┐                    ┌─────────────────┐  │
│  │   Machine A     │                    │   Machine B     │  │
│  │   (Work)        │                    │   (Home)        │  │
│  │                 │                    │                 │  │
│  │  ┌───────────┐  │                    │  ┌───────────┐  │  │
│  │  │  Claude   │  │                    │  │  Claude   │  │  │
│  │  │  Cursor   │  │                    │  │  Codex    │  │  │
│  │  └─────┬─────┘  │                    │  └─────┬─────┘  │  │
│  │        │        │                    │        │        │  │
│  │        ▼        │                    │        ▼        │  │
│  │  ┌───────────┐  │  push      pull    │  ┌───────────┐  │  │
│  │  │  Source   │──┼────────►───────────┼──│  Source   │  │  │
│  │  │  (git)    │  │   ┌──────────┐     │  │  (git)    │  │  │
│  │  └───────────┘  │   │  GitHub  │     │  └───────────┘  │  │
│  │                 │   │  Remote  │     │                 │  │
│  └─────────────────┘   └──────────┘     └─────────────────┘  │
│                                                              │
└──────────────────────────────────────────────────────────────┘
```

---

## Setup

### Option 1: New Setup with Remote

```bash
skillshare init --remote git@github.com:you/my-skills.git
```

This:
1. Creates source directory
2. Initializes git
3. Adds remote
4. Auto-detects and configures targets

### Option 2: Add Remote to Existing

```bash
cd ~/.config/skillshare/skills
git init
git remote add origin git@github.com:you/my-skills.git
git add .
git commit -m "Initial commit"
git push -u origin main
```

---

## Daily Workflow

### Machine A: Make Changes and Push

```bash
# Edit skills in source or any target
# Then push to remote:
skillshare push -m "Add new pdf skill"
```

### Machine B: Pull and Sync

```bash
skillshare pull
```

That's it. Your skills are now synced across machines.

---

## Commands

### Push

Commit and push local changes to remote.

```bash
skillshare push                  # Auto-generated commit message
skillshare push -m "Add pdf"     # Custom message
```

**What happens:**
```
┌─────────────────────────────────────────────────────────────────┐
│ skillshare push -m "Add pdf skill"                              │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│ cd ~/.config/skillshare/skills                                  │
│ git add .                                                       │
│ git commit -m "Add pdf skill"                                   │
│ git push origin main                                            │
└─────────────────────────────────────────────────────────────────┘
```

### Pull from Remote

Pull remote changes and sync to all targets.

```bash
skillshare pull
```

**What happens:**
```
┌─────────────────────────────────────────────────────────────────┐
│ skillshare pull                                                 │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│ 1. cd ~/.config/skillshare/skills                               │
│    git pull origin main                                         │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│ 2. skillshare sync (automatic)                                  │
│    Updates all targets with new/changed skills                  │
└─────────────────────────────────────────────────────────────────┘
```

---

## Second Machine Setup

Setting up a new machine to use your shared skills:

```bash
# 1. Clone your skills repo
git clone git@github.com:you/my-skills.git ~/.config/skillshare/skills

# 2. Initialize skillshare (uses existing source)
skillshare init --source ~/.config/skillshare/skills

# 3. Sync to all local targets
skillshare sync
```

**Flow diagram:**
```
┌─────────────────────────────────────────────────────────────────┐
│                   SECOND MACHINE SETUP                          │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│ 1. git clone                                                    │
│                                                                 │
│    GitHub ─────────────────► ~/.config/skillshare/skills/       │
│    Remote                    (your skills are here now)         │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│ 2. skillshare init --source ~/.config/skillshare/skills         │
│                                                                 │
│    • Detects existing source directory                          │
│    • Auto-detects installed AI CLIs                             │
│    • Creates config.yaml                                        │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│ 3. skillshare sync                                              │
│                                                                 │
│    Source ─────────────────► Claude, Cursor, Codex, ...         │
│    (symlinks created)                                           │
└─────────────────────────────────────────────────────────────────┘
```

---

## Conflict Handling

### Push Fails (Remote Ahead)

```
$ skillshare push
Error: remote has changes. Run 'skillshare pull' first.
```

**Solution:**
```bash
skillshare pull   # Get remote changes first
skillshare push   # Now push works
```

### Pull Fails (Local Uncommitted Changes)

```
$ skillshare pull
Error: local has uncommitted changes. Run 'skillshare push' first.
```

**Solution:**
```bash
skillshare push -m "Save local changes"
skillshare pull
```

### Merge Conflicts

If git encounters merge conflicts:

```bash
cd ~/.config/skillshare/skills
git status                    # See conflicted files
# Edit files to resolve conflicts
git add .
git commit -m "Resolve conflicts"
skillshare sync               # Sync resolved changes
```

---

## Tips

### Private Repository

Use SSH URL for private repos:
```bash
skillshare init --remote git@github.com:you/private-skills.git
```

### Check Remote Status

```bash
skillshare status
# Shows: Git: clean (remote: origin) or Git: 2 commits ahead
```

### Multiple Remotes

You can add multiple remotes manually:
```bash
cd ~/.config/skillshare/skills
git remote add backup git@gitlab.com:you/skills-backup.git
git push backup main
```

---

## Related

- [sync](/docs/commands/sync) — Sync operations
- [team-edition](/docs/guides/team-edition) — Team sharing with tracked repos
- [configuration](/docs/guides/configuration) — Config file reference
