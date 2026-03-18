---
sidebar_position: 1
---

# ui

Launch the web dashboard for visual skill management.

```bash
skillshare ui
```

Opens `http://127.0.0.1:19420` in your default browser.

## When to Use

- Manage skills, targets, and sync through a visual web interface
- Browse and install skills without memorizing CLI flags
- Run security audits with a visual findings report
- Share a dashboard view with team members less comfortable with CLI

## Flags

| Flag | Default | Description |
|------|---------|-------------|
| `-p`, `--project` | | Run in project mode (uses `.skillshare/`) |
| `-g`, `--global` | | Run in global mode (uses `~/.config/skillshare/`) |
| `--port <port>` | `19420` | HTTP server port |
| `--host <host>` | `127.0.0.1` | Bind address (use `0.0.0.0` for Docker) |
| `-b`, `--base-path <path>` | | Sub-path for reverse proxy (e.g., `/skillshare`) |
| `--no-open` | `false` | Don't open browser automatically |
| `--clear-cache` | | Clear downloaded UI cache and exit |

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
| **Skills** | Searchable skill grid with metadata. Toggle between **Grid** and **Grouped** (by directory) views. Click to view SKILL.md content. **"+ New Skill"** button opens the creation wizard |
| **New Skill** | Step-by-step wizard to create a skill: Name → Pattern → Category → Scaffold → Confirm. Accessible via the **"+ New Skill"** button on the Skills page |
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
| **Config** | YAML config editor + `.skillignore` editor (tab switching) |

### Theme System

The dashboard supports two visual styles and three color modes, switchable via the **Theme** button in the sidebar:

| Setting | Options | Default |
|---------|---------|---------|
| **Style** | `Clean` (professional), `Playful` (hand-drawn borders, organic shapes) | Playful |
| **Mode** | `Light`, `Dark`, `System` (follows OS preference) | Light |

Theme preferences persist in localStorage across sessions.

### Project Mode Differences

When running in project mode (`-p`), the dashboard adapts:

- **"Project" badge** in the sidebar indicates project mode
- **Git Sync page** is hidden (project skills use the project's own git)
- **Backup & Restore page** is hidden (use version control instead)
- **Tracked Repos section** is hidden from Dashboard (not applicable)
- **Config page** shows `.skillshare/config.yaml` and project-level `.skillignore` instead of the global versions
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
| GET | `/api/skills/templates` | Get available patterns and categories for skill creation |
| POST | `/api/skills` | Create a new skill (name, pattern, category, scaffoldDirs) |
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
| GET | `/api/skillignore` | Get `.skillignore` content + ignore stats |
| PUT | `/api/skillignore` | Update `.skillignore` content |
| GET | `/api/doctor` | Run all health checks (JSON) |

## Reverse Proxy

If you run the dashboard on a shared server behind a reverse proxy (e.g., homelab, internal tools platform), use `--base-path` to serve it under a sub-path alongside other services:

```bash
skillshare ui --base-path /skillshare --host 0.0.0.0 --no-open
```

Or via environment variable:

```bash
SKILLSHARE_UI_BASE_PATH=/skillshare skillshare ui --host 0.0.0.0 --no-open
```

### Nginx

```nginx
location /skillshare/ {
    proxy_pass http://127.0.0.1:19420;
    proxy_set_header Host $host;
    proxy_set_header X-Real-IP $remote_addr;
}
```

### Caddy

```
handle_path /skillshare/* {
    reverse_proxy 127.0.0.1:19420
}
```

:::tip
Without `--base-path`, the dashboard behaves identically to before — no configuration needed for direct access on `localhost:19420`.
:::

## Docker Usage

To use the web UI inside Docker (requires network access for first-time UI download):

```bash
make playground

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

## Runtime UI Download

`skillshare ui` automatically downloads pre-built UI assets from the matching GitHub Release on first launch. The assets are cached in `~/.cache/skillshare/ui/<version>/` (respects `XDG_CACHE_HOME`) so subsequent launches are instant and offline.

- **First run** requires an internet connection to download the UI assets (~1 MB)
- **Subsequent runs** use the cached assets — no network needed
- **On upgrade**, old cached versions are automatically cleaned up; the new UI is pre-downloaded during `skillshare upgrade`
- **To clear the cache manually**, run `skillshare ui --clear-cache`

## Homebrew Note

All install methods (Homebrew, installer script, manual binary) use runtime UI download. When you run `skillshare ui`, it automatically downloads the UI assets from GitHub on first launch. After that, the cached assets are used offline.

To clear the downloaded UI cache:

```bash
skillshare ui --clear-cache
```

## Architecture

The web UI is a single-page React application downloaded at runtime from the matching GitHub Release and served from disk cache (`~/.cache/skillshare/ui/<version>/`).

```
skillshare ui
  ├── Go HTTP server (net/http)
  │   ├── /api/*    → REST API handlers
  │   └── /*        → Cached React SPA (runtime download)
  └── Browser opens http://127.0.0.1:19420
```

## See Also

- [status](/docs/reference/commands/status) — CLI status check
- [sync](/docs/reference/commands/sync) — CLI sync command
- [Project Setup](/docs/how-to/sharing/project-setup) — Project mode setup guide
- [Docker Sandbox](/docs/how-to/advanced/docker-sandbox) — Run UI in Docker
