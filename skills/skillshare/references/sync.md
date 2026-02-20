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
skillshare sync --dry-run      # Preview
skillshare sync --force        # Override conflicts
skillshare sync -g             # Force global mode
```

### Sync modes (quick reference)

- `merge` (default): per-skill symlinks, preserves local target skills.
- `copy`: real-file copies with `.skillshare-manifest.json` tracking managed entries.
- `symlink`: whole target directory symlinked to source.

Copy mode note:
- `skillshare doctor` duplicate checks ignore manifest-managed copy entries (expected mirrors of source).
- Duplicate warnings in copy mode are for true local copies that collide with source skill names.

## collect

Import skills from target(s) to source.

```bash
# Global
skillshare collect claude      # From specific target
skillshare collect --all       # From all targets
skillshare collect --dry-run   # Preview

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
