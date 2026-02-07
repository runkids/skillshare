<p align="center" style="margin-bottom: 0;">
  <img src=".github/assets/logo.png" alt="skillshare" width="280">
</p>

<h1 align="center" style="margin-top: 0.5rem; margin-bottom: 0.5rem;">skillshare</h1>

<p align="center">
  <a href="https://skillshare.runkids.cc"><img src="https://img.shields.io/badge/Website-skillshare.runkids.cc-blue?logo=docusaurus" alt="Website"></a>
  <a href="LICENSE"><img src="https://img.shields.io/badge/License-MIT-yellow.svg" alt="License: MIT"></a>
  <a href="go.mod"><img src="https://img.shields.io/github/go-mod/go-version/runkids/skillshare" alt="Go Version"></a>
  <a href="https://github.com/runkids/skillshare/releases"><img src="https://img.shields.io/github/v/release/runkids/skillshare" alt="Release"></a>
  <img src="https://img.shields.io/badge/platform-macOS%20%7C%20Linux%20%7C%20Windows-blue" alt="Platform">
  <a href="https://goreportcard.com/report/github.com/runkids/skillshare"><img src="https://goreportcard.com/badge/github.com/runkids/skillshare" alt="Go Report Card"></a>
  <a href="https://deepwiki.com/runkids/skillshare"><img src="https://deepwiki.com/badge.svg" alt="Ask DeepWiki"></a>
</p>

<p align="center">
  <a href="https://github.com/runkids/skillshare/stargazers"><img src="https://img.shields.io/github/stars/runkids/skillshare?style=social" alt="Star on GitHub"></a>
</p>

<p align="center">
  <strong>One source of truth for AI CLI skills. Sync everywhere with one command — from personal to organization-wide.</strong><br>
  Claude Code, OpenClaw, OpenCode & 40+ more.
</p>

<p align="center">
  <img src=".github/assets/demo.gif" alt="skillshare demo" width="960">
</p>

<p align="center">
  <a href="https://skillshare.runkids.cc">Website</a> •
  <a href="#installation">Install</a> •
  <a href="#quick-start">Quick Start</a> •
  <a href="#commands">Commands</a> •
  <a href="#web-dashboard">Web UI</a> •
  <a href="#project-skills">Project Skills</a> •
  <a href="#organization-skills">Organization Skills</a> •
  <a href="https://skillshare.runkids.cc/docs">Docs</a>
</p>

