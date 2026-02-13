---
sidebar_position: 2
---

# diff

Show differences between source and targets.

```bash
skillshare diff              # All targets
skillshare diff claude       # Specific target
```

![diff demo](/img/diff-demo.png)

## Example Output

```
claude
  + missing-skill       missing
  ~ local-copy          local copy (sync --force to replace)
  - local-only          local only

  Run 'sync' to add missing, 'sync --force' to replace local copies
  Run 'collect claude' to import local-only skills to source
```

## Symbols

| Symbol | Meaning | Action |
|--------|---------|--------|
| `+` | In source, missing in target | `sync` will add it |
| `~` | In both, but target has local copy (not symlink) | `sync --force` to replace |
| `-` | Only in target, not in source | `collect` to import |

## What Diff Shows

### Merge Mode Targets

For targets using merge mode (default):
- Lists skills in that target's expected set (after `include`/`exclude`) not yet synced
- Shows skills that exist as local copies instead of symlinks
- Identifies local-only skills in target

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
# Shows: - my-local-skill    local only

skillshare collect claude  # Import to source
```

### Troubleshooting

When sync status shows issues:

```bash
skillshare status          # Shows "needs sync"
skillshare diff claude     # See exactly what's different
skillshare sync            # Fix it
```

## Related

- [sync](/docs/commands/sync) — Sync to targets
- [collect](/docs/commands/collect) — Import local skills
- [status](/docs/commands/status) — Quick overview
