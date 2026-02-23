---
sidebar_position: 3
---

# File Structure

Directory layout and file locations for skillshare.

## Overview

```
~/.config/skillshare/        # XDG_CONFIG_HOME
├── config.yaml              # Configuration file
├── audit-rules.yaml         # Custom audit rules (optional)
└── skills/                  # Source directory
    ├── my-skill/            # Regular skill
    │   ├── SKILL.md         # Skill definition (required)
    │   └── .skillshare-meta.json # Install metadata (auto-generated)
    ├── code-review/         # Another skill
    │   └── SKILL.md
    └── _team-skills/        # Tracked repository
        ├── .git/            # Git history preserved
        ├── frontend/
        │   └── ui/
        │       └── SKILL.md
        └── backend/
            └── api/
                └── SKILL.md

~/.local/share/skillshare/   # XDG_DATA_HOME
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

~/.local/state/skillshare/   # XDG_STATE_HOME
└── logs/                    # Operation logs (JSONL)
    ├── operations.log       # install, sync, update, etc.
    └── audit.log            # Security audit scans

~/.cache/skillshare/         # XDG_CACHE_HOME      
├── version-check.json       # Version check cache (24h TTL)
└── ui/                      # Web UI dist cache
    └── 0.13.0/              # Per-version cached assets
        ├── index.html
        └── assets/
```

---

## Configuration File

### Location

```
~/.config/skillshare/config.yaml
```

**Override with XDG:**
```
XDG_CONFIG_HOME=/custom/path → /custom/path/skillshare/config.yaml
```

**Windows default:**
```
%AppData%\skillshare\config.yaml
```

### Contents

```yaml
# yaml-language-server: $schema=https://raw.githubusercontent.com/runkids/skillshare/main/schemas/config.schema.json
source: ~/.config/skillshare/skills
mode: merge
targets:
  claude:
    path: ~/.claude/skills
  cursor:
    path: ~/.cursor/skills
skills:                    # auto-managed by install/uninstall
  - name: pdf
    source: anthropics/skills/skills/pdf
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
%AppData%\skillshare\skills\
```

### Structure

```
skills/
├── skill-name/                   # Skill directory
│   ├── SKILL.md                  # Required: skill definition
│   ├── .skillshare-meta.json     # Optional: install metadata
│   ├── examples/                 # Optional: example files
│   └── templates/                # Optional: code templates
├── frontend/                     # Category folder (via --into or manual)
│   └── react-skill/              # Skill in subdirectory
│       └── SKILL.md              # Synced as frontend__react-skill
└── _tracked-repo/                # Tracked repository
    ├── .git/                     # Git history
    └── ...                       # Skill subdirectories
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

### .skillignore (Optional, repo-level)

A file in the **root of a skill repository** that excludes skills from discovery during `skillshare install`:

```text
# Hide internal tooling from discovery
validation-scripts
scaffold-template
prompt-eval-*

# Exclude an entire group directory
internal-tools
```

One pattern per line. Patterns match against skill paths — a group name like `internal-tools` excludes all skills under that directory, while `internal-tools/helper` excludes only a specific skill. Also supports trailing wildcard (`prefix-*`). Lines starting with `#` are comments. Only applies to discovery — already-installed skills are not affected.

### .skillshare-meta.json (Auto-generated)

Metadata about where the skill was installed from:

```json
{
  "source": "github.com/org/repo/path/to/skill",
  "type": "github",
  "installed_at": "2026-01-20T15:30:00Z",
  "repo_url": "https://github.com/org/repo.git",
  "subdir": "path/to/skill",
  "version": "abc1234"
}
```

**Don't edit this file manually.** It's used by `skillshare update` and `skillshare check`.

---

## Backup Directory

### Location

```
~/.local/share/skillshare/backups/
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
~/.local/share/skillshare/trash/
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
~/.local/state/skillshare/logs/
```

**Project mode:**
```
<project>/.skillshare/logs/
```

---

## Target Directories

Targets are AI CLI skill directories. After sync, they contain symlinks (or copies) to source.

### Merge mode

Each skill is symlinked individually. A manifest tracks managed skills for orphan cleanup:
```
~/.claude/skills/
├── my-skill -> ~/.config/skillshare/skills/my-skill
├── code-review -> ~/.config/skillshare/skills/code-review
├── local-only/              # Not symlinked (user-created, preserved)
└── .skillshare-manifest.json  # Tracks managed skills
```

### Copy mode

Each skill is copied as real files. The manifest tracks checksums for incremental sync:
```
~/.cursor/skills/
├── my-skill/                  # Real files (copied from source)
├── code-review/               # Real files
├── local-only/                # User-created, preserved
└── .skillshare-manifest.json  # Tracks managed skills + checksums
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

:::tip XDG Base Directory
skillshare respects the XDG Base Directory Specification. Override base directories with `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, and `XDG_CACHE_HOME`.

See [Environment Variables](./environment-variables.md#xdg_config_home) for details.
:::

### macOS / Linux

| Item | Path |
|------|------|
| Config | `~/.config/skillshare/config.yaml` |
| Source | `~/.config/skillshare/skills/` |
| Backups | `~/.local/share/skillshare/backups/` |
| Trash | `~/.local/share/skillshare/trash/` |
| Logs | `~/.local/state/skillshare/logs/` |
| Version cache | `~/.cache/skillshare/version-check.json` |
| UI cache | `~/.cache/skillshare/ui/{version}/` |
| Link type | Symlinks |

### Windows

| Item | Path |
|------|------|
| Config | `%AppData%\skillshare\config.yaml` |
| Source | `%AppData%\skillshare\skills\` |
| Backups | `%AppData%\skillshare\backups\` |
| Trash | `%AppData%\skillshare\trash\` |
| Logs | `%AppData%\skillshare\logs\` |
| Version cache | `%AppData%\skillshare\version-check.json` |
| UI cache | `%AppData%\skillshare\ui\{version}\` |
| Link type | NTFS Junctions |

## XDG Base Directory Layout

Skillshare follows the [XDG Base Directory Specification](https://specifications.freedesktop.org/basedir/latest/) on Unix systems:

| XDG Variable | Default Path | Skillshare Uses For |
|-------------|-------------|---------------------|
| `XDG_CONFIG_HOME` | `~/.config` | `skillshare/config.yaml`, `skillshare/skills/` |
| `XDG_DATA_HOME` | `~/.local/share` | `skillshare/backups/`, `skillshare/trash/` |
| `XDG_STATE_HOME` | `~/.local/state` | `skillshare/logs/` |
| `XDG_CACHE_HOME` | `~/.cache` | `skillshare/ui/` (downloaded web dashboard) |

### Windows Paths

| Purpose | Path |
|---------|------|
| Config + Skills | `%AppData%\skillshare\` |
| Data (backups, trash) | `%AppData%\skillshare\` |
| State (logs) | `%AppData%\skillshare\` |
| Cache (UI) | `%AppData%\skillshare\` |

### Migration Note

If upgrading from a version before the XDG split, skillshare automatically migrates data from the old location (`~/.config/skillshare/`) to the correct XDG directories on first run.

---

## Related

- [Configuration](/docs/targets/configuration) — Config file details
- [Skill Format](/docs/concepts/skill-format) — SKILL.md format
- [Tracked Repositories](/docs/concepts/tracked-repositories) — Tracked repos
