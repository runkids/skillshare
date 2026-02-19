---
sidebar_position: 2
---

# Troubleshooting Workflow

Systematic approach to diagnosing and fixing issues.

## Overview

```mermaid
flowchart LR
    DIAGNOSE["DIAGNOSE"] --> IDENTIFY["IDENTIFY"] --> FIX["FIX"] --> VERIFY["VERIFY"]
```

---

## Step 1: Diagnose

Run the doctor command:

```bash
skillshare doctor
```

**What it checks:**
- Source directory exists and is valid
- Config file is properly formatted
- All targets are accessible
- Symlinks are not broken
- Git repository status (if initialized)
- Skill format validity

---

## Step 2: Identify the Issue

### Common symptoms and causes

| Symptom | Likely Cause | Quick Fix |
|---------|--------------|-----------|
| Skill not showing in AI CLI | Not synced | `skillshare sync` |
| Symlink broken | Source deleted | Restore or reinstall |
| Config errors | Invalid YAML | `skillshare doctor` shows details |
| Can't push/pull | Git issues | Check git status manually |
| Permission denied | Wrong ownership | Check file permissions |

---

## Step 3: Fix

### Sync issues

```bash
# Re-sync all targets
skillshare sync

# Force sync (recreate symlinks)
skillshare sync --force
```

### Broken symlinks

```bash
# Check status
skillshare status

# Sync to recreate
skillshare sync
```

### Config issues

```bash
# View current config
cat ~/.config/skillshare/config.yaml

# Reset config
rm ~/.config/skillshare/config.yaml
skillshare init
```

### Git issues

```bash
cd ~/.config/skillshare/skills

# Check status
git status

# Pull fails (local changes)
git stash
git pull
git stash pop

# Push fails (remote ahead)
git pull
git push
```

### Target issues

```bash
# Remove and re-add
skillshare target remove claude
skillshare target add claude ~/.claude/skills
skillshare sync
```

---

## Step 4: Verify

```bash
# Check status
skillshare status

# Run doctor again
skillshare doctor

# Test in AI CLI
# (invoke a skill)
```

---

## Recovery Options

### Light recovery

```bash
# Just resync
skillshare sync
```

### Medium recovery

```bash
# Restore from backup
skillshare restore claude
skillshare sync
```

### Heavy recovery (start fresh)

```bash
# Backup current state
skillshare backup

# Remove config (preserves skills)
rm ~/.config/skillshare/config.yaml

# Re-initialize
skillshare init

# Sync
skillshare sync
```

---

## Getting Help

If you can't resolve the issue:

1. **Gather information:**
   ```bash
   skillshare doctor > doctor-output.txt
   skillshare status >> doctor-output.txt
   ```

2. **Check the FAQ:** [Common Errors](/docs/troubleshooting/common-errors)

3. **Report the issue:** [GitHub Issues](https://github.com/runkids/skillshare/issues)
   - Include doctor output
   - Include error messages
   - Describe what you were trying to do

---

## Related

- [Common Errors](/docs/troubleshooting/common-errors) — Error messages and solutions
- [Windows Issues](/docs/troubleshooting/windows) — Windows-specific problems
- [FAQ](/docs/troubleshooting/faq) — Frequently asked questions
- [Commands: doctor](/docs/commands/doctor) — Doctor command
