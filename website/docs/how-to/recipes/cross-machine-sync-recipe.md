---
sidebar_position: 5
---

# Recipe: Cross-Machine Sync

> Keep skills in sync across multiple machines using git push/pull.

## Scenario

You work on a desktop and a laptop (or home and office machines). You want the same skill library available everywhere without re-running install commands on each machine.

## Solution

### Initial Setup (Machine A)

```bash
# Initialize skillshare
skillshare init

# Install your skills
skillshare install your-org/team-skills
skillshare install another/repo --into tools

# Push source to a git remote
skillshare push
```

`skillshare push` commits your source directory to a git-tracked branch and pushes to the configured remote.

### Setup on New Machine (Machine B)

```bash
# Install skillshare
curl -fsSL https://raw.githubusercontent.com/runkids/skillshare/main/install.sh | sh

# Initialize
skillshare init

# Pull from remote
skillshare pull

# Sync to local targets
skillshare sync
```

### Daily Sync Workflow

On any machine:

```bash
# Pull latest changes from other machines
skillshare pull

# Sync to local AI tools
skillshare sync

# After making changes locally
skillshare push
```

## Verification

- `skillshare push` exits 0 and reports committed changes
- `skillshare pull` on another machine shows received changes
- `skillshare list` shows identical skills on both machines
- `skillshare sync` creates symlinks on the target machine

## Variations

- **Auto-sync on login**: Add `skillshare pull && skillshare sync` to your shell profile (`.bashrc` / `.zshrc`)
- **Conflict resolution**: If two machines modify the same skill, `pull` uses git merge — resolve conflicts in the source directory
- **Selective sync**: Use the `ignore` field in `config.yaml` to exclude machine-specific skills from sync

## Related

- [Cross-machine sync guide](/docs/how-to/sharing/cross-machine-sync)
- [`push` command reference](/docs/reference/commands/push)
- [`pull` command reference](/docs/reference/commands/pull)
