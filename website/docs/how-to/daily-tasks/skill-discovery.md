---
sidebar_position: 3
---

# Skill Discovery

Find, evaluate, and install skills from the community.

## Overview

```mermaid
flowchart LR
    SEARCH["SEARCH"] --> BROWSE["BROWSE"] --> EVALUATE["EVALUATE"] --> INSTALL["INSTALL"] --> SYNC["SYNC"]
```

---

## Step 1: Search

Find skills by keyword, or browse popular skills:

```bash
skillshare search              # Browse popular skills
skillshare search pdf
skillshare search "code review"
skillshare search react
```

---

## Step 2: Browse Repositories

Explore skills in a repository:

```bash
# Official Anthropic skills
skillshare install anthropics/skills

# Community skills
skillshare install ComposioHQ/awesome-claude-skills
```

This enters **discovery mode** — shows all available skills in the repo.

---

## Step 3: Evaluate

Before installing, consider:

- **Does it solve my problem?** Read the description
- **Is it well-maintained?** Check the repo's activity
- **Will it conflict?** Check for name collisions with existing skills

Preview what would be installed:
```bash
skillshare install anthropics/skills/skills/pdf --dry-run
```

---

## Step 4: Install

### Single skill

```bash
skillshare install anthropics/skills/skills/pdf
```

### Multiple skills from one repo

```bash
# Interactive browse
skillshare install anthropics/skills

# Select specific skills (non-interactive)
skillshare install anthropics/skills -s pdf,commit

# Install all skills
skillshare install anthropics/skills --all
```

### Entire repo (for teams)

```bash
skillshare install github.com/team/skills --track
```

---

## Step 5: Sync

Don't forget to sync after installing:

```bash
skillshare sync
```

---

## Popular Skill Sources

| Source | URL |
|--------|-----|
| Anthropic Official | `anthropics/skills` |
| Vercel Agent Skills | `vercel-labs/agent-skills` |
| Community | [skillsmp.com](https://skillsmp.com/) |

---

## Discovery Commands

| Command | Purpose |
|---------|---------|
| `search` | Browse popular skills |
| `search <query>` | Search for skills |
| `check` | Check for available updates |
| `install <repo>` | Browse repo (discovery mode) |
| `install <repo/path>` | Install specific skill |
| `list` | Show installed skills |

---

## Installing Options

```bash
# Custom name
skillshare install anthropics/skills/skills/pdf --name my-pdf

# Force overwrite
skillshare install anthropics/skills/skills/pdf --force

# Update existing
skillshare install anthropics/skills/skills/pdf --update

# Track for team sharing
skillshare install github.com/team/skills --track
```

`--name` is valid only when the install target is a single skill.  
Using `--name` with repo discovery that returns multiple skills will return an error.

---

## After Installing

### Verify

```bash
skillshare list
skillshare status
```

### Test

Use the skill in your AI CLI to make sure it works as expected.

### Check for updates

```bash
skillshare check              # See what has updates available
```

### Update later

```bash
# Single skill (with source metadata)
skillshare install pdf --update

# Tracked repo
skillshare update _team-skills
```

---

## See Also

- [search](/docs/reference/commands/search) — Search command reference
- [install](/docs/reference/commands/install) — Install command reference
- [Hub Index](/docs/how-to/sharing/hub-index) — Managing skill hubs
- [Daily Workflow](./daily-workflow.md) — After installing, use daily
