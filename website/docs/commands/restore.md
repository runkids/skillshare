---
sidebar_position: 3
---

# restore

Restore a target from a backup.

```bash
skillshare restore claude                              # Latest backup
skillshare restore claude --from 2026-01-19_10-00-00   # Specific backup
skillshare restore claude --dry-run                    # Preview
```

## What Happens

```
┌─────────────────────────────────────────────────────────────────┐
│ skillshare restore claude                                       │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│ 1. Find latest backup for claude                                │
│    → backups/2026-01-20_15-30-00/claude/                        │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│ 2. Remove current target directory                              │
│    → ~/.claude/skills/ (removed)                                │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│ 3. Copy backup to target                                        │
│    → ~/.claude/skills/ (restored)                               │
└─────────────────────────────────────────────────────────────────┘
```

## Options

| Flag | Description |
|------|-------------|
| `--from <timestamp>` | Restore from specific backup |
| `--force` | Overwrite without confirmation |
| `--dry-run, -n` | Preview without making changes |

## Finding Backups

List available backups:

```bash
skillshare backup --list
```

```
All backups (15.3 MB total)
  2026-01-20_15-30-00  claude, cursor     4.2 MB
  2026-01-19_10-00-00  claude             2.1 MB
  2026-01-18_09-00-00  claude, cursor     4.0 MB
```

## Examples

```bash
# Restore from latest backup
skillshare restore claude

# Restore from specific backup
skillshare restore claude --from 2026-01-19_10-00-00

# Preview restore
skillshare restore claude --dry-run

# Force restore (skip confirmation)
skillshare restore claude --force
```

## After Restoring

After restore, your target will have regular directories instead of symlinks. To re-sync:

```bash
skillshare restore claude    # Restore backup
skillshare sync              # Re-sync from source (creates symlinks)
```

Or keep the restored state:
```bash
skillshare restore claude    # Restore backup
# Done - skills are now local copies
```

## Use Cases

### Accidental Deletion

If you accidentally deleted a skill:

```bash
skillshare restore claude --from 2026-01-19_10-00-00
```

### Reverting Changes

If a sync went wrong:

```bash
skillshare restore claude  # Go back to pre-sync state
```

### Testing

Restore to test old skill versions:

```bash
skillshare restore claude --from 2026-01-15_10-00-00
# Test old skills...
skillshare sync  # Return to current state
```

## Related

- [backup](/docs/commands/backup) — Create and manage backups
- [sync](/docs/commands/sync) — Re-sync after restore
