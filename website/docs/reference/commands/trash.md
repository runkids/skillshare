---
sidebar_position: 4
---

# trash

Manage uninstalled skills in the trash directory.

```bash
skillshare trash list                    # List trashed skills
skillshare trash restore my-skill        # Restore from trash
skillshare trash restore my-skill -p     # Restore in project mode
skillshare trash delete my-skill         # Permanently delete from trash
skillshare trash empty                   # Empty the trash
```

## When to Use

- Recover a skill you recently uninstalled (within 7 days)
- Permanently delete trashed skills to free space
- Check what's in the trash before it auto-expires

## Subcommands

### list

Show all skills currently in the trash:

```bash
skillshare trash list
```

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
| `--project, -p` | Use project-level trash (`.skillshare/trash/`) |
| `--global, -g` | Use global trash |
| `--help, -h` | Show help |

## Auto-Cleanup

Expired trash items (older than 7 days) are automatically cleaned up when you run `uninstall` or `sync`. No cron or scheduled task is needed.

## See Also

- [uninstall](/docs/reference/commands/uninstall) — Remove skills (moves to trash)
- [backup](/docs/reference/commands/backup) — Backup target directories
- [restore](/docs/reference/commands/restore) — Restore targets from backup
