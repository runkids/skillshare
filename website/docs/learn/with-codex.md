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

If the installer prints PATH setup instructions, follow them before running the commands below. If it prints no PATH warning, no extra setup is needed.

## Step 2: Initialize

```bash
skillshare init
```

Codex reads `~/.agents/skills/`, the shared directory it documents as the user-level skills path. `init` detects Codex through its config directory (`~/.codex/`) and sets up the shared `universal` target for it automatically.

## Step 3: Install Your First Skill

```bash
skillshare install runkids/my-skills
```

## Step 4: Sync

```bash
skillshare sync
```

Skills are symlinked to `~/.agents/skills/`.

## Step 5: Verify

```bash
ls ~/.agents/skills/
```

You should see your installed skill symlinked.

## Codex-Specific Notes

- **Skill path**: `~/.agents/skills/` (global) or `.agents/skills/` (project)
- **Existing setups**: if your config still points `codex` at `~/.codex/skills`, Codex keeps reading it. But with `universal` enabled as well, every skill shows up twice — remove the `codex` target (preview with `skillshare target remove codex --dry-run`)
- **Description limit**: Codex has a 1024-character limit on skill descriptions. Keep the `description` field in `SKILL.md` frontmatter concise
- **Project mode**: Run `skillshare init -p` to manage project-level Codex skills

## What's Next?

- [Manage multiple skills →](/docs/how-to/daily-tasks/organizing-skills)
- [Share with your team →](/docs/how-to/sharing/organization-sharing)
- [Explore more skills →](/docs/reference/commands/search)
