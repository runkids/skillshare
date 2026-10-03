---
sidebar_position: 2
---

# diff

Show differences between source and targets.

```bash
skillshare diff              # All targets (interactive TUI)
skillshare diff claude       # Specific target
skillshare diff agents       # Agent targets only
skillshare diff --stat       # File-level changes
skillshare diff --patch      # Full unified diff
```

```text
skillshare diff --no-tui

claude, claude-work, gemini, opencode, universal
  New       remotion-captions

cursor
  Local only  cursor-shortcuts
  New         remotion-captions

Extras
✓ commands  ~/.claude/commands · in sync
✓ rules     ~/.claude/rules · in sync
✓ rules     ~/.cursor/rules · in sync
✓ team      ~/.codex · in sync
✓ team      ~/.claude · in sync
✓ team      ~/.gemini · in sync
✓ team      ~/notes · in sync

! 6 targets: 6 to sync

Next
  skillshare sync     apply the changes
  skillshare collect  copy local-only skills into source
```

## Interactive TUI

On a TTY, `diff` opens an interactive view: targets on the left, and what differs for the selected target on the right, down to file-level diffs. The keys are listed at the bottom of the screen. Use `--no-tui`, or pipe the output, for plain text.

## When to Use

- See exactly what's different between source and a target before syncing
- Find skills that exist only in a target (local-only, not yet collected)
- Identify local copies that could be replaced by symlinks
- Inspect file-level changes with `--stat` or full text diff with `--patch`

## Example Output

```
claude
  Local override  local-copy
  Local only      my-local-skill
  New             another-skill, missing-skill

✓ cursor    in sync

! 2 targets: 1 to sync, 1 in sync

Next
  skillshare sync          apply the changes
  skillshare sync --force  also replace local copies
  skillshare collect       copy local-only skills into source
```

### Grouped Multi-Target Output

When multiple targets have identical diff results, they are grouped into a single block to reduce noise:

```
agents, claude
  New       skill-1, skill-2

cursor
  New       skill-1

✓ codex, copilot  in sync
```

Targets with different results (e.g. due to `include`/`exclude` filters) are still shown separately.

## Labels

| Label | Meaning | Action |
|-------|---------|--------|
| New | In source, missing in target | `sync` will add it |
| Restore | Was in target, deleted | `sync` will restore it |
| Modified | Content changed (copy mode) | `sync` will update it |
| Local override | Local copy instead of symlink | `sync --force` to replace |
| Orphan | In manifest but not in source | `sync` will prune it |
| Local only | Only in target, not in source | `collect` to import |

## File-Level Details

### `--stat`

Shows which files differ within each skill:

```bash
skillshare diff --stat
```

```
claude
  Modified  my-skill
            + new-file.md (120 bytes)
            ~ SKILL.md (840 → 912 bytes)
            - old-file.md (64 bytes)
```

### `--patch`

Shows full unified text diff for modified files:

```bash
skillshare diff --patch
```

```
claude
  Modified  my-skill
            ~ SKILL.md (840 → 912 bytes)
            --- SKILL.md
            - old line
            + new line
```

Both `--stat` and `--patch` imply `--no-tui` (plain text output).

## What Diff Shows

### Merge Mode Targets

For targets using merge mode (default):
- Lists skills in source not yet symlinked to target
- Shows skills that exist as local copies instead of symlinks
- Identifies local-only skills in target (not in source — preserved by sync)

### Copy Mode Targets

For targets using copy mode:
- Lists skills in source not yet managed (missing from manifest)
- Shows content changes via checksum comparison
- Shows orphan managed copies no longer in source (will be pruned on sync)
- Identifies local-only skills (not in source and not managed)

### Symlink Mode Targets

For targets using symlink mode:
- Simply checks if symlink points to correct source
- Shows "in sync" or warns about wrong symlink

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
# Shows: Local only  my-local-skill

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

## Agent Diff {#agent-diff}

Use the `agents` keyword to diff only agent targets:

```bash
skillshare diff agents             # All agent-capable targets
skillshare diff agents claude      # Specific target
skillshare diff agents --json      # JSON output
```

Agent diff shows missing agents (need sync), orphan symlinks (need prune), and local-only agent files. Only targets with an `agents` path configuration are included. See [Agents — Supported Targets](/docs/understand/agents#supported-targets) for the full list.

---

## Options

| Flag | Description |
|------|-------------|
| `--project, -p` | Use project mode |
| `--global, -g` | Use global mode |
| `--stat` | Show file-level changes (implies `--no-tui`) |
| `--patch` | Show full unified diff (implies `--no-tui`) |
| `--no-tui` | Plain text output (skip interactive TUI) |
| `--json` | Output as JSON (implies `--no-tui`) |

## JSON Output

```bash
skillshare diff --json
```

```json
{
  "targets": [
    {
      "name": "claude",
      "mode": "merge",
      "synced": false,
      "items": [
        {"action": "link", "name": "missing-skill", "reason": "not in target", "is_sync": true},
        {"action": "update", "name": "local-copy", "reason": "local override", "is_sync": true}
      ],
      "include": [],
      "exclude": []
    }
  ],
  "duration": "0.045s"
}
```

## See Also

- [sync](/docs/reference/commands/sync) — Sync to targets
- [collect](/docs/reference/commands/collect) — Import local skills
- [status](/docs/reference/commands/status) — Quick overview
