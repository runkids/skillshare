# Install, Update, Uninstall & New

All commands support project mode with `-p` flag. Auto-detected for `install -p` (when config lists remote skills).

## install

Install skills from local path or git repository.

### Source Formats

```bash
# GitHub shorthand
user/repo                     # Browse repo for skills
user/repo/path/to/skill       # Direct path

# GitLab / Bitbucket / other hosts
gitlab.com/user/repo          # GitLab shorthand
bitbucket.org/team/skills     # Bitbucket shorthand
git.company.com/team/skills   # Self-hosted

# Full URLs
github.com/user/repo          # Discovers skills in repo
github.com/user/repo/path     # Direct subdirectory
https://github.com/...        # HTTPS URL
git@github.com:...            # SSH URL
git@host:owner/repo//subdir   # SSH with subpath (// separator)

# Local
~/path/to/skill               # Local directory
```

### Examples

```bash
# Global
skillshare install anthropics/skills              # Browse official skills
skillshare install anthropics/skills/skills/pdf   # Direct install
skillshare install ~/Downloads/my-skill           # Local
skillshare install github.com/team/repo --track   # Team repo

# Project
skillshare install anthropics/skills/skills/pdf -p    # Install to .skillshare/skills/
skillshare install github.com/team/repo --track -p    # Track in project
skillshare install -p                                 # Install all remote skills from config

# Selective install (non-interactive)
skillshare install anthropics/skills -s pdf,commit    # Specific skills
skillshare install anthropics/skills --all            # All skills
skillshare install anthropics/skills -y               # Auto-accept
skillshare install anthropics/skills -s pdf -p        # Selective + project mode
```

### Flags

| Flag | Description |
|------|-------------|
| `-p, --project` | Install to project source |
| `--name <n>` | Override skill name |
| `--force, -f` | Overwrite existing |
| `--update, -u` | Update if exists |
| `--track, -t` | Track for updates (preserves .git) |
| `--skill, -s <names>` | Select specific skills from multi-skill repo (comma-separated) |
| `--all` | Install all discovered skills without prompting |
| `--yes, -y` | Auto-accept all prompts (CI/CD friendly) |
| `--dry-run, -n` | Preview |

**Tracked repos:** Prefixed with `_`, nested with `__` (e.g., `_team__frontend__ui`).

**Project `install -p` (no source):** Installs all remote skills listed in `.skillshare/config.yaml`. Useful for new team members.

**Security audit:** Install auto-scans skills after download. CRITICAL findings block install — use `--force` to override. HIGH/MEDIUM shown as warnings.

**After install:** `skillshare sync`

## check

Check for available updates without applying changes.

```bash
skillshare check             # Show update status for all repos/skills
skillshare check --json      # JSON output (CI-friendly)
skillshare check -p          # Check project skills
```

- **Tracked repos:** Fetches from origin, shows commits behind
- **Remote skills:** Compares installed version with remote HEAD
- **Local skills:** Shown as "local source"

## update

Update installed skills or tracked repositories.

- **Tracked repos (`_repo-name`):** Runs `git pull`
- **Regular skills:** Reinstalls from stored source metadata

```bash
# Global
skillshare update my-skill       # Update from stored source
skillshare update _team-skills   # Git pull tracked repo
skillshare update --all          # All tracked repos + skills
skillshare update --all -n       # Preview updates

# Project
skillshare update my-skill -p       # Update project skill
skillshare update _team-skills -p   # Pull tracked repo in project
skillshare update --all -p          # All project remote/tracked skills
skillshare update _repo --force -p  # Discard local changes
```

**Safety:** Tracked repos with uncommitted changes are skipped. Use `--force` to override.

**After update:** `skillshare sync`

## uninstall

Remove a skill from source. Moves to trash (7-day retention) instead of permanent deletion.

```bash
# Global
skillshare uninstall my-skill          # With confirmation → moves to trash
skillshare uninstall my-skill --force  # Skip confirmation

# Project
skillshare uninstall my-skill -p          # Remove from .skillshare/skills/
skillshare uninstall my-skill --force -p  # Skip confirmation
```

**Undo:** `skillshare trash restore <name>` to recover. See [trash.md](trash.md).

**After uninstall:** `skillshare sync`

## new

Create a new skill template.

```bash
# Global
skillshare new <name>               # Create SKILL.md template

# Project
skillshare new <name> -p            # Create in .skillshare/skills/
skillshare new <name> --dry-run -p  # Preview
```

**After create:** Edit SKILL.md → `skillshare sync`
