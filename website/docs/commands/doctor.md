---
sidebar_position: 1
---

# doctor

Check environment and diagnose issues with your skillshare setup.

```bash
skillshare doctor
```

![doctor demo](/img/doctor-demo.png)

## What It Checks

```
┌─────────────────────────────────────────────────────────────────┐
│ skillshare doctor                                               │
└─────────────────────────────────────────────────────────────────┘

Checking environment
  ✓ Config: ~/.config/skillshare/config.yaml
  ✓ Source: ~/.config/skillshare/skills (12 skills)
  ✓ Link support: OK
  ✓ Git: initialized with remote

Checking targets
  ✓ claude    [merge]: merged (8 shared, 2 local)
  ✓ cursor    [merge]: merged (8 shared, 0 local)
  ⚠ codex     [merge]: needs sync

Version
  ✓ CLI: 1.2.0
  ✓ Skill: 1.1.0

Summary
  ✓ All checks passed!
```

## Checks Performed

### Environment

| Check | What It Verifies |
|-------|-----------------|
| Config | Config file exists and is valid |
| Source | Source directory exists and is readable |
| Link support | System can create symlinks |
| Git | Repository status and remote configuration |

### Targets

For each target:
- Path exists and is writable
- Sync mode matches actual state
- No broken symlinks
- No duplicate skills (symlink mode)

### Version

- CLI version
- Skillshare skill version
- Checks for available updates

### Other

- Skills without `SKILL.md` files
- Last backup timestamp
- Broken symlinks in targets

## Common Issues

### "Needs sync"

Target mode was changed but not applied:

```bash
skillshare sync
```

### "Has uncommitted changes"

Tracked repo has local changes:

```bash
cd ~/.config/skillshare/skills/_team-repo
git status
# Commit or discard changes
```

### "Broken symlink"

A skill was removed from source but symlink remains:

```bash
skillshare sync  # Will prune orphaned symlinks
```

### "Skills without SKILL.md"

Skill folders missing required file:

```bash
# Add SKILL.md to each skill, or remove the folder
skillshare new my-skill  # Creates proper structure
```

### "Link not supported"

On Windows without Developer Mode:

1. Enable Developer Mode in Settings
2. Or run as Administrator

## Example Output with Issues

```
Checking environment
  ✓ Config: ~/.config/skillshare/config.yaml
  ✓ Source: ~/.config/skillshare/skills (12 skills)
  ✓ Link support: OK
  ⚠ Git: 3 uncommitted change(s)

Checking targets
  ✓ claude    [merge]: merged (8 shared, 2 local)
  ✗ cursor    [merge]: 2 broken symlink(s): old-skill, removed-skill
  ⚠ codex     [merge->needs sync]: linked (needs sync to apply merge mode)

⚠ Skills without SKILL.md: test-dir, temp

Version
  ✓ CLI: 1.2.0
  ⚠ Skill: 1.0.0 (update available: 1.1.0)
    Run: skillshare upgrade --skill && skillshare sync

Backups: last backup 2026-01-18_09-00-00 (3 days ago)

ℹ Update available: 1.2.0 -> 1.3.0
  brew upgrade skillshare  OR  curl -fsSL .../install.sh | sh

Summary
  ✗ 1 error(s), 4 warning(s)
```

## Related

- [status](/docs/commands/status) — Quick status check
- [sync](/docs/commands/sync) — Fix sync issues
- [upgrade](/docs/commands/upgrade) — Update CLI and skill
