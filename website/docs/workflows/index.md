---
sidebar_position: 1
---

# Workflows

Common usage patterns for skillshare.

## Choose Your Workflow

| I want to... | Workflow |
|-------------|----------|
| Use skills day-to-day | [Daily Workflow](./daily-workflow) |
| Find and install new skills | [Skill Discovery](./skill-discovery) |
| Protect my skills | [Backup & Restore](./backup-restore) |
| Fix something broken | [Troubleshooting Workflow](./troubleshooting-workflow) |

---

## Quick Reference

### Daily cycle
```bash
# Edit skills (in source or any target)
$EDITOR ~/.config/skillshare/skills/my-skill/SKILL.md

# Sync to all targets (if needed)
skillshare sync

# Push to remote (if using cross-machine sync)
skillshare push -m "Update my-skill"
```

### Discovery cycle
```bash
# Search for skills
skillshare search pdf

# Browse a repository
skillshare install anthropics/skills

# Install and sync
skillshare install anthropics/skills/skills/pdf
skillshare sync
```

### Safety cycle
```bash
# Before risky changes
skillshare backup

# If something breaks
skillshare restore claude

# Check health
skillshare doctor
```
