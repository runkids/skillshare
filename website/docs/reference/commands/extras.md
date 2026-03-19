---
sidebar_position: 2
---

# extras

Manage non-skill resources (rules, commands, prompts, etc.) that are synced alongside skills.

## Overview

Extras are additional resource types managed by skillshare — think of them as "skills for non-skill content." Common use cases include syncing AI rules, editor commands, or prompt templates across tools.

Each extra has:
- A **name** (e.g., `rules`, `prompts`, `commands`)
- A **source directory** — configurable via `extras_source` or per-extra `source`, defaults to `~/.config/skillshare/extras/<name>/` (global) or `.skillshare/extras/<name>/` (project)
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
| `--source <path>` | Custom source directory for this extra (overrides `extras_source` and default) |
| `--project, -p` | Create in project config (`.skillshare/`) |
| `--global, -g` | Create in global config |

**Examples:**

```bash
# Sync rules to Claude and Cursor
skillshare extras init rules --target ~/.claude/rules --target ~/.cursor/rules

# Use a custom source directory
skillshare extras init rules --target ~/.claude/rules --source ~/company-shared/rules

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
| `--json` | JSON output (includes `source_type`: `per-extra` / `extras_source` / `default`) |
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
| `N` | Create new extra |
| `X` | Remove extra (with confirmation) |
| `S` | Sync extra to target(s) |
| `C` | Collect from target(s) |
| `M` | Change sync mode of a target |
| `Ctrl+U/D` | Scroll detail panel |
| `q` / `Ctrl+C` | Quit |

The color bar on each row reflects aggregate sync status: cyan = all synced, yellow = drift, red = not synced, gray = no source.

For extras with multiple targets, `S`, `C`, and `M` open a target sub-menu. `S` and `C` allow selecting all targets at once; `M` requires picking a specific target.

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

### `extras mode`

Change the sync mode of an extra's target.

```bash
skillshare extras mode <name> --mode <mode> [--target <path>] [-p|-g]
# Shorthand (no "mode" subcommand):
skillshare extras <name> --mode <mode> [--target <path>]
```

**Options:**

| Flag | Description |
|------|-------------|
| `--mode <mode>` | New sync mode: `merge`, `copy`, or `symlink` (required) |
| `--target <path>` | Target directory path (optional if extra has only one target) |
| `--project, -p` | Use project-mode extras (`.skillshare/`) |
| `--global, -g` | Use global extras (`~/.config/skillshare/`) |

**Examples:**

```bash
# Change rules mode (single target — auto-resolved)
skillshare extras rules --mode copy

# Specify target explicitly (required for multi-target extras)
skillshare extras mode rules --target ~/.claude/rules --mode copy

# Change to symlink in project mode
skillshare extras mode commands --target ~/.cursor/commands --mode symlink -p
```

Also available via the TUI (`M` key) and Web UI (mode dropdown on each target).

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

When switching modes (e.g., from `merge` to `copy`), the next `sync` automatically replaces existing symlinks with the new mode's format. No `--force` is needed — symlinks are always safe to replace. Regular files created locally require `--force` to overwrite.

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
# Optional: set a global default extras source directory
extras_source: ~/my-extras

extras:
  - name: rules
    source: ~/company-shared/rules    # optional per-extra override
    targets:
      - path: ~/.claude/rules
      - path: ~/.cursor/rules
        mode: copy
  - name: prompts
    targets:
      - path: ~/.claude/prompts
```

### Source Resolution Priority

The source directory for each extra is resolved with three-level priority:

1. **Per-extra `source`** (highest) — exact path, used as-is
2. **`extras_source`** — `<extras_source>/<name>/`
3. **Default** — `~/.config/skillshare/extras/<name>/` (global) or `.skillshare/extras/<name>/` (project)

The `extras list --json` output includes a `source_type` field (`per-extra`, `extras_source`, or `default`) indicating which level resolved the path.

:::tip Auto-populated
`extras_source` is automatically set to the default path (`~/.config/skillshare/extras/`) when you run `skillshare init` or create your first extra with `extras init`. You don't need to add it manually.
:::

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

# 1b. Or with a custom source directory
skillshare extras init rules --target ~/.claude/rules --source ~/my-rules

# 2. Add files to the source directory
# (edit the resolved source dir — check with: skillshare extras list --json)

# 3. Sync to targets
skillshare sync extras

# 4. List status (source_type shows where each extra's source is resolved from)
skillshare extras list

# 5. Collect a file edited in a target back to source
skillshare extras collect rules --from ~/.claude/rules
```

---

## See Also

- [sync](/docs/reference/commands/sync#sync-extras) — Sync extras to targets
- [status](/docs/reference/commands/status) — Show extras file and target counts
- [Configuration](/docs/reference/targets/configuration#extras) — Extras config reference
