---
name: skillshare
version: 0.6.3
description: Syncs skills across AI CLI tools from a single source of truth. Use when asked to "sync skills", "pull skills", "show status", "list skills", "install skill", "initialize skillshare", or manage skill targets.
argument-hint: "[command] [target] [--dry-run]"
---

# Skillshare CLI

> **Shorthand:** `ss` = `skillshare` (e.g., `ss status`, `ss sync`)

```
Source: ~/.config/skillshare/skills  ← Edit here (single source of truth)
         ↓ sync
Targets: ~/.claude/skills, ~/.cursor/skills, ...  ← Symlinked from source
```

## Quick Reference

```bash
skillshare status              # Always run first
skillshare sync                # Push to all targets
skillshare sync --dry-run      # Preview changes
skillshare pull claude         # Import from target → source
skillshare list                # Show skills and tracked repos
```

## Command Patterns

| Intent | Command |
|--------|---------|
| Sync skills | `skillshare sync` |
| Preview first | `skillshare sync --dry-run` then `sync` |
| Pull from target | `skillshare pull <name>` then `sync` |
| Install skill | `skillshare install <source>` then `sync` |
| Install from repo (browse) | `skillshare install owner/repo` (discovery mode) |
| Install team repo | `skillshare install <git-url> --track` then `sync` |
| Update skill/repo | `skillshare update <name>` then `sync` |
| Update all tracked | `skillshare update --all` then `sync` |
| Remove skill | `skillshare uninstall <name>` then `sync` |
| List skills | `skillshare list` or `list --verbose` |
| Cross-machine push | `skillshare push -m "message"` |
| Cross-machine pull | `skillshare pull --remote` |
| Backup/restore | `skillshare backup --list`, `restore <target>` |
| Add custom target | `skillshare target add <name> <path>` |
| Change sync mode | `skillshare target <name> --mode merge\|symlink` |
| Upgrade CLI/skill | `skillshare upgrade` |
| Diagnose issues | `skillshare doctor` |

## Init (Non-Interactive)

**CRITICAL:** Use flags — AI cannot respond to CLI prompts.

**Source path:** Always use default `~/.config/skillshare/skills`. Only use `--source` if user explicitly requests a different location.

**Step 1:** Check existing skills
```bash
ls ~/.claude/skills ~/.cursor/skills 2>/dev/null | head -10
```

**Step 2:** Run init based on findings

| Found | Command |
|-------|---------|
| Skills in one target | `skillshare init --copy-from <name> --all-targets --git` |
| Skills in multiple | Ask user which to import |
| No existing skills | `skillshare init --no-copy --all-targets --git` |

**Step 3:** `skillshare status`

**Adding new agents later (AI must use --select):**
```bash
skillshare init --discover --select "windsurf,kilocode"   # Non-interactive (AI use this)
# skillshare init --discover                              # Interactive only (NOT for AI)
```

See [init.md](references/init.md) for all flags.

## Team Edition

```bash
skillshare install github.com/team/skills --track   # Install as tracked repo
skillshare update _team-skills                       # Update later
```

Tracked repos: `_` prefix, nested paths use `__` (e.g., `_team__frontend__ui`).

**Naming convention:** Use `{team}:{name}` in SKILL.md to avoid collisions.

## Safety

- **NEVER** `rm -rf` on symlinked skills — deletes source
- Use `skillshare uninstall <name>` to safely remove

## Zero-Install

```bash
curl -fsSL https://raw.githubusercontent.com/runkids/skillshare/main/skills/skillshare/scripts/run.sh | sh -s -- status
```

## References

- [init.md](references/init.md) - Init flags
- [sync.md](references/sync.md) - Sync, pull, push
- [install.md](references/install.md) - Install, update, uninstall
- [status.md](references/status.md) - Status, diff, list, doctor
- [targets.md](references/targets.md) - Target management
- [backup.md](references/backup.md) - Backup, restore
- [TROUBLESHOOTING.md](references/TROUBLESHOOTING.md) - Recovery
