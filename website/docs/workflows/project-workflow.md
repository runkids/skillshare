---
sidebar_position: 6
---

# Project Workflow

The edit → sync → commit cycle for project-level skill management.

## Overview

```
┌─────────────────────────────────────────────────────────────────┐
│                  PROJECT WORKFLOW                               │
│                                                                 │
│   EDIT ──► SYNC ──► COMMIT ──► PUSH                             │
│     │        │                    │                             │
│     ▼        ▼                    ▼                             │
│   .skillshare/   .claude/       Remote                          │
│   skills/        .cursor/       (GitHub)                        │
│                  etc.              │                            │
│                                    ▼                            │
│                              Team members                       │
│                              clone + install + sync             │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

---

## Team Collaboration Scenario

A typical team workflow showing how project skills stay in sync:

```
Alice (adds a skill)                    Bob (gets the update)
──────────────────────                  ──────────────────────
skillshare new api-guide -p
$EDITOR .skillshare/skills/api-guide/
skillshare sync
git add . && git commit && git push
                                        git pull
                                        skillshare install -p
                                        skillshare sync
                                        → api-guide now in .claude/skills/
```

Bob doesn't need to know which skills were added — `skillshare install -p` reads the config and installs everything listed.

---

## Common Operations

### Add a New Skill

```bash
# Create the skill
skillshare new my-skill -p
$EDITOR .skillshare/skills/my-skill/SKILL.md

# Sync to targets
skillshare sync

# Commit
git add .skillshare/
git commit -m "Add my-skill"
```

### Install a Remote Skill

```bash
# Install from GitHub
skillshare install anthropics/skills/skills/pdf -p

# Sync to targets
skillshare sync

# Commit config changes
git add .skillshare/
git commit -m "Add pdf skill from anthropic"
```

### Update Remote Skills

```bash
# Update a specific skill
skillshare update pdf -p

# Or update all remote skills
skillshare update --all -p

# Sync updated skills
skillshare sync

# Commit if config changed
git add .skillshare/
git commit -m "Update remote skills"
```

### Remove a Skill

```bash
# Uninstall
skillshare uninstall my-skill -p

# Sync to clean up symlinks
skillshare sync

# Commit
git add .skillshare/
git commit -m "Remove my-skill"
```

### Anyone Joins the Project

Whether it's a new team member, an open source contributor, or someone trying a community template — the setup is the same:

```bash
# Clone the project
git clone github.com/team/project
cd project

# Install remote skills listed in config
skillshare install -p

# Sync to targets
skillshare sync
```

The `config.yaml` acts as a portable skill manifest — no manual skill hunting required.

---

## Managing Targets

### Add a Target

```bash
# Add a known target
skillshare target add windsurf -p

# Add a custom target with path
skillshare target add custom-tool ./tools/ai/skills -p

# Sync to new target
skillshare sync
```

### Remove a Target

```bash
skillshare target remove windsurf -p
```

### List Targets

```bash
skillshare target list -p
```

```
Project Targets
  claude    .claude/skills (merge)
  cursor         .cursor/skills (merge)
```

---

## Check Status

```bash
skillshare status
```

```
Project Skills (.skillshare/)

Source
  ✓ .skillshare/skills (3 skills)

Targets
  ✓ claude  [merge] .claude/skills (3 synced)
  ✓ cursor       [merge] .cursor/skills (3 synced)

Remote Skills
  ✓ pdf          anthropic/skills/pdf
  ✓ review       github.com/team/tools
```

---

## List Skills

```bash
skillshare list
```

```
Installed skills (project)
─────────────────────────────────────────
  → my-skill            local
  → pdf                 anthropic/skills/pdf
  → review              github.com/team/tools

→ 3 skill(s): 2 remote, 1 local
```

---

## Web Dashboard

Use the web UI for visual project skill management:

```bash
skillshare ui -p
```

Or just `skillshare ui` if `.skillshare/config.yaml` exists (auto-detected). The dashboard hides Git Sync (use your project's own git) and edits `.skillshare/config.yaml` directly.

---

## Tips

### Auto-Detection

Once `.skillshare/config.yaml` exists, most commands auto-detect project mode:

```bash
cd my-project/
skillshare sync          # Auto project mode
skillshare status        # Auto project mode
skillshare list          # Auto project mode
```

:::tip Zero Config
Just `cd` into a project directory — skillshare detects `.skillshare/config.yaml` and automatically switches to project mode. No flags needed.
:::

### Edit and See Changes Instantly

Skills are symlinked — editing in `.skillshare/skills/` is immediately visible in targets:

```bash
$EDITOR .skillshare/skills/my-skill/SKILL.md
# Change is already in .claude/skills/my-skill/ (symlink)
```

Only run `sync` when adding/removing skills or targets.

### Preview Before Syncing

```bash
skillshare sync --dry-run
```

---

## Related

- [Project Skills](/docs/concepts/project-skills) — Core concepts
- [Project Setup](/docs/guides/project-setup) — Initial setup guide
- [Daily Workflow](/docs/workflows/daily-workflow) — Global mode daily workflow
- [Commands: sync](/docs/commands/sync) — Sync command details
