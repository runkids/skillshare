---
sidebar_position: 4
---

# Backup & Restore

Protect your skills and recover from mistakes.

## Overview

skillshare maintains automatic backups and provides manual backup/restore commands.

```mermaid
flowchart LR
    T1["TARGETS"] -- backup --> B["~/.local/share/skillshare/backups/"]
    B -- restore --> T2["TARGETS"]
```

---

## Automatic Backups

Backups are created automatically before:

- `skillshare sync` (skill targets and agent targets)
- `skillshare sync agents` (agent targets only)
- `skillshare target remove`

**Location:** `~/.local/share/skillshare/backups/<timestamp>/` (agent backups appear as `<target>-agents/` next to the regular `<target>/` directories).

**Scope:** Only local target content is captured. Merge-mode symlinks are skipped because they point into your source — `sync` recreates them. See [What Gets Backed Up](/docs/reference/commands/backup#what-gets-backed-up).

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

```mermaid
flowchart TD
    TITLE["skillshare restore claude"]
    S1["1. Find latest backup"]
    S2["2. Remove current target"]
    S3["3. Copy backup to target"]
    TITLE --> S1 --> S2 --> S3
```

**Note:** After restore, the target contains real files (not symlinks). Run `skillshare sync` to re-establish symlinks.

---

## Cleanup Old Backups

```bash
skillshare backup --cleanup
```

Removes backups older than the configured retention period. Retention already runs automatically after every `sync`, so this is only for pruning on demand.

To check how much space snapshots use:

```bash
du -sh ~/.local/share/skillshare/backups
skillshare backup --cleanup --dry-run   # Preview what would be removed
```

See [Backups & Disk Space](/docs/reference/commands/backup#backups--disk-space) for how backup scope differs from `.gitignore` and `ignore:`.

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

### Recover an agent

Agents have their own backup entries (`<target>-agents`) and follow the same flow as skills:

```bash
# Manual agent backup
skillshare backup agents claude

# Restore from latest
skillshare restore agents claude

# Restore from a specific timestamp
skillshare restore agents claude --from 2026-01-19_10-00-00
```

In project mode, only agents can be backed up or restored — `skillshare backup -p agents` works, but plain `skillshare backup -p` errors. See [backup](/docs/reference/commands/backup#agent-backup) for the project-mode rule.

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

## See Also

- [backup](/docs/reference/commands/backup) — Backup command reference
- [restore](/docs/reference/commands/restore) — Restore command reference
- [trash](/docs/reference/commands/trash) — Soft-delete management
- [Troubleshooting](/docs/troubleshooting) — When things go wrong
