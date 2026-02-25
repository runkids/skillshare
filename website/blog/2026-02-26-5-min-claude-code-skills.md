---
slug: 5-min-claude-code-skills
title: 5 Minutes to Manage Your Claude Code Skills
authors: [runkids]
tags: [tutorial, claude-code]
---

Getting Claude Code skills organized shouldn't take more than a coffee break. Here's how to go from zero to a fully managed skill library in under 5 minutes.

<!-- truncate -->

## The Problem

Claude Code stores skills in `~/.claude/skills/`. Over time, you accumulate skills from different sources — some you wrote, some from teammates, some from GitHub. Without structure, it becomes hard to track what's installed, what's outdated, and what conflicts with what.

## Step 1: Install skillshare

```bash
curl -fsSL https://raw.githubusercontent.com/runkids/skillshare/main/install.sh | sh
```

Or via Homebrew (macOS/Linux):

```bash
brew install skillshare
```

:::note
Homebrew releases may lag behind the latest version by a few days. For the newest release, use the install script above.
:::

Verify:

```bash
skillshare version
```

## Step 2: Initialize

```bash
skillshare init
```

This creates a source directory at `~/.config/skillshare/skills/` and auto-detects Claude Code as a sync target.

## Step 3: Import Your Existing Skills

Already have skills in `~/.claude/skills/`? Collect them into your source:

```bash
skillshare collect
```

skillshare copies each skill from Claude Code's directory into your source. Now you have a single place that holds everything.

## Step 4: Install a Skill from GitHub

Let's add a popular skill set:

```bash
skillshare install anthropics/courses/prompt-eng
```

This clones the repository and places skills in your source directory. Run `skillshare list` to see everything:

```bash
skillshare list
```

## Step 5: Sync

```bash
skillshare sync
```

By default, skillshare uses **merge mode** — creating per-skill symlinks from your source into `~/.claude/skills/`. Your original local skills are preserved — only new ones are added.

## Step 6: Verify

Open Claude Code and check that your skills are available:

```bash
claude
# Inside Claude Code, your skills should be loaded
```

## Symlink Not Working? Use Copy Mode

Some platforms and tools have inconsistent symlink support — WSL, certain Docker setups, or AI tools that don't follow symlinks. If you run into issues:

```bash
skillshare target claude --mode copy
skillshare sync
```

Copy mode physically copies skill files instead of symlinking. The trade-off:

| | Merge (symlink) | Copy |
|---|---|---|
| Source changes reflected | Instantly | After `sync` |
| Cross-platform compatibility | May have issues | Works everywhere |
| Local skill preservation | Yes | Yes |

See [Sync Modes](/docs/understand/sync-modes) for the full comparison.

## What You Get

After these steps:

- **Single source of truth** — all skills live in `~/.config/skillshare/skills/`
- **Non-destructive sync** — local Claude Code skills are untouched
- **Version tracking** — installed repos are tracked for updates via `skillshare check`
- **Backup ready** — `skillshare backup` snapshots your entire skill library

## Next Steps

- [Set up multi-tool sync](/docs/learn/with-multiple-tools) if you also use Cursor, Codex, or other tools
- [Create your first custom skill](/docs/how-to/daily-tasks/creating-skills)
- [Share skills with your team](/docs/how-to/sharing/organization-sharing)
