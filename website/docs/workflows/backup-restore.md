---
sidebar_position: 4
---

# Backup & Restore

Protect your skills and recover from mistakes.

## Overview

Skillshare maintains automatic backups and provides manual backup/restore commands.

```
┌─────────────────────────────────────────────────────────────┐
│                    BACKUP SYSTEM                            │
│                                                             │
│   TARGETS ──► backup ──► ~/.config/skillshare/backups/      │
│                                                             │
│   BACKUPS ──► restore ──► TARGETS                           │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

---

## Automatic Backups

Backups are created automatically before:

- `skillshare sync`
- `skillshare target remove`

**Location:** `~/.config/skillshare/backups/<timestamp>/`

---

## Manual Backup

### All targets

```bash
skillshare backup
```

### Specific target

```bash
skillshare backup claude
```

### Preview

```bash
skillshare backup --dry-run
```

---

## List Backups

```bash
skillshare backup --list
```

**Example output:**
```
Backups
─────────────────────────────────────────
  2026-01-20_15-30-00/
    claude/    5 skills, 2.1 MB
    cursor/    5 skills, 2.1 MB
  2026-01-19_10-00-00/
    claude/    4 skills, 1.8 MB
```

---

## Restore

### From latest backup

```bash
skillshare restore claude
```

### From specific backup

```bash
skillshare restore claude --from 2026-01-19_10-00-00
```

### Preview

```bash
skillshare restore claude --dry-run
```

---

## What Restore Does

```
┌─────────────────────────────────────────────────────────────┐
│ skillshare restore claude                                   │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│ 1. Find latest backup for claude                            │
│    → backups/2026-01-20_15-30-00/claude/                    │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│ 2. Remove current target directory                          │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│ 3. Copy backup to target                                    │
│    → ~/.claude/skills/                                      │
└─────────────────────────────────────────────────────────────┘
```

**Note:** After restore, the target contains real files (not symlinks). Run `skillshare sync` to re-establish symlinks.

---

## Cleanup Old Backups

```bash
skillshare backup --cleanup
```

Removes backups older than the configured retention period.

---

## Recovery Scenarios

### Accidentally deleted a skill through symlink

```bash
# If git is initialized (recommended)
cd ~/.config/skillshare/skills
git checkout -- deleted-skill/

# Or restore from backup
skillshare restore claude
skillshare sync
```

### Messed up sync mode

```bash
skillshare restore claude
skillshare target claude --mode merge
skillshare sync
```

### Want to undo recent changes

```bash
skillshare backup --list
skillshare restore claude --from <earlier-timestamp>
```

---

## Best Practices

### Before risky operations

```bash
skillshare backup
```

### After major changes

```bash
skillshare push -m "Major update"  # Git backup
```

### Weekly maintenance

```bash
skillshare backup --cleanup
```

---

## Git as Backup

Git provides an additional backup layer:

```bash
# Recover deleted skill
cd ~/.config/skillshare/skills
git checkout -- deleted-skill/

# See history
git log --oneline

# Restore to previous commit
git checkout <commit-hash> -- specific-skill/
```

---

## Related

- [Commands: backup](/docs/commands/backup) — Backup command details
- [Commands: restore](/docs/commands/restore) — Restore command details
- [Troubleshooting](/docs/troubleshooting) — When things go wrong
