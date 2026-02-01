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

## Custom Name

```bash
skillshare install github.com/team/skills --track --name acme-skills
# Installed as: _acme-skills/
```

---

## Collision Detection

When multiple skills have the same `name` field, sync warns you:

```
Warning: skill name collision detected
  "ui" defined in:
    - _team-a/frontend/ui/SKILL.md
    - _team-b/components/ui/SKILL.md
```

**Best practice** — namespace your skills:

```yaml
# In _team-a/frontend/ui/SKILL.md
name: team-a:ui

# In _team-b/components/ui/SKILL.md
name: team-b:ui
```

---

## Related

- [Team Sharing](/docs/guides/team-sharing) — Full team workflow
- [Commands: install](/docs/commands/install) — Install options
- [Commands: update](/docs/commands/update) — Update command
