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
  <a href="#team-edition">Team Edition</a> •
  <a href="#reference">Reference</a> •
  <a href="#faq">FAQ</a> •
  <a href="#common-issues">Common Issues</a>
</p>

> [!NOTE]
> **[What's New in 0.6.0 — Team Edition 🎉](https://github.com/runkids/skillshare/releases/tag/v0.6.0)**
> - **Tracked Repositories**: `install --track` to clone team skill repos, `update` to keep them current
> - **Nested Skills**: Organize skills in folders (`work/api/` → `work__api/`)
> - **Auto-Pruning**: Orphaned symlinks are automatically cleaned on sync
> - **Collision Detection**: Warns when multiple skills share the same name
> - [Learn more →](#team-edition)

## Why skillshare?

**The problem:** You create a skill in Claude, but need it in Cursor, Codex, and Gemini too. Manually copying? Tedious. What if you update it? Copy again.

**The solution:** One source of truth. Create once, sync everywhere.

```bash
skillshare pull claude && skillshare sync  # Pull from Claude → sync to all
```

| What makes it different | |
|-------------------------|---|
| 🔄 Bidirectional sync | `pull` from any target, `sync` to all |
| 🌐 Cross-machine sync | `push` / `pull --remote` via git |
| 💾 Backup & restore | Automatic before sync, restore anytime |
| 🔍 Diagnostics | `doctor` checks git, broken links, duplicates |
| 🤖 AI-native | Built-in skill lets your AI manage everything |

## AI-Native Execution

The built-in [`skillshare` skill](https://github.com/runkids/skillshare/tree/main/skills/skillshare) enables your AI CLI to manage skills directly. Just download the skill folder into your AI CLI's skills directory — the binary is auto-downloaded on first use.

```text
┌─────────────────────────────────────────────────────────────┐
│  User: "sync my skills to all targets"                      │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│  AI reads skillshare skill                                  │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│  skillshare sync                                            │
│  ✓ Synced 5 skills to claude, codex, cursor                 │
└─────────────────────────────────────────────────────────────┘
```

> [!TIP]
> Once installed, just say:
> - *"Show my skillshare status"*
> - *"Sync my skills to all targets"*
> - *"Pull skills from Claude and sync everywhere"*
> - *"Install the pdf skill from anthropics/skills"*

## Installation

### Quick Install

```bash
curl -fsSL https://raw.githubusercontent.com/runkids/skillshare/main/install.sh | sh
```

Installs to `/usr/local/bin/`. Works on macOS and Linux.

### Homebrew (macOS)

```bash
brew install runkids/tap/skillshare
```

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
       │  Claude   │   │   Codex   │   │  OpenCode │
       │  skills/  │   │  skills/  │   │  skills/  │
       └───────────┘   └───────────┘   └───────────┘
```

> Nested paths like `work/api/` are flattened to `work__api/` in targets. See [Nested Skills](#nested-skills-manual-organization).

## Commands

| Command | Description |
|---------|-------------|
| `init` | Initialize skillshare, detect CLIs, setup git |
| `install` | Install a skill from local path or git repo |
| `install --track` | Install a git repo as tracked repository (Team Edition) |
| `uninstall` | Remove a skill or tracked repo from source |
| `list` | List all installed skills and tracked repos |
| `sync` | Sync skills to all targets (with auto-pruning) |
| `status` | Show source, targets, tracked repos, and sync state |
| `diff` | Show differences between source and targets |
| `pull` | Pull skills from target back to source |
| `pull --remote` | Pull from git remote and sync to all targets |
| `push` | Commit and push skills to git remote |
| `update` | Update a skill or tracked repository |
| `backup` | Manually backup targets |
| `restore` | Restore from backup |
| `doctor` | Diagnose configuration issues |
| `upgrade` | Upgrade CLI and built-in skillshare skill |

### Target Management

| Command | Description |
|---------|-------------|
| `target list` | List all configured targets |
| `target <name>` | Show target info (path, mode, status) |
| `target <name> --mode <mode>` | Change sync mode (merge/symlink) |
| `target add <name> <path>` | Add custom target |
| `target remove <name>` | Safely unlink target (backs up first) |

---

## Team Edition

Share skills across your team with tracked repositories. Clone once, update with `git pull`, sync everywhere.

### Why Team Edition?

| Feature | Before | After |
|---------|--------|-------|
| Share skills | Manual copy | `install --track` + `sync` |
| Update skills | Re-download | `update <name>` |
| Nested organization | Not supported | `team/frontend/ui` → `team__frontend__ui` |
| Clean up | Manual | Auto-pruning on sync |

### Quick Start

```bash
# Install a team skills repo
skillshare install github.com/team/shared-skills --track

# Sync to all targets
skillshare sync

# Later, update the repo
skillshare update _shared-skills
skillshare sync
```

### How It Works

```
Source Directory                    Target Directory
─────────────────────────────────────────────────────────────
skills/
├── my-local-skill/                 my-local-skill/
├── personal/
│   └── writing/
│       └── email/                  personal__writing__email/
└── _team-repo/          ─────►
    ├── frontend/
    │   └── ui/                     _team-repo__frontend__ui/
    └── backend/
        └── api/                    _team-repo__backend__api/
```

**Key concepts:**
- **Tracked repos** start with `_` (e.g., `_team-repo`)
- **Nested paths** use `__` separator (e.g., `a/b/c` → `a__b__c`)
- **Auto-pruning** removes orphaned symlinks on sync

### Commands

```bash
# Install as tracked repository (preserves .git)
skillshare install github.com/team/skills --track
skillshare install github.com/team/skills --track --name custom-name

# Update tracked repos
skillshare update _team-repo          # Update specific repo
skillshare update --all               # Update all tracked repos

# View tracked repos
skillshare list                       # Shows skills + tracked repos
skillshare status                     # Shows repo status (up-to-date/has changes)

# Uninstall tracked repo
skillshare uninstall _team-repo       # Checks for uncommitted changes
skillshare uninstall _team-repo -f    # Force (ignore uncommitted changes)
```

### Nested Skills (Manual Organization)

Organize skills in subdirectories for better categorization. Skillshare automatically flattens them during sync:

```
Source (your organization)          Target (flattened for CLI)
─────────────────────────────────────────────────────────────
skills/
├── my-skill/                  →    my-skill/
├── personal/
│   └── writing/
│       └── email/             →    personal__writing__email/
└── work/
    ├── frontend/
    │   └── react/             →    work__frontend__react/
    └── backend/
        └── api/               →    work__backend__api/
```

**How it works:**
- A directory with `SKILL.md` is treated as a skill
- Nested paths use `__` (double underscore) as separator
- Original folder structure is preserved in source
- CLI tools see flat names (required by most AI CLIs)

```bash
# Create nested structure manually
mkdir -p ~/.config/skillshare/skills/personal/writing/email
cat > ~/.config/skillshare/skills/personal/writing/email/SKILL.md << 'EOF'
---
name: email-helper
description: Help write professional emails
---
# Email Helper
EOF

# Sync - creates personal__writing__email/ symlink in all targets
skillshare sync
```

> [!NOTE]
> This works for both personal organization and tracked repos (`_repo/path` → `_repo__path`).

### Name Collision Detection

If multiple skills have the same `name` in their SKILL.md, `sync` will warn you:

```
Name conflicts detected
─────────────────────────────────────────
! Skill name 'deploy' is defined in multiple places:
→   - work/frontend/deploy
→   - work/backend/deploy
→ CLI tools may not distinguish between them.
→ Suggestion: Rename one in SKILL.md (e.g., 'frontend-deploy')
```

**Why this matters:** CLI tools identify skills by the `name` field in SKILL.md, not the directory name. Duplicate names cause unpredictable behavior.

> [!TIP]
> **Best Practice for Team Repos:** Use `{team}:{name}` format in SKILL.md to avoid collisions:
> ```yaml
> # In _acme-corp/frontend/ui/SKILL.md
> name: acme:ui
> ```
> This ensures skills from different teams never conflict.

### Auto-Pruning

When you delete a skill from source, `sync` automatically removes it from targets:

```bash
rm -rf ~/.config/skillshare/skills/_team/old-skill
skillshare sync
# Output: ✓ target: merged (5 linked, 0 local, 0 updated, 1 pruned)
```

**Safety rules:**
- Only removes symlinks pointing to source
- Only removes directories with `_` prefix or `__` in name
- Warns about unknown directories (never auto-deletes)

---

## Reference

Jump to a section:

- [Target Management](#target-management-1)
- [Install Skills](#install-skills)
- [Uninstall Skills](#uninstall-skills)
- [List Skills](#list-skills)
- [Update Skills](#update-skills)
- [Upgrade](#upgrade)
- [Dry Run](#dry-run)
- [Sync Modes](#sync-modes)
- [Backup & Restore](#backup--restore)
- [Cross-Machine Sync](#cross-machine-sync)
- [Configuration](#configuration)
- [FAQ](#faq)
- [Common Issues](#common-issues)

## Target Management

Add, remove, or configure targets for any AI CLI tool.

```bash
skillshare target list                        # List all targets
skillshare target claude                      # Show target info
skillshare target claude --mode merge         # Change sync mode
skillshare target add myapp ~/.myapp/skills   # Add custom target
skillshare target remove myapp                # Safely unlink (backs up first)
```

### Adding Custom Targets

Support any tool with a skills directory:

```bash
skillshare target add windsurf ~/.windsurf/skills
skillshare target add aider ~/.aider/skills
skillshare sync
```

### Removing Targets Safely

`target remove` is safe — it backs up first, then replaces symlinks with copies:

```bash
skillshare target remove claude        # Backup → unlink → copy back
skillshare target remove --all         # Remove all targets
```

> [!WARNING]
> Never use `rm -rf` on a symlinked target — it deletes your source files!

## Install Skills

Install skills from local paths or git repositories directly into your source directory.

### GitHub Shorthand

Use `owner/repo` shorthand for quick installs (automatically expands to `github.com/owner/repo`):

```bash
skillshare install anthropics/skills                    # Discovery mode
skillshare install anthropics/skills/skills/pdf         # Direct install
skillshare install ComposioHQ/awesome-claude-skills     # Another repo
```

### From Git Repository (Discovery Mode)

When installing from a git repo without a specific path, skillshare discovers all skills and lets you choose:

```bash
$ skillshare install anthropics/skills

Discovering skills
─────────────────────────────────────────
→ Source: github.com/anthropics/skills
→ Cloning repository...

✓ Found 17 skill(s)

? Select skills to install:
  [Use arrows to move, space to select, <right> to all, <left> to none, type to filter]
> [ ]  algorithmic-art  (skills/algorithmic-art)
  [ ]  brand-guidelines  (skills/brand-guidelines)
  [ ]  canvas-design  (skills/canvas-design)
  [ ]  doc-coauthoring  (skills/doc-coauthoring)
  [ ]  frontend-design  (skills/frontend-design)
  [ ]  mcp-builder  (skills/mcp-builder)
  [ ]  pdf  (skills/pdf)
  ...
```

### Direct Install (Specific Path)

Install a specific skill directly by providing the full path:

```bash
# GitHub shorthand with subdirectory
skillshare install google-gemini/gemini-cli/packages/core/src/skills/builtin/skill-creator

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
| `--update, -u` | Update existing (git pull if possible, else reinstall) |
| `--track, -t` | Install as tracked repo (preserves .git for updates) |
| `--dry-run, -n` | Preview discovered skills without installing |

### Examples

```bash
# Preview available skills in a repo
skillshare install github.com/ComposioHQ/awesome-claude-skills --dry-run

# Install specific skill with custom name
skillshare install github.com/google-gemini/gemini-cli/packages/core/src/skills/builtin/skill-creator --name my-skill-creator

# Force overwrite existing
skillshare install ~/my-skill --force

# Update existing skill by name (uses stored source metadata)
skillshare install my-skill --update

# Update from source URL directly
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

View all installed skills, tracked repos, and their sources.

```bash
skillshare list            # List all skills and tracked repos
skillshare list --verbose  # Show detailed info (source, type, install date)
```

Example output:
```
Installed skills
-------------------------------------------------------
  my-skill                        (local)
  _team-repo__frontend__ui        (tracked: _team-repo)
  personal__writing__email        (local)

Tracked repositories
-------------------------------------------------------
  _team-repo            5 skills, up-to-date
```

## Update Skills

Update skills or tracked repositories.

```bash
# Update regular skills (reinstalls from stored source)
skillshare update my-skill            # Update skill from its source
skillshare update my-skill --dry-run  # Preview update

# Update tracked repos (git pull)
skillshare update _team-repo          # Update specific repo
skillshare update team-repo           # _ prefix is optional
skillshare update --all               # Update all tracked repos
```

After updating, run `skillshare sync` to distribute changes to all targets.

## Upgrade

Upgrade the CLI binary and built-in `skillshare` skill to the latest version.

```bash
skillshare upgrade              # Upgrade CLI + skill
skillshare upgrade --skill      # Upgrade skill only
skillshare upgrade --cli        # Upgrade CLI only
skillshare upgrade --force      # Skip confirmation
skillshare upgrade --dry-run    # Preview without upgrading
```

After upgrading the skill, run `skillshare sync` to distribute to all targets.

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

skillshare upgrade -n                  # Preview upgrade
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

## Cross-Machine Sync

Sync your skills across multiple computers using git.

```
┌─────────────────┐                      ┌─────────────────┐
│   Machine A     │                      │   Machine B     │
│                 │    skillshare push   │                 │
│  Claude/Codex   │ ─────────────────►   │  Claude/Cursor  │
│   ↕ symlink     │                      │   ↕ symlink     │
│  source/skills  │   ┌───────────┐      │  source/skills  │
│        │        │   │  GitHub   │      │        │        │
│        └────────┼──►│  Remote   │◄─────┼────────┘        │
│                 │   └───────────┘      │                 │
│                 │ ◄─────────────────── │                 │
│                 │  skillshare pull     │                 │
│                 │     --remote         │                 │
└─────────────────┘                      └─────────────────┘
```

### Setup

```bash
# New setup with remote
skillshare init --remote git@github.com:you/my-skills.git

# Or add remote to existing setup
cd ~/.config/skillshare/skills
git remote add origin git@github.com:you/my-skills.git
```

### Daily Workflow

```bash
# Push changes to remote (git add + commit + push)
skillshare push
skillshare push -m "Add new skill"   # Custom message

# Pull from remote and sync to all targets
skillshare pull --remote
```

### Second Machine Setup

```bash
# Clone your skills repo
git clone git@github.com:you/my-skills.git ~/.config/skillshare/skills

# Initialize skillshare with existing source
skillshare init --source ~/.config/skillshare/skills

# Sync to all local targets
skillshare sync
```

### Conflict Handling

- `push` fails if remote is ahead → run `pull --remote` first
- `pull --remote` refuses if local has uncommitted changes → run `push` first

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

## FAQ

**Isn't this just `ln -s`?**

Yes, at its core. But skillshare handles multi-target detection, backup/restore, merge mode, cross-device sync, and broken symlink recovery — so you don't have to.

**Can I sync skills to a custom or uncommon tool?**

Yes. Use `skillshare target add <name> <path>` with the tool's skills directory.

**How do I sync across multiple machines?**

See [Cross-Machine Sync](#cross-machine-sync) section.

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
- `target add` fails with "path does not exist": create the directory first with `mkdir -p <path>`.

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
