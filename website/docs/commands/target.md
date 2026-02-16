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
2. Detects sync mode:
   - **Symlink mode:** Removes the directory symlink, copies source contents back as a real directory
   - **Merge mode:** Removes only symlinks pointing to source (by path prefix), copies each skill back as real files. Local (non-symlink) skills are preserved.
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

## Target Filters (include/exclude)

Manage per-target include/exclude filters from the CLI:

```bash
skillshare target claude --add-include "team-*"
skillshare target claude --add-exclude "_legacy*"
skillshare target claude --remove-include "team-*"
skillshare target claude --remove-exclude "_legacy*"
```

After changing filters, run `skillshare sync` to apply.

Filters work in **merge mode** only. Patterns use Go `filepath.Match` syntax (`*`, `?`, `[...]`).

See [Configuration](/docs/targets/configuration#include--exclude-target-filters) for pattern cheat sheet and scenarios.

## Options

### target add

No additional options.

### target remove

| Flag | Description |
|------|-------------|
| `--all, -a` | Remove all targets |
| `--dry-run, -n` | Preview without making changes |

### target info / filters

| Flag | Description |
|------|-------------|
| `--mode, -m <mode>` | Set sync mode (merge or symlink) |
| `--add-include <pattern>` | Add an include filter pattern |
| `--add-exclude <pattern>` | Add an exclude filter pattern |
| `--remove-include <pattern>` | Remove an include filter pattern |
| `--remove-exclude <pattern>` | Remove an exclude filter pattern |

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
| ... and 45+ more | See [supported targets](/docs/targets/supported-targets) |

## Examples

```bash
# Add custom target
skillshare target add my-tool ~/my-tool/skills

# Check target status
skillshare target claude

# Switch to symlink mode
skillshare target claude --mode symlink
skillshare sync

# Add/remove filters
skillshare target claude --add-include "team-*"
skillshare target claude --add-exclude "_legacy*"
skillshare target claude --remove-include "team-*"
skillshare sync

# Remove target (restores skills)
skillshare target remove cursor
```

## Project Mode

Manage targets for the current project:

```bash
skillshare target add windsurf -p                                # Add known target
skillshare target add custom ./tools/ai/skills -p                # Add custom path
skillshare target remove cursor -p                                # Remove target
skillshare target list -p                                         # List project targets
skillshare target claude -p                                  # Show target info
skillshare target claude --add-include "team-*" -p          # Add filter
```

### How It Differs

| | Global | Project (`-p`) |
|---|---|---|
| Config | `~/.config/skillshare/config.yaml` | `.skillshare/config.yaml` |
| Paths | Absolute (e.g., `~/.claude/skills`) | Relative or absolute (e.g., `.claude/skills`) |
| Sync mode | Merge or symlink | Merge or symlink (default merge) |
| Mode change | `--mode` flag | `--mode` flag |

### Project Target List Example

```
Project Targets
  claude    .claude/skills (merge)
  cursor         .cursor/skills (merge)
  custom-tool    ./tools/ai/skills (merge)
```

Targets in project mode support:
- **Known target names** (e.g., `claude`, `cursor`) — resolved to project-local paths
- **Custom paths** — relative to project root or absolute with `~` expansion

## Related

- [sync](/docs/commands/sync) — Sync skills to targets
- [status](/docs/commands/status) — Show target status
- [Targets](/docs/targets) — Target management guide
- [Project Skills](/docs/concepts/project-skills) — Project mode concepts
