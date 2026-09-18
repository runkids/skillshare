# Sync, Collect, Commit, Push & Pull

| Command | Direction | Project? |
|---------|-----------|:--------:|
| `sync` | Source → Targets | ✓ (auto) |
| `collect` | Targets → Source | ✓ (auto) |
| `push` | Source → Remote | ✗ |
| `pull` | Remote → Source → Targets | ✗ |

**Auto-detection:** `sync` and `collect` auto-detect project mode when `.skillshare/config.yaml` or `skillshare/config.yaml` is found. Use `-g` to force global.

## sync

Distribute skills from source to all targets using each target's sync mode (`merge` / `copy` / `symlink`).

```bash
skillshare sync                # Execute (auto-detects mode)
skillshare sync --all          # Sync skills + agents + extras + MCP
skillshare sync --dry-run      # Preview
skillshare sync --force        # Override conflicts
skillshare sync --json         # JSON output
skillshare sync -g             # Force global mode
```

### Sync modes (quick reference)

- `merge` (default): per-skill symlinks, preserves local target skills.
- `copy`: real-file copies with `.skillshare-manifest.json` tracking managed entries.
- `symlink`: whole target directory symlinked to source.

Copy mode note:
- `skillshare doctor` duplicate checks ignore manifest-managed copy entries (expected mirrors of source).
- Duplicate warnings in copy mode are for true local copies that collide with source skill names.

## Agents and MCP

`skillshare sync agents` distributes native agents. `skillshare sync mcp` applies
MCP settings; see [native-agents.md](native-agents.md) and [mcp.md](mcp.md).

## sync plugins

`skillshare sync plugins [name]` is an alias for `plugin sync [name]`.
It uses native clients to install selected bindings and uninstall deselected ones,
retaining their definitions. `plugin enable` / `plugin disable` only save selection.
Plugins are excluded from `sync --all`; ordinary sync flags such as `--force` do
not apply. See [plugins.md](plugins.md) for compatibility and recovery.

```bash
skillshare sync plugins --dry-run --json
skillshare sync plugins demo --target claude --no-tui
```

## sync extras

Sync non-skill resources (rules, commands, prompts) to arbitrary directories. Supports both global and project mode.

```bash
skillshare sync extras            # Sync all configured extras
skillshare sync extras --dry-run  # Preview
skillshare sync extras --force    # Overwrite conflicts
```

Config example:
```yaml
extras:
  - name: rules
    targets:
      - path: ~/.claude/rules
      - path: ~/.cursor/rules
        mode: copy
```

Source: `~/.config/skillshare/extras/<name>/` (global) or `.skillshare/extras/<name>/` (project). Modes: `merge` (default, per-file symlinks), `copy`, `symlink`.

For full extras management (`init`, `list`, `remove`, `collect`), see [extras.md](extras.md).

## collect

Import skills or agents from target(s) to source.

```bash
# Global
skillshare collect claude      # From specific target
skillshare collect --all       # From all targets
skillshare collect --dry-run   # Preview
skillshare collect claude --json   # JSON output; existing items still require --force to overwrite
skillshare collect agents claude   # Collect agents instead of skills

# Project (auto-detected or -p)
skillshare collect claude     # From project target
skillshare collect --all           # All project targets
skillshare collect --all --force   # Skip confirmation
skillshare collect -p --json       # Project JSON output
skillshare collect -p agents --json   # Project agent JSON output
```

## commit and Git scope

`skillshare commit -m "Update resources"` creates a local checkpoint without pushing.
Global `commit`, `push`, and `pull` use `git_root`: `skills` (default), `agents`,
`extras`, or `root`. Inspect the configured repository rather than assuming it is
always the skills directory. A Git-root mismatch is an error; do not relocate `.git`
or initialize a replacement repository as an automatic recovery step.

## push

Git commit and push source to remote. **Global mode only.**

```bash
skillshare push                # Default message
skillshare push -m "message"   # Custom message
skillshare push --dry-run      # Preview
```

**Project mode:** Use `git push` directly on the project repo.

## pull

Git pull from remote and sync to all targets. **Global mode only.**

```bash
skillshare pull                # Pull + sync
skillshare pull --dry-run      # Preview
```

**Project mode:** Use `git pull` directly, then `skillshare sync`.

## Common Workflows

**Local editing:** Edit the source → `sync`. Editing a merge/symlink target also edits
the source; editing a copy target does not. Inspect `diff` before collecting copies.

**Import local changes:** `collect <target>` → `sync`

**Cross-machine sync (global):** Machine A: `push` → Machine B: `pull`

**Team sharing (project):** Edit `.skillshare/skills/` → `git commit && git push` → Team: `git pull && skillshare install -p && skillshare sync`
