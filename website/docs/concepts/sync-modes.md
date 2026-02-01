---
sidebar_position: 3
---

# Sync Modes

How skillshare links source to targets.

## Overview

| Mode | Behavior | Use Case |
|------|----------|----------|
| `merge` | Each skill symlinked individually | **Default.** Preserves local skills. |
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

**When to use:**
- You want some skills only in specific AI CLIs
- You want to try local skills before syncing

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
mode: merge  # or symlink

targets:
  claude:
    path: ~/.claude/skills
    # inherits default mode

  codex:
    path: ~/.codex/skills
    mode: symlink  # override default
```

---

## Mode Comparison

| Aspect | Merge | Symlink |
|--------|-------|---------|
| Local skills preserved | ✅ Yes | ❌ No |
| All targets identical | ❌ Can differ | ✅ Yes |
| Orphan cleanup needed | ✅ Yes | ❌ No |
| Delete safety | ✅ Safe | ⚠️ Caution |
| Complexity | Higher | Lower |

---

## Orphaned Symlinks

In merge mode, when you remove a skill from source, the target symlinks become "orphaned" (pointing to nothing).

**Handling:** `skillshare sync` automatically prunes orphaned symlinks:

```
$ skillshare sync
✓ claude: merged (5 linked, 2 local, 0 updated, 1 pruned)
                                                  ^^^^^^^^
```

---

## Related

- [Source & Targets](./source-and-targets) — Core model
- [Target Management](/docs/targets) — Managing targets
- [Commands: sync](/docs/commands/sync) — Sync command details
