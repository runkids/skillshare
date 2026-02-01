---
sidebar_position: 1
---

# target

Manage sync targets (AI CLI skill directories).

```bash
skillshare target add <name> <path>    # Add a target
skillshare target remove <name>        # Remove a target
skillshare target list                 # List all targets
skillshare target <name>               # Show target info
skillshare target <name> --mode merge  # Change sync mode
```

## Subcommands

### target add

Add a new target for skill synchronization.

```bash
skillshare target add windsurf ~/.windsurf/skills
```

The command validates:
- Path exists or parent directory exists
- Path looks like a skills directory
- Target name is unique

### target remove

Remove a target and restore its skills to regular directories.

```bash
skillshare target remove cursor           # Remove single target
skillshare target remove --all            # Remove all targets
skillshare target remove cursor --dry-run # Preview
```

**What happens:**
1. Creates backup of target
2. Removes symlinks, copies skills back
3. Removes target from config

### target list

List all configured targets.

```bash
skillshare target list
```

```
Configured Targets
  claude       ~/.claude/skills (merge)
  cursor       ~/.cursor/skills (merge)
  codex        ~/.openai-codex/skills (symlink)
```

### target info / mode

Show target details or change sync mode.

```bash
# Show info
skillshare target claude

# Change mode
skillshare target claude --mode symlink
skillshare target claude --mode merge
skillshare sync  # Apply change
```

## Sync Modes

| Mode | Behavior |
|------|----------|
| `merge` | Each skill symlinked individually. Preserves local skills. **Default.** |
| `symlink` | Entire directory is one symlink. Exact copies everywhere. |

```bash
# Set target to symlink mode
skillshare target claude --mode symlink
skillshare sync  # Apply the change
```

## Options

### target add

No additional options.

### target remove

| Flag | Description |
|------|-------------|
| `--all, -a` | Remove all targets |
| `--dry-run, -n` | Preview without making changes |

### target info

| Flag | Description |
|------|-------------|
| `--mode, -m <mode>` | Set sync mode (merge or symlink) |

## Supported AI CLIs

Skillshare auto-detects these during `init`:

| CLI | Default Path |
|-----|-------------|
| Claude Code | `~/.claude/skills` |
| Cursor | `~/.cursor/skills` |
| OpenCode | `~/.opencode/skills` |
| Windsurf | `~/.windsurf/skills` |
| Codex | `~/.openai-codex/skills` |
| Gemini CLI | `~/.gemini/skills` |
| Amp | `~/.amp/skills` |
| ... and 30+ more | See [supported targets](/docs/targets/supported-targets) |

## Examples

```bash
# Add custom target
skillshare target add my-tool ~/my-tool/skills

# Check target status
skillshare target claude

# Switch to symlink mode
skillshare target claude --mode symlink
skillshare sync

# Remove target (restores skills)
skillshare target remove cursor
```

## Related

- [sync](/docs/commands/sync) — Sync skills to targets
- [status](/docs/commands/status) — Show target status
- [Targets](/docs/targets) — Target management guide
