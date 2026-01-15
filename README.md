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
  Supports Amp, Claude Code, Codex CLI, Crush, Cursor, Gemini CLI, GitHub Copilot, Goose, Letta, Antigravity, OpenCode
</p>

<p align="center">
  <img src=".github/assets/demo.gif" alt="skillshare demo" width="600">
</p>

<p align="center">
  <a href="#installation">Install</a> •
  <a href="#quick-start">Quick Start</a> •
  <a href="#commands">Commands</a> •
  <a href="#reference">Reference</a> •
  <a href="#faq">FAQ</a> •
  <a href="#common-issues">Common Issues</a>
</p>

## Highlights

- One source of truth for skills across CLI tools.
- Auto-detects installed targets and bootstraps git.
- Choose `merge` or `symlink` sync modes.
- Automatic backups + restores protect local skills.
- Built-in `skillshare` skill enables AI-driven sync.

> [!TIP]
> **Let your AI manage skills for you.** After syncing, tell your AI:
>
> *"I just created a new skill in Claude Code. Pull it to source and sync to all targets."*
>
> No manual copying needed — the `skillshare` skill handles everything.

## Installation

### Quick Install

```bash
curl -fsSL https://raw.githubusercontent.com/runkids/skillshare/main/install.sh | sh
```

Works on macOS and Linux. Automatically detects your system and installs the latest version.

### Homebrew (macOS)

```bash
brew install runkids/tap/skillshare
```

### Manual install

#### macOS

```bash
# Apple Silicon (M1/M2/M3/M4)
curl -sL https://github.com/runkids/skillshare/releases/latest/download/skillshare_darwin_arm64.tar.gz | tar xz
sudo mv skillshare /usr/local/bin/

# Intel
curl -sL https://github.com/runkids/skillshare/releases/latest/download/skillshare_darwin_amd64.tar.gz | tar xz
sudo mv skillshare /usr/local/bin/
```

#### Linux

```bash
# x86_64
curl -sL https://github.com/runkids/skillshare/releases/latest/download/skillshare_linux_amd64.tar.gz | tar xz
sudo mv skillshare /usr/local/bin/

# ARM64
curl -sL https://github.com/runkids/skillshare/releases/latest/download/skillshare_linux_arm64.tar.gz | tar xz
sudo mv skillshare /usr/local/bin/
```

#### Windows

