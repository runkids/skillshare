# Install, Update, Uninstall & New

All commands support project mode with `-p` flag. Auto-detected for `install -p` (when config lists remote skills).

## install

Install skills from local path or git repository.

### Source Formats

```bash
# GitHub shorthand
user/repo                     # Browse repo for skills
user/repo/path/to/skill       # Direct path
user/repo/skill-name          # Fuzzy resolve (finds nested skill by name)

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
skillshare install                                # Install all remote skills from config

# Project
skillshare install anthropics/skills/skills/pdf -p    # Install to .skillshare/skills/
skillshare install github.com/team/repo --track -p    # Track in project
skillshare install -p                                 # Install all remote skills from config

# Organize into subdirectories
skillshare install anthropics/skills --into frontend  # → skills/frontend/
skillshare install user/repo --into tools -p          # → .skillshare/skills/tools/

# Selective install (non-interactive)
skillshare install anthropics/skills -s pdf,commit    # Specific skills
skillshare install anthropics/skills --all            # All skills
skillshare install anthropics/skills -y               # Auto-accept
skillshare install anthropics/skills -s pdf -p        # Selective + project mode
skillshare install user/repo --skip-audit             # Skip security scan
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
| `--into <dir>` | Install into subdirectory (e.g., `--into frontend`) |
| `--all` | Install all discovered skills without prompting |
| `--yes, -y` | Auto-accept all prompts (CI/CD friendly) |
| `--exclude <name>` | Skip specific skills during multi-skill install (repeatable) |
| `--skip-audit` | Skip security audit for this install |
| `--dry-run, -n` | Preview |

**Fuzzy subdirectory resolution:** When a monorepo has nested skill directories, you can specify just the skill name — e.g., `user/repo/vue-best-practices` finds `skills/vue-best-practices/` automatically. Fails with an error if multiple matches exist.

**Tracked repos:** Prefixed with `_`, nested with `__` (e.g., `_team__frontend__ui`).

**No-arg install:** `skillshare install` (global) or `skillshare install -p` (project) installs all remote skills listed in `config.yaml`. Useful for new machines, new team members, or reproducing a skill setup from a shared config.

**`.skillignore`:** Repo authors can add a `.skillignore` file at the repo root to hide skills from discovery. Supports exact match (`my-skill`), trailing wildcard (`prefix-*`), and group match (`feature-radar` excludes all skills under that directory). Applied before any selection prompt.

**`--exclude`:** Skip specific skills during multi-skill install. Filters before the interactive prompt so excluded skills never appear. Example: `skillshare install user/repo --exclude debug --exclude experimental`.

**License display:** If a SKILL.md has a `license` frontmatter field, it's shown in selection prompts (e.g., `my-skill (MIT)`) and in the single-skill confirmation box.

**Security audit:** Install auto-scans skills after download. CRITICAL findings block install — use `--force` to override, `--skip-audit` to skip entirely. HIGH/MEDIUM shown as warnings.

**After install:** `skillshare sync`

## check

Check for available updates and validate skill metadata.

```bash
skillshare check             # Show update status for all repos/skills
skillshare check --json      # JSON output (CI-friendly)
skillshare check -p          # Check project skills
```

- **Tracked repos:** Fetches from origin, shows commits behind
- **Remote skills:** Compares installed version with remote HEAD
- **Local skills:** Shown as "local source"
- **Targets validation:** Warns about unknown target names in skill-level `targets` field

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

Remove one or more skills from source. Moves to trash (7-day retention) instead of permanent deletion.

```bash
# Single skill
skillshare uninstall my-skill              # With confirmation → moves to trash
skillshare uninstall my-skill --force      # Skip confirmation

# Multiple skills
skillshare uninstall a b c --force         # Batch removal

# Group removal (prefix match)
skillshare uninstall --group frontend      # Remove all skills under frontend/
skillshare uninstall -G frontend -G backend --force  # Multiple groups
skillshare uninstall my-skill -G frontend --force    # Mix names and groups

# Project
skillshare uninstall my-skill -p
skillshare uninstall -G frontend -p --force

# Preview
skillshare uninstall --group frontend --dry-run
```

**Group auto-detection:** When uninstalling a directory that contains sub-skills, the confirmation prompt shows `Uninstalling group (N skills)` with a list of contained skills.

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
