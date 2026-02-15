---
sidebar_position: 1
---

# ui

Launch the web dashboard for visual skill management.

```bash
skillshare ui
```

Opens `http://127.0.0.1:19420` in your default browser.

## Flags

| Flag | Default | Description |
|------|---------|-------------|
| `-p`, `--project` | | Run in project mode (uses `.skillshare/`) |
| `-g`, `--global` | | Run in global mode (uses `~/.config/skillshare/`) |
| `--port <port>` | `19420` | HTTP server port |
| `--host <host>` | `127.0.0.1` | Bind address (use `0.0.0.0` for Docker) |
| `--no-open` | `false` | Don't open browser automatically |

:::tip Auto-Detection
If `.skillshare/config.yaml` exists in the current directory, the dashboard automatically starts in project mode. Use `-g` to force global mode.
:::

## Examples

```bash
# Default: opens browser on localhost:19420
skillshare ui

# Project mode (manage .skillshare/ skills)
skillshare ui -p

# Custom port
skillshare ui --port 8080

# Docker / remote access
skillshare ui --host 0.0.0.0 --no-open

# Background mode
skillshare ui --no-open &
```

## Dashboard Pages

| Page | Description |
|------|-------------|
| **Dashboard** | Overview cards — skill count, target count, sync mode, version |
| **Skills** | Searchable skill grid with metadata. Toggle between **Grid** and **Grouped** (by directory) views. Click to view SKILL.md content |
| **Install** | Install from local path, git URL, or GitHub shorthand |
| **Targets** | Target list with status badges. Add/remove targets |
| **Sync** | Sync controls with dry-run toggle. Diff preview |
| **Collect** | Scan targets and collect selected skills back to source |
| **Backup** | View backup list, restore snapshots, and clean up entries |
| **Git Sync** | Push/pull source repo with dirty-state checks and force pull |
| **Search** | GitHub skill search with one-click install |
| **Audit** | Security scan all skills, view findings by severity |
| **Audit Rules** | Create and edit custom `audit-rules.yaml` with YAML editor |
| **Trash** | View soft-deleted skills, restore or permanently delete |
| **Log** | Operations and audit logs with command/status/time filters |
| **Config** | YAML config editor with validation |

### Project Mode Differences

When running in project mode (`-p`), the dashboard adapts:

- **"Project" badge** in the sidebar indicates project mode
- **Git Sync page** is hidden (project skills use the project's own git)
- **Backup & Restore page** is hidden (use version control instead)
- **Tracked Repos section** is hidden from Dashboard (not applicable)
- **Config page** shows `.skillshare/config.yaml` instead of the global config
- **Available targets** lists project-level targets (e.g., `.claude/skills/` relative to project root)
- **Install** automatically reconciles `skills:` entries in the project config

## UI Preview

<div style={{display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(320px, 1fr))', gap: '1rem'}}>
  <img src="/img/web-install-demo.png" alt="Install flow" />
  <img src="/img/web-dashboard-demo.png" alt="Dashboard overview" />
  <img src="/img/web-skills-demo.png" alt="Skills browser" />
  <img src="/img/web-skill-detail-demo.png" alt="Skill detail view" />
  <img src="/img/web-sync-demo.png" alt="Sync controls" />
  <img src="/img/web-search-skills-demo.png" alt="GitHub search view" />
</div>

## REST API

The web dashboard exposes a REST API at `/api/`. All endpoints return JSON.

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/overview` | Skill/target counts, mode, version |
| GET | `/api/skills` | List all skills with metadata |
| GET | `/api/skills/{name}` | Skill detail + SKILL.md content |
| DELETE | `/api/skills/{name}` | Uninstall a skill |
| GET | `/api/targets` | List targets with status, include/exclude filters, and per-target expected counts |
| POST | `/api/targets` | Add a target |
| DELETE | `/api/targets/{name}` | Remove a target |
| POST | `/api/sync` | Run sync (supports `dryRun`, `force`) |
| GET | `/api/diff` | Diff between source and targets |
| GET | `/api/search?q=` | Search GitHub for skills |
| POST | `/api/install` | Install a skill from source |
| GET | `/api/audit` | Scan all skills for security threats |
| GET | `/api/audit/rules` | Get custom audit rules YAML |
| PUT | `/api/audit/rules` | Save custom audit rules (validates regex) |
| POST | `/api/audit/rules` | Create starter audit-rules.yaml |
| GET | `/api/log` | List log entries with optional filters |
| GET | `/api/config` | Get config as YAML |
| PUT | `/api/config` | Update config YAML |

## Docker Usage

The playground container includes pre-built frontend assets. To use the web UI inside Docker:

```bash
make sandbox-up
make sandbox-shell

# Inside container:
skillshare ui --host 0.0.0.0 --no-open
```

Then open `http://localhost:19420` on your host machine (port 19420 is mapped automatically).

## Project Mode

The web dashboard fully supports project-level skills:

```bash
cd my-project
skillshare ui -p
```

Or simply `skillshare ui` if `.skillshare/config.yaml` exists (auto-detected).

The dashboard reads and writes `.skillshare/config.yaml`, syncs to project-local targets, and reconciles remote skill entries after install — just like the CLI.

## Homebrew Note

The Homebrew formula (`brew install skillshare`) is optimized for CLI-only usage and does not include the web UI. If you installed via Homebrew and want the dashboard, reinstall with the full installer:

```bash
brew uninstall skillshare
curl -fsSL https://raw.githubusercontent.com/runkids/skillshare/main/install.sh | sh
```

All other CLI commands work identically regardless of install method.

## Architecture

The web UI is a single-page React application embedded in the Go binary via `go:embed`. No external dependencies are needed at runtime — just the `skillshare` binary.

```
skillshare ui
  ├── Go HTTP server (net/http)
  │   ├── /api/*    → REST API handlers
  │   └── /*        → Embedded React SPA
  └── Browser opens http://127.0.0.1:19420
```

## Related

- [status](/docs/commands/status) — CLI status check
- [sync](/docs/commands/sync) — CLI sync command
- [Project Setup](/docs/guides/project-setup) — Project mode setup guide
- [Docker Sandbox](/docs/guides/docker-sandbox) — Run UI in Docker
