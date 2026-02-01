---
sidebar_position: 3
---

# install

Add skills from GitHub repos, git URLs, or local paths.

## Overview

```
┌──────────────────────────────────────────────────────────────┐
│                    SKILL LIFECYCLE                           │
│                                                              │
│   install ──► source ──► sync ──► targets                    │
│                 │                                            │
│              update                                          │
│                 │                                            │
│            uninstall ──► sync ──► removed from targets       │
└──────────────────────────────────────────────────────────────┘
```

---

## Quick Examples

```bash
# From GitHub (shorthand)
skillshare install anthropics/skills/skills/pdf

# Browse available skills in a repo
skillshare install anthropics/skills

# From local path
skillshare install ~/Downloads/my-skill

# As tracked repo (for team sharing)
skillshare install github.com/team/skills --track
```

## GitHub Shorthand

Use `owner/repo` format — automatically expands to `github.com/owner/repo`:

```bash
skillshare install anthropics/skills                    # Browse mode
skillshare install anthropics/skills/skills/pdf         # Direct install
skillshare install ComposioHQ/awesome-claude-skills     # Another repo
```

## Discovery Mode (Browse Skills)

When you don't specify a path, skillshare lists all available skills:

```bash
skillshare install anthropics/skills
```

<p>
  <img src="/img/install-demo.png" alt="install demo" width="720" />
</p>

**Tip**: Use `--dry-run` to preview without installing:
```bash
skillshare install anthropics/skills --dry-run
```

## Direct Install (Specific Path)

Provide the full path to install immediately:

```bash
# GitHub with subdirectory
skillshare install anthropics/skills/skills/pdf
skillshare install google-gemini/gemini-cli/packages/core/src/skills/builtin/skill-creator

# Full URL
skillshare install github.com/user/repo/path/to/skill

# SSH URL
skillshare install git@github.com:user/repo.git

# Local path
skillshare install ~/Downloads/my-skill
skillshare install /absolute/path/to/skill
```

## Options

| Flag | Short | Description |
|------|-------|-------------|
| `--name <name>` | | Custom name for the skill |
| `--force` | `-f` | Overwrite existing skill |
| `--update` | `-u` | Update if exists (git pull or reinstall) |
| `--track` | `-t` | Keep `.git` for tracked repos |
| `--dry-run` | `-n` | Preview only |

## Common Scenarios

**Install with custom name:**
```bash
skillshare install google-gemini/gemini-cli/.../skill-creator --name my-creator
# Installed as: ~/.config/skillshare/skills/my-creator/
```

**Force overwrite existing:**
```bash
skillshare install ~/my-skill --force
```

**Update existing skill:**
```bash
# By skill name (uses stored source)
skillshare install pdf --update

# By source URL
skillshare install anthropics/skills/skills/pdf --update
```

**Install team repo (tracked):**
```bash
skillshare install anthropics/skills --track
```

<p>
  <img src="/img/team-reack-demo.png" alt="tracked repo install demo" width="720" />
</p>

## After Installing

Always sync to distribute to targets:

```bash
skillshare install anthropics/skills/skills/pdf
skillshare sync  # ← Don't forget!
```

## Related

- [list](/docs/commands/list) — View installed skills
- [update](/docs/commands/update) — Update skills or tracked repos
- [upgrade](/docs/commands/upgrade) — Upgrade CLI and built-in skill
- [uninstall](/docs/commands/uninstall) — Remove skills
- [sync](/docs/commands/sync) — Sync skills to targets
- [Team Sharing](/docs/guides/team-sharing) — Tracked repos and team sharing
