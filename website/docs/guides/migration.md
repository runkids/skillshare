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
# Find original sources
cat ~/.claude/skills/pdf/.source  # or similar

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

## From Team-Specific Solutions

If your team has custom skill sharing:

### Step 1: Identify current approach

- Where are skills stored?
- How are they shared?
- How are they updated?

### Step 2: Create team skills repo

```bash
# Export existing skills
cp -r /current/team/skills ~/new-team-skills
cd ~/new-team-skills
git init
git add .
git commit -m "Migrate to skillshare"
git push origin main
```

### Step 3: Team members install

Share the command:
```bash
skillshare install github.com/org/team-skills --track && skillshare sync
```

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

## Related

- [Getting Started](/docs/getting-started/from-existing-skills) — Detailed migration walkthrough
- [Commands: collect](/docs/commands/collect) — Collect command
- [Team Sharing](./team-sharing) — Team migration
