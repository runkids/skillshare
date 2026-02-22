---
sidebar_position: 2
---

# update

Update one or more skills or tracked repositories to the latest version.

```bash
skillshare update my-skill           # Update single skill
skillshare update a b c              # Update multiple at once
skillshare update --group frontend   # Update all skills in a group
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
    S3["3. Security audit gate"]
    S4["4. Show changes"]
    TITLE --> S1 -- clean --> S2 --> S3
    S3 -- pass --> S4
    S3 -- "HIGH/CRITICAL" --> RB["Rollback"]

    style RB fill:#ef4444,color:#fff
```

### For Regular Skills

```mermaid
flowchart TD
    TITLE["skillshare update my-skill"]
    S1["1. Read metadata"]
    S2["2. Re-install from source"]
    S3["3. Security audit gate"]
    TITLE --> S1 --> S2 --> S3
```

## Options

| Flag | Description |
|------|-------------|
| `--all, -a` | Update all tracked repos and skills with metadata |
| `--group, -G <name>` | Update all updatable skills in a group (repeatable) |
| `--force, -f` | Discard local changes and force update |
| `--dry-run, -n` | Preview without making changes |
| `--skip-audit` | Skip the post-update security audit gate |
| `--diff` | Show file-level change summary after update |
| `--help, -h` | Show help |

## Update Multiple

Update several skills at once:

```bash
skillshare update skill-a skill-b skill-c
```

Only updatable skills (tracked repos or skills with metadata) are processed. Skills not found are warned but don't cause failure. However, if any skill is **blocked by the security audit gate**, the batch command exits with a non-zero code.

## Update Group

Update all updatable skills within a group directory:

```bash
skillshare update --group frontend        # Update all in frontend/
skillshare update -G frontend -G backend  # Multiple groups
skillshare update x -G backend            # Mix names and groups
```

Local skills (without metadata or `.git`) inside the group are silently skipped.

A positional argument that matches a group directory (not a repo or skill) is auto-expanded:

```bash
skillshare update frontend   # Same as --group frontend
# ℹ 'frontend' is a group — expanding to 3 updatable skill(s)
```

:::note
`--all` cannot be combined with skill names or `--group`.
:::

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

## Security Audit Gate

After pulling updates for tracked repositories, `update` automatically runs a security audit. If **HIGH** or **CRITICAL** findings are detected, the update is rolled back to protect against supply-chain attacks.

### Interactive Mode (TTY)

When findings are detected, you're prompted to decide:

```
  [HIGH] Source repository link detected — may be used for supply-chain redirects (SKILL.md:5)

  Security findings at HIGH or above detected.
  Apply anyway? [y/N]:
```

- **`y`** — Accept the update despite findings
- **`N`** (default) — Roll back to the pre-pull state

### Non-Interactive Mode (CI/CD)

In non-interactive environments, the update is automatically rolled back and the command exits with a non-zero code. This ensures fail-closed behavior in CI pipelines.

```bash
# Bypass the audit gate when you trust the source
skillshare update --all --skip-audit
```

:::caution
`--skip-audit` disables the post-update security scan entirely. Use it only when you trust the source or have an external audit process.
:::

## File Change Summary (`--diff`)

Use `--diff` to see a file-level change summary after each tracked repo update:

```bash
skillshare update team-skills --diff
```

```
┌─ Files Changed ─────────────────────────────┐
│                                             │
│  ~ SKILL.md (+12 -3)                        │
│  + scripts/deploy.sh (+45 -0)               │
│  - old-helper.sh (+0 -22)                   │
│  ~ utils/format.md (+5 -2)                  │
│                                             │
└─────────────────────────────────────────────┘
```

Markers: `+` added, `-` deleted, `~` modified. Shows up to 20 files; additional files are summarized as "... and N more file(s)".

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
skillshare update --all --diff   # Update with file-level change summary
skillshare sync
```

## Project Mode

Update skills and tracked repos in the project:

```bash
skillshare update pdf -p              # Update single skill (reinstall)
skillshare update a b c -p            # Update multiple skills
skillshare update --group frontend -p # Update all in a group
skillshare update team-skills -p      # Update tracked repo (git pull)
skillshare update --all -p            # Update everything
skillshare update --all -p --dry-run  # Preview
skillshare update --all -p --diff     # Update with file change summary
skillshare update --all -p --skip-audit  # Skip security audit gate
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
