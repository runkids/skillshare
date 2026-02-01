---
sidebar_position: 3
---

# install

Add skills from GitHub repos, git URLs, or local paths.

## Overview

```
┌──────────────────────────────────────────────────────────────┐
│                    SKILL LIFECYCLE                           │
│                                                              │
│   install ──► source ──► sync ──► targets                   │
│                 │                                            │
│              update                                          │
│                 │                                            │
│            uninstall ──► sync ──► removed from targets       │
└──────────────────────────────────────────────────────────────┘
```

---

## Quick Examples

```bash
# From GitHub (shorthand)
skillshare install anthropics/skills/skills/pdf

# Browse available skills in a repo
skillshare install anthropics/skills

# From local path
skillshare install ~/Downloads/my-skill

# As tracked repo (for team sharing)
skillshare install github.com/team/skills --track
```

## GitHub Shorthand

Use `owner/repo` format — automatically expands to `github.com/owner/repo`:

```bash
skillshare install anthropics/skills                    # Browse mode
skillshare install anthropics/skills/skills/pdf         # Direct install
skillshare install ComposioHQ/awesome-claude-skills     # Another repo
```

## Discovery Mode (Browse Skills)

When you don't specify a path, skillshare lists all available skills:

```bash
skillshare install anthropics/skills
```

<p>
  <img src="/img/install-demo.png" alt="install demo" width="720" />
</p>

**Tip**: Use `--dry-run` to preview without installing:
```bash
skillshare install anthropics/skills --dry-run
```

## Direct Install (Specific Path)

Provide the full path to install immediately:

```bash
# GitHub with subdirectory
skillshare install anthropics/skills/skills/pdf
skillshare install google-gemini/gemini-cli/packages/core/src/skills/builtin/skill-creator

# Full URL
skillshare install github.com/user/repo/path/to/skill

# SSH URL
skillshare install git@github.com:user/repo.git

# Local path
skillshare install ~/Downloads/my-skill
skillshare install /absolute/path/to/skill
```

## Options

| Flag | Short | Description |
|------|-------|-------------|
| `--name <name>` | | Custom name for the skill |
| `--force` | `-f` | Overwrite existing skill |
| `--update` | `-u` | Update if exists (git pull or reinstall) |
| `--track` | `-t` | Keep `.git` for tracked repos |
| `--dry-run` | `-n` | Preview only |

## Common Scenarios

**Install with custom name:**
```bash
skillshare install google-gemini/gemini-cli/.../skill-creator --name my-creator
# Installed as: ~/.config/skillshare/skills/my-creator/
```

**Force overwrite existing:**
```bash
skillshare install ~/my-skill --force
```

**Update existing skill:**
```bash
# By skill name (uses stored source)
skillshare install pdf --update

# By source URL
skillshare install anthropics/skills/skills/pdf --update
```

**Install team repo (tracked):**
```bash
skillshare install anthropics/skills --track
```

<p>
  <img src="/img/team-reack-demo.png" alt="tracked repo install demo" width="720" />
</p>

## After Installing

Always sync to distribute to targets:

```bash
skillshare install anthropics/skills/skills/pdf
skillshare sync  # ← Don't forget!
```

---

## list

View installed skills and their sources.

```bash
skillshare list              # Basic list
skillshare list --verbose    # Detailed info
```

<p>
  <img src="/img/list-demo.png" alt="list demo" width="720" />
</p>

---

## update

Update skills or tracked repositories.

### Update Regular Skills

```bash
skillshare update pdf              # Update from stored source
skillshare update pdf --dry-run    # Preview
```

<p>
  <img src="/img/update-skilk-demo.png" alt="update skill demo" width="720" />
</p>

**How it works:**
```
┌─────────────────────────────────────────┐
│ skillshare update pdf                   │
└─────────────────────────────────────────┘
         │
         ▼
┌─────────────────────────────────────────┐
│ 1. Read stored source from metadata     │
│    → github.com/anthropics/skills/pdf   │
└─────────────────────────────────────────┘
         │
         ▼
┌─────────────────────────────────────────┐
│ 2. Re-download from source              │
└─────────────────────────────────────────┘
         │
         ▼
┌─────────────────────────────────────────┐
│ 3. Replace skill in source directory    │
└─────────────────────────────────────────┘
```

### Update Tracked Repos

```bash
skillshare update _team-repo       # git pull specific repo
skillshare update team-repo        # _ prefix optional
skillshare update --all            # Update all tracked repos + skills with metadata
```

**How it works:**
```
┌─────────────────────────────────────────┐
│ skillshare update team-repo            │
└─────────────────────────────────────────┘
         │
         ▼
┌───────────────────────────────────────────┐
│ cd ~/.config/skillshare/skills/_team-repo │
│ git pull                                  │
└───────────────────────────────────────────┘
```

**Safety check:** If the repo has uncommitted changes, update is blocked:
```bash
skillshare update team-repo
# → "Repository has uncommitted changes"
# → Shows modified files
# → Requires --force to discard and update

skillshare update team-repo --force   # Discards local changes, then pulls
skillshare update --all               # Update all (skips dirty repos)
skillshare update --all --force       # Discards local changes, updates all
```

### After Updating

```bash
skillshare update team-repo
skillshare sync  # ← Distribute changes to targets
```

---

## upgrade

Upgrade the skillshare CLI binary and/or built-in skill.

```bash
skillshare upgrade              # Both CLI + skill
skillshare upgrade --cli        # CLI only
skillshare upgrade --skill      # Skill only
skillshare upgrade --force      # Skip confirmation
skillshare upgrade --dry-run    # Preview
```

<p>
  <img src="/img/upgrade-demo.png" alt="upgrade demo" width="720" />
</p>

**What gets upgraded:**

| Component | Location | What happens |
|-----------|----------|--------------|
| CLI | `/usr/local/bin/skillshare` | Downloads latest binary |
| Skill | `~/.config/skillshare/skills/skillshare/` | Downloads latest skill files |

**After upgrading skill:**
```bash
skillshare upgrade --skill
skillshare sync  # ← Update skill in all targets
```

**Troubleshooting:**

If CLI upgrade fails (e.g., GitHub API rate limit), use `--force`:
```bash
skillshare upgrade --cli --force
```

To avoid rate limits, set a GitHub token or login to `gh` cli:
```bash
export GITHUB_TOKEN=ghp_your_token_here
skillshare upgrade
```

---

## uninstall

Remove a skill from source.

```bash
skillshare uninstall pdf              # With confirmation
skillshare uninstall pdf --force      # Skip confirmation
skillshare uninstall pdf --dry-run    # Preview
```

<p>
  <img src="/img/uninstall-demo.png" alt="uninstall demo" width="720" />
</p>

**For tracked repos:**
```bash
skillshare uninstall team-repo       # Checks for uncommitted changes
skillshare uninstall team-repo -f    # Force (ignore uncommitted)
```

### After Uninstalling

```bash
skillshare uninstall pdf
skillshare sync  # ← Remove from all targets
```

---

## Related

- [sync](/docs/commands/sync) — Sync skills to targets
- [team-edition](/docs/guides/team-edition) — Tracked repos and team sharing
- [faq](/docs/faq) — Troubleshooting
