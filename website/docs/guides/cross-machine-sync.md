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

## First Machine Setup

```bash
skillshare init --remote git@github.com:you/my-skills.git
```

This:
1. Creates source directory
2. Initializes git with initial commit
3. Adds remote
4. Auto-detects and configures targets

Then push your skills:
```bash
skillshare push
```

:::tip Already initialized?
Add a remote to an existing setup:
```bash
skillshare init --remote git@github.com:you/my-skills.git
```
This works even after initial setup — it just adds the remote.
:::

---

## Second Machine Setup

On a new machine, **the same command works**:

```bash
skillshare init --remote git@github.com:you/my-skills.git
```

Init automatically detects that the remote has existing skills and pulls them down. No manual `git clone` needed.

:::info What happens behind the scenes
1. Creates source directory and initializes git
2. Adds remote and runs `git fetch`
3. Detects remote has skills → auto-pulls them
4. Sets up tracking branch
5. Auto-detects and configures local targets
:::

If you prefer manual control:

```bash
# Clone directly, then init with existing source
git clone git@github.com:you/my-skills.git ~/.config/skillshare/skills
skillshare init --source ~/.config/skillshare/skills
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

- [Organization-Wide Skills](./organization-sharing) — Share across your organization
- [Commands: push](/docs/commands/push) — Push command details
- [Commands: pull](/docs/commands/pull) — Pull command details
