---
sidebar_position: 2
---

# update

Update a skill or tracked repository to the latest version.

```bash
skillshare update my-skill           # Update single skill
skillshare update team-skills        # Update tracked repo
skillshare update --all              # Update everything
```

![update demo](/img/update-skilk-demo.png)

## What Happens

### For Tracked Repositories

```
┌─────────────────────────────────────────────────────────────────┐
│ skillshare update _team-skills                                  │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│ 1. Check for uncommitted changes                                │
│    → "Repository has uncommitted changes"                       │
│    → Use --force to discard and update                          │
└─────────────────────────────────────────────────────────────────┘
                              │ (clean)
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│ 2. Run git pull                                                 │
│    → Fetching from origin...                                    │
│    → 3 commits, 5 files changed                                 │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│ 3. Show changes                                                 │
│    → abc1234  Add new feature                                   │
│    → def5678  Fix bug in parser                                 │
└─────────────────────────────────────────────────────────────────┘
```

### For Regular Skills

```
┌─────────────────────────────────────────────────────────────────┐
│ skillshare update my-skill                                      │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│ 1. Read metadata                                                │
│    → Source: github.com/user/skills                             │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│ 2. Re-install from source                                       │
│    → Cloning repository...                                      │
│    → Updated my-skill                                           │
└─────────────────────────────────────────────────────────────────┘
```

## Options

| Flag | Description |
|------|-------------|
| `--all, -a` | Update all tracked repos and skills with metadata |
| `--force, -f` | Discard local changes and force update |
| `--dry-run, -n` | Preview without making changes |
| `--help, -h` | Show help |

## Update All

Update everything at once:

```bash
skillshare update --all
```

This updates:
1. All tracked repositories (git pull)
2. All skills with source metadata (re-install)

### Example Output

```
┌─────────────────────────────────────────────────────────────────┐
│ skillshare update --all                                         │
│ Updating 2 tracked repos + 3 skills                             │
└─────────────────────────────────────────────────────────────────┘

[1/5] ✓ _team-skills       Already up to date
[2/5] ✓ _personal-repo     3 commits, 2 files
[3/5] ✓ my-skill           Reinstalled from source
[4/5] ! other-skill        has uncommitted changes (use --force)
[5/5] ✓ another-skill      Reinstalled from source

┌────────────────────────────┐
│ Summary                    │
│   Total:    5              │
│   Updated:  4              │
│   Skipped:  1              │
└────────────────────────────┘
```

## Handling Conflicts

If a tracked repo has uncommitted changes:

```bash
# Option 1: Commit your changes first
cd ~/.config/skillshare/skills/_team-skills
git add . && git commit -m "My changes"
skillshare update _team-skills

# Option 2: Discard and force update
skillshare update _team-skills --force
```

## After Updating

Run `skillshare sync` to distribute changes to all targets:

```bash
skillshare update --all
skillshare sync
```

## Related

- [install](/docs/commands/install) — Install skills
- [upgrade](/docs/commands/upgrade) — Upgrade CLI and built-in skill
- [sync](/docs/commands/sync) — Sync to targets
