# Sync, Collect, Pull & Push Commands

## Command Overview

| Type | Command | Direction |
|------|---------|-----------|
| **Local sync** | `sync` / `collect` | Source ↔ Targets |
| **Remote sync** | `push` / `pull` | Source ↔ Git Remote |

## sync

Pushes skills from source to all targets.

```bash
skillshare sync                # Execute sync
skillshare sync --dry-run      # Preview only
```

## collect

Brings skills from target(s) to source.

```bash
skillshare collect claude      # Collect from specific target
skillshare collect --all       # Collect from all targets
skillshare collect --dry-run   # Preview only
```

## pull

Pulls from git remote and syncs to all targets.

```bash
skillshare pull                # Pull from git remote + sync all
skillshare pull --dry-run      # Preview only
```

## push

Commits and pushes source to git remote.

```bash
skillshare push                # Default commit message
skillshare push -m "message"   # Custom commit message
skillshare push --dry-run      # Preview only
```

## Workflows

**Local workflow:**
1. Create skill in any target (e.g., `~/.claude/skills/my-skill/`)
2. `skillshare collect claude` - bring to source
3. `skillshare sync` - distribute to all targets

**Cross-machine workflow:**
1. Machine A: `skillshare push` - commit and push to remote
2. Machine B: `skillshare pull` - pull from remote + sync
