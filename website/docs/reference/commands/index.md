---
sidebar_position: 1
---

# Commands

Complete reference for all skillshare commands.

## What do you want to do?

| I want to... | Command |
|--------------|---------|
| Set up skillshare for the first time | [`init`](./init.md) |
| Install a skill from GitHub | [`install`](./install.md) |
| Create my own skill | [`new`](./new.md) |
| Sync skills to all AI CLIs | [`sync`](./sync.md) |
| Check what's out of sync | [`status`](./status.md) / [`diff`](./diff.md) |
| Search for community skills | [`search`](./search.md) |
| Update installed skills | [`check`](./check.md) then [`update`](./update.md) |
| Temporarily hide a skill without removing it | [`enable` / `disable`](./enable.md) |
| Sync across machines | [`push`](./push.md) / [`pull`](./pull.md) |
| Manage non-skill resources (rules, commands) | [`extras`](./extras.md) |
| Manage native Claude/Codex plugins | [`plugins`](./plugins.md) |
| Manage standalone Claude/Codex hooks | [`hooks`](./hooks.md) |
| Manage single-file `.md` agents | Most commands accept `agents` or `--kind agent` — see [Agents](/docs/understand/agents) |
| See which skills use the most context tokens | [`analyze`](./analyze.md) |
| Fix something broken | [`doctor`](./doctor.md) |
| Open the web dashboard | [`ui`](./ui.md) |

---

## Overview

| Category | Commands |
|----------|----------|
| **Core** | `init`, `install`, `uninstall`, `list`, `search`, `sync`, `status` |
| **Skill Management** | `new`, `check`, `update`, `upgrade`, `enable`, `disable` |
| **Target Management** | `target`, `diff` |
| **Native Integrations** | `plugins`, `hooks` |
| **Extras Management** | `extras` (`init`, `list`, `remove`, `collect`) |
| **Sync Operations** | `collect`, `backup`, `restore`, `trash`, `push`, `pull` |
| **Security & Utilities** | `analyze`, `audit`, `hub`, `log`, `doctor`, `tui`, `ui`, `version` |

---

## Core Commands

| Command | Description |
|---------|-------------|
| [init](./init.md) | First-time setup |
| [install](./install.md) | Add a skill from a repo or path |
| [uninstall](./uninstall.md) | Remove a skill |
| [list](./list.md) | List all skills |
| [search](./search.md) | Search for skills |
| [sync](./sync.md) | Push skills to all targets |
| [status](./status.md) | Show sync state |

## Skill Management

| Command | Description |
|---------|-------------|
| [new](./new.md) | Create a new skill |
| [check](./check.md) | Check for available updates |
| [update](./update.md) | Update a skill or tracked repo |
| [upgrade](./upgrade.md) | Upgrade CLI or built-in skill |
| [enable / disable](./enable.md) | Temporarily enable or disable skills |

## Target Management

| Command | Description |
|---------|-------------|
| [target](./target.md) | Manage targets |
| [diff](./diff.md) | Show differences between source and targets |

## Extras Management

| Command | Description |
|---------|-------------|
| [extras](./extras.md) | Manage non-skill resources (rules, commands, prompts) |

## Native Integrations

| Command | Description |
|---------|-------------|
| [plugins](./plugins.md) | Manage native Claude/Codex plugin bundles |
| [hooks](./hooks.md) | Manage standalone Claude/Codex hook bundles |

## Sync Operations

| Command | Description |
|---------|-------------|
| [collect](./collect.md) | Collect skills from target to source |
| [backup](./backup.md) | Create backup of targets |
| [restore](./restore.md) | Restore targets from backup |
| [trash](./trash.md) | Manage uninstalled skills in trash |
| [push](./push.md) | Push to git remote |
| [pull](./pull.md) | Pull from git remote and sync |

## Security & Utilities

| Command | Description |
|---------|-------------|
| [analyze](./analyze.md) | Analyze context window usage |
| [audit](./audit.md) | Scan skills for security threats |
| [log](./log.md) | View operations and audit logs |
| [doctor](./doctor.md) | Diagnose issues |
| [tui](./tui.md) | Toggle interactive TUI mode |
| [ui](./ui.md) | Launch web dashboard |
| [hub](./hub.md) | Manage skill hub sources |
| [version](./version.md) | Show CLI version |

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
skillshare sync --all --json

# Cross-machine
skillshare push -m "Add skill"
skillshare pull

# Status
skillshare status
skillshare list
skillshare diff

# Enable/disable skills
skillshare disable draft-*
skillshare enable draft-*

# Maintenance
skillshare update --all
skillshare analyze
skillshare audit
skillshare log
skillshare doctor
skillshare backup

# TUI preferences
skillshare tui            # Show current status
skillshare tui off        # Disable interactive TUI
skillshare tui on         # Re-enable TUI

# Web UI
skillshare ui
skillshare ui -p          # Project mode

# Native integrations
skillshare plugins list
skillshare hooks sync --target all

# Hub
skillshare hub list
skillshare hub add https://hub.example.com/index.json

# Check for updates
skillshare check

# Trash management
skillshare trash list
skillshare trash restore my-skill

# Version
skillshare version
```

---

## Related

- [Quick Reference](/docs/getting-started/quick-reference) — Command cheat sheet
- [Workflows](/docs/how-to/daily-tasks) — Common usage patterns
