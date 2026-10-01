# Backup & Restore

**Skills: global mode only** (project skill targets are reproducible via `install -p && sync`). **Agents: both modes** — `backup -p agents` works in project mode; plain `backup -p` errors.

## backup

Create backups of target skill directories.

```bash
skillshare backup                # All targets
skillshare backup claude         # Specific target
skillshare backup agents         # Agent targets
skillshare backup --all          # Skills + agents
skillshare backup --list [-p]    # List existing backups (-p: the project's)
skillshare backup --cleanup [-p] # Remove old backups
skillshare backup --delete <timestamp> [-p] [--dry-run]  # Delete one backup (timestamp from --list)
```

**Location:** `~/.local/share/skillshare/backups/<timestamp>/` (XDG data dir, not config). Project agent backups: `.skillshare/backups/`.

**Scope:** Local target content only. Merge-mode symlinks are skipped — they point into the source, which is already the source of truth, and `sync` recreates them. A target holding only symlinks produces no backup. Retention (10 snapshots / 30 days / 500 MB, newest always kept) runs automatically after every `sync`. Change the count and size in the global config with `backup.max_count` and `backup.max_size_mb` (`0` = no limit).

## backup files

Earlier versions of single files skillshare rewrote (AGENTS.md, CLAUDE.md, shared-file locations).

```bash
skillshare backup files [list] [-p|-g]                        # Files with saved versions
skillshare backup files show <path>                           # Versions, newest first
skillshare backup files restore <path> <id> [--unlink] [--dry-run]
```

IDs: `<time>[.<reason>]` (saved before a write; reason `convert`, `shim`, `edit`, `collect`, `attach`, `restore`), `drift:<time>[.<reason>]` (a user edit skillshare replaced; `overwrite`, `mode`, `restore`), `origin` (the file before a shared file was first attached). Restore saves the current content first; a symlinked path needs `--unlink`. In a project, only files inside it. A target named `files` needs `backup -t files`.

The dashboard's **Settings › Backup** has the same in three tabs: **Target folders**, **Files**, **MCP** (also in project mode, scoped to the project).

## restore

Restore target from backup.

```bash
skillshare restore claude                            # Latest backup
skillshare restore claude --from 2026-01-14_21-22   # Specific backup
```

For native agents, use `restore agents <target>` (and `-p` for project agents).
MCP recovery is separate: `mcp restore <backup-id>` restores managed native entries
without changing source definitions; see [mcp.md](mcp.md).

## Best Practices

- Run `backup` before major changes
- Use `--dry-run` with restore to preview
- Run `sync` after `restore` to recreate symlinks for synced skills
- Retention is automatic; use `--cleanup` only to prune on demand
