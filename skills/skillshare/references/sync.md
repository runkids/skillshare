# Sync, Collect, Push & Pull

| Command | Direction | Project? |
|---------|-----------|:--------:|
| `sync` | Source → Targets | ✓ (auto) |
| `collect` | Targets → Source | ✓ (auto) |
| `push` | Source → Remote | ✗ |
| `pull` | Remote → Source → Targets | ✗ |

**Auto-detection:** `sync` and `collect` auto-detect project mode when `.skillshare/config.yaml` exists. Use `-g` to force global.

## sync

Distribute skills from source to all targets using each target's sync mode (`merge` / `copy` / `symlink`).

```bash
skillshare sync                # Execute (auto-detects mode)
skillshare sync --all          # Sync skills + extras
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

Import skills from target(s) to source.

```bash
# Global
skillshare collect claude      # From specific target
skillshare collect --all       # From all targets
skillshare collect --dry-run   # Preview
skillshare collect claude --json   # JSON output (implies --force)

# Project (auto-detected or -p)
skillshare collect claude     # From project target
skillshare collect --all           # All project targets
skillshare collect --all --force   # Skip confirmation
```

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

**Local editing:** Edit skill anywhere → `sync` (symlinks update source automatically)

**Import local changes:** `collect <target>` → `sync`

**Cross-machine sync (global):** Machine A: `push` → Machine B: `pull`

**Team sharing (project):** Edit `.skillshare/skills/` → `git commit && git push` → Team: `git pull && skillshare install -p && skillshare sync`
