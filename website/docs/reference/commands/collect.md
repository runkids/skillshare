---
sidebar_position: 1
---

# collect

Collect local skills from targets back to source.

```bash
skillshare collect claude           # From specific target
skillshare collect --all            # From all targets
skillshare collect claude --dry-run # Preview
```

## When to Use

Use `collect` when you've created or edited skills directly in a target directory (e.g., `~/.claude/skills/`) and want to:

1. Add them to your source for sharing
2. Sync them to other AI CLIs
3. Back them up with git

## What Happens

```mermaid
flowchart TD
    CMD["skillshare collect claude"]
    FIND["1. Find local skills in target"]
    CONFIRM["2. Confirm collection"]
    COPY["3. Copy to source"]
    SYMLINK["4. Replace with symlink"]
    CMD --> FIND --> CONFIRM --> COPY --> SYMLINK
```

## Options

| Flag | Description |
|------|-------------|
| `--all, -a` | Collect from all targets |
| `--force, -f` | Overwrite existing skills in source |
| `--dry-run, -n` | Preview without making changes |

## Example Output

```bash
$ skillshare collect claude

Local skills found
  ℹ new-skill       [claude] ~/.claude/skills/new-skill
  ℹ another-skill   [claude] ~/.claude/skills/another-skill

Collect these skills to source? [y/N]: y

Collecting skills
  ✓ new-skill: copied to source
  ✓ another-skill: copied to source

Run 'skillshare sync' to distribute to all targets
```

## Handling Conflicts

If a skill already exists in source:

```bash
$ skillshare collect claude

Collecting skills
  ⚠ my-skill: skipped (already exists in source, use --force to overwrite)

# To overwrite:
$ skillshare collect claude --force
```

## Workflow

Typical workflow after creating a skill in a target:

```bash
# 1. Create skill in Claude
# (edit ~/.claude/skills/my-new-skill/SKILL.md)

# 2. Collect to source
skillshare collect claude

# 3. Sync to all other targets
skillshare sync

# 4. Commit to git (optional)
skillshare push -m "Add my-new-skill"
```

## See Also

- [sync](/docs/reference/commands/sync) — Sync from source to targets
- [diff](/docs/reference/commands/diff) — See local-only skills
- [push](/docs/reference/commands/push) — Push to git remote
