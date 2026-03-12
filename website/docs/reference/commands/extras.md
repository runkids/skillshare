---
sidebar_position: 2
---

# extras

Manage non-skill resources (rules, commands, prompts, etc.) that are synced alongside skills.

## Overview

Extras are additional resource types managed by skillshare — think of them as "skills for non-skill content." Common use cases include syncing AI rules, editor commands, or prompt templates across tools.

Each extra has:
- A **name** (e.g., `rules`, `prompts`, `commands`)
- A **source directory** under `~/.config/skillshare/extras/<name>/` (global) or `.skillshare/extras/<name>/` (project)
- One or more **targets** where files are synced to

## Commands

### `extras init`

Create a new extra resource type.

```bash
# Interactive wizard
skillshare extras init

# CLI flags
skillshare extras init <name> --target <path> [--target <path2>] [--mode <mode>]
```

**Options:**

| Flag | Description |
|------|-------------|
| `--target <path>` | Target directory path (repeatable) |
| `--mode <mode>` | Sync mode: `merge` (default), `copy`, or `symlink` |
| `--project, -p` | Create in project config (`.skillshare/`) |
| `--global, -g` | Create in global config |

**Examples:**

```bash
# Sync rules to Claude and Cursor
skillshare extras init rules --target ~/.claude/rules --target ~/.cursor/rules

# Project-scoped extra with copy mode
skillshare extras init prompts --target .claude/prompts --mode copy -p
```

### `extras list`

List all configured extras and their sync status. Launches an interactive TUI by default.

```bash
skillshare extras list [--json] [--no-tui] [-p|-g]
```

**Options:**

| Flag | Description |
|------|-------------|
| `--json` | JSON output |
| `--no-tui` | Disable interactive TUI, use plain text output |
| `--project, -p` | Use project-mode extras (`.skillshare/`) |
| `--global, -g` | Use global extras (`~/.config/skillshare/`) |

#### Interactive TUI

The TUI provides a split-pane interface with extras list on the left and detail panel on the right. Key bindings:

| Key | Action |
|-----|--------|
| `↑↓` | Navigate list |
| `/` | Filter by name |
| `Enter` | Content viewer (browse source files) |
| `X` | Remove extra (with confirmation) |
| `S` | Sync extra to target(s) |
| `C` | Collect from target(s) |
| `Ctrl+U/D` | Scroll detail panel |
| `q` / `Ctrl+C` | Quit |

The color bar on each row reflects aggregate sync status: cyan = all synced, yellow = drift, red = not synced, gray = no source.

For extras with multiple targets, `S` and `C` open a target sub-menu to select a specific target or all targets.

The TUI can be permanently disabled with `skillshare tui off`.

#### Plain text output

When TUI is disabled (via `--no-tui`, `skillshare tui off`, or piped output):

```
$ skillshare extras list --no-tui

Rules            ~/.config/skillshare/extras/rules/  (2 files)
  ✔ ~/.claude/rules   merge   synced
  ✔ ~/.cursor/rules   copy    synced

Prompts          ~/.config/skillshare/extras/prompts/  (1 file)
  ✔ ~/.claude/prompts  merge  synced
```

### `extras remove`

Remove an extra from configuration.

```bash
skillshare extras remove <name> [--force] [-p|-g]
```

Source files and synced targets are not deleted — only the config entry is removed.

### `extras collect`

Collect local files from a target back into the extras source directory. Files are copied to source and replaced with symlinks.

```bash
skillshare extras collect <name> [--from <path>] [--dry-run] [-p|-g]
```

**Options:**

| Flag | Description |
|------|-------------|
| `--from <path>` | Target directory to collect from (required if multiple targets) |
| `--dry-run` | Show what would be collected without making changes |

**Example:**

```bash
# Collect rules from Claude back to source
skillshare extras collect rules --from ~/.claude/rules

# Preview what would be collected
skillshare extras collect rules --from ~/.claude/rules --dry-run
```

---

## Sync Modes

| Mode | Behavior |
|------|----------|
| `merge` (default) | Per-file symlinks from target to source |
| `copy` | Per-file copies |
| `symlink` | Entire directory symlink |

---

## Directory Structure

```
~/.config/skillshare/
├── config.yaml          # extras config lives here
├── skills/              # skill source
└── extras/              # extras source root
    ├── rules/           # extras/rules/ source files
    │   ├── coding.md
    │   └── testing.md
    └── prompts/
        └── review.md
```

---

## Configuration

In `config.yaml`:

```yaml
extras:
  - name: rules
    targets:
      - path: ~/.claude/rules
      - path: ~/.cursor/rules
        mode: copy
  - name: prompts
    targets:
      - path: ~/.claude/prompts
```

---

## Syncing

Extras are synced with:

```bash
skillshare sync extras        # sync extras only
skillshare sync --all         # sync skills + extras together
```

See [sync extras](/docs/reference/commands/sync#sync-extras) for full sync documentation including `--json`, `--dry-run`, and `--force` options.

---

## Workflow

```bash
# 1. Create a new extra
skillshare extras init rules --target ~/.claude/rules --target ~/.cursor/rules

# 2. Add files to the source directory
# (edit ~/.config/skillshare/extras/rules/coding.md)

# 3. Sync to targets
skillshare sync extras

# 4. List status
skillshare extras list

# 5. Collect a file edited in a target back to source
skillshare extras collect rules --from ~/.claude/rules
```

---

## See Also

- [sync](/docs/reference/commands/sync#sync-extras) — Sync extras to targets
- [status](/docs/reference/commands/status) — Show extras file and target counts
- [Configuration](/docs/reference/targets/configuration#extras) — Extras config reference
