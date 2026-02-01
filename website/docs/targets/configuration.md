---
sidebar_position: 4
---

# Configuration

Configuration file reference for skillshare.

## Overview

```
┌─────────────────────────────────────────────────────────────────┐
│                    SKILLSHARE FILES                             │
│                                                                 │
│  ~/.config/skillshare/                                          │
│  ├── config.yaml          ← Configuration file                  │
│  ├── skills/              ← Source directory (your skills)      │
│  │   ├── my-skill/                                              │
│  │   ├── another/                                               │
│  │   └── _team-repo/      ← Tracked repository                  │
│  └── backups/             ← Automatic backups                   │
│      └── 2026-01-20.../                                         │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

---

## Config File

**Location:** `~/.config/skillshare/config.yaml`

### Full Example

```yaml
# Source directory (where you edit skills)
source: ~/.config/skillshare/skills

# Default sync mode for new targets
mode: merge

# Targets (AI CLI skill directories)
targets:
  claude:
    path: ~/.claude/skills
    # mode: merge (inherits from default)

  codex:
    path: ~/.codex/skills
    mode: symlink  # Override default mode

  cursor:
    path: ~/.cursor/skills

  # Custom target
  myapp:
    path: ~/apps/myapp/skills

# Files to ignore during sync
ignore:
  - "**/.DS_Store"
  - "**/.git/**"
  - "**/node_modules/**"
  - "**/*.log"
```

---

## Fields

### `source`

Path to your skills directory (single source of truth).

```yaml
source: ~/.config/skillshare/skills
```

**Default:** `~/.config/skillshare/skills`

### `mode`

Default sync mode for all targets.

```yaml
mode: merge
```

| Value | Behavior |
|-------|----------|
| `merge` | Each skill symlinked individually. Local skills preserved. **(default)** |
| `symlink` | Entire target directory is one symlink. |

### `targets`

AI CLI skill directories to sync to.

```yaml
targets:
  <name>:
    path: <path>
    mode: <mode>  # optional, overrides default
```

**Example:**
```yaml
targets:
  claude:
    path: ~/.claude/skills

  codex:
    path: ~/.codex/skills
    mode: symlink

  custom:
    path: ~/my-app/skills
```

### `ignore`

Glob patterns for files to skip during sync.

```yaml
ignore:
  - "**/.DS_Store"
  - "**/.git/**"
  - "**/node_modules/**"
```

**Default patterns:**
- `**/.DS_Store`
- `**/.git/**`

---

## Managing Config

### View current config

```bash
skillshare status
# Shows source, targets, modes
```

### Edit config directly

```bash
# Open in editor
$EDITOR ~/.config/skillshare/config.yaml

# Then sync to apply changes
skillshare sync
```

### Reset config

```bash
rm ~/.config/skillshare/config.yaml
skillshare init
```

---

## Environment Variables

| Variable | Description |
|----------|-------------|
| `SKILLSHARE_CONFIG` | Override config file path |
| `GITHUB_TOKEN` | For API rate limit issues |

**Example:**
```bash
SKILLSHARE_CONFIG=~/custom-config.yaml skillshare status
```

---

## Skill Metadata

When you install a skill, skillshare creates a `.skillshare.yaml` file:

```yaml
# ~/.config/skillshare/skills/pdf/.skillshare.yaml
source: github.com/anthropics/skills/skills/pdf
installed_at: 2026-01-20T15:30:00Z
type: git
```

This is used by `skillshare update` to know where to fetch updates from.

**Don't edit this file manually.**

---

## Platform Differences

### macOS / Linux

```yaml
source: ~/.config/skillshare/skills
targets:
  claude:
    path: ~/.claude/skills
```

Uses symlinks.

### Windows

```yaml
source: %USERPROFILE%\.config\skillshare\skills
targets:
  claude:
    path: %USERPROFILE%\.claude\skills
```

Uses NTFS junctions (no admin required).

---

## Related

- [Source & Targets](/docs/concepts/source-and-targets) — Core concepts
- [Sync Modes](/docs/concepts/sync-modes) — Merge vs symlink
- [Environment Variables](/docs/reference/environment-variables) — All variables
