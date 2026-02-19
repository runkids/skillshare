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

## When to Use

- A tracked repository has new commits (found via `check`)
- An installed skill has a newer version available
- You want to re-download a skill from its original source

![update demo](/img/update-skilk-demo.png)

## What Happens

### For Tracked Repositories

```mermaid
flowchart TD
    TITLE["skillshare update _team-skills"]
    S1["1. Check for uncommitted changes"]
    S2["2. Run git pull"]
    S3["3. Show changes"]
    TITLE --> S1 -- clean --> S2 --> S3
```

### For Regular Skills

```mermaid
flowchart TD
    TITLE["skillshare update my-skill"]
    S1["1. Read metadata"]
    S2["2. Re-install from source"]
    TITLE --> S1 --> S2
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

## Project Mode

Update skills and tracked repos in the project:

```bash
skillshare update pdf -p             # Update single skill (reinstall)
skillshare update team-skills -p     # Update tracked repo (git pull)
skillshare update --all -p           # Update everything
skillshare update --all -p --dry-run # Preview
```

### How It Works

| Type | Method | Detected by |
|------|--------|-------------|
| **Tracked repo** (`_repo`) | `git pull` | Has `.git/` directory |
| **Remote skill** (with metadata) | Reinstall from source | Has `.skillshare-meta.json` |
| **Local skill** | Skipped | No metadata |

The `_` prefix is optional — `skillshare update team-skills -p` auto-detects `_team-skills`.

### Handling Conflicts

Tracked repos with uncommitted changes are blocked by default:

```bash
# Option 1: Commit changes first
cd .skillshare/skills/_team-skills
git add . && git commit -m "My changes"
skillshare update team-skills -p

# Option 2: Discard and force update
skillshare update team-skills -p --force
```

### Typical Workflow

```bash
skillshare update --all -p
skillshare sync
git add .skillshare/ && git commit -m "Update remote skills"
```

## See Also

- [install](/docs/commands/install) — Install skills
- [upgrade](/docs/commands/upgrade) — Upgrade CLI and built-in skill
- [sync](/docs/commands/sync) — Sync to targets
- [Project Skills](/docs/concepts/project-skills) — Project mode concepts
