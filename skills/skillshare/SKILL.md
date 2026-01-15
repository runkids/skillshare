---
name: skillshare
description: Manage and sync skills across AI CLI tools. Use when asked to "sync my skills", "pull skills", "show skillshare status", "list my skills", "install a skill", "create a new skill", "backup skills", or manage skill targets.
argument-hint: [command] [target] [--dry-run]
---

# Skillshare CLI

Manage and sync skills across multiple AI CLI tools from a single source of truth.

## When to Use This Skill

Activate when the user:
- Asks to sync, pull, or manage skills across CLI tools
- Wants to see skillshare status or differences
- Needs to install, uninstall, or list skills
- Asks about backup, restore, or target management
- Creates a new skill and wants to distribute it
- Mentions "skillshare" directly

**Natural language triggers:**
- "sync my skills to all tools"
- "pull my new skill from Claude"
- "what skills do I have?"
- "show skillshare status"
- "add cursor as a target"
- "backup my skills"
- "install that skill from GitHub"

## How It Works

1. **Check current state** with `skillshare status`
2. **Preview changes** with `--dry-run` before any destructive operation
3. **Execute the command** if user approves or didn't request preview
4. **Report results** with relevant output

## AI Behavior Guide

Map user intents to command sequences:

| User Says | Commands to Run |
|-----------|-----------------|
| "sync my skills" | `skillshare sync` |
| "sync but show me first" | `skillshare sync --dry-run`, then `skillshare sync` if approved |
| "pull from Claude and sync" | `skillshare pull claude` then `skillshare sync` |
| "pull all and sync everywhere" | `skillshare pull --all` then `skillshare sync` |
| "show status" / "what's the state" | `skillshare status` |
| "show differences" | `skillshare diff` |
| "what skills do I have" | `skillshare list` (or `skillshare list --verbose`) |
| "install X skill" | `skillshare install <source>` then `skillshare sync` |
| "remove/uninstall X skill" | `skillshare uninstall <name>` then `skillshare sync` |
| "add cursor as target" | `skillshare target add cursor ~/.cursor/skills` |
| "remove target X" | `skillshare target remove <name>` |
| "backup my skills" | `skillshare backup` |
| "restore from backup" | `skillshare restore <target>` |
| "something's broken" | `skillshare doctor` |
| "create a new skill" | Guide user to create in source: `~/.config/skillshare/skills/` |

## Quick Reference

| Priority | Command | Use Case |
|----------|---------|----------|
| 1 | `status` | First command - see current state |
| 2 | `sync` | Push skills to all targets |
| 3 | `sync --dry-run` | Preview sync changes |
| 4 | `pull <target>` | Bring target's new skills to source |
| 5 | `pull --all` | Pull from all targets |
| 6 | `diff` | See what differs between source and targets |
| 7 | `list` | Show installed skills |
| 8 | `install <source>` | Add skill from path or git repo |
| 9 | `uninstall <name>` | Remove a skill |
| 10 | `doctor` | Diagnose configuration issues |

## Commands Reference

### Status & Inspection

```bash
skillshare status              # Source, targets, sync state
skillshare diff                # All targets
skillshare diff claude         # Specific target
skillshare list                # List skills
skillshare list --verbose      # With source and install info
```

**Expected output (status):**
```
Source: ~/.config/skillshare/skills (4 skills)
Targets:
  claude   ✓ synced   ~/.claude/skills
  codex    ✓ synced   ~/.codex/skills
  cursor   ⚠ 1 diff   ~/.cursor/skills
```

### Sync & Pull

```bash
skillshare sync                # Sync to all targets
skillshare sync --dry-run      # Preview only

skillshare pull claude         # Pull from specific target
skillshare pull --all          # Pull from all targets
skillshare pull --all -n       # Preview pull
```

**Workflow:**
1. Create skill in any target (e.g., `~/.claude/skills/my-skill/`)
2. `skillshare pull claude` - bring to source
3. `skillshare sync` - distribute to all targets
4. Commit: `cd ~/.config/skillshare/skills && git add . && git commit`

