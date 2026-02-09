# Trash (Soft-Delete)

`skillshare uninstall` moves skills to trash with 7-day retention instead of permanent deletion.

## Commands

```bash
skillshare trash list                    # List trashed skills (alias: ls)
skillshare trash restore <name>          # Restore to source
skillshare trash delete <name>           # Permanently delete one (alias: rm)
skillshare trash empty                   # Permanently delete all (confirmation required)
```

## Project Mode

```bash
skillshare trash list -p                 # Project trash
skillshare trash restore <name> -p       # Restore to project source
skillshare trash delete <name> -p        # Delete from project trash
skillshare trash empty -p                # Empty project trash
```

Auto-detects mode when `.skillshare/config.yaml` exists.

## Behavior

- **7-day TTL**: Items auto-expire after 7 days
- **Restore**: Copies skill back to source directory; run `skillshare sync` afterward
- **Empty**: Requires interactive confirmation (`y/yes`)
- **List output**: Shows name, size, and age (minutes/hours/days)

## AI Usage

```bash
# Undo an accidental uninstall
skillshare trash list                   # Find the skill
skillshare trash restore my-skill       # Restore it
skillshare sync                         # Re-sync to targets

# Clean up
skillshare trash delete old-skill       # Remove specific item
```

**Note:** `trash empty` requires interactive confirmation — not suitable for non-interactive AI use.
