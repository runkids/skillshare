---
sidebar_position: 3
---

# From Existing Skills

Migrate your existing skills from various AI CLIs to skillshare.

## Overview

If you already have skills in `~/.claude/skills/`, `~/.cursor/skills/`, or other locations, skillshare can consolidate them into a single source.

```
BEFORE:                              AFTER:
─────────────────────────────────────────────────────────
~/.claude/skills/                    Source (single truth)
  ├── skill-a/                       ~/.config/skillshare/skills/
  └── skill-b/                         ├── skill-a/
                                       ├── skill-b/
~/.cursor/skills/                      ├── skill-c/
  ├── skill-b/  (duplicate!)           └── skill-d/
  └── skill-c/
                                     Targets (symlinked)
~/.codex/skills/                     ~/.claude/skills/ → source
  └── skill-d/                       ~/.cursor/skills/ → source
                                     ~/.codex/skills/  → source
```

---

## Step 1: Initialize skillshare

```bash
skillshare init
```

---

## Step 2: Backup your existing skills

Before collecting, create backups:

```bash
skillshare backup
```

---

## Step 3: Collect from each target

Collect skills from each AI CLI to your source:

```bash
# Collect from Claude
skillshare collect claude

# Collect from Cursor
skillshare collect cursor

# Or collect from all at once
skillshare collect --all
```

**What happens:**
1. Local skills (non-symlinked) are copied to source
2. Original files are replaced with symlinks
3. Duplicates are detected and reported

---

## Step 4: Handle duplicates

If you have the same skill in multiple locations, `collect` will warn you:

```
Warning: skill-b exists in source
  Source:  ~/.config/skillshare/skills/skill-b/
  Skipped: ~/.cursor/skills/skill-b/
```

**Options:**
- Keep the source version (default)
- Manually merge differences
- Use `--force` to overwrite

---

## Step 5: Sync to all targets

```bash
skillshare sync
```

Now all targets are symlinked to your single source.

---

## Step 6: Set up git (optional but recommended)

For cross-machine sync:

```bash
skillshare init --remote git@github.com:you/my-skills.git
skillshare push -m "Initial commit: migrated skills"
```

This initializes git, creates the initial commit, and adds the remote. Then `push` sends your skills to the remote.

---

## Verify Migration

```bash
# Check sync status
skillshare status

# List all skills
skillshare list

# Run diagnostics
skillshare doctor
```

---

## Rollback

If something goes wrong:

```bash
# Restore from backup
skillshare restore claude
skillshare restore cursor
```

---

## What's Next?

- [Daily Workflow](/docs/workflows/daily-workflow) — Edit, sync, push/pull
- [Cross-Machine Sync](/docs/guides/cross-machine-sync) — Sync across computers
- [Concepts](/docs/concepts) — Understand source, targets, and sync modes
