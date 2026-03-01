---
sidebar_position: 2
---

# backup

Create, list, and manage backups of target directories.

```bash
skillshare backup              # Backup all targets
skillshare backup claude       # Backup specific target
skillshare backup --list       # List all backups
skillshare backup --cleanup    # Remove old backups
```

## When to Use

- Create a manual backup before risky changes
- List existing backups to check recovery options
- Clean up old backups to free disk space

## Automatic Backups

Backups are created **automatically** before:
- `skillshare sync`
- `skillshare target remove`

Location: `~/.local/share/skillshare/backups/<timestamp>/`

## Commands

### Create Backup

```bash
skillshare backup              # All targets
skillshare backup claude       # Specific target
skillshare backup --dry-run    # Preview
```

### List Backups

```bash
skillshare backup --list
```

```
All backups (15.3 MB total)
  2026-01-20_15-30-00  claude, cursor     4.2 MB  ~/.config/.../2026-01-20_15-30-00
  2026-01-19_10-00-00  claude             2.1 MB  ~/.config/.../2026-01-19_10-00-00
  2026-01-18_09-00-00  claude, cursor     4.0 MB  ~/.config/.../2026-01-18_09-00-00
```

### Cleanup Old Backups

```bash
skillshare backup --cleanup           # Remove old backups
skillshare backup --cleanup --dry-run # Preview cleanup
```

Default cleanup policy:
- Keep last 10 backups
- Remove backups older than 30 days
- Cap total size at 500 MB

## Options

| Flag | Description |
|------|-------------|
| `--list, -l` | List all backups |
| `--cleanup, -c` | Remove old backups |
| `--target, -t <name>` | Target specific backup |
| `--dry-run, -n` | Preview without making changes |

## Backup Structure

```
~/.local/share/skillshare/backups/
├── 2026-01-20_15-30-00/
│   ├── claude/
│   │   ├── skill-a/
│   │   └── skill-b/
│   └── cursor/
│       ├── skill-a/
│       └── skill-b/
└── 2026-01-19_10-00-00/
    └── claude/
        └── ...
```

## What Gets Backed Up

- Regular directories in targets (actual skill files)
- Per-skill symlinks in merge-mode targets are **resolved and followed** — the real content is backed up

This means:
- In merge mode: All skills are backed up (symlinks resolved, local copies included)
- In copy mode: All managed skill directories are backed up
- In symlink mode: Nothing is backed up (entire directory is a single symlink)

## See Also

- [restore](/docs/reference/commands/restore) — Restore from backup
- [sync](/docs/reference/commands/sync) — Auto-creates backups
- [target remove](/docs/reference/commands/target) — Auto-creates backups
