---
sidebar_position: 1
---

# ui

Launch the web dashboard for visual skill management.

```bash
skillshare ui                  # Run in the foreground
skillshare ui start            # Start (or reuse) a background server
skillshare ui stop             # Stop the background server
```

Opens `http://127.0.0.1:19420` in your default browser.

## Modes

| Mode | Behavior |
|------|----------|
| `skillshare ui` (default) | Runs the UI server in the foreground; `Ctrl+C` stops it |
| `skillshare ui start` | Starts the UI server as a background process and returns control to the shell. Re-running `start` reuses the existing process if it's still healthy |
| `skillshare ui stop` | Stops the background UI server started by `skillshare ui start` |

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
| `--app` | `false` | Open the dashboard as a desktop-style Chromium app window when available (`start` mode only) |
| `--clear-cache` | | In the foreground form: clear cached UI assets and exit. Combined with `start`: clear cache, then start in the background |

:::tip Auto-Detection
If `.skillshare/config.yaml` exists in the current directory, the dashboard automatically starts in project mode. Use `-g` to force global mode.
:::

## Examples

```bash
# Default: opens browser on localhost:19420 (foreground)
skillshare ui

# Project mode (manage .skillshare/ skills)
skillshare ui -p

# Custom port
skillshare ui --port 8080

# Docker / remote access
skillshare ui --host 0.0.0.0 --no-open

# Start in the background and return to the shell
skillshare ui start

# Start as a chrome-less desktop-style app window
skillshare ui start --app

# Stop the background server (uses the remembered host/port)
skillshare ui stop

# Clear the cached UI assets, then start fresh in the background
skillshare ui start --clear-cache
```

## Dashboard Pages

The sidebar groups pages by task: syncing, what you manage, where it goes, and upkeep. The line under the name shows the mode and its folder, such as `Global · ~/.config/skillshare`.

Some pages show a count in the sidebar when they need attention. The counts refresh every 15 seconds while the dashboard tab is open:

- **Sync**: changes a sync would apply
- **Git Sync**: uncommitted files, or commits not pushed yet when the tree is clean
- **Audit**: skills and agents blocked by the last scan (shown after you run one)

| Page | Description |
|------|-------------|
| **Dashboard** | Counts for skills, agents, extras, MCP servers, plugins, and targets, plus items that need attention |
| **Sync** | Preview every change per target before writing. Choose which parts to include (Skills, Agents, Extras, MCP). Files edited inside a target are kept unless **Force** is on. Items that exist only in a target can be collected back to source from here. Each sync backs up target folders first |
| **Git Sync** | Commit and push the source repo, push commits that aren't on the remote yet, and pull. Pull syncs what the repo scope holds (`skills`, `agents`, `extras`, or `root`), like [`pull`](/docs/reference/commands/pull). When a first pull can't merge with the remote, it offers a force pull that replaces local files with the remote branch |
| **Hubs** | Reached from the Skills page. **Browse** filters a hub and installs from it; **My hubs** assembles an index from installed skills, validates it and exports it. See [`hub`](/docs/reference/commands/hub) |
| **Skills** / **Agents** | Installed items, the **Updates** tab, and the **Trash** tab. Skills also have an **Analyze** tab that estimates how many tokens each skill adds to a target's context. **Install** searches GitHub or installs from a URL or path. **+ New Skill** opens the creation wizard |
| **Extras** | Rules, commands, and other folders synced alongside skills |
| **MCP** | One row per server, with the Agents it syncs to as toggles. **Add server** takes a URL, a command, a pasted snippet or a file; **Import** reads what an installed Agent already has. Each server's menu has **View what each Agent gets**, which shows the native config per Agent, including unsaved edits. Conflicts offer Import or Replace, and backups can be previewed before restoring |
| **Plugins** | One row per plugin, with its Agents as toggles. Expanding a row also lists the other Agents the source supports; ticking one previews an install. The row menu syncs, updates, removes, or opens **View files**, a read-only browser of the local copy Skillshare reviewed. See [Manage plugins across tools](/docs/how-to/daily-tasks/sharing-plugins) |
| **Targets** | Target list with status. Each target's page edits include/exclude filters and collects local-only skills back to source |
| **Audit** | Security scan of skills and agents, with findings by severity. The **Rules** tab browses every rule by category: switch one off, change its severity, apply a severity to a whole category, pick the scan profile (`default`, `strict`, `permissive`), or open the editor for custom `audit-rules.yaml` |
| **Settings** | Tabbed: **General** (source paths, sync mode, appearance), **Backup** (snapshots and restore), **Log** (operation history), **Health** (the same checks as [`doctor`](/docs/reference/commands/doctor)), **Extensions** (sync-time file transforms), **Files** (direct editors for `config.yaml`, `.skillignore`, and `.agentignore`) |

Old links such as `/collect`, `/install`, `/search`, `/trash`, `/analyze`, `/backup`, `/log`, and `/doctor` redirect to their new place.

