---
slug: managing-50-skills
title: How I Manage 50+ Skills Across 3 AI Tools
authors: [runkids]
tags: [tutorial, advanced]
---

When you use Claude Code, Cursor, and Codex daily, skill management gets chaotic fast. Here's how I keep 50+ skills organized across all three tools with a single workflow.

<!-- truncate -->

## The Setup

My daily toolkit:
- **Claude Code** — primary coding assistant
- **Cursor** — IDE-integrated AI
- **Codex** — quick prototyping and script generation

Each tool has its own skill directory, its own format quirks, and its own update cycle. Without skillshare, I was manually copying files between three locations every time I changed a skill.

## Directory Structure

After `skillshare init`, my source looks like this:

```
~/.config/skillshare/skills/
├── _anthropics__courses__prompt-eng/   # tracked repo
├── _team__frontend/                    # team shared
├── code-review/                        # my custom skill
├── commit-message/                     # my custom skill
├── debugging/                          # organized into folders
├── testing/
└── frontend/
    ├── react-patterns/
    ├── css-guidelines/
    └── accessibility/
```

Skills prefixed with `_` are tracked repositories. The rest are local skills organized into logical groups.

## Organizing with `--into`

The `--into` flag places installed skills into subdirectories:

```bash
skillshare install user/repo --into frontend
skillshare install another/repo --into backend
```

This creates a clean hierarchy without flat-file chaos.

## Multi-Target Sync

Global config targets use a map format:

```yaml
targets:
  claude:
    path: ~/.claude/skills
  cursor:
    path: ~/.cursor/skills
  codex:
    path: ~/.codex/skills
```

One `skillshare sync` pushes all 50+ skills to all three tools simultaneously. Each target gets per-skill symlinks, so tool-specific local skills remain untouched.

## Per-Target Mode and Filtering

You can set a different sync mode per target, and use `include`/`exclude` patterns to control which skills go where:

```yaml
targets:
  claude:
    path: ~/.claude/skills
    mode: merge
  cursor:
    path: ~/.cursor/skills
    mode: copy          # copy mode for better compatibility
    exclude:
      - "_experimental*"
  codex:
    path: ~/.codex/skills
    mode: merge
    include:
      - "coding-*"
```

## Daily Workflow

My morning routine takes about 30 seconds:

```bash
# Check for upstream updates
skillshare check

# If updates available
skillshare update --all

# Sync to all targets
skillshare sync
```

That's it. Three tools, 50+ skills, one command chain.

## Handling Conflicts

Occasionally, a tool creates a skill with the same name as one in my source. skillshare handles this gracefully:

- In **merge mode**, the symlink points to your source skill
- The target's original file is not deleted — it's simply overshadowed by the symlink
- Running `skillshare diff` shows exactly what differs between source and target

## Backup Strategy

Before major changes, I snapshot:

```bash
skillshare backup
```

Backups are stored in `~/.local/share/skillshare/backups/` with timestamps. To restore a specific target:

```bash
skillshare restore claude
```

## Key Takeaways

1. **One source, many targets** — the fundamental principle
2. **Use `--into` for organization** — avoid flat directory sprawl
3. **Check + update + sync** — the daily three-command chain
4. **Backup before experiments** — it takes 2 seconds and saves hours

## Resources

- [Multi-tool quickstart](/docs/learn/with-multiple-tools)
- [Organizing skills guide](/docs/how-to/daily-tasks/organizing-skills)
- [Backup & restore guide](/docs/how-to/daily-tasks/backup-restore)
