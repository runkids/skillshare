---
sidebar_position: 2
---

# backup

Create, list, and manage backups of target directories.

```bash
skillshare backup              # Backup all skill targets
skillshare backup claude       # Backup specific target
skillshare backup agents       # Backup all agent targets
skillshare backup --all        # Backup skills + agents
skillshare backup --list       # List all backups
skillshare backup --cleanup    # Remove old backups
```

## When to Use

- Create a manual backup before risky changes
- List existing backups to check recovery options
- Clean up old backups to free disk space

## Automatic Backups

Backups are created **automatically** before:
- `skillshare sync` (skill targets and agent targets)
- `skillshare sync agents` (agent targets only)
- `skillshare target remove`

Location: `~/.local/share/skillshare/backups/<timestamp>/` (global), `.skillshare/backups/` (project mode, agents only)

Retention is applied automatically after each automatic backup, using the same policy as `--cleanup`. You do not need to prune snapshots by hand.

## Commands

### Create Backup

```bash
skillshare backup              # All targets
skillshare backup claude       # Specific target
skillshare backup --dry-run    # Preview
```

### List Backups

```bash
skillshare backup --list
```

```
All backups (15.3 MB total)
  2026-01-20_15-30-00  claude, cursor     4.2 MB  ~/.local/share/.../2026-01-20_15-30-00
  2026-01-19_10-00-00  claude             2.1 MB  ~/.local/share/.../2026-01-19_10-00-00
  2026-01-18_09-00-00  claude, cursor     4.0 MB  ~/.local/share/.../2026-01-18_09-00-00
```

### Cleanup Old Backups

```bash
skillshare backup --cleanup           # Remove old backups
skillshare backup --cleanup --dry-run # Preview cleanup
```

Default cleanup policy:
- Keep last 10 backups
- Remove backups older than 30 days
- Cap total size at 500 MB

The newest snapshot is always kept, even when it alone exceeds the size cap — you are never left without a restore point.

This same policy runs automatically after every `sync`, so `--cleanup` is only needed to prune on demand.

## Options

| Flag | Description |
|------|-------------|
| `--all` | Backup both skills and agents |
| `--project, -p` | Use project mode (`.skillshare/backups/`); **agents only** |
| `--global, -g` | Use global mode (default for skills) |
| `--list, -l` | List all backups |
| `--cleanup, -c` | Remove old backups |
| `--target, -t <name>` | Target specific backup (alternative to positional arg) |
| `--dry-run, -n` | Preview without making changes |

`backup` also accepts a positional kind argument: `skillshare backup agents` scopes the backup to agent targets only.

## Backup Structure

```
~/.local/share/skillshare/backups/
├── 2026-01-20_15-30-00/
│   ├── claude/
│   │   ├── skill-a/
│   │   └── skill-b/
│   └── cursor/
│       ├── skill-a/
│       └── skill-b/
└── 2026-01-19_10-00-00/
    └── claude/
        └── ...
```

The skill directories present depend on the target's mode — see [What Gets Backed Up](#what-gets-backed-up).

## What Gets Backed Up

A backup protects only what `sync` could destroy: **local content that exists in the target but not in your source.**

- Regular files and directories in targets are backed up
- Per-skill symlinks in merge-mode targets are **skipped** — they point into your source, which is the single source of truth and already safe. `skillshare sync` recreates them

This means:
- In merge mode: Only local (non-symlinked) skills are backed up. Synced skills live in the source
- In copy mode: All managed skill directories are backed up (they are real files)
- In symlink mode: Nothing is backed up (entire directory is a single symlink)

If a target contains nothing but symlinks, no backup is created and `backup` reports nothing to do — an empty restore point is not useful.

## Backups & Disk Space

Backups never copy your source, so they stay small. Three separate mechanisms are easy to confuse:

| Mechanism | Scope | What it controls |
|-----------|-------|------------------|
| `.gitignore` in your source | Git only | What Git tracks. Ignored files still exist on disk |
| `ignore:` in `config.yaml` | `sync` | Which files `sync` copies into targets (mainly copy mode). See [sync](/docs/reference/commands/sync) |
| Backup | Snapshot | Local target content only — symlinks and therefore source artifacts are excluded |

Because symlinked skills are not followed, heavy artifacts living inside a source skill (model weights, `.venv`, browser profiles, media) are **never** copied into a snapshot, whether or not `.gitignore` or `ignore:` mentions them.

Retention runs automatically after every `sync`, using the default policy below. To inspect usage manually:

```bash
du -sh ~/.local/share/skillshare/backups   # Total size on disk
skillshare backup --list                   # Per-snapshot sizes
skillshare backup --cleanup --dry-run      # Preview what retention would remove
```

Copy-mode targets are the one case where snapshots can still grow: those are real files, so anything under a skill directory is copied. Keep runtime caches and large artifacts outside the skill tree, or exclude them with `ignore:` so they never reach the target in the first place.

## Agent Backup

Agents have their own backup flow that runs alongside skill backups, with two distinctions worth knowing:

**Entry naming.** Agent backups are stored under `<target>-agents/` inside each timestamp directory, parallel to the skill backup. For example, after `skillshare backup --all` the layout looks like:

```
~/.local/share/skillshare/backups/2026-01-20_15-30-00/
├── claude/          # Skills backup for claude
├── claude-agents/   # Agents backup for claude
└── cursor/
```

**Project mode is the inverse of skills.** In project mode (`-p`), `backup` refuses to back up skill targets but **does** back up agent targets. The error you'll see if you forget the `agents` filter:

```
backup is not supported in project mode (except for agents)
```

So in project mode you must say either `skillshare backup -p agents` or `skillshare backup -p --all`.

```bash
skillshare backup agents                  # All agent targets (global)
skillshare backup agents claude           # Only claude's agents
skillshare backup agents -p               # Project agent targets
skillshare backup --all                   # Skills + agents in one shot
```

See [Agents](/docs/understand/agents) for the agent resource model and [restore](/docs/reference/commands/restore) for recovery.

## See Also

- [restore](/docs/reference/commands/restore) — Restore from backup
- [sync](/docs/reference/commands/sync) — Auto-creates backups
- [target remove](/docs/reference/commands/target) — Auto-creates backups
