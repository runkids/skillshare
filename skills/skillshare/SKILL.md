---
name: skillshare
version: v0.16.6
description: |
  Syncs skills across AI CLI tools from a single source of truth.
  Global (~/.config/skillshare/) or project (.skillshare/) mode.
  Install from any Git host. Use when: "skillshare" CLI, skill management, or troubleshooting.
argument-hint: "[command] [target] [--dry-run] [-p|-g]"
---

# Skillshare CLI

Global: `~/.config/skillshare/skills/` → all AI CLIs. Project: `.skillshare/skills/` → repo-local.
Auto-detects project mode when `.skillshare/config.yaml` exists. Force with `-p` or `-g`.

## Recipes

### Getting Started
```bash
skillshare init --no-copy --all-targets --git --skill  # Fresh global setup
skillshare init -p --targets "claude,cursor"            # Fresh project setup
skillshare init --copy-from claude --all-targets --git  # Import from existing CLI
skillshare init --discover --select "windsurf"          # Add new AI tool later
```
### Installing Skills
```bash
skillshare install user/repo -s pdf,commit       # Select specific skills
skillshare install user/repo --all               # Install everything
skillshare install user/repo --into frontend     # Place in subdirectory
skillshare install gitlab.com/team/repo          # Any Git host
skillshare install user/repo --track             # Enable `update` later
skillshare install user/repo -s pdf -p           # Install to project
skillshare install                               # Reinstall all tracked remotes from config
skillshare sync                                  # Always sync after install
```
### Creating & Discovering Skills
```bash
skillshare new my-skill                          # Create a new skill from template
skillshare search "react testing"                # Search GitHub for skills
skillshare collect                               # Pull target-local changes back to source
```
### Removing Skills
```bash
skillshare uninstall my-skill                    # Remove one (moves to trash)
skillshare uninstall skill-a skill-b             # Remove multiple
skillshare uninstall -G frontend                 # Remove entire group
skillshare sync                                  # Always sync after uninstall
```
### Team / Organization
```bash
# Creator: init project (see Getting Started) → add skills → commit .skillshare/
skillshare install -p && skillshare sync                  # Member: clone → install → sync
skillshare install github.com/team/repo --track -p        # Track shared repo
skillshare push                                           # Cross-machine: push on A
skillshare pull                                           # Cross-machine: pull on B
```
### Controlling Where Skills Go
```bash
# SKILL.md frontmatter: targets: [claude]        → only syncs to Claude
skillshare target claude --add-include "team-*"   # glob filter
skillshare target claude --add-exclude "_legacy*"  # exclude pattern
skillshare target codex --mode copy && skillshare sync --force  # copy mode
```
See [targets.md](references/targets.md) for details.
### Updates & Maintenance
```bash
skillshare check                              # See what has updates
skillshare update my-skill && skillshare sync  # Update one
skillshare update --all && skillshare sync     # Update all
skillshare update --all --diff                 # Show what changed
```
### Recovery & Troubleshooting
```bash
skillshare trash restore <name> && skillshare sync  # Undo delete
skillshare sync                                     # Skill missing? Re-sync
skillshare doctor && skillshare status              # Diagnose issues
skillshare install user/repo --force                 # Override audit block
skillshare install user/repo --skip-audit            # Bypass scan entirely
```
See [TROUBLESHOOTING.md](references/TROUBLESHOOTING.md) for more.

## Quick Lookup
| Commands | Project? |
|----------|:--------:|
| `status`, `diff`, `list`, `doctor` | ✓ (auto) |
| `sync`, `collect` | ✓ (auto) |
| `install`, `uninstall`, `update`, `check`, `search`, `new` | ✓ (`-p`) |
| `target`, `audit`, `trash`, `log` | ✓ (`-p`) |
| `push`, `pull`, `backup`, `restore` | ✗ |
| `ui`, `upgrade` | ✓ |

## AI Caller Rules
1. **Non-interactive** — AI cannot answer prompts. Use `--force`, `--all`, `-s`, `--targets`, `--no-copy`, `--all-targets`, `--yes`.
2. **Sync after mutations** — `install`, `uninstall`, `update`, `collect`, `target` all need `sync`.
3. **Audit** — `install` auto-scans; CRITICAL blocks. `--force` to override, `--skip-audit` to bypass.
4. **Uninstall safely** — moves to trash (7 days). `trash restore <name>` to undo. **NEVER** `rm -rf` symlinks.
5. **Output** — `--json` for structured data, `--no-tui` for plain text, `--dry-run` to preview.

## References
| Topic | File |
|-------|------|
| Init flags | [init.md](references/init.md) |
| Sync/collect/push/pull | [sync.md](references/sync.md) |
| Install/update/uninstall/new | [install.md](references/install.md) |
| Status/diff/list/search/check | [status.md](references/status.md) |
| Security audit | [audit.md](references/audit.md) |
| Trash | [trash.md](references/trash.md) |
| Operation log | [log.md](references/log.md) |
| Targets | [targets.md](references/targets.md) |
| Backup/restore | [backup.md](references/backup.md) |
| Troubleshooting | [TROUBLESHOOTING.md](references/TROUBLESHOOTING.md) |
