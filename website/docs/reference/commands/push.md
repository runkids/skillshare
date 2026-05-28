---
sidebar_position: 1
---

# push

Commit and push source to git remote.

Use [`commit`](./commit.md) instead when you only want a local checkpoint without pushing.

```bash
skillshare push                  # Auto-generated message
skillshare push -m "Add pdf"     # Custom message
skillshare push --dry-run        # Preview
```

## When to Use

- Share skill changes with your other machines via git
- Back up your skills to a remote repository
- After editing skills, commit and push in one command

If you want to save a local checkpoint without sharing it yet, use [`skillshare commit`](./commit.md).

## What Happens

```mermaid
flowchart TD
    CMD["skillshare push"]
    CHECK["1. Check repository status"]
    STAGE["2. Stage all changes"]
    COMMIT["3. Commit"]
    PUSH["4. Push to remote"]
    CMD --> CHECK --> STAGE --> COMMIT --> PUSH
```

## Options

| Flag | Description |
|------|-------------|
| `-m, --message <msg>` | Commit message (default: "Update skills") |
| `--dry-run, -n` | Preview without making changes |

## Git Root Scope

`push` operates on the directory selected by the `git_root` config field (default: `skills` source). See [commit — Git Root Scope](./commit.md#git-root-scope) for the scope table. If `git_root` was changed but the git repo still lives in another scope's directory, `push` prints a "Git root mismatch" error and asks you to re-run `skillshare init`.

## Prerequisites

Your source directory must be a git repository with a remote:

```bash
# Set up during init (recommended):
skillshare init --remote git@github.com:you/my-skills.git

# Or add remote to existing setup:
skillshare init --remote git@github.com:you/my-skills.git
```

Init automatically creates the initial commit, so `push` works immediately after setup.

## First Push Upstream Mapping

On first push (no upstream tracking yet), `skillshare push` auto-configures upstream:

- If remote already has a default branch (for example `main` or `trunk`), local changes are pushed to that remote default branch.
- If remote is empty, it pushes to your current local branch.

This avoids accidentally creating the wrong remote branch (for example local `master` while remote uses `main`).

## Examples

```bash
# Quick push with auto message
skillshare push

# Custom commit message
skillshare push -m "Add commit-commands skill"

# Preview what would be pushed
skillshare push --dry-run
```

## Conflict Handling

If the remote has newer commits:

```bash
$ skillshare push
Push failed
  Remote may have newer changes
  Run: skillshare pull
  Then: skillshare push
```

Solution:
```bash
skillshare pull    # Get remote changes
skillshare push    # Push your changes
```

## Workflow

Typical workflow for sharing skills:

```bash
# 1. Make changes to skills
# 2. Push to remote
skillshare push -m "Update my-skill"

# On another machine:
skillshare pull    # Gets changes and syncs
```

## See Also

- [commit](/docs/reference/commands/commit) — Commit locally without pushing
- [pull](/docs/reference/commands/pull) — Pull from remote
- [sync](/docs/reference/commands/sync) — Sync to local targets
- [Cross-Machine Sync](/docs/how-to/sharing/cross-machine-sync) — Full setup
