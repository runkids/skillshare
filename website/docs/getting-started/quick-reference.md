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
| `uninstall <name>` | Remove a skill |
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
| `upgrade` | Upgrade CLI and built-in skill |

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

## Utilities

| Command | Description |
|---------|-------------|
| `doctor` | Diagnose issues |
| `log` | View operations and audit logs |
| `ui` | Launch web dashboard on `localhost:19420` |
| `mise run test:docker` | Run offline Docker sandbox tests |
| `mise run test:docker:online` | Run optional online Docker tests |
| `mise run sandbox:up` | Start persistent playground container |
| `mise run sandbox:shell` | Enter playground shell |
| `mise run sandbox:down` | Stop playground container |
| `make test-docker` | Run offline Docker sandbox tests |
| `make test-docker-online` | Run optional online Docker tests |
| `make sandbox-up` | Start persistent playground container |
| `make sandbox-shell` | Enter playground shell |
| `make sandbox-down` | Stop playground container |
| `mise run ui:build` | Build frontend + copy to embed |
| `mise run build:all` | Full binary with embedded frontend |
| `make ui-build` | Build frontend + copy to embed |
| `make build-all` | Full binary with embedded frontend |

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
# Machine A: push changes
skillshare push -m "Add new skill"

# Machine B: pull and sync
skillshare pull
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
mise run sandbox:up
mise run sandbox:shell
skillshare --help
ss status
mise run sandbox:down
```

```bash
make sandbox-up
make sandbox-shell
skillshare --help
ss status
make sandbox-down
```

---

## Key Paths

| Path | Description |
|------|-------------|
| `~/.config/skillshare/config.yaml` | Configuration file |
| `~/.config/skillshare/skills/` | Source directory |
| `~/.config/skillshare/backups/` | Backup directory |

---

## Flags Available on Most Commands

| Flag | Description |
|------|-------------|
| `--dry-run`, `-n` | Preview without making changes |
| `--help`, `-h` | Show help |

---

## See Also

- [Commands Reference](/docs/commands) — Full command documentation
- [Concepts](/docs/concepts) — Core concepts explained
