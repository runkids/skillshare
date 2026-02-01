---
sidebar_position: 1
---

# Commands

Complete reference for all skillshare commands.

## Overview

| Category | Commands |
|----------|----------|
| **Core** | `init`, `install`, `uninstall`, `list`, `search`, `sync`, `status` |
| **Skill Management** | `new`, `update`, `upgrade` |
| **Target Management** | `target`, `diff` |
| **Sync Operations** | `collect`, `backup`, `restore`, `push`, `pull` |
| **Utilities** | `doctor` |

---

## Core Commands

| Command | Description |
|---------|-------------|
| [init](./init) | First-time setup |
| [install](./install) | Add a skill from a repo or path |
| [uninstall](./uninstall) | Remove a skill |
| [list](./list) | List all skills |
| [search](./search) | Search for skills |
| [sync](./sync) | Push skills to all targets |
| [status](./status) | Show sync state |

## Skill Management

| Command | Description |
|---------|-------------|
| [new](./new) | Create a new skill |
| [update](./update) | Update a skill or tracked repo |
| [upgrade](./upgrade) | Upgrade CLI or built-in skill |

## Target Management

| Command | Description |
|---------|-------------|
| [target](./target) | Manage targets |
| [diff](./diff) | Show differences between source and targets |

## Sync Operations

| Command | Description |
|---------|-------------|
| [collect](./collect) | Collect skills from target to source |
| [backup](./backup) | Create backup of targets |
| [restore](./restore) | Restore targets from backup |
| [push](./push) | Push to git remote |
| [pull](./pull) | Pull from git remote and sync |

## Utilities

| Command | Description |
|---------|-------------|
| [doctor](./doctor) | Diagnose issues |

---

## Common Flags

Most commands support:

| Flag | Description |
|------|-------------|
| `--dry-run`, `-n` | Preview without making changes |
| `--help`, `-h` | Show help |

---

## Quick Reference

```bash
# Setup
skillshare init
skillshare init --remote git@github.com:you/skills.git

# Install skills
skillshare install anthropics/skills/skills/pdf
skillshare install github.com/team/skills --track

# Create skill
skillshare new my-skill

# Sync
skillshare sync
skillshare sync --dry-run

# Cross-machine
skillshare push -m "Add skill"
skillshare pull

# Status
skillshare status
skillshare list
skillshare diff

# Maintenance
skillshare update --all
skillshare doctor
skillshare backup
```

---

## Related

- [Quick Reference](/docs/getting-started/quick-reference) — Command cheat sheet
- [Workflows](/docs/workflows) — Common usage patterns
