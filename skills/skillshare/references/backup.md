# Backup & Restore

**Skills: global mode only** (project skill targets are reproducible via `install -p && sync`). **Agents: both modes** — `backup -p agents` works in project mode; plain `backup -p` errors.

## backup

Create backups of target skill directories.

```bash
skillshare backup                # All targets
skillshare backup claude         # Specific target
skillshare backup agents         # Agent targets
skillshare backup --all          # Skills + agents
skillshare backup --list         # List existing backups
skillshare backup --cleanup      # Remove old backups
```

**Location:** `~/.local/share/skillshare/backups/<timestamp>/` (XDG data dir, not config). Project agent backups: `.skillshare/backups/`.

**Scope:** Local target content only. Merge-mode symlinks are skipped — they point into the source, which is already the source of truth, and `sync` recreates them. A target holding only symlinks produces no backup. Retention (10 snapshots / 30 days / 500 MB, newest always kept) runs automatically after every `sync`.

## restore

Restore target from backup.

```bash
skillshare restore claude                            # Latest backup
skillshare restore claude --from 2026-01-14_21-22   # Specific backup
```

## Best Practices

- Run `backup` before major changes
- Use `--dry-run` with restore to preview
- Run `sync` after `restore` to recreate symlinks for synced skills
- Retention is automatic; use `--cleanup` only to prune on demand
