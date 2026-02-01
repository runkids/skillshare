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

**With git remote (recommended for cross-machine sync):**
```bash
skillshare init --remote git@github.com:you/my-skills.git
```

---

## Step 3: Install your first skill

```bash
# Browse available skills
skillshare install anthropics/skills

# Or install directly
skillshare install anthropics/skills/skills/pdf
```

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
- [Share with your team](/docs/guides/team-sharing)
