---
sidebar_position: 3
---

# Using skillshare with Codex

> From install to first sync — 5 minutes.

## Prerequisites

- [OpenAI Codex CLI](https://github.com/openai/codex) installed and working
- macOS, Linux, or Windows

## Step 1: Install skillshare

```bash
curl -fsSL https://raw.githubusercontent.com/runkids/skillshare/main/install.sh | sh
```

## Step 2: Initialize

```bash
skillshare init
```

This detects Codex's skill directory (`~/.codex/skills/`) and adds it as a target automatically.

## Step 3: Install Your First Skill

```bash
skillshare install runkids/my-skills
```

## Step 4: Sync

```bash
skillshare sync
```

Skills are symlinked to `~/.codex/skills/`.

## Step 5: Verify

```bash
ls ~/.codex/skills/
```

You should see your installed skill symlinked.

## Codex-Specific Notes

- **Skill path**: `~/.codex/skills/` (global) or `.agents/skills/` (project)
- **Description limit**: Codex has a 1024-character limit on skill descriptions. Keep the `description` field in `SKILL.md` frontmatter concise
- **Project mode**: Run `skillshare init -p` to manage project-level Codex skills

## What's Next?

- [Manage multiple skills →](/docs/how-to/daily-tasks/organizing-skills)
- [Share with your team →](/docs/how-to/sharing/organization-sharing)
- [Explore more skills →](/docs/reference/commands/search)
