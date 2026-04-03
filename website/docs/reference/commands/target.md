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
skillshare target <name> --target-naming standard  # Change naming
```

## When to Use

- Add a new AI CLI target after installing a new tool
- Remove a target you no longer use
- Change sync mode (merge, copy, or symlink) for a target
- Change target naming (flat or standard) for a target
- Tune compatibility target-by-target instead of forcing one global mode
- Set up include/exclude filters for selective skill syncing

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
   - **Copy mode:** Removes `.skillshare-manifest.json`. Managed copies and local skills are preserved as regular directories.
3. Removes target from config

### target list

List all configured targets.

```bash
skillshare target list                 # Interactive TUI (default on TTY)
skillshare target list --no-tui        # Plain text output
skillshare target list --json          # JSON output for CI/scripts
```

#### Interactive TUI

On a TTY, `target list` launches an interactive terminal UI with:

- **Split layout** — target list on the left, detail panel on the right (falls back to vertical layout on narrow terminals)
- **Fuzzy filter** — press `/` to filter targets by name
- **Mode picker** — press `M` to change the sync mode (merge, copy, symlink) for the selected target
- **Naming picker** — press `N` to change the target naming (flat, standard) for the selected target
- **Include/Exclude editor** — press `I` or `E` to open the filter pattern editor for the selected target. Use `a` to add patterns, `d` to delete
- **Remove target** — press `R` to remove the selected target. Shows a confirmation prompt before proceeding (backs up and unlinks, same as `target remove`)
- **Keyboard navigation** — `↑`/`↓` to browse, `Ctrl+d`/`Ctrl+u` to scroll the detail panel, `q` to quit

Changes made through the TUI (mode, include/exclude) are saved to config immediately. Run `skillshare sync` to apply.

Use `--no-tui` to skip the TUI and print plain text instead:

```
Configured Targets
  claude       ~/.claude/skills (merge)
  cursor       ~/.cursor/skills (merge)
  codex        ~/.openai-codex/skills (symlink)
```

#### JSON Output

```bash
skillshare target list --json
```

```json
{
  "targets": [
    {
      "name": "claude",
      "path": "~/.claude/skills",
      "mode": "merge",
      "targetNaming": "flat",
      "include": [],
      "exclude": []
    },
    {
      "name": "cursor",
      "path": "~/.cursor/skills",
      "mode": "merge",
      "targetNaming": "standard",
      "include": [],
      "exclude": []
    }
  ]
}
```

### target info / settings

Show target details or change settings.

```bash
# Show info
skillshare target claude

# Change mode
skillshare target claude --mode symlink
skillshare target claude --mode merge

# Change target naming
skillshare target claude --target-naming standard
skillshare target claude --target-naming flat

skillshare sync  # Apply changes
```

## Sync Modes

| Mode | Behavior |
|------|----------|
| `merge` | Each skill symlinked individually. Preserves local skills. **Default.** |
| `copy` | Each skill copied as real files. For AI CLIs that can't follow symlinks. |
| `symlink` | Entire directory is one symlink. Exact copies everywhere. |

`target --mode` is the main compatibility control surface. Keep your global default simple, then override only where needed.

## Target Naming

| Naming | Behavior |
|--------|----------|
| `flat` | Nested skills flattened with `__` separators (e.g. `frontend__dev`). **Default.** |
| `standard` | Uses SKILL.md `name` field directly (e.g. `dev`). Follows the [Agent Skills spec](https://agentskills.io/specification). |

`target --target-naming` controls how skill directories are named in targets. In `standard` mode, skills with invalid or colliding names are warned and skipped. Ignored in symlink mode.

```bash
# Set target to copy mode (for Cursor, Copilot CLI, etc.)
skillshare target cursor --mode copy
skillshare sync  # Apply the change
```

### Mixed strategy example

```bash
# Keep default merge behavior for most targets
skillshare target claude --mode merge

# Compatibility-first for one target
skillshare target cursor --mode copy

# Exact mirror for another target
skillshare target codex --mode symlink

skillshare sync
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

Filters work in **merge and copy modes**. Patterns use Go `filepath.Match` syntax (`*`, `?`, `[...]`). In symlink mode, filters are ignored.

See [Configuration](/docs/reference/targets/configuration#include--exclude-target-filters) for pattern cheat sheet and scenarios.

:::tip
Target filters are one of three filtering layers. See [Filtering Reference](/docs/reference/filtering) for how they interact with `.skillignore` and SKILL.md `targets`.
:::

## Options

### target add

No additional options.

### target remove

| Flag | Description |
|------|-------------|
| `--all, -a` | Remove all targets |
| `--dry-run, -n` | Preview without making changes |

### target list

| Flag | Description |
|------|-------------|
| `--json` | Output as JSON |
| `--no-tui` | Disable interactive TUI, use plain text output |

### target info / settings

| Flag | Description |
|------|-------------|
| `--mode, -m <mode>` | Set sync mode (merge, copy, or symlink) |
| `--target-naming <naming>` | Set target naming (flat or standard) |
| `--add-include <pattern>` | Add an include filter pattern |
| `--add-exclude <pattern>` | Add an exclude filter pattern |
| `--remove-include <pattern>` | Remove an include filter pattern |
| `--remove-exclude <pattern>` | Remove an exclude filter pattern |

## Supported AI CLIs

skillshare auto-detects these during `init`:

| CLI | Default Path |
|-----|-------------|
| Claude Code | `~/.claude/skills` |
| Cursor | `~/.cursor/skills` |
| OpenCode | `~/.opencode/skills` |
| Windsurf | `~/.windsurf/skills` |
| Codex | `~/.openai-codex/skills` |
| Gemini CLI | `~/.gemini/skills` |
| Amp | `~/.amp/skills` |
| ... and 45+ more | See [supported targets](/docs/reference/targets/supported-targets) |

## Examples

```bash
# Add custom target
skillshare target add my-tool ~/my-tool/skills

# Check target status
skillshare target claude

# Switch to copy mode (for AI CLIs that can't read symlinks)
skillshare target cursor --mode copy
skillshare sync

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
| Sync mode | Merge, copy, or symlink | Merge, copy, or symlink (default merge) |
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

## See Also

- [sync](/docs/reference/commands/sync) — Sync skills to targets
- [status](/docs/reference/commands/status) — Show target status
- [Targets](/docs/reference/targets) — Target management guide
- [Project Skills](/docs/understand/project-skills) — Project mode concepts
