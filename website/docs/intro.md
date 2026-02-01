---
sidebar_position: 1
slug: /intro
---

# Introduction

**skillshare** is a CLI tool that syncs AI CLI skills from a single source to all your AI coding assistants.

## Why skillshare?

Install tools get skills onto agents. **Skillshare keeps them in sync.**

| | Install-once tools | skillshare |
|---|-------------------|------------|
| After install | Done, no management | **Continuous sync** across all agents |
| Update a skill | Re-install manually | **Edit once**, sync everywhere |
| Pull back edits | ✗ | **Bidirectional** — pull from any agent |
| Cross-machine | ✗ | **push/pull** via git |
| Team sharing | Copy-paste | **Tracked repos** — `update` to stay current |
| AI integration | Manual CLI | **Built-in skill** — AI operates it directly |

## Quick Start

```bash
# Install
curl -fsSL https://raw.githubusercontent.com/runkids/skillshare/main/install.sh | sh

# Initialize (auto-detects CLIs, sets up git)
skillshare init

# Sync to all targets
skillshare sync
```

Done. Your skills are now synced across all AI CLI tools.

## How It Works

```
┌─────────────────────────────────────────────────────────────┐
│                       Source Directory                      │
│                 ~/.config/skillshare/skills/                │
└─────────────────────────────────────────────────────────────┘
                              │ sync
              ┌───────────────┼───────────────┐
              ▼               ▼               ▼
       ┌───────────┐   ┌───────────┐   ┌───────────┐
       │  Claude   │   │  OpenCode │   │  Cursor   │   ...
       └───────────┘   └───────────┘   └───────────┘
```

- **Source**: Single directory where you edit skills (`~/.config/skillshare/skills/`)
- **Targets**: AI CLI skill directories (symlinked from source)
- **Sync**: Creates/updates symlinks from source to targets

## Core Concepts

### Source vs Targets

```
┌─────────────────────────────────────────┐
│         SOURCE (edit here)              │
│   ~/.config/skillshare/skills/          │
│                                         │
│   my-skill/   another/   _team-repo/    │
└─────────────────────────────────────────┘
                    │
                    │ skillshare sync
                    ▼
┌─────────────────────────────────────────┐
│              TARGETS                    │
│   ~/.claude/skills/  (symlinks)         │
│   ~/.cursor/skills/  (symlinks)         │
│   ~/.codex/skills/   (symlinks)         │
└─────────────────────────────────────────┘
```

- **Source**: Single directory where you edit skills
- **Targets**: AI CLI skill directories (symlinked from source)
- **Sync**: Creates/updates symlinks from source to targets

### Sync Modes

| Mode | How it works |
|------|--------------|
| `merge` | Each skill symlinked individually. Local skills preserved. **(default)** |
| `symlink` | Entire directory is one symlink. All targets identical. |

See [sync](/docs/commands/sync#sync-modes) for details.

## Supported Platforms

| Platform | Source Path | Link Type |
|----------|-------------|-----------|
| macOS/Linux | `~/.config/skillshare/skills/` | Symlinks |
| Windows | `%USERPROFILE%\.config\skillshare\skills\` | NTFS Junctions |

## Command Quick Reference

| Command | What it does | Docs |
|---------|--------------|------|
| `init` | First-time setup | [init](/docs/commands/init) |
| `search` | Discover skills | [search](/docs/commands/search) |
| `new` | Create a skill | [new](/docs/commands/new) |
| `install` | Add a skill | [install](/docs/commands/install) |
| `uninstall` | Remove a skill | [install](/docs/commands/install#uninstall) |
| `update` | Update a skill | [install](/docs/commands/install#update) |
| `upgrade` | Upgrade CLI/skill | [install](/docs/commands/install#upgrade) |
| `sync` | Push to targets | [sync](/docs/commands/sync) |
| `pull` | Pull from git remote | [sync](/docs/commands/sync#pull) |
| `push` | Push to git remote | [sync](/docs/commands/sync#push) |
| `backup` | Backup targets | [sync](/docs/commands/sync#backup) |
| `restore` | Restore from backup | [sync](/docs/commands/sync#restore) |
| `target` | Manage targets | [targets](/docs/guides/targets) |
| `list` | List skills | [install](/docs/commands/install#list) |
| `status` | Show sync state | [sync](/docs/commands/sync#status) |
| `diff` | Show differences | [sync](/docs/commands/sync#diff) |
| `doctor` | Diagnose issues | [faq](/docs/faq#doctor) |

## Next Steps

- [Commands Reference](/docs/commands/init) — All available commands
- [Team Edition](/docs/guides/team-edition) — Share skills with your team
- [Cross-Machine Sync](/docs/guides/cross-machine) — Sync across computers
- [FAQ](/docs/faq) — Troubleshooting
