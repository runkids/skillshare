---
sidebar_position: 1
---

# init

First-time setup. Auto-detects installed AI CLIs and configures targets.

```bash
skillshare init              # Interactive setup
skillshare init --dry-run    # Preview without changes
```

## What Happens

```
┌─────────────────────────────────────────────────────────────────┐
│ skillshare init                                                 │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│ 1. Create source directory                                      │
│    → ~/.config/skillshare/skills/                               │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│ 2. Auto-detect AI CLIs                                          │
│    → Found: claude, cursor, codex                               │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│ 3. Initialize git (optional)                                    │
│    → Creates initial commit (always)                            │
│    → Ready for push / pull immediately                          │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│ 4. Set up remote (optional)                                     │
│    → Adds git remote                                            │
│    → Auto-pulls if remote has existing skills                   │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│ 5. Create config.yaml                                           │
│    → ~/.config/skillshare/config.yaml                           │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│ 6. Built-in skill (opt-in)                                      │
│    → Install built-in skillshare skill? [y/N]                   │
│    → Adds /skillshare command to AI CLIs                        │
└─────────────────────────────────────────────────────────────────┘
```

## Project Mode

Initialize project-level skills with `-p`:

```bash
skillshare init -p                              # Interactive
skillshare init -p --targets claude-code,cursor  # Non-interactive
```

### What Happens

```
┌─────────────────────────────────────────────────────────────────┐
│ skillshare init -p                                              │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│ 1. Create .skillshare/ directory                                │
│    → .skillshare/skills/                                        │
│    → .skillshare/config.yaml                                    │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│ 2. Detect AI CLI directories in project                         │
│    → Found: .claude/, .cursor/                                  │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│ 3. Create target skill directories                              │
│    → .claude/skills/                                            │
│    → .cursor/skills/                                            │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│ 4. Write config.yaml                                            │
│    → targets: claude-code, cursor                               │
└─────────────────────────────────────────────────────────────────┘
```

After init, commit `.skillshare/` to git. See [Project Setup](/docs/guides/project-setup) for the full guide.

## Discover Mode

Re-run init on an existing setup to detect and add new AI CLI targets:

### Global

```bash
skillshare init --discover              # Interactive selection
skillshare init --discover --select codex,opencode  # Non-interactive
```

Scans for newly installed AI CLIs not yet in your config and prompts you to add them.

### Project

```bash
skillshare init -p --discover           # Interactive selection
skillshare init -p --discover --select gemini-cli  # Non-interactive
```

Scans the project directory for new AI CLI directories (e.g., `.gemini/`) and adds them as targets.

:::tip
If you run `skillshare init` on an already-initialized setup without `--discover`, the error message will hint you to use it.
:::

## Options

| Flag | Description |
|------|-------------|
| `--source <path>` | Custom source directory |
| `--remote <url>` | Set git remote (implies `--git`; auto-pulls if remote has skills) |
| `--project, -p` | Initialize project-level skills in current directory |
| `--copy-from <name\|path>` | Copy skills from a specific CLI or path |
| `--no-copy` | Start with empty source (skip copy prompt) |
| `--targets <list>` | Comma-separated target names |
| `--all-targets` | Add all detected targets |
| `--no-targets` | Skip target selection |
| `--git` | Initialize git without prompting |
| `--no-git` | Skip git initialization |
| `--skill` | Install built-in skillshare skill without prompting (adds `/skillshare` to AI CLIs) |
| `--no-skill` | Skip built-in skill installation |
| `--discover, -d` | Detect and add new AI CLI targets to existing config |
| `--select <list>` | Comma-separated targets to add (requires `--discover`) |
| `--dry-run` | Preview without changes |

## Common Scenarios

```bash
# Standard setup (auto-detect everything)
skillshare init

# Setup with git remote for cross-machine sync
# → Auto-pulls existing skills if the remote is not empty
skillshare init --remote git@github.com:you/my-skills.git

# Second machine: same command pulls your skills automatically
skillshare init --remote git@github.com:you/my-skills.git

# Use existing skills directory
skillshare init --source ~/.config/skillshare/skills

# Project-level setup
skillshare init -p
skillshare init -p --targets claude-code,cursor

# Fully non-interactive setup
skillshare init --no-copy --all-targets --git --skill

# Add newly installed CLIs to existing config
skillshare init --discover
skillshare init -p --discover
```
