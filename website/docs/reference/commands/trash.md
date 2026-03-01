---
sidebar_position: 4
---

# trash

Manage uninstalled skills in the trash directory.

```bash
skillshare trash list                    # Interactive TUI (in TTY)
skillshare trash list --no-tui           # Plain text output
skillshare trash restore my-skill        # Restore from trash
skillshare trash restore my-skill -p     # Restore in project mode
skillshare trash delete my-skill         # Permanently delete from trash
skillshare trash empty                   # Empty the trash
```

## When to Use

- Recover a skill you recently uninstalled (within 7 days)
- Permanently delete trashed skills to free space
- Check what's in the trash before it auto-expires

## Interactive TUI

In a TTY, `trash list` launches an interactive TUI with multi-select, filtering, and inline restore/delete operations:

```
Trash (global) — 5 items

  [ ] my-skill    (512 B, 2d ago)
  [x] old-tool    (1.2 KB, 5d ago)
  [ ] another     (128 B, 1d ago)

  ─────────────────────────────────────────
  Name:         old-tool
  Trashed:      2026-02-27 14:30:05
  Size:         1.2 KB
  Path:         ~/.local/share/skillshare/trash/old-tool_...

  ↑↓ navigate  / filter  space select  r restore(1)  d delete(1)  D empty  q quit
```

### Key Bindings

| Key | Action |
|-----|--------|
| `↑`/`↓` | Navigate items |
| `←`/`→` | Change page |
| `/` | Enter filter mode (substring match on name) |
| `Space` | Toggle select current item |
| `a` | Toggle select all visible items |
| `r` | Restore selected items (with confirmation) |
| `d` | Permanently delete selected items (with confirmation) |
| `D` | Empty all trash (ignores selection, with confirmation) |
| `q`/`Ctrl+C` | Quit |

In confirmation mode: `y`/`Enter` to confirm, `n`/`Esc` to cancel.

### Batch Operations

When multiple items are selected, `r` and `d` operate on all of them. If some items fail (e.g., restoring a skill whose name already exists in source), the TUI continues processing the remaining items and shows a combined result:

```
Restored 2 item(s)  Failed: my-skill: already exists
```

Use `--no-tui` to skip the TUI and print plain text instead:

```bash
skillshare trash list --no-tui           # Plain text output
skillshare trash list --no-tui | less    # Pipe to pager manually
```

## Subcommands

### list

Show all skills currently in the trash. Launches the interactive TUI in a terminal, or prints plain text with `--no-tui` or in non-TTY:

```bash
skillshare trash list
skillshare trash list --no-tui
```

Plain text output:

```
Trash
  my-skill      (1.2 KB, 2d ago)
  old-helper    (800 B, 5d ago)

2 item(s), 2.0 KB total
Items are automatically cleaned up after 7 days
```

### restore

Restore the most recent trashed version of a skill back to the source directory:

```bash
skillshare trash restore my-skill
```

```
✓ Restored: my-skill
ℹ Trashed 2d ago, now back in ~/.config/skillshare/skills
ℹ Run 'skillshare sync' to update targets
```

If a skill with the same name already exists in source, restore will fail. Uninstall the existing skill first or use a different name.

### delete

Permanently delete a single item from the trash:

```bash
skillshare trash delete my-skill
```

```
✓ Permanently deleted: my-skill
```

### empty

Permanently delete all items from the trash (with confirmation prompt):

```bash
skillshare trash empty
```

```
⚠ This will permanently delete 3 item(s) from trash
Continue? [y/N]: y
✓ Emptied trash: 3 item(s) permanently deleted
```

## Backup vs Trash

These two safety mechanisms protect different things:

| | backup | trash |
|---|---|---|
| **Protects** | target directories (sync snapshots) | source skills (uninstall) |
| **Location** | `~/.local/share/skillshare/backups/` | `~/.local/share/skillshare/trash/` |
| **Triggered by** | `sync`, `target remove` | `uninstall` |
| **Restore with** | `skillshare restore <target>` | `skillshare trash restore <name>` |
| **Auto-cleanup** | manual (`backup --cleanup`) | 7 days |

## Options

| Flag | Description |
|------|-------------|
| `--no-tui` | Disable interactive TUI, use plain text output |
| `--project, -p` | Use project-level trash (`.skillshare/trash/`) |
| `--global, -g` | Use global trash |
| `--help, -h` | Show help |

## Auto-Cleanup

Expired trash items (older than 7 days) are automatically cleaned up when you run `uninstall` or `sync`. No cron or scheduled task is needed.

## See Also

- [uninstall](/docs/reference/commands/uninstall) — Remove skills (moves to trash)
- [backup](/docs/reference/commands/backup) — Backup target directories
- [restore](/docs/reference/commands/restore) — Restore targets from backup