The **Files** tab puts a panel beside the editor. For `config.yaml` it shows what the field under the cursor does, the file's structure, and the unsaved changes; for the ignore files it lists what the patterns currently hide. `Cmd+S` / `Ctrl+S` saves. The rules editor under **Audit -> Rules -> Edit YAML** has the same panel plus a **Test** tab that runs a rule's regex against lines you paste.

### Theme System

The dashboard supports two visual styles and three color modes, switchable via the **Theme** button in the sidebar:

| Setting | Options | Default |
|---------|---------|---------|
| **Style** | `Clean` (professional), `Playful` (bold outlines, hard shadows, handwritten headings) | Playful |
| **Mode** | `Light`, `Dark`, `System` (follows OS preference) | Light |

Theme preferences persist in localStorage across sessions.

### Project Mode Differences

When running in project mode (`-p`), the dashboard adapts:

- **Sidebar** shows `Project · <project path>` under the name
- **Git Sync page** is hidden (project skills use the project's own git)
- **Sync** backs up agent target folders only, like `skillshare sync -p`
- **Backup tab** is hidden in Settings (use version control instead)
- **Tracked Repos section** is hidden from Dashboard (not applicable)
- **Settings -> Files** shows `.skillshare/config.yaml` and the project-level `.skillignore` instead of the global versions
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
| GET | `/api/overview` | Skill/target counts, mode, version, config folder (`configDir`) |
| GET | `/api/skills` | List all skills with metadata |
| GET | `/api/skills/{name}` | Skill detail + SKILL.md content |
| GET | `/api/skills/templates` | Get available patterns and categories for skill creation |
| POST | `/api/skills` | Create a new skill (name, pattern, category, scaffoldDirs) |
| DELETE | `/api/skills/{name}` | Uninstall a skill |
| GET | `/api/targets` | List targets with status, include/exclude filters, and per-target expected counts |
| POST | `/api/targets` | Add a target |
| DELETE | `/api/targets/{name}` | Remove a target |
| POST | `/api/sync` | Run sync (supports `dryRun`, `force`, `kind`). Backs up targets first unless `dryRun` is set |
| POST | `/api/git/commit` | Create a local git commit from the source repo without pushing |
| GET | `/api/git/status` | Source repo status, including commits not pushed yet (`ahead`) |
| POST | `/api/push` | Commit any changes, then push. Sets the upstream on the first push |
| POST | `/api/pull` | Pull, then sync what the repo scope holds. When a first pull can't merge, it fails with error code `merge_failed`; retry with `force: true` to replace local files with the remote branch |
| GET | `/api/diff` | Diff between source and targets |
| GET | `/api/search?q=` | Search GitHub for skills |
| POST | `/api/install` | Install a skill from source |
| GET | `/api/audit` | Scan all skills for security threats |
| GET | `/api/audit/rules` | Get custom audit rules YAML |
| PUT | `/api/audit/rules` | Save custom audit rules (validates regex) |
| POST | `/api/audit/rules` | Create starter audit-rules.yaml |
| GET | `/api/audit/rules/compiled` | Every rule after merging built-ins with custom rules, plus the active profile |
| POST | `/api/audit/rules/toggle` | Enable, disable, or re-rate a rule or a whole pattern |
| POST | `/api/audit/rules/reset` | Delete custom rules and restore the built-in defaults |
| PATCH | `/api/audit/policy` | Set `blockThreshold`, `profile`, or both |
| GET | `/api/log` | List log entries with optional filters |
| GET | `/api/config` | Get config as YAML |
| PUT | `/api/config` | Update config YAML |
| GET | `/api/skillignore` | Get `.skillignore` content + ignore stats |
| PUT | `/api/skillignore` | Update `.skillignore` content |
| GET | `/api/doctor` | Run all health checks (JSON) |
| GET | `/api/health` | Liveness probe; returns `200` once the server is ready |
| GET | `/api/version` | Current/latest version + whether an upgrade is available |
| POST | `/api/upgrade` | Run `skillshare upgrade` in place (returns `devMode: true` when the binary is a dev build) |
| POST | `/api/restart` | Restart the local UI server; optional `{ "clearCache": true }` body clears cached UI assets first |

## In-place Upgrade

When the dashboard detects that a newer CLI release is available, the **Update** dialog and the **Doctor** page's *Version* card both expose an **Update now** button:

1. The UI calls `POST /api/upgrade`, which runs `skillshare upgrade` on the host.
2. Once the new binary is in place, the UI calls `POST /api/restart` to restart the local server.
3. The browser polls `GET /api/health` and reloads automatically once the new server is ready.

If the running binary is a development build (`version == "dev"`), the upgrade endpoint returns `devMode: true` and the UI simulates a restart without modifying anything on disk.

If the auto-reload doesn't complete, the dialog tells you to run `skillshare ui start` to bring the background server back up.

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

:::note MCP settings
The MCP page works only when the browser opens the dashboard by `localhost` or an IP address, such as `http://192.168.1.20:19420`. Through a domain name, including a reverse proxy, MCP requests return 403: DNS rebinding attacks always use a domain name. To manage MCP settings on a remote machine, forward the port with `ssh -L 19420:127.0.0.1:19420 HOST` and open `http://localhost:19420`.
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
