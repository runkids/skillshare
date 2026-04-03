---
name: skillshare
description: |
  Manages and syncs AI CLI skills across 50+ tools from a single source.
  Use this skill whenever the user mentions "skillshare", runs skillshare commands,
  manages skills (install, update, uninstall, sync, audit, analyze, check, diff, search),
  or troubleshoots skill configuration (orphaned symlinks, broken targets, sync
  issues). Covers both global (~/.config/skillshare/) and project (.skillshare/)
  modes. Also use when: adding new AI tool targets (Claude, Cursor, Windsurf, etc.),
  setting target include/exclude filters or copy vs symlink mode, using backup/restore
  or trash recovery, piping skillshare output to scripts (--json), setting up CI/CD
  audit pipelines, or building/sharing skill hubs (hub index, hub add).
argument-hint: "[command] [target] [--json] [--dry-run] [-p|-g]"
metadata:
  version: v0.18.6
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
skillshare install user/repo -b develop --all    # Install from branch
skillshare install user/repo --track -b develop  # Track specific branch
skillshare install user/repo -s pdf -p           # Install to project
skillshare install                               # Reinstall all tracked remotes from config
skillshare sync                                  # Always sync after install
```
### Extras (Rules, Commands, Prompts)
```bash
skillshare extras init rules --target ~/.claude/rules --target ~/.cursor/rules
skillshare extras init commands --target ~/.claude/commands --mode copy
skillshare extras init rules --target ~/.claude/rules --source ~/shared/rules  # custom source (global only)
skillshare extras init rules --target ~/.cursor/rules --force                  # overwrite existing
skillshare extras init                               # Interactive TUI wizard (incl. source step)
skillshare extras source                             # Show current extras_source
skillshare extras source ~/shared/extras             # Set global extras_source
skillshare extras list                               # Show status per target
skillshare extras list --json                        # JSON with source_type field
skillshare extras collect rules                      # Pull local files into source
skillshare extras remove rules                       # Remove from config (source preserved)
skillshare extras init agents --target ~/.claude/agents --flatten  # Flatten subdirs into root
skillshare extras rules --mode copy                  # Change sync mode of a target
skillshare extras agents --flatten                   # Enable flatten on existing target
skillshare sync extras                               # Sync all extras to targets
skillshare sync extras --dry-run --force             # Preview / overwrite conflicts
skillshare sync --all                                # Sync skills + extras together
```
See [extras.md](references/extras.md) for details.
### Creating & Discovering Skills
```bash
skillshare new my-skill                          # Create with interactive pattern selection
skillshare new my-skill -P reviewer              # Use reviewer pattern directly
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
### Enable / Disable Skills
```bash
skillshare disable draft-*                       # Hide from sync (adds to .skillignore)
skillshare enable draft-*                        # Restore (removes from .skillignore)
skillshare disable my-skill -p                   # Project mode
skillshare disable my-skill --dry-run            # Preview
# TUI: press E in `skillshare list` to toggle
skillshare sync                                  # Always sync after toggle
```
### Team / Organization
```bash
# Creator: init project (see Getting Started) → add skills → commit .skillshare/
skillshare install -p && skillshare sync                  # Member: clone → install → sync
skillshare install github.com/team/repo --track -p        # Track shared repo
skillshare push                                           # Cross-machine: push on A
skillshare pull                                           # Cross-machine: pull on B
```
### Skill Hubs
```bash
skillshare hub add https://example.com/hub.json          # Save a hub source
skillshare hub add https://example.com/hub.json --label my-hub  # With custom label
skillshare hub list                                      # List saved hubs
skillshare hub default my-hub                            # Set default hub
skillshare hub remove my-hub                             # Remove a hub
skillshare hub index --source ~/.config/skillshare/skills/ --full --audit  # Build hub index
```
### Controlling Where Skills Go
```bash
# SKILL.md frontmatter: targets: [claude]        → only syncs to Claude
skillshare target claude --add-include "team-*"   # glob filter
skillshare target claude --add-exclude "_legacy*"  # exclude pattern
skillshare target codex --mode copy && skillshare sync --force  # copy mode
# .skillignore — hide skills/dirs from discovery (gitignore syntax)
#   Root-level: <source>/.skillignore (affects all commands)
#   Repo-level: <source>/_repo/.skillignore (scoped to that repo)
#   .skillignore.local — local override (not committed), negation overrides base
```
See [targets.md](references/targets.md) for details.
### Updates & Maintenance
```bash
skillshare check                              # See what has updates
skillshare update my-skill && skillshare sync  # Update one
skillshare update --all && skillshare sync     # Update all
skillshare update --all --diff                 # Show what changed
```
### Scripting & CI/CD
```bash
skillshare status --json                       # Full status as JSON
skillshare check --json                        # Update status as JSON
skillshare sync --json                         # Sync results as JSON
skillshare diff --json                         # Diff results as JSON
skillshare install user/repo --json            # Install result as JSON (implies --force --all)
skillshare update --all --json                 # Update results as JSON
skillshare uninstall my-skill --json           # Uninstall result as JSON (implies --force)
skillshare collect claude --json               # Collect result as JSON (implies --force)
skillshare target list --json                  # Target list as JSON
skillshare list --json                         # Skill list as JSON
skillshare search react --json                 # Search results as JSON
skillshare audit --format json                 # Audit results as JSON
skillshare doctor --json                       # Health check as JSON (exit 1 on errors)
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
| Commands | Project? | `--json`? |
|----------|:--------:|:---------:|
| `status`, `diff`, `list`, `doctor` | ✓ (auto) | ✓ |
| `sync`, `collect` | ✓ (auto) | ✓ |
| `install`, `uninstall`, `update`, `check`, `search`, `new` | ✓ (`-p`) | ✓ (except new) |
| `target`, `audit`, `analyze`, `trash`, `log`, `hub` | ✓ (`-p`) | ✓ (target list, audit, analyze, log) |
| `extras init/list/remove/collect/source/mode` | ✓ (`-p`, except source) | ✓ (list, mode) |
| `enable`, `disable` | ✓ (auto) | ✗ |
| `push`, `pull`, `backup`, `restore` | ✗ | ✗ |
| `tui`, `upgrade` | ✗ | ✗ |
| `ui` | ✓ (`-p`) | ✗ |

## AI Caller Rules
1. **Non-interactive** — AI cannot answer prompts. Use `--force`, `--all`, `-s`, `--targets`, `--no-copy`, `--all-targets`, `--yes`.
2. **Sync after mutations** — `install`, `uninstall`, `update`, `collect`, `target` all need `sync`.
3. **Audit** — `install` auto-scans; CRITICAL blocks. `--force` to override, `--skip-audit` to bypass. Detects hardcoded secrets (API keys, tokens, private keys).
4. **Uninstall safely** — moves to trash (7 days). `trash restore <name>` to undo. **NEVER** `rm -rf` symlinks.
5. **Output** — `--json` for structured data (12 commands support it, see Quick Lookup). `--no-tui` for plain text on TUI commands (`list`, `log`, `audit`, `analyze`, `diff`, `trash list`, `backup list`, `target list`). `tui off` disables TUI globally. `--dry-run` to preview.

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
| Extras (rules/commands/prompts) | [extras.md](references/extras.md) |
| Backup/restore | [backup.md](references/backup.md) |
| Troubleshooting | [TROUBLESHOOTING.md](references/TROUBLESHOOTING.md) |
