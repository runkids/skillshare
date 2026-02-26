---
sidebar_position: 4
---

# Using skillshare with Multiple AI Tools

> One source of truth, synced to every AI CLI you use.

## The Problem

You use Claude Code at work, Cursor for side projects, and Codex for experiments. Each has its own skill directory. Keeping them in sync manually is tedious and error-prone.

## The Solution

skillshare maintains a single source directory and syncs to all your targets with one command.

## Step 1: Install and Initialize

```bash
curl -fsSL https://raw.githubusercontent.com/runkids/skillshare/main/install.sh | sh
skillshare init
```

`init` auto-detects all installed AI tools and adds them as targets.

## Step 2: Check Your Targets

```bash
skillshare target list
```

Example output:

```
  claude       ~/.claude/skills (merge)
  cursor       ~/.cursor/skills (merge)
  codex        ~/.codex/skills (merge)
  opencode     ~/.config/opencode/skills (merge)
```

## Step 3: Install Skills

```bash
skillshare install runkids/my-skills
skillshare install anthropics/courses/prompt-eng
```

## Step 4: Sync Everything

```bash
skillshare sync
```

One command pushes all skills to every target. Each target gets symlinks pointing back to your single source.

## Step 5: Verify

```bash
skillshare status
```

Shows sync status across all targets — which skills are synced, missing, or out of date.

## Per-Target Mode Control

Different tools have different needs. You can set sync mode per target:

```bash
# Cursor follows symlinks fine (default)
skillshare target cursor --mode merge

# Some tools need real files
skillshare target opencode --mode copy
```

## What's Next?

- [Understand sync modes →](/docs/understand/sync-modes)
- [Cross-machine sync →](/docs/how-to/sharing/cross-machine-sync)
- [Team sharing →](/docs/how-to/sharing/organization-sharing)
