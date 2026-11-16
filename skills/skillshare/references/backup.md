# Backup & Restore

**Manual skills backup: global mode only.** Existing profile-created project skills snapshots can be restored with explicit `-p` and a name. **Agents: both modes** — `backup -p agents` works in project mode; plain `backup -p` errors.

## backup

Create backups of target skill directories.

```bash
skillshare backup                # All targets
skillshare backup claude         # Specific target
skillshare backup agents         # Agent targets
skillshare backup --all          # Skills + agents
skillshare backup --list [-p]    # List existing backups (-p: the project's)
skillshare backup --cleanup [-p] # Remove old snapshots and abandoned incomplete directories
skillshare backup --delete <timestamp> [-p] [--dry-run]  # Delete one backup (timestamp from --list)
```

**Location:** `~/.local/share/skillshare/backups/<timestamp>/` (XDG data dir, not config). Project snapshots: `.skillshare/backups/`.

**Scope:** Local target content only. Merge-mode symlinks are skipped — they point into the source, which is already the source of truth, and `sync` recreates them. A target holding only symlinks produces no backup. Retention (10 snapshots / 30 days / 500 MB, newest always kept) runs automatically after every `sync`. Change the count and size in the global config with `backup.max_count` and `backup.max_size_mb` (`0` = no limit).

Cleanup also removes incomplete `.snapshot-*` directories left by interrupted backups, including when there are no completed snapshots. `--list` and its size total exclude them; `backup --cleanup --dry-run` previews their removal. Cleanup skips the backup directory while a backup is being written; retry after it finishes.

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

Project recovery (from the project root):

```bash
skillshare backup --list -p
skillshare restore <entry> --from <timestamp> -p --dry-run
skillshare restore <entry> --from <timestamp> -p --force
```

Normal names restore current skills folders. Recognized `<target>-moved-<hex-relative-directory>` keys restore former folders, leaving the current folder alone; the configured target and declared former path must still exist in the mapping. Unknown or ambiguous keys refuse. Restore replaces the whole folder, omits symlinks, and does not change the active profile. Save authored edits into source before resync. Bare skills restore and `-g` stay global; `restore -p` requires a name. Legacy `--all` follows the global skills branch, even with `-p`; restore agents separately. The dashboard Backup restore shares these project destinations.

For native agents, use `restore agents <target>` (and `-p` for project agents).
MCP recovery is separate: `mcp restore <backup-id>` restores managed native entries
without changing source definitions; see [mcp.md](mcp.md).

## Best Practices

- Run `backup` before major changes
- Use `--dry-run` with restore to preview
- Run `sync` after `restore` to recreate symlinks for synced skills
- Retention is automatic; use `--cleanup` only to prune on demand
