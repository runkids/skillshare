---
sidebar_position: 1
---

# Core Concepts

Understanding these concepts helps you get the most out of skillshare.

## Overview

```
┌─────────────────────────────────────────────────────────────────┐
│                      SKILLSHARE MODEL                           │
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
│    │                                                 │          │
│    │   my-skill/   another/   _team-repo/            │          │
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

## Key Concepts

| Concept | What It Is | Learn More |
|---------|-----------|------------|
| **Source & Targets** | Single source of truth, multiple destinations | [→ Source & Targets](./source-and-targets) |
| **Sync Modes** | Merge vs symlink — how files are linked | [→ Sync Modes](./sync-modes) |
| **Tracked Repos** | Git repos installed with `--track` | [→ Tracked Repositories](./tracked-repositories) |
| **Skill Format** | SKILL.md structure and metadata | [→ Skill Format](./skill-format) |

---

## Quick Summary

### Source & Targets
- **Source**: `~/.config/skillshare/skills/` — where you edit skills
- **Targets**: AI CLI skill directories — where skills are deployed via symlinks

### Sync Modes
- **Merge** (default): Each skill symlinked individually, local skills preserved
- **Symlink**: Entire directory is one symlink

### Tracked Repos
- Git repos installed with `--track`
- Prefixed with `_` (e.g., `_team-skills/`)
- Updated via `skillshare update <name>`

### Skill Format
- `SKILL.md` with YAML frontmatter
- Required: `name` field
- Optional: `description`, custom metadata
