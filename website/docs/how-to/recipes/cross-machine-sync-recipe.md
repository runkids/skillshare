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
```

If the installer prints PATH setup instructions, follow them before running the commands below. If it prints no PATH warning, no extra setup is needed.

```bash
# Choose "Connect my existing skillshare repo" and paste the repo URL.
# init pulls your skills and offers a first sync.
skillshare init
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

# Or do both directions in one step: merge, push, then sync
skillshare push --pull
```

## Verification

- `skillshare push` exits 0 and reports committed changes
- `skillshare pull` on another machine shows received changes
- `skillshare list` shows identical skills on both machines
- `skillshare sync` creates symlinks on the target machine

## Variations

- **Auto-sync on login**: Add `skillshare pull && skillshare sync` to your shell profile (`.bashrc` / `.zshrc`)
- **Conflict resolution**: `pull` merges commits from both machines and resolves `.metadata.json` conflicts on its own. If both machines edited the same skill file, `pull` stops, undoes the merge, and names the file — resolve it with git in the source directory
- **Selective sync**: Use per-target `include` / `exclude` filters in `config.yaml` to control which skills sync to each machine
- **Plugins, MCP and hooks**: These live in `config.yaml`, which `push` / `pull` never carry. See [Plugins, MCP and hooks](/docs/how-to/sharing/cross-machine-sync#plugins-mcp-hooks)

## Related

- [Cross-machine sync guide](/docs/how-to/sharing/cross-machine-sync)
- [`push` command reference](/docs/reference/commands/push)
- [`pull` command reference](/docs/reference/commands/pull)
