---
sidebar_position: 1
slug: /
---

# Introduction

**skillshare** is a CLI tool that syncs AI CLI skills from a single source to all your AI coding assistants.

## Why skillshare?

Install tools get skills onto agents. **Skillshare keeps them in sync.**

| | Install-once tools | skillshare |
|---|-------------------|------------|
| After install | Run update commands manually | **Merge sync** — per-skill symlinks, local skills preserved |
| Update a skill | `npx skills update` / re-run | **Edit source**, changes reflect instantly |
| Pull back edits | — | **Bidirectional** — collect from any agent |
| Cross-machine | Re-run install on each machine | **git push/pull** — one command sync |
| Local + installed | Managed separately | **Unified** in single source directory |
| Team sharing | Commit skills.json or re-install | **Tracked repos** — git pull to update |
| AI integration | Manual CLI only | **Built-in skill** — AI operates directly |

## Quick Start

```bash
# Install
curl -fsSL https://raw.githubusercontent.com/runkids/skillshare/main/install.sh | sh

# Initialize (auto-detects CLIs, sets up git)
skillshare init

# Install a skill
skillshare install anthropics/skills/skills/pdf

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

Edit in source → all targets update. Edit in target → changes go to source (via symlinks).

## Supported Platforms

| Platform | Source Path | Link Type |
|----------|-------------|-----------|
| macOS/Linux | `~/.config/skillshare/skills/` | Symlinks |
| Windows | `%USERPROFILE%\.config\skillshare\skills\` | NTFS Junctions |

## Next Steps

**New to skillshare?**
- [First Sync](/docs/getting-started/first-sync) — Get synced in 5 minutes

**Already have skills?**
- [From Existing Skills](/docs/getting-started/from-existing-skills) — Migrate and consolidate

**Learn more:**
- [Core Concepts](/docs/concepts) — Source, targets, sync modes
- [Commands Reference](/docs/commands) — All available commands
- [FAQ](/docs/troubleshooting/faq) — Common questions
