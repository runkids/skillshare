---
sidebar_position: 1
---

# Using skillshare with Claude Code

> From install to first sync — 5 minutes.

## Prerequisites

- [Claude Code](https://docs.anthropic.com/en/docs/claude-code/overview) installed and working
- macOS, Linux, or Windows (WSL)

## Step 1: Install skillshare

```bash
curl -fsSL https://raw.githubusercontent.com/runkids/skillshare/main/install.sh | sh
```

## Step 2: Initialize

```bash
skillshare init
```

This detects Claude Code's skill directory (`~/.claude/skills/`) and adds it as a target automatically.

## Step 3: Install Your First Skill

```bash
skillshare install anthropics/courses/prompt-eng
```

The skill is downloaded, audited for security, and added to your source directory.

## Step 4: Sync

```bash
skillshare sync
```

This creates symlinks from your source to `~/.claude/skills/`. Claude Code picks up the skills immediately — no restart needed.

## Step 5: Verify

```bash
ls ~/.claude/skills/
```

You should see your installed skill symlinked.

## Claude Code Integration Details

- **Skill path**: `~/.claude/skills/` (global) or `.claude/skills/` (project)
- **CLAUDE.md**: skillshare skills use `SKILL.md` format, which Claude Code reads natively
- **Project mode**: Run `skillshare init -p` inside a repo to manage `.claude/skills/` per-project

## What's Next?

- [Manage multiple skills →](/docs/how-to/daily-tasks/organizing-skills)
- [Share with your team →](/docs/how-to/sharing/organization-sharing)
- [Explore more skills →](/docs/reference/commands/search)
