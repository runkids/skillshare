# Trash (Soft-Delete)

`skillshare uninstall` moves skills and agents to trash with 7-day retention instead of permanent deletion.

## Commands

```bash
skillshare trash list                    # List trashed skills (alias: ls)
skillshare trash restore <name>          # Restore to source
skillshare trash delete <name>           # Permanently delete one (alias: rm)
skillshare trash empty                   # Permanently delete all (confirmation required)
```

### Agent Trash

```bash
skillshare trash agents list             # List trashed agents
skillshare trash agents restore <name>   # Restore agent to source
skillshare trash agents delete <name>    # Permanently delete one agent
skillshare trash agents empty            # Empty agent trash
skillshare trash --all list              # List both skills and agents
```

## Project Mode

```bash
skillshare trash list -p                 # Project trash
skillshare trash restore <name> -p       # Restore to project source
skillshare trash agents list -p          # Project agent trash
```

Use `-p` or `-g` to select scope explicitly when both configurations exist.

## Behavior

- **7-day TTL**: Items auto-expire after 7 days
- **Restore**: Copies skill/agent back to source directory; run `skillshare sync` / `skillshare sync agents` afterward
- **Empty**: Requires interactive confirmation (`y/yes`)
- **List output**: Shows name, size, and age (minutes/hours/days)

## AI Usage

```bash
# Undo an accidental uninstall
skillshare trash list --no-tui          # Find the skill
skillshare trash restore my-skill       # Restore it
skillshare sync                         # Re-sync to targets

# Undo an agent uninstall
skillshare trash agents restore tutor   # Restore agent
skillshare sync agents                  # Re-sync agents

# Clean up
skillshare trash delete old-skill       # Remove specific item
```

**Note:** `trash empty` requires interactive confirmation — not suitable for non-interactive AI use.
