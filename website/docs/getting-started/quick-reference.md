---
sidebar_position: 4
---

# Quick Reference

Command cheat sheet for skillshare.

## Core Commands

| Command | Description |
|---------|-------------|
| `init` | First-time setup |
| `install <source>` | Add a skill |
| `uninstall <name>...` | Remove one or more skills |
| `list` | List all skills |
| `search <query>` | Search for skills |
| `sync` | Push to all targets |
| `status` | Show sync state |

## Skill Management

| Command | Description |
|---------|-------------|
| `new <name>` | Create a new skill |
| `update <name>` | Update a skill (git pull) |
| `update --all` | Update all tracked repos |
| `check` | Check for skill updates |
| `check --json` | Check for updates (JSON output) |
| `upgrade` | Upgrade CLI and built-in skill |
| `hub list` | List configured skill hubs |
| `hub add <url>` | Add a skill hub |

## Target Management

| Command | Description |
|---------|-------------|
| `target list` | List all targets |
| `target <name>` | Show target details |
| `target <name> --mode <mode>` | Change sync mode |
| `target add <name> <path>` | Add custom target |
| `target remove <name>` | Remove target safely |
| `diff [target]` | Show differences |

## Sync Operations

| Command | Description |
|---------|-------------|
| `collect <target>` | Collect skills from target to source |
| `collect --all` | Collect from all targets |
| `backup [target]` | Create backup |
| `backup --list` | List backups |
| `restore <target>` | Restore from backup |
| `push [-m "msg"]` | Push to git remote |
| `pull` | Pull from git and sync |
| `trash list` | List soft-deleted skills |
| `trash restore <name>` | Restore a soft-deleted skill |

## Utilities

| Command | Description |
|---------|-------------|
| `doctor` | Diagnose issues |
| `log` | View operations and audit logs |
| `ui` | Launch web dashboard on `localhost:19420` |
| `ui -p` | Launch web dashboard in project mode |
| `version` | Show CLI version |
| `make test-docker` | Run offline Docker sandbox tests |
| `make playground` | Start playground + enter shell (one step) |
| `make playground-down` | Stop and remove playground |
| `./scripts/sandbox.sh <cmd>` | Advanced sandbox management (up/down/shell/reset/status/logs/bare) |
| `make ui-build` | Build frontend |
| `make build-all` | Full binary with frontend |

---

## Common Workflows

### Install and sync a skill
```bash
skillshare install anthropics/skills/skills/pdf
skillshare sync
```

### Create and deploy a skill
```bash
skillshare new my-skill
# Edit ~/.config/skillshare/skills/my-skill/SKILL.md
skillshare sync
```

### Cross-machine sync
```bash
# Setup (pick one)
# Interactive (guided prompts)
skillshare init --remote git@github.com:you/my-skills.git

# Non-interactive (no prompts, auto-detect installed targets)
skillshare init --remote git@github.com:you/my-skills.git --no-copy --all-targets --no-skill

# Machine A: push changes
skillshare push -m "Add new skill"

# Machine B: pull and sync
skillshare pull
```

Optional later (only if you install additional AI CLIs after setup):

```bash
skillshare init --discover
```

With mode override during discover, only newly added targets are affected:

```bash
skillshare init --discover --select cursor --mode copy
```

### Team skill sharing
```bash
# Install team repo
skillshare install github.com/team/skills --track

# Update from team
skillshare update --all
skillshare sync
```

### Sandbox playground session
```bash
make playground          # start + enter shell
skillshare --help
ss status
exit                     # leave shell
make playground-down     # stop container
```

---

## Key Paths

| Path | Description |
|------|-------------|
| `~/.config/skillshare/config.yaml` | Configuration file |
| `~/.config/skillshare/registry.yaml` | Installed skill registry (auto-managed) |
| `~/.config/skillshare/skills/` | Source directory |
| `~/.local/state/skillshare/logs/` | Operation and audit logs |
| `~/.local/share/skillshare/backups/` | Backup directory |

---

## Flags Available on Most Commands

| Flag | Description |
|------|-------------|
| `--dry-run`, `-n` | Preview without making changes |
| `--help`, `-h` | Show help |

---

## See Also

- [Commands Reference](/docs/reference/commands) — Full command documentation
- [Concepts](/docs/understand) — Core concepts explained
