---
sidebar_position: 1
slug: /
---

# Introduction

**skillshare** is a CLI tool that syncs AI CLI skills from a single source to all your AI coding assistants.

## Why skillshare?

Install tools get skills onto agents. **skillshare keeps them in sync.**

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

:::tip[Try without installing]
Want to explore first? Use the [Docker Playground](/docs/how-to/advanced/docker-sandbox#playground) — one command, no local install needed:

```bash
git clone https://github.com/runkids/skillshare.git && cd skillshare
make playground
```
:::

## How It Works

```mermaid
flowchart LR
    subgraph ORG["ORGANIZATION"]
        ORG_SRC["~/.config/skillshare/skills/"] -- sync --> ORG_TGT["~/.claude/skills/ etc."]
    end

    subgraph PROJ["PROJECT"]
        PROJ_SRC[".skillshare/skills/"] -- sync --> PROJ_TGT[".claude/skills/ etc."]
    end
```

Edit in source → all targets update. Edit in target → changes go to source (via symlinks).

## Key Features

- **Auto-Detection** — `cd` into a project with `.skillshare/` and skillshare switches to project mode automatically
- **Dual-Level Architecture** — Organization skills for company standards + project skills for repo context
- **Instant Updates** — Symlink-based sync means edits reflect immediately across all AI tools
- **Team Ready** — Organization skills via tracked repos, project skills via git commit
- **Any Git Host** — Install, update, and check from GitHub, GitLab, Bitbucket, or any self-hosted Git
- **Security Audit** — Scan skills for prompt injection, data exfiltration, and threats. Auto-scans on install

## Supported Platforms

| Platform | Source Path | Link Type |
|----------|-------------|-----------|
| macOS/Linux | `~/.config/skillshare/skills/` | Symlinks |
| Windows | `%AppData%\skillshare\skills\` | NTFS Junctions |

## Next Steps

### Individual Developer

1. [First Sync](/docs/getting-started/first-sync) — Get synced in 5 minutes
2. [Creating Skills](/docs/how-to/daily-tasks/creating-skills) — Write your first skill
3. [Cross-Machine Sync](/docs/how-to/sharing/cross-machine-sync) — Keep skills in sync across machines

### Team Lead / Organization

1. [Organization-Wide Skills](/docs/how-to/sharing/organization-sharing) — Share standards across the team
2. [Project Setup](/docs/how-to/sharing/project-setup) — Set up project-scoped skills
3. [Security Audit](/docs/reference/commands/audit) — Scan third-party skills before deployment

### Already Have Skills?

- [From Existing Skills](/docs/getting-started/from-existing-skills) — Migrate and consolidate

### Explore More

- [Core Concepts](/docs/understand) — Source, targets, sync modes
- [Commands Reference](/docs/reference/commands) — All available commands
- [Docker Sandbox](/docs/how-to/advanced/docker-sandbox) — Try skillshare in an isolated environment
- [FAQ](/docs/troubleshooting/faq) — Common questions
