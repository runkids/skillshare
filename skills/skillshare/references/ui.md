# Web Dashboard (`skillshare ui`)

Launch a browser-based dashboard for visual skill management. Single binary — no extra setup.

## Usage

```bash
skillshare ui              # Global mode (auto-opens browser)
skillshare ui -p           # Project mode
skillshare ui -g           # Force global mode
```

Auto-detects project mode when `.skillshare/config.yaml` exists in cwd.

## Flags

| Flag | Default | Description |
|------|---------|-------------|
| `-p, --project` | | Project mode (`.skillshare/`) |
| `-g, --global` | | Global mode (`~/.config/skillshare/`) |
| `--port <port>` | `19420` | HTTP server port |
| `--host <host>` | `127.0.0.1` | Bind address (`0.0.0.0` for Docker/remote) |
| `--no-open` | `false` | Don't open browser automatically |

## Dashboard Pages

| Page | Description |
|------|-------------|
| **Dashboard** | Overview cards — skill count, target count, sync mode, version |
| **Skills** | Searchable skill grid with metadata. Click to view SKILL.md |
| **Install** | Install from local path, git URL, or GitHub shorthand |
| **Targets** | Target list with status badges and drift indicators. Add/remove targets |
| **Sync** | Sync controls with dry-run toggle. Diff preview |
| **Collect** | Scan targets and collect selected skills back to source |
| **Audit** | Security scan all skills. Severity badges per finding |
| **Trash** | List/restore/delete/empty trashed skills |
| **Log** | Operation and audit log viewer |
| **Backup** | View/restore/cleanup backups (global only) |
| **Git Sync** | Push/pull source repo (global only) |
| **Search** | GitHub skill search with one-click install |
| **Config** | YAML config editor with validation |

## Project Mode Differences

When running with `-p`, the dashboard adapts:

- **Git Sync** and **Backup** pages are hidden
- **Audit**, **Trash**, and **Log** pages adapt to project scope
- **Config** edits `.skillshare/config.yaml`
- **Install** reconciles remote skill entries in project config
- **Install** shows confirm dialog on CRITICAL audit findings with "Force Install" option
- **"Project" badge** in the sidebar indicates mode

## REST API

All endpoints at `/api/`, returning JSON:

```bash
# Examples
curl http://127.0.0.1:19420/api/overview
curl http://127.0.0.1:19420/api/skills
curl -X POST http://127.0.0.1:19420/api/sync -d '{"dryRun":true}'
```

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/overview` | Skill/target counts, mode, version |
| GET | `/api/skills` | List all skills with metadata |
| GET | `/api/skills/{name}` | Skill detail + SKILL.md content |
| DELETE | `/api/skills/{name}` | Uninstall a skill |
| GET | `/api/targets` | List targets with status |
| POST | `/api/targets` | Add a target |
| DELETE | `/api/targets/{name}` | Remove a target |
| POST | `/api/sync` | Run sync (`dryRun`, `force`) |
| GET | `/api/diff` | Diff between source and targets |
| GET | `/api/search?q=` | Search GitHub for skills |
| POST | `/api/install` | Install a skill from source |
| GET | `/api/audit` | Scan all skills |
| GET | `/api/audit/{name}` | Scan specific skill |
| GET | `/api/trash` | List trashed skills |
| POST | `/api/trash/{name}/restore` | Restore from trash |
| DELETE | `/api/trash/{name}` | Delete from trash |
| DELETE | `/api/trash` | Empty trash |
| GET | `/api/log` | Operation log entries |
| GET | `/api/log/audit` | Audit log entries |
| GET | `/api/config` | Get config as YAML |
| PUT | `/api/config` | Update config YAML |

## Docker / Remote

```bash
skillshare ui --host 0.0.0.0 --no-open        # Global
skillshare ui -p --host 0.0.0.0 --no-open     # Project
```

Access from host: `http://localhost:19420`
