---
sidebar_position: 4
---

# Configuration

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

Location: `~/.config/skillshare/config.yaml`

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

### View Current Config

```bash
skillshare status
# Shows source, targets, modes
```

### Edit Config Directly

```bash
# Open in editor
$EDITOR ~/.config/skillshare/config.yaml

# Then sync to apply changes
skillshare sync
```

### Reset Config

```bash
rm ~/.config/skillshare/config.yaml
skillshare init
```

---

## Auto-Detected Targets

When running `skillshare init`, these paths are checked:

| Target | Path |
|--------|------|
| amp | `~/.amp/skills` |
| claude | `~/.claude/skills` |
| codex | `~/.codex/skills` |
| crush | `~/.crush/skills` |
| cursor | `~/.cursor/skills` |
| gemini | `~/.gemini/skills` |
| copilot | `~/.github-copilot/skills` |
| goose | `~/.goose/skills` |
| letta | `~/.letta/skills` |
| antigravity | `~/.antigravity/skills` |
| opencode | `~/.opencode/skills` |

Only paths that exist are added as targets.

---

## Environment Variables

| Variable | Description |
|----------|-------------|
| `SKILLSHARE_CONFIG` | Override config file path |
| `SKILLSHARE_SOURCE` | Override source directory |

**Example:**
```bash
SKILLSHARE_CONFIG=~/custom-config.yaml skillshare status
```

---

## Skill Metadata

Each skill can have a `.skillshare.yaml` file storing install metadata:

```yaml
# ~/.config/skillshare/skills/pdf/.skillshare.yaml
source: github.com/anthropics/skills/skills/pdf
installed_at: 2026-01-20T15:30:00Z
type: git
```

This is used by `skillshare update` to know where to fetch updates from.

---

## Related

- [targets](/docs/guides/targets) — Managing targets
- [sync](/docs/commands/sync) — Sync modes explained
- [faq](/docs/faq) — Troubleshooting
