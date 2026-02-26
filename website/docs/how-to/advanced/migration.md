---
sidebar_position: 5
---

# Migration

Migrate from other skill management approaches to skillshare.

## From Manual Management

If you've been manually copying skills between AI CLIs:

### Step 1: Initialize skillshare

```bash
skillshare init
```

### Step 2: Collect existing skills

```bash
# Collect from each AI CLI
skillshare collect claude
skillshare collect cursor
skillshare collect codex

# Or collect from all at once
skillshare collect --all
```

### Step 3: Handle duplicates

If the same skill exists in multiple places, `collect` warns you. Choose which to keep.

### Step 4: Sync

```bash
skillshare sync
```

Now all targets are symlinked to your single source.

---

## From Other Install Tools

If you've used `npx install-skill` or similar:

### Step 1: Initialize skillshare

```bash
skillshare init
```

### Step 2: Backup existing skills

```bash
skillshare backup
```

### Step 3: Collect or reinstall

**Option A: Collect existing** (keeps current versions)
```bash
skillshare collect --all
```

**Option B: Reinstall from source** (gets latest versions)
```bash
# Find original sources from skill metadata
cat ~/.config/skillshare/skills/pdf/.skillshare-meta.json
# or check the registry
cat ~/.config/skillshare/registry.yaml

# Reinstall
skillshare install anthropics/skills/skills/pdf
```

### Step 4: Sync

```bash
skillshare sync
```

---

## From Git Submodules

If you've been using git submodules:

### Step 1: Export submodule contents

```bash
# In your existing skills repo
git submodule foreach 'cp -r $toplevel/$sm_path ~/temp-skills/$name'
```

### Step 2: Initialize skillshare

```bash
skillshare init
```

### Step 3: Import skills

```bash
# Copy to source
cp -r ~/temp-skills/* ~/.config/skillshare/skills/

# Or install as tracked repos
skillshare install github.com/org/skill-repo --track
```

### Step 4: Sync

```bash
skillshare sync
```

---

## From Committed Project Skills

If your repo already has skills committed in `.claude/skills/`, `.cursor/skills/`, or similar directories:

### Step 1: Initialize project mode

```bash
cd my-project
skillshare init -p
```

### Step 2: Move skills to `.skillshare/skills/`

```bash
# Copy existing skills to skillshare source
cp -r .claude/skills/my-skill .skillshare/skills/
cp -r .claude/skills/api-guide .skillshare/skills/

# Remove originals (sync will recreate as symlinks)
rm -rf .claude/skills/my-skill .claude/skills/api-guide
```

### Step 3: Sync

```bash
skillshare sync
```

Now `.claude/skills/my-skill` is a symlink to `.skillshare/skills/my-skill` — and all other targets (Cursor, Windsurf, etc.) get the same skills automatically.

### Step 4: Commit the migration

```bash
git add .skillshare/ .claude/skills/ .cursor/skills/
git commit -m "Migrate project skills to skillshare"
```

:::tip Multi-Tool Benefit
Before: skills only worked in one AI CLI. After: the same skills are automatically available in every configured target.
:::

---

## From Team-Specific Solutions

If your team has custom skill sharing:

### Step 1: Identify current approach

- Where are skills stored?
- How are they shared?
- How are they updated?

### Step 2: Choose your migration path

**Option A: Global mode** — skills available across all projects on each machine.

```bash
# Create team skills repo
cp -r /current/team/skills ~/new-team-skills
cd ~/new-team-skills && git init && git add . && git commit -m "Migrate to skillshare"
git push origin main

# Team members install globally
skillshare install github.com/org/team-skills --track && skillshare sync
```

**Option B: Project mode** — skills scoped to a specific repo, shared via git.

```bash
cd my-project
skillshare init -p

# Move team skills into project source
cp -r /current/team/skills/* .skillshare/skills/

# Sync and commit
skillshare sync
git add .skillshare/
git commit -m "Add team skills via skillshare"
```

New team members get everything with:
```bash
git clone github.com/org/my-project
cd my-project
skillshare install -p && skillshare sync
```

**Option C: Both** — organization-wide standards globally, project-specific skills per-repo.

```bash
# Organization standards (global)
skillshare install github.com/org/standards --track && skillshare sync

# Project-specific skills (project mode)
cd my-project
skillshare init -p
skillshare install github.com/org/project-skills -p && skillshare sync
```

:::tip Which to Choose?
- **Global**: coding standards, security audits — things every project needs
- **Project**: API conventions, domain rules, deployment guides — things specific to one repo
- **Both**: most teams end up here as they grow
:::

---

## From Global to Project

If you have skills in global mode that belong to a specific project:

### Step 1: Initialize project mode

```bash
cd my-project
skillshare init -p
```

### Step 2: Copy skills from global source

```bash
# Copy specific skills
cp -r ~/.config/skillshare/skills/api-guide .skillshare/skills/
cp -r ~/.config/skillshare/skills/deploy-rules .skillshare/skills/
```

### Step 3: Remove from global (optional)

```bash
skillshare uninstall api-guide
skillshare uninstall deploy-rules
skillshare sync   # Clean up global symlinks
```

### Step 4: Sync and commit

```bash
skillshare sync   # Auto-detects project mode
git add .skillshare/
git commit -m "Move project-specific skills to project mode"
```

After this, the skills are scoped to this repo and shared with the team via git — no longer cluttering your global setup.

---

## Preserving History

If you want to keep git history:

### For personal skills

```bash
# Clone your existing repo to skillshare location
git clone your-existing-repo ~/.config/skillshare/skills

# Initialize skillshare with existing source
skillshare init --source ~/.config/skillshare/skills
```

### For team repos

```bash
# Use --track to preserve .git
skillshare install github.com/team/skills --track
```

---

## Rollback

If migration goes wrong:

### Restore from backup

```bash
skillshare restore claude
skillshare restore cursor
```

### Start fresh

```bash
rm ~/.config/skillshare/config.yaml
skillshare init
```

---

## Checklist

Before migrating:

- [ ] List all current skill locations
- [ ] Identify duplicates
- [ ] Note any custom configurations
- [ ] Create backups

After migrating:

- [ ] Verify all skills appear in `skillshare list`
- [ ] Test skills in each AI CLI
- [ ] Set up git remote (if desired)
- [ ] Share new workflow with team

---

## See Also

- [From Existing Skills](/docs/getting-started/from-existing-skills) — Quick migration path
- [collect](/docs/reference/commands/collect) — Collect from targets
- [Comparison](/docs/understand/philosophy/comparison) — Compare approaches
