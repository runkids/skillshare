---
sidebar_position: 4
---

# trash

Manage uninstalled skills and agents in the trash directory.

```bash
skillshare trash list                    # Interactive TUI (in TTY)
skillshare trash list --no-tui           # Plain text output
skillshare trash restore my-skill        # Restore from trash
skillshare trash restore my-skill -p     # Restore in project mode
skillshare trash delete my-skill         # Permanently delete from trash
skillshare trash empty                   # Empty the trash
skillshare trash agents list             # List trashed agents
skillshare trash agents restore tutor    # Restore an agent from trash
skillshare trash --all list              # List trashed skills + agents
```

## When to Use

- Recover a skill or agent you recently uninstalled (within 7 days)
- Permanently delete trashed items to free space
- Check what's in the trash before it auto-expires

## Interactive TUI

In a TTY, `trash list` launches an interactive TUI with multi-select, filtering, and inline restore/delete operations. Each item shows a kind badge: `[S]` for skills, `[A]` for agents.

```
Trash (global) — 5 items

  [ ] [S] my-skill    (512 B, 2d ago)
  [x] [S] old-tool    (1.2 KB, 5d ago)
  [ ] [A] tutor       (2.0 KB, 3d ago)
  [ ] [S] another     (128 B, 1d ago)

  ─────────────────────────────────────────
  Name:         old-tool
  Type:         Skill
  Trashed:      2026-02-27 14:30:05
  Size:         1.2 KB
  Path:         ~/.local/share/skillshare/trash/old-tool_...

  ── SKILL.md ──────────────────────────────
  ---
  name: old-tool
  description: A helpful tool
  ---
  # old-tool
  ...

  ↑↓ navigate  / filter  space select  r restore(1)  d delete(1)  D empty  q quit
```

When using `--all` or without a kind filter, the TUI merges skills and agents into a single list sorted by date (newest first).

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
| `Ctrl+d`/`Ctrl+u` | Scroll detail panel down/up |
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

## Kind Filter

By default, trash operates on **skills**. Use the `agents` positional keyword to target agents, or `--all` to include both:

```bash
skillshare trash list                    # Skills only (default)
skillshare trash agents list             # Agents only
skillshare trash --all list              # Both skills and agents
skillshare trash agents restore tutor    # Restore a trashed agent
skillshare trash agents empty            # Empty agent trash only
```

## Subcommands

### list (alias: `ls`)

Show all items currently in the trash. Launches the interactive TUI in a terminal, or prints plain text with `--no-tui` or in non-TTY:

```bash
skillshare trash list
skillshare trash agents list
skillshare trash --all list --no-tui
```

Plain text output:

```
Trash
  my-skill      1.2 KB · 2d ago
  old-helper    800 B · 5d ago

2 items, 2.0 KB
  Each item is removed for good 7 days after it was trashed
```

### restore

Restore the most recent trashed version back to the source directory:

```bash
skillshare trash restore my-skill
skillshare trash agents restore tutor
```

```
✓ Restore   my-skill → ~/.config/skillshare/skills · trashed 2d ago

Next
  skillshare sync  link it into your targets again
```

For agents, the restore hint will suggest `skillshare sync agents` instead.

If an item with the same name already exists in source, restore will fail. Uninstall the existing item first or use a different name.

### delete (alias: `rm`)

Permanently delete a single item from the trash:

```bash
skillshare trash delete my-skill
skillshare trash agents delete tutor
```

```
✓ Permanently deleted my-skill
```

### empty

Permanently delete all items from the trash (with confirmation prompt):

```bash
skillshare trash empty
skillshare trash agents empty
```

```
! This will permanently delete 3 items from trash
? Continue? [y/N] y
✓ Emptied trash: 3 items permanently deleted · 0.1s
```

## Backup vs Trash

These two safety mechanisms protect different things:

| | backup | trash |
|---|---|---|
| **Protects** | target directories (sync snapshots) | source skills and agents (uninstall) |
| **Location** | `~/.local/share/skillshare/backups/` | `~/.local/share/skillshare/trash/` (skills), `.../trash/agents/` (agents) |
| **Triggered by** | `sync`, `target remove` | `uninstall` |
| **Restore with** | `skillshare restore <target>` | `skillshare trash restore <name>` |
| **Auto-cleanup** | manual (`backup --cleanup`) | 7 days |

## Options

| Flag | Description |
|------|-------------|
| `agents` | Positional keyword — operate on agents instead of skills |
| `--all` | Include both skills and agents |
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
