---
sidebar_position: 4
---

# Tracked Repositories

Git repos installed with `--track` for team sharing and easy updates.

## Overview

Tracked repositories are git repos cloned into your source with their `.git` directory preserved. This enables:

- **Team sharing**: Everyone installs the same repo
- **Easy updates**: `skillshare update <name>` runs git pull
- **Version control**: Track which commit you're on

```
┌─────────────────────────────────────────────────────────────────┐
│              GitHub: team/shared-skills                         │
│   frontend/ui/   backend/api/   devops/deploy/                  │
└─────────────────────────────────────────────────────────────────┘
                              │
              skillshare install --track
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│  Source: _team-skills/                                          │
│  ├── .git/              ← Git history preserved                 │
│  ├── frontend/ui/                                               │
│  ├── backend/api/                                               │
│  └── devops/deploy/                                             │
└─────────────────────────────────────────────────────────────────┘
```

---

## Regular Skills vs Tracked Repos

| Aspect | Regular Skill | Tracked Repo |
|--------|---------------|--------------|
| Source | Copied to source | Cloned with `.git` |
| Update | `install --update` | `update <name>` (git pull) |
| Prefix | None | `_` prefix |
| Nested skills | Flattened | Flattened with `__` |

---

## Installing a Tracked Repo

```bash
skillshare install github.com/team/shared-skills --track
skillshare sync
```

**What happens:**
1. Repo is cloned to `~/.config/skillshare/skills/_team-skills/`
2. `.git` directory is preserved
3. Nested skills are flattened for AI CLIs

---

## The Underscore Prefix

Tracked repos are prefixed with `_` to distinguish them from regular skills:

```
~/.config/skillshare/skills/
├── my-skill/           # Regular skill (no prefix)
├── code-review/        # Regular skill
└── _team-skills/       # Tracked repo (underscore prefix)
```

---

## Nested Skills & Auto-Flattening

Skill repos often organize skills in folders. Skillshare automatically flattens them for AI CLIs:

```
SOURCE                              TARGET
(your organization)                 (what AI CLI sees)
────────────────────────────────────────────────────────────
_team-skills/
├── frontend/
│   ├── react/          ───►   _team-skills__frontend__react/
│   └── vue/            ───►   _team-skills__frontend__vue/
├── backend/
│   └── api/            ───►   _team-skills__backend__api/
└── devops/
    └── deploy/         ───►   _team-skills__devops__deploy/

• _ prefix = tracked repository
• __ (double underscore) = path separator
```

### Why Auto-Flattening?

| Benefit | Description |
|---------|-------------|
| **AI CLI compatibility** | Most AI CLIs expect skills in a flat directory, not nested folders |
| **Preserve organization** | Keep logical folder structure in source while meeting CLI requirements |
| **Traceability** | Flattened name shows origin path (e.g., `_team__frontend__react` → came from `_team/frontend/react/`) |
| **No manual work** | Skillshare handles the transformation automatically during sync |

**You organize, skillshare adapts.** Write skills in any folder structure; they'll work everywhere.

:::tip
Auto-flattening works for **all skills**, not just tracked repos. You can organize your personal skills in folders too. See [Organize with Folders](/docs/concepts/source-and-targets#organize-with-folders-auto-flattening).
:::

---

## Updating Tracked Repos

### Single repo

```bash
skillshare update _team-skills
skillshare sync
```

### All tracked repos

```bash
skillshare update --all
skillshare sync
```

**What happens:**
```
cd ~/.config/skillshare/skills/_team-skills
git pull origin main
```

---

## Uninstalling

```bash
skillshare uninstall _team-skills
```

**What happens:**
1. Checks for uncommitted changes (warns if found)
2. Removes the directory
3. Next `sync` removes the symlinks from targets

---

## Project Mode

Tracked repos also work in project mode. The repo is cloned into `.skillshare/skills/` and added to `.skillshare/.gitignore` (so the tracked repo's git history doesn't conflict with your project's git). Project logs (`.skillshare/logs/`) are also ignored by default.

Installing a tracked repo auto-records `tracked: true` in `.skillshare/config.yaml`, so new team members get the correct clone behavior via `skillshare install -p`:

```yaml
# .skillshare/config.yaml (auto-updated after install --track -p)
skills:
  - name: _team-shared-skills
    source: github.com/team/shared-skills
    tracked: true
```

```bash
# Install tracked repo into project
skillshare install github.com/team/shared-skills --track -p
skillshare sync

# Update via git pull
skillshare update team-skills -p
skillshare sync

# Force update (discard local changes)
skillshare update team-skills -p --force

# Uninstall
skillshare uninstall team-skills -p
```

**Directory structure:**

```
<project-root>/
└── .skillshare/
    ├── .gitignore           # Contains: logs/ and skills/_team-skills
    └── skills/
        └── _team-skills/    # Tracked repo with .git/ preserved
            ├── .git/
            ├── frontend/ui/
            └── backend/api/
```

If you intentionally want to commit project logs, add `!logs/` and `!logs/*.log` after the managed block in `.skillshare/.gitignore`.

Nested skills are auto-flattened the same way as global mode — `_team-skills/frontend/ui` becomes `_team-skills__frontend__ui` in targets.

---

## Custom Name

```bash
skillshare install github.com/team/skills --track --name acme-skills
# Installed as: _acme-skills/
```

---

## Collision Detection

When multiple skills share the same `name` field, sync checks whether they actually land on the same target after `include`/`exclude` filters are applied.

**Filters isolate the collision** — informational only:

```
ℹ Duplicate skill names exist but are isolated by target filters:
  'ui' (2 definitions)
```

**Collision reaches the same target** — actionable warning:

```
⚠ Target 'claude': skill name 'ui' is defined in multiple places:
  - _team-a/frontend/ui
  - _team-b/components/ui
Rename one in SKILL.md or adjust include/exclude filters
```

**Best practice** — namespace your skills or use filters:

```yaml
# Option 1: Namespace in SKILL.md
name: team-a:ui

# Option 2: Route with filters (global config)
targets:
  codex:
    path: ~/.codex/skills
    include: [_team-a__*]
  claude:
    path: ~/.claude/skills
    include: [_team-b__*]
```

```yaml
# Option 2: Route with filters (project config)
targets:
  - name: claude
    exclude: [codex-*]
  - name: codex
    include: [codex-*]
```

See [Target Filters](/docs/targets/configuration#include--exclude-target-filters) for full syntax and examples.

---

## Related

- [Organization-Wide Skills](/docs/guides/organization-sharing) — Full organization workflow
- [Project Skills](/docs/concepts/project-skills) — Project mode concepts
- [Commands: install](/docs/commands/install) — Install options
- [Commands: update](/docs/commands/update) — Update command