Download from [Releases](https://github.com/runkids/skillshare/releases) and add to PATH.

### Uninstall

```bash
brew uninstall skillshare              # Homebrew
sudo rm /usr/local/bin/skillshare      # Manual install
rm -rf ~/.config/skillshare            # Config & data (optional)
```

## Quick Start

```bash
skillshare init --dry-run  # Preview setup
skillshare init            # Auto-detects installed CLIs, sets up git
skillshare sync            # Syncs skills to all targets
```

Done! Your skills are now synced across all AI CLI tools.

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
| `install` | Install a skill from local path or git repo |
| `uninstall` | Remove a skill from source directory |
| `list` | List all installed skills |
| `sync` | Sync skills to all targets |
| `status` | Show source, targets, and sync state |
| `diff` | Show differences between source and targets |
| `pull` | Pull skills from target back to source |
| `backup` | Manually backup targets |
| `restore` | Restore from backup |
| `doctor` | Diagnose configuration issues |
| `target` | Manage targets (add/remove/list/mode) |
| `update` | Update built-in skillshare skill from GitHub |

---

## Reference

Jump to a section:

- [Install Skills](#install-skills)
- [Uninstall Skills](#uninstall-skills)
- [List Skills](#list-skills)
- [Update Built-in Skill](#update-built-in-skill)
- [Dry Run](#dry-run)
- [Sync Modes](#sync-modes)
- [Backup & Restore](#backup--restore)
- [Configuration](#configuration)
- [FAQ](#faq)
- [Common Issues](#common-issues)

## Install Skills

Install skills from local paths or git repositories directly into your source directory.

### From Git Repository (Discovery Mode)

When installing from a git repo without a specific path, skillshare discovers all skills and lets you choose:

```bash
$ skillshare install github.com/ComposioHQ/awesome-claude-skills

Discovering skills
---------------------------------------------
Source: github.com/ComposioHQ/awesome-claude-skills
Cloning repository...

✓ Found 5 skill(s):

  [1] commit-reviewer
      skills/commit-reviewer
  [2] code-documenter
      skills/code-documenter
  [3] test-generator
      skills/test-generator
  ...

Enter numbers to install (e.g., 1,2,3 or 'all' or 'q' to quit): 1,3
```

### Direct Install (Specific Path)

Install a specific skill directly by providing the full path:

```bash
# From GitHub subdirectory (monorepo)
skillshare install github.com/google-gemini/gemini-cli/packages/core/src/skills/builtin/skill-creator

# From local path
skillshare install ~/Downloads/my-skill

# From SSH git URL
skillshare install git@github.com:user/repo.git
```

### Options

| Option | Description |
|--------|-------------|
| `--name <name>` | Override the skill name (direct install only) |
| `--force, -f` | Overwrite if skill already exists |
| `--update, -u` | Update existing installation (git pull) |
| `--dry-run, -n` | Preview discovered skills without installing |

### Examples

```bash
# Preview available skills in a repo
skillshare install github.com/ComposioHQ/awesome-claude-skills --dry-run

# Install specific skill with custom name
skillshare install github.com/google-gemini/gemini-cli/packages/core/src/skills/builtin/skill-creator --name my-skill-creator

# Force overwrite existing
skillshare install ~/my-skill --force

# Update existing git-based installation
skillshare install github.com/user/skill-repo --update
```

After installing, run `skillshare sync` to distribute the skill to all targets.

## Uninstall Skills

Remove a skill from the source directory.

```bash
skillshare uninstall my-skill           # Prompts for confirmation
skillshare uninstall my-skill --force   # Skip confirmation
skillshare uninstall my-skill --dry-run # Preview without removing
```

After uninstalling, run `skillshare sync` to update all targets.

## List Skills

View all installed skills and their sources.

```bash
skillshare list            # List all skills
skillshare list --verbose  # Show detailed info (source, type, install date)
```

Example output:
```
Installed skills
--------------------------------------------------
  my-skill                   (local)
  skill-creator              github.com/google-gemini/gemini-cli/...
  composio-skills            github.com/ComposioHQ/awesome-claude-skills
```

## Update Built-in Skill

> **Note:** Available since v0.3.1

Update the built-in `skillshare` skill to the latest version from GitHub. This skill enables AI-driven management of your skills.

```bash
skillshare update              # Update skillshare skill
skillshare update --force      # Skip confirmation
skillshare update --dry-run    # Preview without updating
```

The skill is downloaded from the [skillshare repository](https://github.com/runkids/skillshare/tree/main/skills/skillshare) and includes AI behavior guides, command references, and troubleshooting tips.

After updating, run `skillshare sync` to distribute to all targets.

## Dry Run

Preview changes without modifying files. Supported commands:

```bash
skillshare init --dry-run              # Preview init setup
skillshare install <source> --dry-run  # Preview install
skillshare uninstall <name> --dry-run  # Preview uninstall

skillshare sync --dry-run              # Preview sync changes
skillshare sync -n                     # Short flag for sync

skillshare pull --dry-run              # Preview pull changes
skillshare pull -n                     # Short flag for pull
skillshare pull claude -n              # Preview pull for one target
skillshare pull --all -n               # Preview pull for all targets

skillshare backup --dry-run            # Preview backups
skillshare backup -n                   # Short flag for backup
skillshare backup --cleanup -n         # Preview cleanup
skillshare backup --list               # List backups (read-only)

skillshare restore claude -n           # Preview restore from latest
skillshare restore claude --from 2026-01-14_21-22-18 -n  # Preview restore from timestamp

skillshare target remove claude -n     # Preview unlink
skillshare target remove --all -n      # Preview unlink all

skillshare update -n                   # Preview skill update
```

## Sync Modes

| Mode | Behavior | When to Use |
|------|----------|-------------|
| `merge` | Each skill symlinked individually. Local skills preserved. | **Recommended.** Safe, flexible. |
| `symlink` | Entire directory becomes symlink. All targets identical. | When you want exact copies everywhere. |

Change mode:

```bash
skillshare target claude --mode merge
skillshare sync
```

> [!WARNING]
> **Symlink Safety** — Deleting through a symlinked target **deletes the source**:
> ```bash
> rm -rf ~/.codex/skills/my-skill  # ❌ Deletes from SOURCE!
> skillshare target remove codex   # ✅ Safe way to unlink
> ```

## Backup & Restore

Backups are created **automatically** before `sync` and `target remove`.

Location: `~/.config/skillshare/backups/<timestamp>/`

```bash
skillshare backup              # Manual backup all targets
skillshare backup claude       # Backup specific target
skillshare backup --list       # List all backups
skillshare backup --cleanup    # Remove old backups

skillshare restore claude      # Restore from latest backup
skillshare restore claude --from 2026-01-14_21-22-18  # Specific backup
```

> **Note:** In `symlink` mode, backups are skipped (no local data to backup).

## Configuration

Config file: `~/.config/skillshare/config.yaml`

```yaml
source: ~/.config/skillshare/skills
mode: merge
targets:
  claude:
    path: ~/.claude/skills
  codex:
    path: ~/.codex/skills
    mode: symlink  # Override default mode
  cursor:
    path: ~/.cursor/skills
ignore:
  - "**/.DS_Store"
  - "**/.git/**"
```

### Managing Targets

Add any CLI or tool by pointing to its skills directory:

```bash
skillshare target list                        # List all targets
skillshare target claude                      # Show target info
skillshare target claude --mode merge         # Change mode
skillshare target add myapp ~/.myapp/skills   # Add custom target
skillshare target remove myapp                # Remove target
```

## FAQ

**Isn't this just `ln -s`?**

Yes, at its core. But skillshare handles multi-target detection, backup/restore, merge mode, cross-device sync, and broken symlink recovery — so you don't have to.

**Can I sync skills to a custom or uncommon tool?**

Yes. Use `skillshare target add <name> <path>` with the tool's skills directory.

**How do I sync across multiple machines?**

```bash
# Machine A: Push to remote
cd ~/.config/skillshare/skills
git remote add origin git@github.com:you/my-skills.git
git push -u origin main

# Machine B: Clone and init
git clone git@github.com:you/my-skills.git ~/.config/skillshare/skills
skillshare init --source ~/.config/skillshare/skills
skillshare sync
```

**What happens if I modify a skill in the target directory?**

Since targets are symlinks, changes are made directly to the source. All targets see the change immediately.

**How do I keep a CLI-specific skill?**

Use `merge` mode. Local skills in the target won't be overwritten.

**What if I accidentally delete a skill through a symlink?**

If you have git initialized (recommended), recover with:

```bash
cd ~/.config/skillshare/skills
git checkout -- deleted-skill/
```

**How does `target remove` work? Is it safe?**

Yes, it's safe. Here's how it works:

```
┌─────────────────────────────────────────────────────────┐
│ skillshare target remove claude                         │
└─────────────────────────────────────────────────────────┘
                         │
                         ▼
              ┌─────────────────────┐
              │  1. Backup target   │
              │  (backups/<ts>/)    │
              └─────────────────────┘
                         │
                         ▼
              ┌─────────────────────┐
              │  2. Detect mode     │
              └─────────────────────┘
                    │         │
         symlink mode         merge mode
                    │         │
                    ▼         ▼
┌───────────────────────┐   ┌───────────────────────┐
│ 3a. Remove symlink    │   │ 3b. For each skill:   │
│     ~/.claude/skills  │   │     Check if symlink  │
│         ↓             │   │     points to source  │
│ 4a. Copy source       │   │         ↓             │
│     contents back     │   │ 4b. If yes:           │
└───────────────────────┘   │     - Remove symlink  │
                            │     - Copy back file  │
                            └───────────────────────┘
                         │
                         ▼
              ┌─────────────────────┐
              │  5. Remove from     │
              │     config.yaml     │
              └─────────────────────┘
```

- **Symlink mode**: Removes symlink, copies source contents back to target
- **Merge mode**: Only removes symlinks pointing to source, preserves local-only skills

This is why `skillshare target remove` is safe, while `rm -rf ~/.claude/skills` would delete your source files.

## Common Issues

- Seeing `config not found: run 'skillshare init' first`: run `skillshare init` (add `--source` if you want a custom path).
- Integration tests cannot find the binary: run `go build -o bin/skillshare ./cmd/skillshare` or set `SKILLSHARE_TEST_BINARY`.
- Deleting a symlinked target removed source files: use `skillshare target remove <name>` to unlink, then recover via git if needed.
- Target directory already exists with files: run `skillshare backup` before `skillshare sync` to migrate safely.
- Target path does not end with `skills`: verify the path and prefer `.../skills` as the suffix.

## Contributing

```bash
git clone https://github.com/runkids/skillshare.git
cd skillshare
go build -o bin/skillshare ./cmd/skillshare
go test ./...
```

[Open an issue](https://github.com/runkids/skillshare/issues) for bugs or feature requests.

## License

MIT
