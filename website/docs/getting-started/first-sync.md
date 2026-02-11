---
sidebar_position: 2
---

# First Sync

Get your skills synced in 5 minutes.

## Prerequisites

- macOS, Linux, or Windows
- At least one AI CLI installed (Claude Code, Cursor, Codex, etc.)

---

## Step 1: Install skillshare

**macOS / Linux:**
```bash
curl -fsSL https://raw.githubusercontent.com/runkids/skillshare/main/install.sh | sh
```

**Windows (PowerShell):**
```powershell
irm https://raw.githubusercontent.com/runkids/skillshare/main/install.ps1 | iex
```

---

## Step 2: Initialize

```bash
skillshare init
```

This:
1. Creates your source directory (`~/.config/skillshare/skills/`)
2. Auto-detects installed AI CLIs
3. Sets up configuration
4. Optionally installs the built-in skillshare skill (adds `/skillshare` command to AI CLIs)

:::tip Built-in Skill
During init, you'll be prompted: `Install built-in skillshare skill? [y/N]`. This adds a skill that lets your AI CLI manage skillshare directly. You can skip it and install later with `skillshare upgrade --skill`.
:::

**With git remote (recommended for cross-machine sync):**
```bash
skillshare init --remote git@github.com:you/my-skills.git
```

If the remote already has skills (e.g., from another machine), they'll be pulled automatically during init.

---

## Step 3: Install your first skill

```bash
# Browse available skills
skillshare install anthropics/skills

# Or install directly
skillshare install anthropics/skills/skills/pdf
```

Skills are automatically scanned for security threats during install. If critical issues are found, the install is blocked — use `--force` to override.

---

## Step 4: Sync to all targets

```bash
skillshare sync
```

Your skill is now available in all your AI CLIs.

---

## Verify

```bash
skillshare status
```

You should see:
- Source directory with your skill
- Targets (Claude, Cursor, etc.) showing "synced"

---

## What's Next?

- [Create your own skill](/docs/guides/creating-skills)
- [Sync across machines](/docs/guides/cross-machine-sync)
- [Organization-wide skills](/docs/guides/organization-sharing)
