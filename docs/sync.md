# Sync, Collect, Pull, Push & Backup

## Command Overview

| 操作類型 | 命令 | 方向 |
|----------|------|------|
| **本地同步** | `sync` / `collect` | Source ↔ Targets |
| **遠端同步** | `push` / `pull` | Source ↔ Git Remote |

- `sync` = 從 Source 分發到 Targets
- `collect` = 從 Targets 收集回 Source
- `push` = 推送到 git 遠端
- `pull` = 從 git 遠端拉取並同步

## Overview

```
┌─────────────────────────────────────────────────────────────────┐
│                      SYNC OPERATIONS                            │
│                                                                 │
│                        ┌──────────┐                             │
│                        │  Remote  │                             │
│                        │  (git)   │                             │
│                        └────┬─────┘                             │
│                    push ↑   │   ↓ pull                          │
│                             │                                   │
│    ┌────────────────────────┼────────────────────────┐          │
│    │                  SOURCE                         │          │
│    │        ~/.config/skillshare/skills/             │          │
│    └────────────────────────┬────────────────────────┘          │
│                 sync ↓      │      ↑ collect                    │
│         ┌───────────────────┼───────────────────┐               │
│         ▼                   ▼                   ▼               │
│   ┌──────────┐        ┌──────────┐        ┌──────────┐          │
│   │  Claude  │        │  Cursor  │        │  Codex   │          │
│   └──────────┘        └──────────┘        └──────────┘          │
│                         TARGETS                                 │
└─────────────────────────────────────────────────────────────────┘
```

| Command | Direction | Description |
|---------|-----------|-------------|
| `sync` | Source → Targets | Push skills to all targets |
| `collect <target>` | Target → Source | Collect skills from target to source |
| `push` | Source → Remote | Commit and push to git |
| `pull` | Remote → Source → Targets | Pull from git, then sync |

---

## Sync

Push skills from source to all targets.

```bash
skillshare sync              # Sync to all targets
skillshare sync --dry-run    # Preview changes
skillshare sync -n           # Short form
```

### What Happens

```
┌─────────────────────────────────────────────────────────────────┐
│ skillshare sync                                                 │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│ 1. Backup targets (automatic)                                   │
│    → ~/.config/skillshare/backups/2026-01-20_15-30-00/          │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│ 2. For each target:                                             │
│    ┌─────────────────────────────────────────────────────────┐  │
│    │ merge mode:                                             │  │
│    │   • Create symlink for each skill                       │  │
│    │   • Preserve local-only skills                          │  │
│    │   • Prune orphaned symlinks                             │  │
│    ├─────────────────────────────────────────────────────────┤  │
│    │ symlink mode:                                           │  │
│    │   • Replace entire directory with symlink               │  │
│    └─────────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│ 3. Report results                                               │
│    ✓ claude: merged (5 linked, 2 local, 0 updated, 1 pruned)    │
│    ✓ cursor: merged (5 linked, 0 local, 0 updated, 0 pruned)    │
└─────────────────────────────────────────────────────────────────┘
```

### Example Output

<p>
  <img src="../.github/assets/sync-demo.png" alt="sync demo" width="720">
</p>

---

## Status

Show current sync state.

```bash
skillshare status
```

<p>
  <img src="../.github/assets/status-demo.png" alt="status demo" width="720">
</p>

---

## Diff

Show differences between source and targets.

```bash
skillshare diff              # All targets
skillshare diff claude       # Specific target
```

<p>
  <img src="../.github/assets/diff-demo.png" alt="diff demo" width="720">
</p>

### Symbols

| Symbol | Meaning | Action |
|--------|---------|--------|
| `+` | In source, missing in target | `sync` will add |
| `~` | Both have it, but target is a local copy (not symlink) | `sync --force` to replace with symlink |
| `-` | Only in target, not in source | `collect` to import to source |

---

## Collect

Collect skills from a target back to source.

```bash
skillshare collect claude           # Collect from Claude
skillshare collect claude --dry-run # Preview
skillshare collect --all            # Collect from all targets
```

**When to use**: You created/edited a skill directly in a target (e.g., `~/.claude/skills/`) and want to bring it to source.

```
┌─────────────────────────────────────────────────────────────────┐
│ skillshare collect claude                                       │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│ 1. Find skills in target that aren't symlinks                   │
│    → ~/.claude/skills/new-skill/ (local)                        │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│ 2. Copy to source                                               │
│    → ~/.config/skillshare/skills/new-skill/                     │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│ 3. Replace original with symlink                                │
│    ~/.claude/skills/new-skill → source/new-skill                │
└─────────────────────────────────────────────────────────────────┘
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

```
┌─────────────────────────────────────────────────────────────────┐
│ skillshare pull                                                 │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│ 1. cd ~/.config/skillshare/skills                               │
│    git pull                                                     │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│ 2. skillshare sync (automatic)                                  │
└─────────────────────────────────────────────────────────────────┘
```

---

## Push

Commit and push source to git remote.

```bash
skillshare push                  # Auto-generated message
skillshare push -m "Add pdf"     # Custom message
```

```
┌─────────────────────────────────────────────────────────────────┐
│ skillshare push -m "Add pdf skill"                              │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│ cd ~/.config/skillshare/skills                                  │
│ git add .                                                       │
│ git commit -m "Add pdf skill"                                   │
│ git push                                                        │
└─────────────────────────────────────────────────────────────────┘
```

**Conflict handling:**
- If remote is ahead, `push` fails → run `pull` first

---

## Sync Modes

| Mode | Behavior | Use case |
|------|----------|----------|
| `merge` | Each skill symlinked individually | **Default.** Preserves local skills. |
| `symlink` | Entire directory is one symlink | Exact copies everywhere. |

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

Location: `~/.config/skillshare/backups/<timestamp>/`

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

```
┌─────────────────────────────────────────────────────────────────┐
│ skillshare restore claude                                       │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│ 1. Find latest backup for claude                                │
│    → backups/2026-01-20_15-30-00/claude/                        │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│ 2. Remove current target directory                              │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│ 3. Copy backup to target                                        │
│    → ~/.claude/skills/                                          │
└─────────────────────────────────────────────────────────────────┘
```

---

## Related

- [targets.md](targets.md) — Manage targets
- [cross-machine.md](cross-machine.md) — Sync across computers
- [install.md](install.md) — Install skills
