---
sidebar_position: 2
---

# Daily Workflow

The edit → sync → push/pull cycle for everyday skill management.

## Overview

```mermaid
flowchart LR
    EDIT["EDIT"] --> SYNC["SYNC"] --> PUSH["PUSH"] --> REMOTE["Remote"]
    EDIT --- SRC["Source"]
    SYNC --- TGT["Targets"]
    REMOTE --> PULL["Pull"]
    PULL -.-> EDIT
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

- You've installed or removed skills
- You've changed sync mode
- You've added or removed targets
- You see "out of sync" in status

```bash
skillshare sync
```

:::tip Why is sync a separate step?
Sync is intentionally decoupled from install/update/uninstall. This lets you batch multiple changes (e.g., install 3 skills → sync once), preview with `--dry-run` before propagating, and keep full control of when targets update. See [Source & Targets: Why Sync is a Separate Step](/docs/concepts/source-and-targets#why-sync-is-a-separate-step) for details.
:::

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
skillshare audit             # Scan for security threats
skillshare backup --cleanup  # Remove old backups
skillshare doctor            # Check for issues
```

---

## See Also

- [sync](/docs/commands/sync) — Core sync command
- [status](/docs/commands/status) — Check sync state
- [push](/docs/commands/push) / [pull](/docs/commands/pull) — Cross-machine sync
- [Skill Discovery](/docs/workflows/skill-discovery) — Find new skills
