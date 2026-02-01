<p align="center" style="margin-bottom: 0;">
  <img src=".github/assets/logo.png" alt="skillshare" width="280">
</p>

<h1 align="center" style="margin-top: 0.5rem; margin-bottom: 0.5rem;">skillshare</h1>

<p align="center">
  <a href="https://skillshare.runkids.work"><img src="https://img.shields.io/badge/Website-skillshare.runkids.work-blue?logo=docusaurus" alt="Website"></a>
  <a href="LICENSE"><img src="https://img.shields.io/badge/License-MIT-yellow.svg" alt="License: MIT"></a>
  <a href="go.mod"><img src="https://img.shields.io/github/go-mod/go-version/runkids/skillshare" alt="Go Version"></a>
  <a href="https://github.com/runkids/skillshare/releases"><img src="https://img.shields.io/github/v/release/runkids/skillshare" alt="Release"></a>
  <img src="https://img.shields.io/badge/platform-macOS%20%7C%20Linux%20%7C%20Windows-blue" alt="Platform">
  <a href="https://goreportcard.com/report/github.com/runkids/skillshare"><img src="https://goreportcard.com/badge/github.com/runkids/skillshare" alt="Go Report Card"></a>
</p>

<p align="center">
  <a href="https://github.com/runkids/skillshare/stargazers"><img src="https://img.shields.io/github/stars/runkids/skillshare?style=social" alt="Star on GitHub"></a>
</p>

<p align="center">
  <strong>One source of truth for AI CLI skills. Sync everywhere with one command and simplify team sharing.</strong><br>
  Claude Code, OpenClaw, OpenCode & 40+ more.
</p>

<p align="center">
  <img src=".github/assets/demo.gif" alt="skillshare demo" width="960">
</p>

<p align="center">
  <a href="https://skillshare.runkids.work">Website</a> •
  <a href="#installation">Install</a> •
  <a href="#quick-start">Quick Start</a> •
  <a href="#commands">Commands</a> •
  <a href="#team-edition">Team Edition</a> •
  <a href="https://skillshare.runkids.work/docs/intro">Docs</a>
</p>

> [!NOTE]
> **Recent Updates**
> | Version | Highlights |
> |---------|------------|
> | [0.8.0](https://github.com/runkids/skillshare/releases/tag/v0.8.0) | `pull` → `collect` rename, clearer command symmetry, refactoring |
> | [0.7.0](https://github.com/runkids/skillshare/releases/tag/v0.7.0) | Windows support, GitHub skill search |
> | [0.6.0](https://github.com/runkids/skillshare/releases/tag/v0.6.0) | Team Edition with tracked repos |

## Why skillshare?

Install tools get skills onto agents. **Skillshare keeps them in sync.**

| Feature | Description |
|---------|-------------|
| 🔗 **Non-destructive Merge** | Sync shared skills while preserving CLI-specific ones. Per-skill symlinks keep local skills untouched. |
| ↔️ **Bidirectional Sync** | Created a skill in Claude? Collect it back to source and share with OpenClaw, OpenCode, and others. |
| 🌐 **Cross-machine Sync** | One git push/pull syncs skills across all your machines. No re-running install commands. |
| 📦 **Unified Source** | Local skills and installed skills live together in one directory. No separate management. |
| 👥 **Team Sharing** | Install team repos once, update anytime with git pull. Changes sync to all agents instantly. |
| ✨ **AI-Native** | Built-in skill lets AI operate skillshare directly. No manual CLI needed. |

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

## Commands

| Command | Description |
|---------|-------------|
| `init` | Initialize, auto-detect CLIs, setup git |
| `new <name>` | Create a new skill with SKILL.md template |
| `search <query>` | [Search GitHub for skills](#search-skills) |
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

## Documentation

📖 **Full documentation at [skillshare.runkids.work](https://skillshare.runkids.work/docs/intro)**

| Guide | Description |
|-------|-------------|
| [Getting Started](https://skillshare.runkids.work/docs/intro) | Quick start guide |
| [Commands](https://skillshare.runkids.work/docs/commands/init) | All CLI commands |
| [Team Edition](https://skillshare.runkids.work/docs/guides/team-edition) | Team sharing with tracked repos |
| [Cross-machine](https://skillshare.runkids.work/docs/guides/cross-machine) | Multi-machine sync |
| [FAQ](https://skillshare.runkids.work/docs/faq) | FAQ & troubleshooting |

---

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

See [Search Guide](https://skillshare.runkids.work/docs/commands/search) for details.

---

## Team Edition

Share skills across your team with tracked repositories.

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

See [Team Edition Guide](https://skillshare.runkids.work/docs/guides/team-edition) for details.

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

See [FAQ & Troubleshooting](https://skillshare.runkids.work/docs/faq) for more.

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
