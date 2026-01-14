<p align="center" style="margin-bottom: 0;">
  <img src=".github/assets/logo.png" alt="skillshare" width="280">
</p>

<h1 align="center" style="margin-top: 0.5rem; margin-bottom: 0.5rem;">skillshare</h1>

<p align="center">
  <a href="LICENSE"><img src="https://img.shields.io/badge/License-MIT-yellow.svg" alt="License: MIT"></a>
  <a href="go.mod"><img src="https://img.shields.io/github/go-mod/go-version/runkids/skillshare" alt="Go Version"></a>
  <a href="https://github.com/runkids/skillshare/releases"><img src="https://img.shields.io/github/v/release/runkids/skillshare" alt="Release"></a>
  <a href="https://github.com/runkids/skillshare/releases"><img src="https://img.shields.io/github/downloads/runkids/skillshare/total" alt="Downloads"></a>
</p>

<p align="center">
  <strong>Sync skills to all your AI CLI tools with one command</strong><br>
  Supports Claude Code, Codex CLI, GitHub Copilot, Cursor, Antigravity, OpenCode
</p>

<p align="center">
  <img src=".github/assets/demo.gif" alt="skillshare demo" width="600">
</p>

<p align="center">
  <a href="#installation">Install</a> •
  <a href="#quick-start">Quick Start</a> •
  <a href="#commands">Commands</a> •
  <a href="docs/DETAILS.md">Detailed Docs</a> •
  <a href="#common-issues">Common Issues</a>
</p>

## Installation

```bash
brew install runkids/tap/skillshare
```

> Other methods: [Detailed Installation](docs/DETAILS.md#detailed-installation)

## Quick Start

```bash
skillshare init    # Auto-detects installed CLIs, sets up git
skillshare sync    # Syncs skills to all targets
```

Done! Your skills are now synced across all AI CLI tools.

## Built-in Skill

After syncing, your AI CLI tools can manage skills directly:

> "Pull my latest skill from Claude and sync to all targets"

The `skillshare` skill is included and ready to use.

## How It Works

```
┌─────────────────────────────────────────────────────────────┐
│                  ~/.config/skillshare/skills/               │
│         my-skill/   another-skill/   shared-util/           │
└─────────────────────────────────────────────────────────────┘
                              │
              ┌───────────────┼───────────────┐
              │               │               │
              ▼               ▼               ▼
       ┌───────────┐   ┌───────────┐   ┌───────────┐
       │  Claude   │   │   Codex   │   │  Gemini   │
       │  skills/  │   │  skills/  │   │  skills/  │
       └───────────┘   └───────────┘   └───────────┘
```

## Commands

| Command | Description |
|---------|-------------|
| `init` | Initialize skillshare, detect CLIs, setup git |
| `sync` | Sync skills to all targets |
| `status` | Show source, targets, and sync state |
| `diff` | Show differences between source and targets |
| `pull` | Pull skills from target back to source |
| `backup` | Manually backup targets |
| `restore` | Restore from backup |
| `doctor` | Diagnose configuration issues |
| `target` | Manage targets (add/remove/list/mode) |

---

# Detailed Documentation

See [`docs/DETAILS.md`](docs/DETAILS.md) for:

- Detailed installation (macOS/Linux/Windows)
- Sync modes and symlink safety
- Backup/restore and configuration
- FAQ and multi-machine workflows

## Contributing

```bash
git clone https://github.com/runkids/skillshare.git
cd skillshare
go build -o bin/skillshare ./cmd/skillshare
go test ./...
```

[Open an issue](https://github.com/runkids/skillshare/issues) for bugs or feature requests.

## Common Issues

- Seeing `config not found: run 'skillshare init' first`: run `skillshare init` (add `--source` if you want a custom path).
- Integration tests cannot find the binary: run `go build -o bin/skillshare ./cmd/skillshare` or set `SKILLSHARE_TEST_BINARY`.
- Deleting a symlinked target removed source files: use `skillshare target remove <name>` to unlink, then recover via git if needed.
- Target directory already exists with files: run `skillshare backup` before `skillshare sync` to migrate safely.
- Target path does not end with `skills`: verify the path and prefer `.../skills` as the suffix.

## License

MIT
