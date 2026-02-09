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
| Update a skill | Run update command / re-run install | **Edit source**, changes reflect instantly |
| Pull back edits | — | **Bidirectional** — collect from any agent |
| Cross-machine | Re-run install on each machine | **git push/pull** — one command sync |
| Local + installed | Managed separately | **Unified** in single source directory |
| Organization sharing | Commit skills.json or re-install | **Tracked repos** — git pull to update |
| Project skills | Copy skills per repo, diverge over time | **Project mode** — auto-detected, shared via git |
| Security audit | None | **Built-in** — auto-scan on install, `audit` command |
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
│                    DUAL-LEVEL ARCHITECTURE                  │
│                                                             │
│  ORGANIZATION LEVEL              PROJECT LEVEL              │
│  ~/.config/skillshare/           .skillshare/skills/        │
│  ├── my-skill/                   ├── api-conventions/       │
│  └── _company-std/               └── deploy-guide/          │
│         │                               │                   │
│         ▼ sync                          ▼ sync              │
│  ~/.claude/skills/               .claude/skills/            │
│  (system-wide targets)           (project-local targets)    │
│                                  ← auto-detected by cd      │
└─────────────────────────────────────────────────────────────┘
```

Edit in source → all targets update. Edit in target → changes go to source (via symlinks).

## Key Features

- **Auto-Detection** — `cd` into a project with `.skillshare/` and skillshare switches to project mode automatically
- **Dual-Level Architecture** — Organization skills for company standards + project skills for repo context
- **Instant Updates** — Symlink-based sync means edits reflect immediately across all AI tools
- **Team Ready** — Organization skills via tracked repos, project skills via git commit
- **Security Audit** — Scan skills for prompt injection, data exfiltration, and threats. Auto-scans on install

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

**Project-level skills:**
- [Project Setup](/docs/guides/project-setup) — Set up project-scoped skills

**Learn more:**
- [Core Concepts](/docs/concepts) — Source, targets, sync modes
- [Commands Reference](/docs/commands) — All available commands
- [Docker Sandbox Guide](/docs/guides/docker-sandbox) — Isolated testing and playground
- [FAQ](/docs/troubleshooting/faq) — Common questions