### Install & Uninstall

```bash
# Install from various sources
skillshare install github.com/user/repo              # Discovery mode
skillshare install github.com/user/repo/path/skill   # Direct path
skillshare install ~/Downloads/my-skill              # Local path
skillshare install <source> --name custom-name       # Custom name
skillshare install <source> --force                  # Overwrite existing
skillshare install <source> --update                 # Update git-based skill

# Uninstall
skillshare uninstall my-skill                        # With confirmation
skillshare uninstall my-skill --force                # Skip confirmation
```

After install/uninstall, run `skillshare sync` to update targets.

### Target Management

```bash
skillshare target list                        # List all targets
skillshare target claude                      # Show target info
skillshare target add myapp ~/.myapp/skills   # Add custom target
skillshare target remove myapp                # Remove target
skillshare target claude --mode merge         # Change sync mode
skillshare target claude --mode symlink       # Change to symlink mode
```

**Sync modes:**
- `merge`: Individual skill symlinks, local skills preserved (recommended)
- `symlink`: Entire directory symlinked, all targets identical

### Backup & Restore

```bash
skillshare backup                         # Backup all targets
skillshare backup claude                  # Backup specific target
skillshare backup --list                  # List available backups
skillshare backup --cleanup               # Remove old backups

skillshare restore claude                 # Restore from latest
skillshare restore claude --from 2026-01-14_21-22-18  # Specific backup
```

Backups location: `~/.config/skillshare/backups/<timestamp>/`

### Diagnostics

```bash
skillshare doctor                         # Check configuration health
skillshare init --dry-run                 # Preview what init would do
```

## Common Issues & Solutions

| Problem | Diagnosis | Solution |
|---------|-----------|----------|
| "config not found" | Config missing | Run `skillshare init` |
| Target shows differences | Files out of sync | Run `skillshare sync` |
| Lost source files | Deleted via symlink | `cd ~/.config/skillshare/skills && git checkout -- <skill>/` |
| Target has local skills | Need to preserve | Ensure `merge` mode, then `skillshare pull` before sync |
| Skill not appearing | Not synced yet | Run `skillshare sync` after install |
| Can't find installed skills | Wrong directory | Check `skillshare status` for source path |

**Recovery workflow:**
```bash
skillshare doctor          # Diagnose issues
skillshare backup          # Create safety backup
skillshare sync --dry-run  # Preview fix
skillshare sync            # Apply fix
```

## Tips for AI

### Symlink Behavior
- In `merge` mode: editing a skill in ANY target edits the source (changes are immediate)
- In `symlink` mode: entire target directory is a symlink to source
- **NEVER** use `rm -rf` on symlinked skills - this deletes the source

### When to Use --dry-run
- Before `sync` if user is cautious or first-time
- Before `pull --all` to see what will be imported
- Before `install` from unknown sources
- Before `restore` to preview what will change
- Before `target remove` to understand impact

### Git Workflow Recommendations
After any skill changes, remind user:
```bash
cd ~/.config/skillshare/skills
git add .
git commit -m "Add/update skills"
git push  # If using remote
```

### Creating New Skills
Guide users to create skills in source directory:
- Path: `~/.config/skillshare/skills/<skill-name>/SKILL.md`
- Must have frontmatter with `name` and `description`
- After creation: `skillshare sync` to distribute

### Safe Target Removal
To unlink a target without losing source:
```bash
skillshare target remove <name>      # Safe: only removes link
# NOT: rm -rf ~/.target/skills       # Dangerous: may delete source
```

## File Locations

| Item | Path |
|------|------|
| Source directory | Defined in config (default: `~/.config/skillshare/skills/`) |
| Config file | `~/.config/skillshare/config.yaml` |
| Backups | `~/.config/skillshare/backups/` |

Run `skillshare status` to see actual paths.
