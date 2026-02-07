---
sidebar_position: 2
---

# Common Errors

Error messages and their solutions.

## Config Errors

### `config not found: run 'skillshare init' first`

**Cause:** No configuration file exists.

**Solution:**
```bash
skillshare init
```

Add `--source` if you want a custom path:
```bash
skillshare init --source ~/my-skills
```

---

## Target Errors

### `target add: path does not exist`

**Cause:** The skills directory doesn't exist yet.

**Solution:**
```bash
mkdir -p ~/.myapp/skills
skillshare target add myapp ~/.myapp/skills
```

### `target path does not end with 'skills'`

**Cause:** Warning that path doesn't follow convention.

**Solution:** This is a warning, not an error. Proceed if your path is intentional, or fix it:
```bash
skillshare target add myapp ~/.myapp/skills  # Preferred
```

### `target directory already exists with files`

**Cause:** Target has existing files that might be overwritten.

**Solution:**
```bash
skillshare backup
skillshare sync
```

---

## Sync Errors

### `deleting a symlinked target removed source files`

**Cause:** You ran `rm -rf` on a target in symlink mode.

**Solution:**
```bash
# If git is initialized
cd ~/.config/skillshare/skills
git checkout -- .

# Or restore from backup
skillshare restore <target>
```

**Prevention:** Use `skillshare target remove` instead of manual deletion.

### `sync seems stuck or slow`

**Cause:** Large files in skills directory.

**Solution:** Add ignore patterns:
```yaml
# ~/.config/skillshare/config.yaml
ignore:
  - "**/.DS_Store"
  - "**/.git/**"
  - "**/node_modules/**"
```

---

## Git Errors

### `push: remote has changes`

**Cause:** Remote repository is ahead of local.

**Solution:**
```bash
skillshare pull   # Get remote changes first
skillshare push   # Now push works
```

### `pull: local has uncommitted changes`

**Cause:** You have local changes that haven't been pushed.

**Solution:**
```bash
# Option 1: Push your changes first
skillshare push -m "Local changes"
skillshare pull

# Option 2: Discard local changes
cd ~/.config/skillshare/skills
git checkout -- .
skillshare pull
```

### `merge conflicts`

**Cause:** Same file was edited on multiple machines.

**Solution:**
```bash
cd ~/.config/skillshare/skills
git status                    # See conflicted files
# Edit files to resolve conflicts
git add .
git commit -m "Resolve conflicts"
skillshare sync
```

---

## Install Errors

### `skill already exists`

**Cause:** A skill with the same name is already installed.

**Solution:**
```bash
# Update the existing skill
skillshare install <source> --update

# Or force overwrite
skillshare install <source> --force
```

### `authentication required — for private repos use SSH URL`

**Cause:** You're trying to install from a private repo using an HTTPS URL. Skillshare disables interactive credential prompts to prevent hanging.

**Solution:**
```bash
# Use SSH URL instead of HTTPS
skillshare install git@github.com:team/private-skills.git
skillshare install git@bitbucket.org:team/skills.git
skillshare install git@gitlab.com:team/skills.git

# With --track for team repos
skillshare install git@bitbucket.org:team/skills.git --track
```

Make sure your SSH key is configured for the git host. See [Private Repositories](/docs/commands/install#private-repositories).

### `invalid skill: SKILL.md not found`

**Cause:** The source doesn't have a valid SKILL.md file.

**Solution:** Check the source path is correct and points to a skill directory.

---

## Upgrade Errors

### `GitHub API rate limit exceeded`

**Cause:** Too many unauthenticated API requests.

**Solution:**
```bash
# Option 1: Set a GitHub token (recommended)
export GITHUB_TOKEN=ghp_your_token_here
skillshare upgrade

# Option 2: Force upgrade
skillshare upgrade --cli --force
```

Create a token at: https://github.com/settings/tokens (no scopes needed for public repos)

---

## Skill Errors

### `skill not appearing in AI CLI`

**Causes:**
1. Skill not synced
2. Invalid SKILL.md format
3. AI CLI caching

**Solutions:**
```bash
# 1. Sync
skillshare sync

# 2. Check format
skillshare doctor

# 3. Restart AI CLI
```

### `skill name collision detected`

**Cause:** Multiple skills have the same `name` field.

**Solution:** Use namespaced names:
```yaml
# In _team-a/skill/SKILL.md
name: team-a:skill-name

# In _team-b/skill/SKILL.md
name: team-b:skill-name
```

---

## Binary Errors

### `integration tests cannot find the binary`

**Cause:** Binary not built or wrong path.

**Solution:**
```bash
go build -o bin/skillshare ./cmd/skillshare
# Or set
export SKILLSHARE_TEST_BINARY=/path/to/skillshare
```

---

## Still Having Issues?

See [Troubleshooting Workflow](/docs/workflows/troubleshooting-workflow) for a systematic debugging approach.
