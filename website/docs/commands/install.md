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
│   install ──► source ──► sync ──► targets                    │
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

# Install into a subdirectory (organize by category)
skillshare install ~/my-skill --into frontend
```

## Source Formats

### GitHub Shorthand

Use `owner/repo` format — automatically expands to `github.com/owner/repo`:

```bash
skillshare install anthropics/skills                    # Browse mode
skillshare install anthropics/skills/skills/pdf         # Direct install
skillshare install ComposioHQ/awesome-claude-skills     # Another repo
```

### GitLab / Bitbucket / Other Hosts

Use `domain/owner/repo` format for non-GitHub hosts:

```bash
skillshare install gitlab.com/user/repo                 # GitLab
skillshare install bitbucket.org/team/skills            # Bitbucket
skillshare install git.company.com/team/skills          # Self-hosted
```

Full URLs and SSH also work:

```bash
skillshare install https://gitlab.com/user/repo.git
skillshare install git@gitlab.com:user/repo.git
```

## Discovery Mode (Browse Skills)

When you don't specify a path, skillshare clones the repo, scans for skills, and presents an interactive picker:

```bash
skillshare install anthropics/skills
```

<p>
  <img src="/img/install-demo.png" alt="install demo" width="720" />
</p>

Discovery scans all directories for `SKILL.md` files, skipping only `.git`. This means skills inside hidden directories like `.curated/` or `.system/` are discovered automatically.

**Tip**: Use `--dry-run` to preview without installing:
```bash
skillshare install anthropics/skills --dry-run
```

## Selective Install (Non-Interactive)

Pick specific skills from a multi-skill repo without prompts. The `--skill` flag supports **fuzzy matching** — if an exact name isn't found, it falls back to the closest match:

```bash
# Install specific skills by name (exact or fuzzy)
skillshare install anthropics/skills -s pdf,commit

# Install all discovered skills
skillshare install anthropics/skills --all

# Auto-accept (same as --all for multi-skill repos)
skillshare install anthropics/skills -y

# Combine with other flags
skillshare install anthropics/skills -s pdf --dry-run
skillshare install anthropics/skills --all -p
```

Useful for CI/CD pipelines and scripted workflows.

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

# SSH URL with subdirectory (use // separator)
skillshare install git@github.com:user/repo.git//path/to/skill

# Local path
skillshare install ~/Downloads/my-skill
skillshare install /absolute/path/to/skill
```

## Project Mode

Install skills into a project's `.skillshare/skills/` directory:

```bash
# Install a skill into the project
skillshare install anthropics/skills/skills/pdf -p

# Install into a subdirectory within the project
skillshare install anthropics/skills -s pdf --into tools -p
# → .skillshare/skills/tools/pdf/

# Install all remote skills from config (for new team members)
skillshare install -p
```

### How It Differs

| | Global | Project (`-p`) |
|---|---|---|
| Destination | `~/.config/skillshare/skills/` | `.skillshare/skills/` |
| `--track` | Supported | Supported |
| Config update | None | Adds to `.skillshare/config.yaml` `skills:` |
| No-arg install | Not available | Installs all skills listed in config |

**No-arg install** reads `.skillshare/config.yaml` and installs all listed remote skills — useful for onboarding new team members, open source contributors, or anyone cloning a project with a skill manifest:

```bash
git clone github.com/team/project && cd project
skillshare install -p    # Install all remote skills from config
skillshare sync          # Sync to targets
```

When using no-arg project install (`skillshare install -p`), `--name` is not supported because multiple configured skills may be installed. Skills with `tracked: true` in the config are cloned with full git history (same as `--track`), so `skillshare update` works correctly.

**Tracked repos in project mode** work the same as global — the repo is cloned with `.git` preserved and added to `.skillshare/.gitignore` (which also ignores `.skillshare/logs/` by default). The `tracked: true` flag is auto-recorded in `.skillshare/config.yaml`:

```bash
skillshare install github.com/team/skills --track -p
skillshare sync
```

See [Project Setup](/docs/guides/project-setup) for the full guide.

## Options

