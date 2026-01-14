# skillshare

Share skills across AI CLI tools (Claude Code, Codex CLI, Cursor, Gemini CLI, OpenCode).

## The Problem

Each AI CLI tool has its own skills directory:

```
~/.claude/skills/
~/.codex/skills/
~/.cursor/skills/
~/.gemini/antigravity/skills/
~/.config/opencode/skills/
```

Keeping them in sync manually is tedious.

## The Solution

`skillshare` maintains a single source directory and symlinks skills to all CLI tools:

```
~/.config/skillshare/
    ├── config.yaml
    └── skills/           <- Your shared skills live here
        ├── my-skill/
        └── another-skill/

~/.claude/skills/
    ├── my-skill     -> ~/.config/skillshare/skills/my-skill (symlink)
    ├── another-skill -> ~/.config/skillshare/skills/another-skill (symlink)
    └── local-only/   <- Local skills are preserved (merge mode)
```

## Installation

### macOS

```bash
# Apple Silicon (M1/M2/M3/M4)
curl -sL https://github.com/runkids/skillshare/releases/latest/download/skillshare_darwin_arm64.tar.gz | tar xz
sudo mv skillshare /usr/local/bin/

# Intel
curl -sL https://github.com/runkids/skillshare/releases/latest/download/skillshare_darwin_amd64.tar.gz | tar xz
sudo mv skillshare /usr/local/bin/
```

### Linux

```bash
# x86_64
curl -sL https://github.com/runkids/skillshare/releases/latest/download/skillshare_linux_amd64.tar.gz | tar xz
sudo mv skillshare /usr/local/bin/

# ARM64
curl -sL https://github.com/runkids/skillshare/releases/latest/download/skillshare_linux_arm64.tar.gz | tar xz
sudo mv skillshare /usr/local/bin/
```

### Windows

Download from [Releases](https://github.com/runkids/skillshare/releases) and add to PATH.

### Homebrew (macOS/Linux)

```bash
brew install runkids/tap/skillshare
```

### Verify Installation

```bash
skillshare version
```

### Uninstall

```bash
# Homebrew
brew uninstall skillshare

# Manual (curl install)
sudo rm /usr/local/bin/skillshare

# Config and data (optional)
rm -rf ~/.config/skillshare
```

## Quick Start

```bash
# 1. Initialize (interactive source selection)
skillshare init

# 2. Check detected targets
skillshare status

# 3. Sync (migrate existing skills + create symlinks)
skillshare sync
```

## Usage

### Initialize

```bash
# Interactive mode - choose source from existing skills directories
skillshare init

# Or specify custom source
skillshare init --source ~/my-skills
```

This will:
- Create the source directory
- Detect installed CLI tools
- Optionally copy skills from existing directories
- Create config at `~/.config/skillshare/config.yaml`

### Sync

```bash
# Preview what will happen
skillshare sync --dry-run

# Actually sync
skillshare sync
```

On first sync, existing skills are **migrated** to the source directory, then symlinks are created.

### Status

```bash
skillshare status
```

Shows:
- Source directory and skill count
- Each target's status, mode, and sync state

### Diff

```bash
# Show differences for all targets
skillshare diff

# Show differences for specific target
skillshare diff claude
```

### Backup

```bash
# Backup all targets
skillshare backup

# Backup specific target
skillshare backup claude
```

### Doctor

```bash
skillshare doctor
```

Diagnoses:
- Config file status
- Source directory
- Symlink support
- Each target's health and mode

### Manage Targets

```bash
# List all targets
skillshare target list

# Show target info
skillshare target claude

# Change target sync mode
skillshare target claude --mode merge
skillshare target claude --mode symlink

# Add custom target
skillshare target add myapp ~/.myapp/skills

# Unlink target (restore skills and remove from config)
skillshare target remove myapp

# Unlink all targets
skillshare target remove --all
```

## Configuration

Config file: `~/.config/skillshare/config.yaml`

```yaml
source: ~/.config/skillshare/skills
mode: merge   # default mode for all targets
targets:
  claude:
    path: ~/.claude/skills
  codex:
    path: ~/.codex/skills
    mode: symlink   # override: use full directory symlink
  cursor:
    path: ~/.cursor/skills
  gemini:
    path: ~/.gemini/antigravity/skills
  opencode:
    path: ~/.config/opencode/skills
ignore:
  - "**/.DS_Store"
  - "**/.git/**"
```

### Sync Modes

| Mode | Behavior |
|------|----------|
| `merge` (default) | Each skill is symlinked individually. Local skills in target are preserved. |
| `symlink` | Entire directory becomes a symlink to source. All targets share the same skills. |

Use `symlink` mode when you want all targets to share exactly the same skills.

Change mode per target:
```bash
skillshare target claude --mode symlink
skillshare sync
```

## How It Works

1. **init**: Detects CLI tools, optionally copy from existing skills
2. **sync**: Create symlinks (merge mode: per-skill, symlink mode: whole directory)
3. **status**: Check symlink health and mode
4. **diff**: Show differences between source and targets
5. **backup**: Create manual backup of targets
6. **doctor**: Diagnose configuration issues
7. **target remove**: Backup, unlink symlinks, restore skills

## Sync Across Machines

Use git to sync your skills across multiple machines:

### Initial Setup (Machine A)

```bash
# Initialize skillshare
skillshare init

# Push skills to remote
cd ~/.config/skillshare/skills
git init
git add .
git commit -m "Initial skills"
git remote add origin git@github.com:you/my-skills.git
git push -u origin main
```

### Clone to Another Machine (Machine B)

```bash
# Clone your skills repo
git clone git@github.com:you/my-skills.git ~/.config/skillshare/skills

# Initialize skillshare with the cloned source
skillshare init --source ~/.config/skillshare/skills

# Sync to all targets
skillshare sync
```

### Daily Workflow

```bash
# Machine A - add/update skills
cd ~/.config/skillshare/skills
# ... edit skills ...
git add . && git commit -m "Update skills" && git push

# Machine B - pull and sync
cd ~/.config/skillshare/skills
git pull
skillshare sync  # New skills are automatically symlinked
```

## Backups

Automatic backups are created before `sync` and `target remove` operations.

Location: `~/.config/skillshare/backups/<timestamp>/<target>/`

## License

MIT
