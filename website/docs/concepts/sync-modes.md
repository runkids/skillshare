---
sidebar_position: 3
---

# Sync Modes

How skillshare links source to targets.

:::tip When does this matter?
Choose merge mode (default) when you want per-skill symlinks and to preserve local skills in targets. Choose copy mode when the AI CLI cannot follow symlinks (e.g. Cursor, Copilot CLI). Choose symlink mode when you want the entire directory linked and don't need local target skills.
:::

## Overview

| Mode | Behavior | Use Case |
|------|----------|----------|
| `merge` | Each skill symlinked individually | **Default.** Preserves local skills. |
| `copy` | Each skill copied as real files | AI CLIs that can't follow symlinks. |
| `symlink` | Entire directory is one symlink | Exact copies everywhere. |

---

## Merge Mode (Default)

Each skill is symlinked individually. Local skills in the target are preserved.

```
Source                          Target (claude)
─────────────────────────────────────────────────────────────
skills/                         ~/.claude/skills/
├── my-skill/        ────────►  ├── my-skill/ → (symlink)
├── another/         ────────►  ├── another/  → (symlink)
└── ...                         ├── local-only/  (preserved)
                                └── ...
```

**Advantages:**
- Keep target-specific skills (not synced)
- Mix installed and local skills
- Granular control
- Per-target include/exclude filtering

**When to use:**
- You want some skills only in specific AI CLIs
- You want to try local skills before syncing
- You want one source but different skill subsets per target

### Filter strategy in merge mode

`include` and `exclude` are evaluated per target in this order:
1. `include` keeps matching names
2. `exclude` removes from that kept set

Quick choices:
- Use `include` when the target should get only a small subset
- Use `exclude` when the target should get almost everything
- Use `include + exclude` when you need a broad subset with explicit carve-outs

Behavior when rules change:
- Previously synced source-linked entries that become filtered out are removed on next `sync`
- Existing local non-symlink folders in target are preserved

See [Target Configuration](/docs/targets/configuration#include--exclude-target-filters) for full examples.

---

## Copy Mode

Each skill is copied as real files to the target directory. A `.skillshare-manifest.json` file tracks which skills are managed, so local skills are preserved.

```
Source                          Target (cursor)
─────────────────────────────────────────────────────────────
skills/                         ~/.cursor/skills/
├── my-skill/        ────copy►  ├── my-skill/    (real files)
├── another/         ────copy►  ├── another/     (real files)
└── ...                         ├── local-only/  (preserved)
                                └── .skillshare-manifest.json
```

**Advantages:**
- Works with AI CLIs that cannot read symlinks (Cursor, Copilot CLI, etc.)
- Preserves local skills (same as merge mode)
- Per-target include/exclude filtering
- Checksum-based skip: unchanged skills are not re-copied

**When to use:**
- Your AI CLI reports "skill not found" or can't read symlinked skills
- You want the same filtering behavior as merge mode but with real files

### How updates work

On each `skillshare sync`, the checksum of each source skill is compared to the value stored in the manifest:

- **Same checksum** → skill is skipped (fast)
- **Different checksum** → skill is overwritten with the new version
- **`--force`** → all managed skills are overwritten regardless of checksum

### Manifest lifecycle

- Created on first `sync` with copy mode
- Removed automatically when switching to merge or symlink mode (non-copy)
- If manually deleted, the next `sync` rebuilds it

---

## Symlink Mode

The entire target directory is a single symlink to source.

```
Source                          Target (claude)
─────────────────────────────────────────────────────────────
skills/              ────────►  ~/.claude/skills → (symlink to source)
├── my-skill/
├── another/
└── ...
```

**Advantages:**
- All targets are identical
- Simpler to manage
- No orphaned symlinks

**When to use:**
- You want all AI CLIs to have exactly the same skills
- You don't need target-specific skills

**Warning:** In symlink mode, deleting through target deletes source!
```bash
rm -rf ~/.claude/skills/my-skill  # ❌ Deletes from SOURCE
skillshare target remove claude   # ✅ Safe way to unlink
```

---

## Changing Mode

### Per-target

```bash
# Switch to copy mode (for AI CLIs that can't read symlinks)
skillshare target cursor --mode copy
skillshare sync

# Switch to symlink mode
skillshare target claude --mode symlink
skillshare sync

# Switch back to merge mode
skillshare target claude --mode merge
skillshare sync
```

### Default mode

Set in config for new targets:

```yaml
# ~/.config/skillshare/config.yaml
mode: merge  # or symlink or copy

targets:
  claude:
    path: ~/.claude/skills
    # inherits default mode

  cursor:
    path: ~/.cursor/skills
    mode: copy  # real files for Cursor

  codex:
    path: ~/.codex/skills
    mode: symlink  # override default
```

---

## Mode Comparison

| Aspect | Merge | Copy | Symlink |
|--------|-------|------|---------|
| Local skills preserved | ✅ Yes | ✅ Yes | ❌ No |
| Symlink-compatible | ✅ Yes | ❌ Real files | ✅ Yes |
| All targets identical | ❌ Can differ | ❌ Can differ | ✅ Yes |
| Per-target include/exclude | ✅ Yes | ✅ Yes | ❌ Ignored |
| Orphan cleanup needed | ✅ Yes | ✅ Yes | ❌ No |
| Delete safety | ✅ Safe | ✅ Safe | ⚠️ Caution |
| Disk usage | Low (symlinks) | Higher (copies) | Low (symlinks) |

---

## Orphan Cleanup

In merge mode, when you remove a skill from source, the target symlinks become "orphaned" (pointing to nothing). In copy mode, the manifest tracks which skills are managed, so orphan copies are identified and removed.

**Handling:** `skillshare sync` automatically prunes orphans in both modes:

```
$ skillshare sync
✓ claude: merged (5 linked, 2 local, 0 updated, 1 pruned)
✓ cursor: copied (3 new, 2 skipped, 0 updated, 1 pruned)
```

---

## See Also

- [sync](/docs/commands/sync) — Run sync to apply mode changes
- [target](/docs/commands/target) — Change a target's sync mode
- [Source & Targets](./source-and-targets.md) — The core architecture
- [Configuration](/docs/targets/configuration) — Per-target settings
