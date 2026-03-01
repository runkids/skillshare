---
sidebar_position: 2
---

# diff

Show differences between source and targets.

```bash
skillshare diff              # All targets (interactive TUI)
skillshare diff claude       # Specific target
skillshare diff --stat       # File-level changes
skillshare diff --patch      # Full unified diff
```

![diff demo](/img/diff-demo.png)

## Interactive TUI

On a TTY, `diff` launches an interactive TUI with a left-right panel layout:

- **Left panel** — Target list with status icons (`✓` synced, `!` has diffs, `✗` error)
- **Right panel** — Detail view for the selected target (mode, filters, categorized diffs)
- Press **Enter** to expand file-level diff for a skill
- Press **/** to filter targets, **Ctrl+d/u** to scroll detail, **q** to quit

Use `--no-tui` for plain text output, or pipe to disable TUI automatically.

## When to Use

- See exactly what's different between source and a target before syncing
- Find skills that exist only in a target (local-only, not yet collected)
- Identify local copies that could be replaced by symlinks
- Inspect file-level changes with `--stat` or full text diff with `--patch`

## Example Output

```
claude
  + New 2 skills:
      missing-skill
      another-skill
  ! Local Override 1 skill:
      local-copy
  ← Local Only 1 skill:
      my-local-skill

  2 new, 1 local override, 1 local only

cursor: fully synced
```

### Grouped Multi-Target Output

When multiple targets have identical diff results, they are grouped into a single block to reduce noise:

```
claude, agents
  + New 2 skills:
      skill-1
      skill-2

cursor
  + New 1 skill:
      skill-1

codex, copilot: fully synced
```

Targets with different results (e.g. due to `include`/`exclude` filters) are still shown separately.

## Symbols

| Symbol | Label | Meaning | Action |
|--------|-------|---------|--------|
| `+` | New | In source, missing in target | `sync` will add it |
| `+` | Restore | Was in target, deleted | `sync` will restore it |
| `~` | Modified | Content changed (copy mode) | `sync` will update it |
| `!` | Local Override | Local copy instead of symlink | `sync --force` to replace |
| `-` | Orphan | In manifest but not in source | `sync` will prune it |
| `←` | Local Only | Only in target, not in source | `collect` to import |

## File-Level Details

### `--stat`

Shows which files differ within each skill:

```bash
skillshare diff --stat
```

```
claude
  ~ Modified 1 skill:
      my-skill
        + new-file.md
        ~ SKILL.md
        - old-file.md
```

### `--patch`

Shows full unified text diff for modified files:

```bash
skillshare diff --patch
```

```
claude
  ~ Modified 1 skill:
      my-skill
        --- SKILL.md
        - old line
        + new line
```

Both `--stat` and `--patch` imply `--no-tui` (plain text output).

## What Diff Shows

### Merge Mode Targets

For targets using merge mode (default):
- Lists skills in that target's expected set (after `include`/`exclude`) not yet synced
- Shows skills that exist as local copies instead of symlinks
- Identifies orphan directories tracked by manifest (will be pruned on sync)
- Identifies local-only skills in target (not in manifest — preserved)

### Copy Mode Targets

For targets using copy mode:
- Lists skills in source not yet managed (missing from manifest)
- Shows content changes via checksum comparison
- Shows orphan managed copies no longer in source (will be pruned on sync)
- Identifies local-only skills (not in source and not managed)

### Symlink Mode Targets

For targets using symlink mode:
- Simply checks if symlink points to correct source
- Shows "Fully synced" or warns about wrong symlink

## Use Cases

### Before Sync

Check what will change:

```bash
skillshare diff
# See what sync will do, then:
skillshare sync
```

### Finding Local Skills

Discover skills you created directly in a target:

```bash
skillshare diff claude
# Shows: ← Local Only 1 skill: my-local-skill

skillshare collect claude  # Import to source
```

### Inspecting Changes

See exactly what changed in a skill before syncing:

```bash
skillshare diff --patch claude   # Full text diff
skillshare diff --stat claude    # File-level summary
```

### Troubleshooting

When sync status shows issues:

```bash
skillshare status          # Shows "needs sync"
skillshare diff claude     # See exactly what's different
skillshare sync            # Fix it
```

## Options

| Flag | Description |
|------|-------------|
| `--project, -p` | Use project mode |
| `--global, -g` | Use global mode |
| `--stat` | Show file-level changes (implies `--no-tui`) |
| `--patch` | Show full unified diff (implies `--no-tui`) |
| `--no-tui` | Plain text output (skip interactive TUI) |

## See Also

- [sync](/docs/reference/commands/sync) — Sync to targets
- [collect](/docs/reference/commands/collect) — Import local skills
- [status](/docs/reference/commands/status) — Quick overview