| Flag | Short | Description |
|------|-------|-------------|
| `--name <name>` | | Override installed name when exactly one skill is installed |
| `--into <dir>` | | Install into subdirectory (e.g. `--into frontend` or `--into frontend/react`) |
| `--force` | `-f` | Overwrite existing skill; also override audit blocking |
| `--update` | `-u` | Update if exists (git pull or reinstall) |
| `--track` | `-t` | Keep `.git` for tracked repos |
| `--skill` | `-s` | Select specific skills from multi-skill repo (comma-separated) |
| `--all` | | Install all discovered skills without prompting |
| `--yes` | `-y` | Auto-accept all prompts (CI/CD friendly) |
| `--skip-audit` | | Skip security audit for this install |
| `--project` | `-p` | Install into project `.skillshare/skills/` |
| `--dry-run` | `-n` | Preview only |

## Common Scenarios

**Install with custom name:**
```bash
skillshare install google-gemini/gemini-cli/.../skill-creator --name my-creator
# Installed as: ~/.config/skillshare/skills/my-creator/
```

`--name` only works when install resolves to a single skill.

```bash
# ✅ Single skill (works)
skillshare install comeonzhj/Auto-Redbook-Skills --name haha

# ❌ Multiple discovered skills (errors)
skillshare install anthropics/skills --name my-skill
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

**Install into a subdirectory:**
```bash
# Organize by category
skillshare install ~/my-skill --into frontend
# → ~/.config/skillshare/skills/frontend/my-skill/

# Multi-level nesting
skillshare install anthropics/skills -s pdf --into frontend/react
# → ~/.config/skillshare/skills/frontend/react/pdf/

# After sync, target shows flat name: frontend__my-skill, frontend__react__pdf
```

See [Organizing Skills](/docs/guides/organizing-skills) for folder strategies.

**Install team repo (tracked):**
```bash
skillshare install anthropics/skills --track
```

<p>
  <img src="/img/team-reack-demo.png" alt="tracked repo install demo" width="720" />
</p>

## Private Repositories

For private repos, use **SSH URL** format to avoid authentication issues:

```bash
# ✅ SSH URL (recommended for private repos)
skillshare install git@bitbucket.org:team/skills.git
skillshare install git@github.com:team/private-skills.git
skillshare install git@gitlab.com:team/skills.git

# ✅ SSH URL with subdirectory
skillshare install git@bitbucket.org:team/skills.git//frontend-react

# ✅ SSH URL with --track
skillshare install git@bitbucket.org:team/skills.git --track

# ❌ HTTPS URL for private repos will fail
# skillshare install https://bitbucket.org/team/skills
```

HTTPS URLs require interactive authentication which is not supported by skillshare. If you accidentally use an HTTPS URL for a private repo, skillshare will fail fast with an error message suggesting the SSH format.

## Security Scanning

Every skill is automatically scanned for security threats during installation:

- Findings at or above `audit.block_threshold` **block installation** (default: `CRITICAL`)
- Lower findings are shown as warnings and include risk score context
- `audit.block_threshold` only controls block level; it does **not** disable scanning
- There is no config switch to always skip audit; use `--skip-audit` per command when needed

Threshold config example:

```yaml
audit:
  block_threshold: HIGH
```

```bash
# Blocked — critical threat detected
skillshare install evil-skill
# → Installation blocked at active threshold. Use --force to override.

# Force install despite warnings
skillshare install suspicious-skill --force

# Skip scan entirely (use with caution)
skillshare install suspicious-skill --skip-audit
```

Use `--force` to override block decisions, or `--skip-audit` to bypass scanning entirely. See [audit](/docs/commands/audit) for scanning details.

### `--force` vs `--skip-audit`

Both can unblock installation, but they do different things:

| Flag | Audit execution | What happens |
|------|------------------|--------------|
| `--force` | Audit still runs | Findings are still generated/logged; install continues even if threshold is hit |
| `--skip-audit` | Audit is skipped | No scan is performed for this install |

Recommended usage:

- Prefer `--force` when you still want visibility into findings.
- Use `--skip-audit` only when you intentionally need to bypass scanning.
- If both are set, `--skip-audit` takes precedence in practice (scan is skipped).

## After Installing

Always sync to distribute to targets:

```bash
skillshare install anthropics/skills/skills/pdf
skillshare sync  # ← Don't forget!
```

## Related

- [list](/docs/commands/list) — View installed skills
- [update](/docs/commands/update) — Update skills or tracked repos
- [upgrade](/docs/commands/upgrade) — Upgrade CLI and built-in skill
- [uninstall](/docs/commands/uninstall) — Remove skills
- [sync](/docs/commands/sync) — Sync skills to targets
- [Organization-Wide Skills](/docs/guides/organization-sharing) — Organization sharing with tracked repos
