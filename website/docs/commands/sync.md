---
sidebar_position: 2
---

# sync

Push skills from source to all targets.

:::info Why is sync a separate command?
Operations like `install` and `uninstall` only modify source — sync propagates to targets. This lets you batch changes, preview with `--dry-run`, and control when targets update. See [Why Sync is a Separate Step](/docs/concepts/source-and-targets#why-sync-is-a-separate-step).
:::

## When to Use

- After installing, uninstalling, or editing skills — propagate changes to all targets
- After changing a target's sync mode — apply the new mode
- Periodically to ensure all targets are in sync

## Command Overview

| Type | Command | Direction |
|------|---------|-----------|
| **Local sync** | `sync` / `collect` | Source ↔ Targets |
| **Remote sync** | `push` / `pull` | Source ↔ Git Remote |

- `sync` = Distribute from Source to Targets
- `collect` = Collect from Targets back to Source
- `push` = Push to git remote
- `pull` = Pull from git remote and sync

## Overview

```mermaid
flowchart TD
    REMOTE["`Remote
(git)`"]
    SOURCE["`SOURCE
~/.config/skillshare/skills/`"]
    CLAUDE["Claude"]
    CURSOR["Cursor"]
    CODEX["Codex"]

    SOURCE -- push --> REMOTE
    REMOTE -- pull --> SOURCE
    SOURCE -- sync --> CLAUDE
    SOURCE -- sync --> CURSOR
    SOURCE -- sync --> CODEX
    CLAUDE -- collect --> SOURCE
    CURSOR -- collect --> SOURCE
    CODEX -- collect --> SOURCE
```

| Command | Direction | Description |
|---------|-----------|-------------|
| `sync` | Source → Targets | Push skills to all targets |
| `collect <target>` | Target → Source | Collect skills from target to source |
| `push` | Source → Remote | Commit and push to git |
| `pull` | Remote → Source → Targets | Pull from git, then sync |

---

## Project Mode

When `.skillshare/config.yaml` exists in the current directory, sync auto-detects project mode:

```bash
cd my-project/
skillshare sync          # Auto-detected project mode
skillshare sync -p       # Explicit project mode
```

**Project sync** defaults to merge mode (per-skill symlinks), but each target can be set to copy or symlink mode via `skillshare target <name> --mode copy -p`. Backup is not created (project targets are reproducible from source).

```
.skillshare/skills/                 .claude/skills/
├── my-skill/          ────────►    ├── my-skill/ → (symlink)
├── pdf/               ────────►    ├── pdf/      → (symlink)
└── ...                             └── local/    (preserved)
```

---

## Sync

Push skills from source to all targets.

```bash
skillshare sync              # Sync to all targets
skillshare sync --dry-run    # Preview changes
skillshare sync -n           # Short form
skillshare sync --force      # Overwrite all managed skills
skillshare sync -f           # Short form
```

| Flag | Short | Description |
|------|-------|-------------|
| `--dry-run` | `-n` | Preview changes without writing |
| `--force` | `-f` | Overwrite all managed entries regardless of checksum (copy mode) or replace existing directories with symlinks (merge mode) |

### What Happens

```mermaid
flowchart TD
    TITLE["skillshare sync"]
    S1["1. Backup targets"]
    S2["2. For each target"]
    MERGE["merge mode"]
    SYMLINK["symlink mode"]
    S3["3. Report results"]
    TITLE --> S1 --> S2
    COPY["copy mode"]
    S2 --> MERGE --> S3
    S2 --> COPY --> S3
    S2 --> SYMLINK --> S3
```

### Example Output

<p>
  <img src="/img/sync-demo.png" alt="sync demo" width="720" />
</p>

---

## Collect

Collect skills from a target back to source.

```bash
skillshare collect claude           # Collect from Claude
skillshare collect claude --dry-run # Preview
skillshare collect --all            # Collect from all targets
```

**When to use**: You created/edited a skill directly in a target (e.g., `~/.claude/skills/`) and want to bring it to source.

```mermaid
flowchart TD
    TITLE["skillshare collect claude"]
    S1["1. Find local skills in target"]
    S2["2. Copy to source"]
    S3["3. Replace with symlink"]
    TITLE --> S1 --> S2 --> S3
```

**After collecting:**
```bash
skillshare collect claude
skillshare sync  # ← Distribute to other targets
```

---

## Pull

Pull from git remote and sync to all targets.

```bash
skillshare pull              # Pull from git remote
skillshare pull --dry-run    # Preview
```

**When to use**: You pushed changes from another machine and want to sync them here.

```mermaid
flowchart TD
    TITLE["skillshare pull"]
    S1["1. git pull"]
    S2["2. skillshare sync"]
    TITLE --> S1 --> S2
```

---

## Push

Commit and push source to git remote.

```bash
skillshare push                  # Auto-generated message
skillshare push -m "Add pdf"     # Custom message
```

```mermaid
flowchart TD
    TITLE["skillshare push -m 'Add pdf skill'"]
    S1["`cd source
git add → commit → push`"]
    TITLE --> S1
```

**Conflict handling:**
- If remote is ahead, `push` fails → run `pull` first

---

## Sync Modes

| Mode | Behavior | Use case |
|------|----------|----------|
| `merge` | Each skill symlinked individually | **Default.** Preserves local skills. |
| `copy` | Each skill copied as real files | Compatibility-first setups, vendoring skills into a project repo, or environments where symlink behavior is unreliable. |
| `symlink` | Entire directory is one symlink | Exact copies everywhere. |

Per-target override remains the primary tuning knob:

```bash
skillshare target <name> --mode copy
skillshare sync
```

When `sync` prints a compatibility hint, the example target is chosen in this priority:
`cursor` → `antigravity` → `copilot` → `opencode`.
If none of these targets exist (or they already run `copy`), no compatibility hint is shown.

See [Sync Modes](/docs/concepts/sync-modes) for a neutral decision matrix.

### Per-target include/exclude filters

In merge and copy modes, each target can define `include` / `exclude` patterns in config:

```yaml
targets:
  codex:
    path: ~/.codex/skills
    include: [codex-*]
  claude:
    path: ~/.claude/skills
    exclude: [codex-*]
```

- Matching is against flat target names (for example `team__frontend__ui`)
- `include` is applied first, then `exclude`
- `diff`, `status`, `doctor`, and UI drift all use the filtered expected set
- In symlink mode, filters are ignored
- In copy mode, filters work the same way as merge mode
- `sync` removes existing source-linked or managed entries that are now excluded

See [Configuration](/docs/targets/configuration#include--exclude-target-filters) for full details.

### Filter behavior examples

Assume source contains:
- `core-auth`
- `core-docs`
- `codex-agent`
- `codex-experimental`
- `team__frontend__ui`

#### `include` only

```yaml
targets:
  codex:
    path: ~/.codex/skills
    include: [codex-*, core-*]
```

After `sync`, codex receives:
- `core-auth`
- `core-docs`
- `codex-agent`
- `codex-experimental`

Use this when a target should receive only a curated subset.

#### `exclude` only

```yaml
targets:
  claude:
    path: ~/.claude/skills
    exclude: [codex-*, *-experimental]
```

After `sync`, claude receives:
- `core-auth`
- `core-docs`
- `team__frontend__ui`

Use this when a target should get "almost everything" except specific groups.

#### `include` + `exclude`

```yaml
targets:
  cursor:
    path: ~/.cursor/skills
    include: [core-*, codex-*]
    exclude: [*-experimental]
```

After `sync`, cursor receives:
- `core-auth`
- `core-docs`
- `codex-agent`

`codex-experimental` is first included, then removed by `exclude`.

#### What gets removed when filters change

When a filter is updated and `sync` runs:
- Source-linked entries (symlink/junction) that are now filtered out are pruned
- Local non-symlink folders already in target are preserved

### Merge Mode (Default)

```
Source                          Target (claude)
─────────────────────────────────────────────────────────────
skills/                         ~/.claude/skills/
├── my-skill/        ────────►  ├── my-skill/ → (symlink)
├── another/         ────────►  ├── another/  → (symlink)
└── ...                         ├── local-only/  (preserved)
                                └── ...
```

### Copy Mode

```
Source                          Target (cursor)
─────────────────────────────────────────────────────────────
skills/                         ~/.cursor/skills/
├── my-skill/        ────copy►  ├── my-skill/    (real files)
├── another/         ────copy►  ├── another/     (real files)
└── ...                         ├── local-only/  (preserved)
                                └── .skillshare-manifest.json
```

A `.skillshare-manifest.json` tracks managed skills and checksums. Unchanged skills are skipped on re-sync; `--force` overwrites all.

### Symlink Mode

```
Source                          Target (claude)
─────────────────────────────────────────────────────────────
skills/              ────────►  ~/.claude/skills → (symlink to source)
├── my-skill/
├── another/
└── ...
```

### Change Mode

```bash
skillshare target claude --mode merge
skillshare target claude --mode copy
skillshare target claude --mode symlink
skillshare sync  # Apply change
```

### Safety Warning

> **In symlink mode, deleting through target deletes source!**
> ```bash
> rm -rf ~/.claude/skills/my-skill  # ❌ Deletes from SOURCE
> skillshare target remove claude   # ✅ Safe way to unlink
> ```

---

## Backup

Backups are created **automatically** before `sync` and `target remove`.

Location: `~/.local/share/skillshare/backups/<timestamp>/`

### Manual Backup

```bash
skillshare backup              # Backup all targets
skillshare backup claude       # Backup specific target
skillshare backup --list       # List all backups
skillshare backup --cleanup    # Remove old backups
skillshare backup --dry-run    # Preview
```

### Example Output

```
$ skillshare backup --list

Backups
─────────────────────────────────────────
  2026-01-20_15-30-00/
    claude/    5 skills, 2.1 MB
    cursor/    5 skills, 2.1 MB
  2026-01-19_10-00-00/
    claude/    4 skills, 1.8 MB
```

---

## Restore

Restore targets from backup.

```bash
skillshare restore claude                              # Latest backup
skillshare restore claude --from 2026-01-19_10-00-00   # Specific backup
skillshare restore claude --dry-run                    # Preview
```

```mermaid
flowchart TD
    CMD["skillshare restore claude"]
    FIND["1. Find latest backup"]
    REMOVE["2. Remove current target"]
    COPY["3. Copy backup to target"]
    CMD --> FIND --> REMOVE --> COPY
```

---

## See Also

- [status](/docs/commands/status) — Show sync state
- [diff](/docs/commands/diff) — Show differences
- [Targets](/docs/targets) — Manage targets
- [Cross-Machine Sync](/docs/guides/cross-machine-sync) — Sync across computers
- [install](/docs/commands/install) — Install skills
