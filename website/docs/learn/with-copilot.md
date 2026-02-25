---
sidebar_position: 2
---

# Using skillshare with GitHub Copilot

> From install to first sync — 5 minutes.

## Prerequisites

- [GitHub Copilot](https://github.com/features/copilot) coding agent enabled in VS Code or JetBrains
- macOS, Linux, or Windows

## Step 1: Install skillshare

```bash
curl -fsSL https://raw.githubusercontent.com/runkids/skillshare/main/install.sh | sh
```

## Step 2: Initialize

```bash
skillshare init
```

This detects Copilot's skill directory (`~/.copilot/skills/`) and adds it as a target automatically.

## Step 3: Switch to Copy Mode (Recommended)

We've received reports that Copilot sometimes fails to follow symlinks correctly. To avoid issues, switch the Copilot target to **copy mode**:

```bash
skillshare target copilot --mode copy
```

Copy mode physically copies skill files into `~/.copilot/skills/` instead of creating symlinks. The trade-off is that source edits aren't reflected instantly — you need to run `skillshare sync` to propagate changes. But it's more reliable across platforms.

:::tip When to use merge (symlink) mode
If you're on macOS or Linux and Copilot reads symlinks correctly on your machine, the default merge mode works fine. You can always switch back:

```bash
skillshare target copilot --mode merge
```
:::

## Step 4: Install Your First Skill

```bash
skillshare install runkids/my-skills
```

## Step 5: Sync

```bash
skillshare sync
```

Skills are copied to `~/.copilot/skills/`. Copilot picks them up as custom instructions.

## Step 6: Verify

```bash
ls ~/.copilot/skills/
```

You should see your installed skills as real directories (in copy mode) or symlinks (in merge mode).

## Copilot-Specific Notes

- **Skill path**: `~/.copilot/skills/` (global) or `.github/skills/` (project)
- **Project mode**: Run `skillshare init -p` to manage project-level Copilot skills — they go into `.github/skills/` alongside your codebase
- **Symlink issues**: If Copilot doesn't pick up your skills, check if your target is in merge mode (`skillshare status`) and switch to copy mode as described above
- **`.github/copilot-instructions.md`**: If you have an existing instructions file, skillshare skills complement it — they don't replace it

## What's Next?

- [Manage multiple skills →](/docs/how-to/daily-tasks/organizing-skills)
- [Share with your team →](/docs/how-to/sharing/organization-sharing)
- [Explore more skills →](/docs/reference/commands/search)
