---
sidebar_position: 3
---

# Sync Modes

How skillshare links source to targets.

:::tip When does this matter?
Choose merge mode (default) when you want per-skill symlinks and to preserve local skills in targets. Choose copy mode when you need real files instead of symlinks (portability, CI, or personal preference). Choose symlink mode when you want the entire directory linked and don't need local target skills.
:::

## Overview

| Mode | Behavior | Use Case |
|------|----------|----------|
| `merge` | Each skill symlinked individually | **Default.** Preserves local skills. |
| `copy` | Each skill copied as real files | Portability, CI/sandboxed environments, or when you prefer real files over symlinks. |
| `symlink` | Entire directory is one symlink | Exact copies everywhere. |

## Decision Matrix (Neutral)

Use this table to pick based on your constraints, not target brand names:

| Decision axis | `merge` | `copy` | `symlink` |
|---|---|---|---|
| Compatibility across different AI CLIs | Medium | High | Low–Medium |
| Edit-once immediate reflection | High | Low (requires `sync`) | High |
| Disk usage | Low | High | Low |
| Safety against accidental delete-from-target | High | High | Low |
| Operational simplicity | Medium | Medium | High |
| Per-target filtering (`include`/`exclude`) | Yes | Yes | No |

If you are unsure, start with `merge` and switch specific targets to `copy` as needed.

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
                                └── .skillshare-manifest.json
```

**Advantages:**
- Keep target-specific skills (not synced)
- Mix installed and local skills
- Granular control
- Per-target include/exclude filtering
- Manifest-based orphan cleanup (safely removes non-symlink residue after uninstall)

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

Each skill is copied as real files to the target directory. A `.skillshare-manifest.json` file tracks which skills are managed and their checksums, so local skills are preserved.

```
Source                          Target (cursor)
─────────────────────────────────────────────────────────────
skills/                         ~/.cursor/skills/
├── my-skill/        ────copy►  ├── my-skill/    (real files)
├── another/         ────copy►  ├── another/     (real files)
└── ...                         ├── local-only/  (preserved)
                                └── .skillshare-manifest.json
```

### Why copy mode?

Even when your AI CLI handles symlinks correctly, copy mode provides value:

- **Defensive design** — not every AI CLI guarantees symlink support, especially on Windows where symlink behavior varies by platform and permission level
- **Sandboxed environments** — strict CI pipelines, containers, and air-gapped setups may not follow symlinks across filesystem boundaries
- **User preference** — some users and teams simply prefer real files over symlinks for transparency and portability

**Advantages:**
- Works everywhere — no symlink support required from the AI CLI or OS
- Preserves local skills (same as merge mode)
- Per-target include/exclude filtering
- Checksum-based skip: unchanged skills are not re-copied

**When to use:**
- Your AI CLI reports "skill not found" or cannot read symlinked skills
- You want to vendor skills into a project repo — copy mode in project mode lets the team commit real skill files to git, so teammates don't need skillshare installed
- You need self-contained skill directories that work without a central source (portable setups, CI pipelines, air-gapped environments)
- You want the same filtering behavior as merge mode but with real files
- Common first candidates for `copy`: `cursor`, `antigravity`, `copilot`, `opencode`

### How updates work

On each `skillshare sync`, the checksum of each source skill is compared to the value stored in the manifest:

- **Same checksum** → skill is skipped (fast)
- **Different checksum** → skill is overwritten with the new version
- **`--force`** → all managed skills are overwritten regardless of checksum

### Manifest lifecycle

Both merge and copy modes write `.skillshare-manifest.json` to track managed skills:

- **Merge mode**: records skill names with value `"symlink"` — used to safely prune orphan real directories (e.g., copy-mode residue) after uninstall
- **Copy mode**: records skill names with SHA-256 checksums — used for incremental sync and orphan detection
- Removed automatically when switching to symlink mode
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

### By-target overrides (recommended)

You do not need one global mode for every target. A common pattern is:

```yaml
mode: merge
targets:
  claude:
    path: ~/.claude/skills
    # inherits merge
  cursor:
    path: ~/.cursor/skills
    mode: copy
  codex:
    path: ~/.codex/skills
    mode: symlink
```

Use per-target overrides when one target needs compatibility-first behavior (`copy`) while others keep instant reflection (`merge`/`symlink`).

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

In both merge and copy modes, `sync` automatically prunes orphans:

- **Symlinks** pointing to deleted source skills are always removed
- **Real directories** are removed if they appear in `.skillshare-manifest.json` (previously managed by skillshare)
- **Unknown directories** not in the manifest are preserved with a warning (assumed to be user-created)

This means that after `uninstall` + `sync`, even non-symlink residue (e.g., directories left from a previous `copy` mode) is safely cleaned up.

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
