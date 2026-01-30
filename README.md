<p align="center" style="margin-bottom: 0;">
  <img src=".github/assets/logo.png" alt="skillshare" width="280">
</p>

<h1 align="center" style="margin-top: 0.5rem; margin-bottom: 0.5rem;">skillshare</h1>

<p align="center">
  <a href="LICENSE"><img src="https://img.shields.io/badge/License-MIT-yellow.svg" alt="License: MIT"></a>
  <a href="go.mod"><img src="https://img.shields.io/github/go-mod/go-version/runkids/skillshare" alt="Go Version"></a>
  <a href="https://github.com/runkids/skillshare/releases"><img src="https://img.shields.io/github/v/release/runkids/skillshare" alt="Release"></a>
</p>

<p align="center">
  <strong>One source of truth for AI CLI skills. Sync everywhere with one command and simplify team sharing.</strong><br>
  Claude Code, OpenClaw, OpenCode & 30+ more.
</p>

<p align="center">
  <img src=".github/assets/demo.gif" alt="skillshare demo" width="960">
</p>

<p align="center">
  <a href="#installation">Install</a> •
  <a href="#quick-start">Quick Start</a> •
  <a href="#commands">Commands</a> •
  <a href="#team-edition">Team Edition</a> •
  <a href="#documentation">Docs</a>
</p>

> [!NOTE]
> **[What's New in 0.6.0 — Team Edition](https://github.com/runkids/skillshare/releases/tag/v0.6.0)**: Tracked repos, nested skills, auto-pruning, collision detection. [Learn more →](docs/team-edition.md)

## Why skillshare?

Install tools get skills onto agents. **Skillshare keeps them in sync.**

| | Install-once tools | skillshare |
|---|-------------------|------------|
| After install | Done, no management | **Continuous sync** across all agents |
| Update a skill | Re-install manually | **Edit once**, sync everywhere |
| Pull back edits | ✗ | **Bidirectional** — pull from any agent |
| Cross-machine | ✗ | **push/pull** via git |
| Team sharing | Copy-paste | **Tracked repos** — `update` to stay current |
| AI integration | Manual CLI | **Built-in skill** — AI operates it directly |

### AI-Native

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

> **Try it:** *"Show my skillshare status"*, *"Pull skills from Claude"*, *"Install the pdf skill from anthropics/skills"*

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

# Windows
Remove-Item "$env:LOCALAPPDATA\Programs\skillshare" -Recurse -Force
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
│                      Source Directory                        │
│   macOS/Linux: ~/.config/skillshare/skills/                  │
│   Windows:     %USERPROFILE%\.config\skillshare\skills\      │
└─────────────────────────────────────────────────────────────┘
                              │ sync
              ┌───────────────┼───────────────┐
              ▼               ▼               ▼
       ┌───────────┐   ┌───────────┐   ┌───────────┐
       │  Claude   │   │  OpenCode │   │ OpenClaw  │   ...
       └───────────┘   └───────────┘   └───────────┘
```

> **Windows Note:** skillshare uses NTFS junctions instead of symlinks, so no admin privileges required.

## Commands

| Command | Description |
|---------|-------------|
| `init` | Initialize, auto-detect CLIs, setup git |
| `new <name>` | Create a new skill with SKILL.md template |
| `sync` | Sync skills to all targets |
| `pull <target>` | Pull skills from target back to source |
| `push` | Push to git remote (cross-machine) |
| `install <source>` | Install skill from path or git repo |
| `uninstall <name>` | Remove skill from source |
| `update <name>` | Update skill or tracked repo |
| `list` | List installed skills |
| `status` | Show sync state |
| `doctor` | Diagnose issues |
| `upgrade` | Upgrade CLI and skill |

### Target Management

```bash
skillshare target list                    # List targets
skillshare target add myapp ~/.myapp/skills  # Add custom target
skillshare target remove claude           # Safely unlink
```

See [Documentation](docs/README.md) for complete reference.

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

See [Team Edition Guide](docs/team-edition.md) for details.

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

See [FAQ & Troubleshooting](docs/faq.md) for more.

---

## Common Issues

| Issue | Solution |
|-------|----------|
| `config not found` | Run `skillshare init` |
| Deleted source via symlink | Use `skillshare target remove`, recover via git |
| Target exists with files | Run `skillshare backup` first |
| Skill not appearing | Run `skillshare doctor`, restart CLI |

---

## Documentation

- **[docs/](docs/README.md)** — Documentation index
- **[install.md](docs/install.md)** — Install, update, upgrade skills
- **[sync.md](docs/sync.md)** — Sync, pull, push, backup
- **[targets.md](docs/targets.md)** — Target management
- **[team-edition.md](docs/team-edition.md)** — Team sharing with tracked repos
- **[cross-machine.md](docs/cross-machine.md)** — Multi-machine sync
- **[faq.md](docs/faq.md)** — FAQ & troubleshooting

---

## Contributing

```bash
git clone https://github.com/runkids/skillshare.git
cd skillshare
go build -o bin/skillshare ./cmd/skillshare
go test ./...
```

[Open an issue](https://github.com/runkids/skillshare/issues) for bugs or feature requests.

## Star History

[![Star History Chart](https://api.star-history.com/svg?repos=runkids/skillshare&type=date&legend=top-left)](https://www.star-history.com/#runkids/skillshare&type=date&legend=top-left)

## License

MIT
