---
sidebar_position: 4
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

## Initial Setup

### Option 1: New setup with remote

```bash
skillshare init --remote git@github.com:you/my-skills.git
```

This:
1. Creates source directory
2. Initializes git
3. Adds remote
4. Auto-detects and configures targets

### Option 2: Add remote to existing setup

```bash
cd ~/.config/skillshare/skills
git init
git remote add origin git@github.com:you/my-skills.git
git add .
git commit -m "Initial commit"
git push -u origin main
```

---

## Second Machine Setup

On a new machine:

```bash
# 1. Clone your skills repo
git clone git@github.com:you/my-skills.git ~/.config/skillshare/skills

# 2. Initialize skillshare (uses existing source)
skillshare init --source ~/.config/skillshare/skills

# 3. Sync to all local targets
skillshare sync
```

---

## Daily Workflow

### Machine A: Make changes and push

```bash
# Edit skills (changes visible immediately via symlinks)
$EDITOR ~/.config/skillshare/skills/my-skill/SKILL.md

# Push to remote
skillshare push -m "Update my-skill"
```

### Machine B: Pull and sync

```bash
skillshare pull
```

That's it. `pull` automatically runs `sync` after pulling.

---

## Commands

### Push

Commit and push local changes:

```bash
skillshare push                  # Auto-generated message
skillshare push -m "Add pdf"     # Custom message
```

**What happens:**
```
git add .
git commit -m "Add pdf"
git push origin main
```

### Pull

Pull remote changes and sync:

```bash
skillshare pull
```

**What happens:**
```
git pull origin main
skillshare sync
```

---

## Conflict Handling

### Push fails (remote ahead)

```
$ skillshare push
Error: remote has changes. Run 'skillshare pull' first.
```

**Solution:**
```bash
skillshare pull
skillshare push
```

### Pull fails (local uncommitted changes)

```
$ skillshare pull
Error: local has uncommitted changes.
```

**Solution:**
```bash
# Option 1: Push your changes first
skillshare push -m "Local changes"
skillshare pull

# Option 2: Discard local changes
cd ~/.config/skillshare/skills
git checkout -- .
skillshare pull
```

### Merge conflicts

```bash
cd ~/.config/skillshare/skills
git status                    # See conflicted files
# Edit files to resolve
git add .
git commit -m "Resolve conflicts"
skillshare sync
```

---

## Check Status

```bash
skillshare status
```

Shows:
- Git status (clean, ahead, behind)
- Remote configuration
- Sync status

---

## Private Repository

Use SSH URL for private repos:

```bash
skillshare init --remote git@github.com:you/private-skills.git
```

---

## Tips

### Use SSH keys

Set up SSH keys to avoid password prompts:
```bash
ssh-keygen -t ed25519 -C "your@email.com"
# Add public key to GitHub
```

### Multiple remotes

Add backup remotes:
```bash
cd ~/.config/skillshare/skills
git remote add backup git@gitlab.com:you/skills-backup.git
git push backup main
```

### Sync on shell startup

Add to `~/.bashrc` or `~/.zshrc`:
```bash
# Sync skillshare on terminal open (if remote configured)
skillshare pull 2>/dev/null
```

---

## Related

- [Team Sharing](./team-sharing) — Share with your team
- [Commands: push](/docs/commands/push) — Push command details
- [Commands: pull](/docs/commands/pull) — Pull command details
