---
sidebar_position: 2
---

# Daily Workflow

The edit → sync → push/pull cycle for everyday skill management.

## Overview

```
┌─────────────────────────────────────────────────────────────┐
│                    DAILY WORKFLOW                           │
│                                                             │
│   EDIT ──► SYNC ──► PUSH ───────────────► Remote            │
│     │        │                               │              │
│     │        │                               │              │
│     ▼        ▼                               ▼              │
│   Source   Targets                         Pull             │
│     │                                        │              │
│     │                                        │              │
│     ◄────────────────────────────────────────┘              │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

---

## Editing Skills

### Option 1: Edit in source (recommended)

```bash
$EDITOR ~/.config/skillshare/skills/my-skill/SKILL.md
```

Changes are immediately visible in all targets (via symlinks).

### Option 2: Edit in target

```bash
$EDITOR ~/.claude/skills/my-skill/SKILL.md
```

Because targets are symlinked, this edits the source file directly.

---

## Syncing

After editing, sync is **usually not needed** because of symlinks. However, run sync when:

- You've added new skills
- You've changed sync mode
- You've added new targets
- You see "out of sync" in status

```bash
skillshare sync
```

### Preview first

```bash
skillshare sync --dry-run
```

---

## Cross-Machine Sync

If you use git remote:

### Push changes (from this machine)

```bash
skillshare push -m "Add new skill"
```

This runs:
1. `git add .`
2. `git commit -m "Add new skill"`
3. `git push`

### Pull changes (to this machine)

```bash
skillshare pull
```

This runs:
1. `git pull`
2. `skillshare sync`

---

## Common Daily Tasks

### Create a new skill

```bash
skillshare new code-review
$EDITOR ~/.config/skillshare/skills/code-review/SKILL.md
skillshare sync
```

### Update a tracked repo

```bash
skillshare update _team-skills
skillshare sync
```

### Update all tracked repos

```bash
skillshare update --all
skillshare sync
```

### Check status

```bash
skillshare status
```

Shows:
- Source directory status
- Git status (commits ahead/behind)
- Target sync status

---

## Tips

### Make it automatic

Add to your shell startup:
```bash
# ~/.bashrc or ~/.zshrc
alias ss="skillshare"
alias sss="skillshare sync"
alias ssp="skillshare push"
alias ssl="skillshare pull"
```

### Check before important work

```bash
# Start of day
skillshare pull
skillshare status

# Before committing
skillshare diff
```

### Keep things clean

```bash
# Weekly maintenance
skillshare backup --cleanup  # Remove old backups
skillshare doctor            # Check for issues
```

---

## Related

- [Cross-Machine Sync](/docs/guides/cross-machine-sync) — Full setup guide
- [Commands: sync](/docs/commands/sync) — Sync command details
- [Commands: status](/docs/commands/status) — Status command
