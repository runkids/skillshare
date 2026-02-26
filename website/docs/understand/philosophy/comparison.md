---
sidebar_position: 9
---

# Comparing Skill Management Approaches

This page compares the two main architectural approaches to AI CLI skill management: **imperative** (install-per-command) and **declarative** (config + sync).

If you're evaluating tools or considering a switch, this breakdown will help you understand the fundamental design differences.

## Architecture at a Glance

### Imperative (Install-per-command)

Imperative tools use an install-per-command model — each install is a standalone operation:

```
tool add owner/repo → select agents → choose method → done
tool add owner/repo → select agents → choose method → done
tool add owner/repo → select agents → choose method → done
```

Every operation requires user input. There's no persistent state describing "what should be installed where."

### Declarative (Config + Sync)

skillshare uses a declarative model — you define your desired state once, then sync:

```yaml
# config.yaml — define once
source: ~/.config/skillshare/skills
targets:
  claude: ~/.claude
  cursor: ~/.cursor/skills
  codex: ~/.codex/skills
```

```bash
skillshare sync  # reconcile actual state to desired state
```

One command, no prompts, deterministic results every time.

## Feature Comparison

| Capability | Imperative (install-per-command) | Declarative (skillshare) |
|------------|------------------------|--------------------------|
| **Configuration** | No config file; prompts on every run | `config.yaml` — set once, reuse forever |
| **Agent selection** | Interactive prompt each time | Defined in config; `sync` handles all |
| **Install method** | Choose copy/symlink per operation | `sync_mode` in config (merge, copy, or symlink) |
| **Single source of truth** | Skills copied to each agent independently | Source directory → symlinks to all targets |
| **Removing a skill from one agent** | May delete source files, breaking other agents | Only affects that target's symlink |
| **Reproducible setup** | No built-in way to restore on new machine | `config.yaml` + source dir = full restore |
| **Project-scoped skills** | Lock file tracks global only | `skillshare init -p` for per-repo skills |
| **Cross-machine sync** | Manual (sync lock file via dotfiles) | Built-in `push` / `pull` with git |
| **Bidirectional flow** | One-way (install only) | `collect` pulls improvements back from targets |
| **Separating own vs installed skills** | Mixed in same directory | Tracked repos use `_` prefix |
| **Offline operation** | Requires npx + network for CLI itself | Single binary, works offline after install |
| **Web dashboard** | None | `skillshare ui` — visual management |
| **Backup / restore** | None | `skillshare backup` / `skillshare restore` |
| **Git platform support** | GitHub only for update/check (hardcoded to GitHub Trees API) | Any Git remote — GitHub, GitLab, Bitbucket, Azure DevOps, Gitea, AtomGit, Gitee, self-hosted |
| **Runtime dependency** | Node.js + npm | None (single Go binary) |

## Common Pain Points Solved

### "I have to select agents every time I install"

With skillshare, you configure targets once:

```yaml
targets:
  claude: ~/.claude
  cursor: ~/.cursor/skills
```

Then every `sync`, `install`, or `collect` knows where to go. No prompts.

### "Removing a skill from one agent breaks the others"

In imperative tools, removing a skill from one agent may delete the shared source files, leaving other agents with broken symlinks.

skillshare's architecture prevents this entirely — the source directory is the single truth. Target symlinks point **to** the source. Removing a target only removes that target's symlinks; source files are untouched.

```
Source: ~/.config/skillshare/skills/my-skill/SKILL.md  (always preserved)
  ├── ~/.claude/skills/my-skill → symlink to source  ✓
  ├── ~/.cursor/skills/my-skill  → symlink to source  ✓  (unaffected)
  └── ~/.codex/skills/my-skill  → symlink to source  ✓  (unaffected)
```

### "I can't restore my setup on a new machine"

With skillshare, your entire setup is portable:

1. Version-control `~/.config/skillshare/` (source + config)
2. On a new machine: `git clone` your config repo
3. Run `skillshare sync`

All targets are recreated instantly.

### "Update and check don't work with GitLab / Bitbucket / Azure DevOps"

Imperative tools often rely on the GitHub Trees API for update checks, which means `update` and `check` silently skip skills from non-GitHub sources.

skillshare uses **local git operations** (`git fetch` + tree hash comparison) — it works with any Git remote, including GitLab, Bitbucket, Azure DevOps, Gitea, AtomGit, Gitee, and any self-hosted instance. No platform-specific API is required.

```bash
# All of these support install, update, and check:
skillshare install https://gitlab.com/team/skills
skillshare install git@bitbucket.org:company/private-skills.git
skillshare install https://git.mycompany.com/org/repo
skillshare update   # checks all sources, regardless of host
```

### "Clone takes forever on large repositories"

skillshare uses shallow clones (`--depth 1`) by default for non-tracked installs, reducing download time significantly. For tracked repos that need full history, use `--track`.

### "My skills are scattered across agent directories"

skillshare keeps everything in one place:

```
~/.config/skillshare/skills/
├── my-custom-skill/          # Your own skills
├── react-best-practices/     # Installed skills
├── _team-repo/               # Tracked repos (prefixed with _)
│   ├── frontend-guidelines/
│   └── code-review/
└── _another-org-repo/
```

The `_` prefix clearly separates tracked (team/org) repos from your personal skills.

## Migrating to skillshare

If you're already using another skill manager:

### Step 1: Install skillshare

```bash
# macOS / Linux
curl -fsSL https://raw.githubusercontent.com/runkids/skillshare/main/install.sh | sh

# Homebrew
brew install skillshare
```

### Step 2: Initialize and collect existing skills

```bash
skillshare init              # Creates config and detects targets
skillshare collect --all     # Imports existing skills from all detected targets
```

### Step 3: Sync

```bash
skillshare sync              # Symlinks source skills to all targets
```

Your existing skills are now managed from one place. For a detailed walkthrough, see the [Migration Guide](/docs/how-to/advanced/migration).

## Choosing the Right Tool

**Choose an imperative tool if:**
- You install skills rarely and don't mind interactive prompts
- You only use one AI CLI
- You don't need cross-machine or team workflows

**Choose skillshare if:**
- You use multiple AI CLIs and want them in sync
- You want a set-and-forget configuration
- You work across multiple machines
- You share skills with a team or organization
- You want backup, restore, and version control for your skills
- You host skills on GitLab, Bitbucket, Azure DevOps, or self-hosted Git
- You prefer a single binary with no runtime dependencies
- You don't want installation/download activity tracked outside your local workflow

---

## See Also

- [Migration](/docs/how-to/advanced/migration) — Migration guide
- [Core Concepts](/docs/understand) — How skillshare works