> [!NOTE]
> **Recent Updates**
> | Version | Highlights |
> |---------|------------|
> | [0.10.0](https://github.com/runkids/skillshare/releases/tag/v0.10.0) | Web Dashboard — visual skill management via `skillshare ui` |
> | [0.9.0](https://github.com/runkids/skillshare/releases/tag/v0.9.0) | Project-level skills — scope skills to a single repo, share via git |
> | [0.8.0](https://github.com/runkids/skillshare/releases/tag/v0.8.0) | `pull` → `collect` rename, clearer command symmetry, refactoring |

## Why skillshare?

Install tools get skills onto agents. **Skillshare keeps them in sync.**

| Feature | Description |
|---------|-------------|
| 🔗 **Non-destructive Merge** | Sync shared skills while preserving CLI-specific ones. Per-skill symlinks keep local skills untouched. |
| ↔️ **Bidirectional Sync** | Created a skill in Claude? Collect it back to source and share with OpenClaw, OpenCode, and others. |
| 🌐 **Cross-machine Sync** | One git push/pull syncs skills across all your machines. No re-running install commands. |
| 📦 **Unified Source** | Local skills and installed skills live together in one directory. No separate management. |
| 👥 **Organization Skills** | Install shared repos once, update anytime with git pull. Changes sync to all agents instantly. |
| 📁 **Project Skills** | Scope skills to a repo, share via git. Auto-detected when you `cd` into the project. |
| ✨ **AI-Native** | Built-in skill lets AI operate skillshare directly. No manual CLI needed. |
| 🖥️ **Web Dashboard** | Visual skill browsing, sync status, and management. Run `skillshare ui` — single binary, no setup. |

## Installation

### Quick Install (macOS/Linux)

```bash
curl -fsSL https://raw.githubusercontent.com/runkids/skillshare/main/install.sh | sh
```

### Quick Install (Windows PowerShell)

```powershell
irm https://raw.githubusercontent.com/runkids/skillshare/main/install.ps1 | iex
```

### Homebrew (macOS)

```bash
brew install runkids/tap/skillshare
```

### Uninstall

```bash
# macOS/Linux
brew uninstall skillshare               # Homebrew
sudo rm /usr/local/bin/skillshare       # Manual install
rm -rf ~/.config/skillshare             # Config & data (optional)

# Windows (PowerShell)
Remove-Item "$env:LOCALAPPDATA\Programs\skillshare" -Recurse -Force
Remove-Item "$env:USERPROFILE\.config\skillshare" -Recurse -Force  # optional
```

### Shorthand (Optional)

Add an alias to your shell config (`~/.zshrc` or `~/.bashrc`):

```bash
alias ss='skillshare'
```

## Quick Start

```bash
skillshare init --dry-run  # Preview setup
skillshare init            # Auto-detects CLIs, sets up git
skillshare sync            # Sync to all targets
```

Done. Your skills are now synced across all AI CLI tools.

## How It Works

```
┌─────────────────────────────────────────────────────────────┐
│                       Source Directory                      │
│                 ~/.config/skillshare/skills/                │
└─────────────────────────────────────────────────────────────┘
                              │ sync
              ┌───────────────┼───────────────┐
              ▼               ▼               ▼
       ┌───────────┐   ┌───────────┐   ┌───────────┐
       │  Claude   │   │  OpenCode │   │ OpenClaw  │   ...
       └───────────┘   └───────────┘   └───────────┘
```

| Platform | Source Path | Link Type |
|----------|-------------|-----------|
| macOS/Linux | `~/.config/skillshare/skills/` | Symlinks |
| Windows | `%USERPROFILE%\.config\skillshare\skills\` | NTFS Junctions (no admin required) |

<p>
  <img src=".github/assets/windows-init.png" alt="Windows init demo" width="720">
</p>

## Documentation

📖 **Full documentation at [skillshare.runkids.cc](https://skillshare.runkids.cc/docs/intro)**

| Guide | Description |
|-------|-------------|
| [Getting Started](https://skillshare.runkids.cc/docs/intro) | Quick start guide |
| [Commands](https://skillshare.runkids.cc/docs/commands/init) | All CLI commands |
| [Web Dashboard](https://skillshare.runkids.cc/docs/commands/ui) | Visual skill management via browser |
| [Project Skills](https://skillshare.runkids.cc/docs/guides/project-setup) | Project-level skills setup |
| [Organization Skills](https://skillshare.runkids.cc/docs/guides/organization-sharing) | Organization-wide sharing with tracked repos |
| [Cross-machine](https://skillshare.runkids.cc/docs/guides/cross-machine-sync) | Multi-machine sync |
| [Docker Sandbox](https://skillshare.runkids.cc/docs/guides/docker-sandbox) | Test sandbox and interactive playground |
| [FAQ](https://skillshare.runkids.cc/docs/troubleshooting/faq) | FAQ & troubleshooting |

## Commands

| Command | Description |
|---------|-------------|
| `init` | Initialize, auto-detect CLIs, setup git |
| `new <name>` | Create a new skill with SKILL.md template |
| `search <query>` | Search GitHub for skills |
| `sync` | Sync skills to all targets |
| `collect <target>` | Collect skills from target back to source |
| `push` | Push to git remote (cross-machine) |
| `pull` | Pull from git remote and sync |
| `install <source>` | Install skill from path or git repo |
| `uninstall <name>` | Remove skill from source |
| `update <name>` | Update skill or tracked repo |
| `list` | List installed skills |
| `status` | Show sync state |
| `doctor` | Diagnose issues |
| `upgrade` | Upgrade CLI and skill |
| `ui` | [Launch web dashboard](#web-dashboard) |

## Web Dashboard

Launch a visual dashboard to browse skills, manage targets, sync, search, and edit config — all from your browser.

```bash
skillshare ui
```

Opens `http://localhost:19420` with a full-featured React dashboard embedded in the binary. No Node.js or extra setup required.

| Page | What you can do |
|------|----------------|
| Dashboard | Overview of skills, targets, sync mode, version |
| Skills | Browse and filter skills, view SKILL.md content, uninstall |
| Install | Install from local path, git URL, or GitHub shorthand |
| Targets | View status, add/remove targets |
| Sync | Run sync with dry-run/force toggles, preview diffs |
| Collect | Collect skills from targets back to source with picker flow |
| Backup | Browse backups, restore, and clean up old snapshots |
| Git Sync | Push/pull source repo, check dirty state, force pull when needed |
| Search | Search GitHub for skills, one-click install |
| Config | Edit config.yaml with validation |

### UI Preview

| Install Flow | Dashboard Overview |
|---|---|
| <img src=".github/assets/ui/web-install-demo.png" alt="web install flow" width="100%"> | <img src=".github/assets/ui/web-dashboard-demo.png" alt="web dashboard overview" width="100%"> |

| Skills Browser | Skill Detail |
|---|---|
| <img src=".github/assets/ui/web-skills-demo.png" alt="web skills browser" width="100%"> | <img src=".github/assets/ui/web-skill-detail-demo.png" alt="web skill detail" width="100%"> |

| Sync Controls | Search Skills |
|---|---|
| <img src=".github/assets/ui/web-sync-demo.png" alt="web sync controls" width="100%"> | <img src=".github/assets/ui/web-search-skills-demo.png" alt="web search skills" width="100%"> |

Also works in Docker: `skillshare ui --host 0.0.0.0 --no-open` (port 19420 is mapped).

---

## Docker Sandbox Testing

Use Docker to run reproducible tests in an isolated environment.

Guide: [Docker Sandbox Guide](https://skillshare.runkids.cc/docs/guides/docker-sandbox)

With `mise`:

```bash
mise run test:docker
```

With `make`:

```bash
make test-docker
```

This offline sandbox runs Go `build + unit + integration` with `network_mode: none`.
It covers install/update flows based on local paths and `file://` git repositories.

Run optional network-dependent install/update checks separately:

With `mise`:

```bash
mise run test:docker:online
```

With `make`:

```bash
make test-docker-online
```

You can also pass custom commands:

```bash
./scripts/test_docker.sh --cmd "go test -v ./tests/integration/..."
./scripts/test_docker_online.sh --cmd "go test -v -tags online ./tests/integration/... -run TestInstall_Remote"
```

### Interactive Playground

Use a dedicated persistent container for manually exploring skillshare commands.

With `mise`:

```bash
mise run sandbox:up
mise run sandbox:shell
```

With `make`:

```bash
make sandbox-up
make sandbox-shell
```

Inside the container you can run directly (no manual build needed):

```bash
skillshare --help
ss --help
skillshare init --dry-run
skillshare ui --host 0.0.0.0 --no-open  # Web UI at http://localhost:19420
```

Stop the playground when done:

With `mise`:

```bash
mise run sandbox:down
```

With `make`:

```bash
make sandbox-down
```

### Target Management

```bash
skillshare target list                    # List targets
skillshare target add myapp ~/.myapp/skills  # Add custom target
skillshare target remove claude           # Safely unlink
```

---

## AI-Native

The built-in [`skillshare` skill](https://github.com/runkids/skillshare/tree/main/skills/skillshare) teaches your AI how to manage skills. The binary auto-downloads on first use.

```
User: "sync my skills to all targets"
       │
       ▼
AI reads skillshare skill → runs: skillshare sync
       │
       ▼
✓ Synced 5 skills to claude, codex, cursor
```

> **Try it:** *"Show my skillshare status"*, *"Collect skills from Claude"*, *"Install the pdf skill from anthropics/skills"*

---

## Search Skills

Discover and install skills from GitHub with interactive search.

```bash
skillshare search runkids
```

<p align="left">
  <img src=".github/assets/search-demo.png" alt="search demo" width="720">
</p>

**Features:**
- **Smart ranking** — Results sorted by repository stars
- **Interactive selector** — Arrow keys to navigate, Enter to install
- **Continuous search** — Search again without restarting
- **Filter forks** — Only shows original repositories

```bash
skillshare search pdf --list      # List only, no install prompt
skillshare search react --json    # JSON output for scripting
skillshare search commit -n 5     # Limit results
```

> **Note:** Requires GitHub authentication. Run `gh auth login` or set `GITHUB_TOKEN`.

See [Search Guide](https://skillshare.runkids.cc/docs/commands/search) for details.

---

## Project Skills

Scope skills to a single repository — shared with your team via git.

```bash
# Initialize project-level skills
skillshare init -p

# Create or install skills
skillshare new my-skill -p
skillshare install anthropics/skills/skills/pdf -p

# Sync to targets
skillshare sync
```

**Features:**
- **Project-scoped** — Skills live in `.skillshare/skills/`, committed to the project repo
- **Auto-detection** — Commands auto-detect project mode when `.skillshare/` exists
- **Team onboarding** — New members run `skillshare install -p && skillshare sync`
- **Coexists with global** — Project and global skills work independently

See [Project Skills Guide](https://skillshare.runkids.cc/docs/guides/project-setup) for details.

---

## Organization Skills

Share skills across your organization with tracked repositories.

```bash
# Install team repo
skillshare install github.com/team/skills --track

# Update later
skillshare update _team-skills
skillshare sync
```

**Features:**
- **Tracked repos** — Clone with `.git`, update via `git pull`
- **Nested skills** — `team/frontend/ui` → `team__frontend__ui`
- **Auto-pruning** — Orphaned symlinks removed on sync
- **Collision detection** — Warns about duplicate skill names

See [Organization Skills Guide](https://skillshare.runkids.cc/docs/guides/organization-sharing) for details.

---

## FAQ

**What if I modify a skill in a target directory?**

Since targets are linked to source, you're editing the source directly. All targets see changes immediately.

**How do I keep CLI-specific skills?**

Use `merge` mode (default). Local skills in targets are preserved.

**Accidentally deleted a skill?**

Recover with git: `cd ~/.config/skillshare/skills && git checkout -- deleted-skill/`

**Windows: Do I need admin privileges?**

No. skillshare uses NTFS junctions (not symlinks), which don't require elevated permissions.

See [FAQ & Troubleshooting](https://skillshare.runkids.cc/docs/troubleshooting/faq) for more.

---

## Common Issues

| Issue | Solution |
|-------|----------|
| `config not found` | Run `skillshare init` |
| Deleted source via symlink | Use `skillshare target remove`, recover via git |
| Target exists with files | Run `skillshare backup` first |
| Skill not appearing | Run `skillshare doctor`, restart CLI |

---

## Contributing

```bash
git clone https://github.com/runkids/skillshare.git
cd skillshare
go build -o bin/skillshare ./cmd/skillshare
go test ./...
```

[Open an issue](https://github.com/runkids/skillshare/issues) for bugs or feature requests.

If you find skillshare useful, consider giving it a ⭐

## Star History

[![Star History Chart](https://api.star-history.com/svg?repos=runkids/skillshare&type=date&legend=top-left)](https://www.star-history.com/#runkids/skillshare&type=date&legend=top-left)

## License

MIT
