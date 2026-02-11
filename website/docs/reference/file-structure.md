---
sidebar_position: 3
---

# File Structure

Directory layout and file locations for skillshare.

## Overview

```
~/.config/skillshare/
├── config.yaml              # Configuration file
├── audit-rules.yaml         # Custom audit rules (optional)
├── skills/                  # Source directory
│   ├── my-skill/            # Regular skill
│   │   ├── SKILL.md         # Skill definition (required)
│   │   └── .skillshare.yaml # Install metadata (auto-generated)
│   ├── code-review/         # Another skill
│   │   └── SKILL.md
│   └── _team-skills/        # Tracked repository
│       ├── .git/            # Git history preserved
│       ├── frontend/
│       │   └── ui/
│       │       └── SKILL.md
│       └── backend/
│           └── api/
│               └── SKILL.md
├── logs/                    # Operation logs (JSONL)
│   ├── operations.log       # install, sync, update, etc.
│   └── audit.log            # Security audit scans
├── backups/                 # Backup directory
│   ├── 2026-01-20_15-30-00/
│   │   ├── claude/
│   │   └── cursor/
│   └── 2026-01-19_10-00-00/
│       └── claude/
└── trash/                   # Uninstalled skills (7-day retention)
    ├── my-skill_2026-01-20_15-30-00/
    │   └── SKILL.md
    └── old-skill_2026-01-19_10-00-00/
        └── SKILL.md
```

---

## Configuration File

### Location

```
~/.config/skillshare/config.yaml
```

**Windows:**
```
%USERPROFILE%\.config\skillshare\config.yaml
```

### Contents

```yaml
source: ~/.config/skillshare/skills
mode: merge
targets:
  claude:
    path: ~/.claude/skills
  cursor:
    path: ~/.cursor/skills
ignore:
  - "**/.DS_Store"
  - "**/.git/**"
```

See [Configuration](/docs/targets/configuration) for full reference.

---

## Source Directory

### Location

```
~/.config/skillshare/skills/
```

**Windows:**
```
%USERPROFILE%\.config\skillshare\skills\
```

### Structure

```
skills/
├── skill-name/              # Skill directory
│   ├── SKILL.md             # Required: skill definition
│   ├── .skillshare.yaml     # Optional: install metadata
│   ├── examples/            # Optional: example files
│   └── templates/           # Optional: code templates
├── frontend/                # Category folder (via --into or manual)
│   └── react-skill/         # Skill in subdirectory
│       └── SKILL.md         # Synced as frontend__react-skill
└── _tracked-repo/           # Tracked repository
    ├── .git/                # Git history
    └── ...                  # Skill subdirectories
```

---

## Skill Files

### SKILL.md (Required)

The skill definition file:

```markdown
---
name: skill-name
description: Brief description
---

# Skill Name

Instructions for the AI...
```

See [Skill Format](/docs/concepts/skill-format) for details.

### .skillshare.yaml (Auto-generated)

Metadata about where the skill was installed from:

```yaml
source: github.com/org/repo/path/to/skill
installed_at: 2026-01-20T15:30:00Z
type: git
```

**Don't edit this file manually.** It's used by `skillshare update`.

---

## Backup Directory

### Location

```
~/.config/skillshare/backups/
```

### Structure

```
backups/
└── <timestamp>/             # YYYY-MM-DD_HH-MM-SS
    ├── claude/              # Backup of target
    │   ├── skill-a/
    │   └── skill-b/
    └── cursor/
        └── ...
```

Backups are created:
- Automatically before `sync` and `target remove`
- Manually via `skillshare backup`

---

## Trash Directory

### Location

```
~/.config/skillshare/trash/
```

**Project mode:**
```
<project>/.skillshare/trash/
```

### Structure

```
trash/
└── <skill-name>_<timestamp>/    # skill-name_YYYY-MM-DD_HH-MM-SS
    ├── SKILL.md
    └── ...                      # All original files preserved
```

Trashed skills are:
- Created by `skillshare uninstall`
- Retained for 7 days, then automatically cleaned up
- Named with the original skill name and a timestamp

---

## Log Directory

### Location

```
~/.config/skillshare/logs/
```

**Project mode:**
```
<project>/.skillshare/logs/
```

---

## Target Directories

Targets are AI CLI skill directories. After sync, they contain symlinks to source:

```
~/.claude/skills/
├── my-skill -> ~/.config/skillshare/skills/my-skill
├── code-review -> ~/.config/skillshare/skills/code-review
└── local-only/              # Not symlinked (local skill in merge mode)
```

### Merge mode

Each skill is symlinked individually:
```
skill/ -> source/skill/
```

### Symlink mode

Entire directory is symlinked:
```
~/.claude/skills -> ~/.config/skillshare/skills/
```

---

## Tracked Repositories

Tracked repos (installed with `--track`) preserve git history:

```
_team-skills/
├── .git/                    # Git preserved
├── frontend/
│   └── ui/
│       └── SKILL.md
└── backend/
    └── api/
        └── SKILL.md
```

### Naming conventions

- `_` prefix: tracked repository
- `__` in flattened name: path separator

**In source:**
```
_team-skills/frontend/ui/SKILL.md
```

**In target (flattened):**
```
_team-skills__frontend__ui/SKILL.md
```

---

## Platform Differences

### macOS / Linux

| Item | Path |
|------|------|
| Config | `~/.config/skillshare/config.yaml` |
| Source | `~/.config/skillshare/skills/` |
| Logs | `~/.config/skillshare/logs/` |
| Backups | `~/.config/skillshare/backups/` |
| Trash | `~/.config/skillshare/trash/` |
| Link type | Symlinks |

### Windows

| Item | Path |
|------|------|
| Config | `%USERPROFILE%\.config\skillshare\config.yaml` |
| Source | `%USERPROFILE%\.config\skillshare\skills\` |
| Logs | `%USERPROFILE%\.config\skillshare\logs\` |
| Backups | `%USERPROFILE%\.config\skillshare\backups\` |
| Trash | `%USERPROFILE%\.config\skillshare\trash\` |
| Link type | NTFS Junctions |

---

## Related

- [Configuration](/docs/targets/configuration) — Config file details
- [Skill Format](/docs/concepts/skill-format) — SKILL.md format
- [Tracked Repositories](/docs/concepts/tracked-repositories) — Tracked repos
