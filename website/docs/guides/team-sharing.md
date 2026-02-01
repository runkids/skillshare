---
sidebar_position: 3
---

# Team Sharing

Share skills across your entire team using tracked repositories.

## Overview

```
┌─────────────────────────────────────────────────────────────────┐
│                    TEAM SHARING WORKFLOW                        │
│                                                                 │
│   ┌─────────────────────────────────────────────────────────┐   │
│   │              GitHub: team/shared-skills                 │   │
│   │   frontend/ui/   backend/api/   devops/deploy/          │   │
│   └─────────────────────────────────────────────────────────┘   │
│                              │                                  │
│              skillshare install --track                         │
│                              │                                  │
│                              ▼                                  │
│   ┌─────────────────────────────────────────────────────────┐   │
│   │  Alice's Machine          Bob's Machine                 │   │
│   │  _team-skills/            _team-skills/                 │   │
│   │  ├── frontend/ui/         ├── frontend/ui/              │   │
│   │  ├── backend/api/         ├── backend/api/              │   │
│   │  └── devops/deploy/       └── devops/deploy/            │   │
│   └─────────────────────────────────────────────────────────┘   │
│                              │                                  │
│              skillshare update _team-skills                     │
│                              │                                  │
│                              ▼                                  │
│   ┌─────────────────────────────────────────────────────────┐   │
│   │  Everyone gets updates instantly                        │   │
│   └─────────────────────────────────────────────────────────┘   │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

---

## Why Team Sharing?

| Without Team Sharing | With Team Sharing |
|---------------------|-------------------|
| "Hey, grab the latest deploy skill from Slack" | `skillshare update --all` |
| Copy-paste skills between machines | One command installs everything |
| "Which version of the skill do you have?" | Everyone syncs from same source |
| Skills scattered across docs/repos | One curated repo for the team |

---

## For Team Leads

### Step 1: Create a skills repo

Create a GitHub/GitLab/Bitbucket repository for your team's skills.

```bash
mkdir team-skills && cd team-skills
git init

# Create skill structure
mkdir -p frontend/ui backend/api devops/deploy

# Add skills
echo "---
name: acme:ui
description: Frontend UI patterns
---
# UI Skill
..." > frontend/ui/SKILL.md

git add .
git commit -m "Initial skills"
git push -u origin main
```

### Step 2: Share the install command

Send this to your team:

```bash
skillshare install github.com/your-org/team-skills --track && skillshare sync
```

---

## For Team Members

### Initial setup

```bash
# Install the team skills repo
skillshare install github.com/team/skills --track

# Sync to your AI CLIs
skillshare sync
```

### Daily usage

```bash
# Check for updates
skillshare update --all
skillshare sync
```

---

## Nested Skills & Auto-Flattening

Organize skills in folders — skillshare auto-flattens them for AI CLI compatibility:

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

**Benefits:**
- Keep logical folder organization in your repo
- AI CLIs see flat structure they expect
- Flattened names preserve origin path for traceability

See [Tracked Repositories](/docs/concepts/tracked-repositories#nested-skills--auto-flattening) for details.

---

## Collision Detection

When multiple skills have the same `name` field, sync warns you:

```
Warning: skill name collision detected
  "ui" defined in:
    - _team-a/frontend/ui/SKILL.md
    - _team-b/components/ui/SKILL.md
```

**Solution:** Use namespaced names:

```yaml
# In _team-a/frontend/ui/SKILL.md
name: team-a:ui

# In _team-b/components/ui/SKILL.md
name: team-b:ui
```

---

## Multiple Team Repos

Install multiple team repos:

```bash
# Frontend team
skillshare install github.com/org/frontend-skills --track --name frontend

# Backend team
skillshare install github.com/org/backend-skills --track --name backend

# DevOps team
skillshare install github.com/org/devops-skills --track --name devops

skillshare sync
```

Update all:
```bash
skillshare update --all
skillshare sync
```

---

## Private Repositories

Use SSH URLs for private repos:

```bash
skillshare install git@github.com:org/private-skills.git --track
```

---

## Commands Reference

| Command | Description |
|---------|-------------|
| `install <url> --track` | Clone repo as tracked repository |
| `update <name>` | Git pull specific tracked repo |
| `update --all` | Update all tracked repos |
| `uninstall <name>` | Remove tracked repo |
| `list` | List all skills and tracked repos |
| `status` | Show sync status |

---

## Best Practices

### For Team Leads

1. **Use clear structure**: Organize by function (frontend, backend, devops)
2. **Namespace skills**: `team:skill-name` to avoid collisions
3. **Document requirements**: README with setup instructions
4. **Version control**: Use tags for stable releases

### For Team Members

1. **Update regularly**: `skillshare update --all` daily
2. **Report issues**: If a skill doesn't work, tell the maintainer
3. **Suggest improvements**: Open PRs to the skills repo

---

## Related

- [Tracked Repositories](/docs/concepts/tracked-repositories) — How tracked repos work
- [Cross-Machine Sync](./cross-machine-sync) — Personal cross-machine sync
- [Commands: install](/docs/commands/install) — Install command details
