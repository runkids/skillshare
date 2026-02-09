---
name: skillshare
version: 0.9.0
description: |
  Syncs skills across AI CLI tools (Claude, Cursor, Windsurf, etc.) from a single source of truth.
  Supports both global mode (~/.config/skillshare/) and project mode (.skillshare/ per-repo).
  Use when: "sync skills", "install skill", "search skills", "list skills", "show skill status",
  "backup skills", "restore skills", "update skills", "new skill", "collect skills",
  "push/pull skills", "add/remove target", "find a skill for X", "is there a skill that can...",
  "how do I do X with skills", "skillshare init", "skillshare upgrade", "skill not syncing",
  "diagnose skillshare", "doctor", "project skills", "init project", "project setup",
  "scope skills to repo", "share skills via git", "web dashboard", "skillshare ui",
  "launch dashboard", "open ui", "visual skill management",
  or any skill/target management across AI tools.
argument-hint: "[command] [target] [--dry-run] [-p|-g]"
---

# Skillshare CLI

## Two Modes

```
Global: ~/.config/skillshare/skills → ~/.claude/skills, ~/.cursor/skills, ...
Project: .skillshare/skills/        → .claude/skills, .cursor/skills (per-repo)
```

Auto-detection: commands run in **project mode** when `.skillshare/config.yaml` exists in cwd.
Force with `-p` (project) or `-g` (global).

## Quick Start

```bash
# Global
skillshare status && skillshare sync

# Project — initialize in a repo
skillshare init -p
skillshare sync
```

## Commands

| Category | Commands | Project? |
|----------|----------|:--------:|
| **Inspect** | `status`, `diff`, `list`, `doctor` | ✓ (auto) |
| **Sync** | `sync`, `collect` | ✓ (auto) |
| **Remote** | `push`, `pull` | ✗ (use git) |
| **Skills** | `new`, `install`, `uninstall`, `update`, `check`, `search` | ✓ (`-p`) |
| **Targets** | `target add/remove/list` | ✓ (`-p`) |
| **Backup** | `backup`, `restore` | ✗ |
| **Web UI** | `ui` | ✓ (`-p`) |
| **Upgrade** | `upgrade [--cli\|--skill]` | — |

**Workflow:** Most commands require `sync` afterward to distribute changes.

## AI Usage Notes

### Non-Interactive Mode

AI cannot respond to CLI prompts. Always use flags:

```bash
# Global init
skillshare init --copy-from claude --all-targets --git  # If skills exist
skillshare init --no-copy --all-targets --git           # Fresh start

# Project init (in repo root)
skillshare init -p --targets "claude-code,cursor"       # Specific targets
skillshare init -p                                      # Interactive (user only)

# Install (multi-skill repo non-interactive)
skillshare install user/repo --all                # Install all
skillshare install user/repo -s pdf,commit        # Select specific
skillshare install user/repo -y                   # Auto-accept

# Add new agents later
skillshare init --discover --select "windsurf,kilocode"
skillshare init -p --discover --select "windsurf"
```

### Safety

**NEVER** `rm -rf` symlinked skills — deletes source. Always use:
- `skillshare uninstall <name>` to remove skills
- `skillshare target remove <name>` to unlink targets

### Finding Skills

```bash
skillshare search                   # Browse popular skills
skillshare search <query>           # Interactive install
skillshare search <query> --list    # List only
skillshare search <query> --json    # JSON output
```

**Query examples:** `react performance`, `pr review`, `commit`, `changelog`

## References

| Topic | File |
|-------|------|
| Init flags (global + project) | [init.md](references/init.md) |
| Sync/collect/push/pull | [sync.md](references/sync.md) |
| Install/update/new | [install.md](references/install.md) |
| Status/diff/list/search | [status.md](references/status.md) |
| Target management | [targets.md](references/targets.md) |
| Backup/restore | [backup.md](references/backup.md) |
| Web dashboard (UI) | [ui.md](references/ui.md) |
| Troubleshooting | [TROUBLESHOOTING.md](references/TROUBLESHOOTING.md) |
