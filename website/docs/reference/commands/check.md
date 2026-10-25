---
sidebar_position: 3
---

# check

Check for available updates to tracked repositories and installed skills without applying changes.

```bash
skillshare check                      # Check all repos and skills
skillshare check my-skill             # Check a single skill
skillshare check a b c                # Check multiple skills
skillshare check --group frontend     # Check all skills in frontend/
skillshare check x -G backend         # Mix names and groups
skillshare check --json               # Machine-readable output
skillshare check my-skill --diff      # Show what you changed in a skill since install
```

## When to Use

### Before Updating

Preview what would change before running `update`:

```bash
skillshare check         # See what has updates
skillshare update --all  # Apply updates
skillshare sync          # Distribute changes
```

### CI/CD Pipeline

Check for stale skills in CI:

```bash
result=$(skillshare check --json)
# Parse JSON to detect outdated skills
```

## Exit Status

`check` exits with code `1` if any checked item has status `error` (for example, an unreachable remote or a missing local source). Available updates, stale items, and local modifications alone do not cause failure. With `--json`, stdout still contains the complete results; inspect each item's `status` and `message` for details.

## What It Does

`check` inspects your source directory and reports update status for:

1. **Tracked repositories** — Fetches from origin, shows how many commits you're behind
2. **Installed skills (with metadata)** — Compares installed version against the remote HEAD
3. **Stale skills** — Detects skills whose subdirectory was deleted from the upstream repository
4. **Locally modified skills** — Compares the files of a skill installed from a repository with the files recorded at install time, and reports `"status": "modified"` when you edited, added, or deleted any of them
5. **Local-path installs** — Compares the files at the path you installed from with the files recorded at install time (see [Local-Path Installs](#local-path-installs))
6. **Local skills** — Marks skills without install metadata as "local source" (nothing to compare)
7. **Skill-level `targets` validation** — Warns about unknown target names in SKILL.md `targets` frontmatter fields

Unlike `update`, `check` never modifies any files.

## Local-Path Installs {#local-path-installs}

A skill installed from a directory on disk (`skillshare install /path/to/skill`) records that path and the hash of every file it copied. `check` re-hashes the files at that path and reports:

- **up to date** — the files match what was installed
- **update available** — a file was changed, added, or removed at the source path
- **error** — the source path no longer exists (`local source not found: <path>`)

This is useful for skills that another application ships and updates, such as a skill inside an app bundle:

```bash
skillshare install /Applications/Surge.app/Contents/Resources/Skills/surge

# After the app updates:
skillshare check surge     # → Update available
skillshare update surge    # Re-copies from the path and runs the security audit
skillshare sync
```

Skills installed before file hashes were recorded keep the "local source" status until they are updated or reinstalled. In project mode, a relative path (such as `./vendor/my-skill`) is resolved against the project root.

When the dashboard installs the root of a directory that also contains child skills, it copies only that root's `SKILL.md`. `check` then compares only `SKILL.md`, and `update` re-copies only `SKILL.md`.

## Example Output

```
$ skillshare check
! _shared-rules   3 commits behind
! _design-system  uncommitted changes
! commit          update available · github.com/anthropics/skills
! old-helper      stale — no longer in the upstream repository

! Updates available for 1 repo, 1 skill, 2 up to date, 1 local skipped, 1 stale · 3.4s

Next
  skillshare update --all          pull the updates
  skillshare update --all --prune  remove stale skills
```

## Check Specific Skills

You can check one or more skills by name instead of scanning everything:

```bash
skillshare check my-skill                # Single skill
skillshare check skill-a skill-b         # Multiple skills
```

With `follow_source_links: true`, you can also check a followed Git checkout by its link name, for example `skillshare check _dev-skills`.

Use `--group` / `-G` to check all updatable skills in a group directory:

```bash
skillshare check --group frontend        # All skills under frontend/
skillshare check -G frontend -G backend  # Multiple groups
skillshare check my-skill -G frontend    # Mix names and groups
```

If a positional name matches a group directory (not a repo or skill itself), it is automatically expanded:

```bash
skillshare check frontend               # Auto-detected as group
```

Skills without metadata (local-only) are skipped when expanding groups.

## Options

| Flag | Description |
|------|-------------|
| `--group`, `-G` `<name>` | Check all updatable skills in a group (repeatable) |
| `--project`, `-p` | Check project-level skills (`.skillshare/`) |
| `--global`, `-g` | Check global skills (`~/.config/skillshare`) |
| `--json` | Output as JSON (for scripting/CI) |
| `--diff` | For the named skills, show each file changed since install as a line diff against the installed version. Needs a name or `--group`; cannot be combined with `--json` |
| `--help`, `-h` | Show help |

:::tip Auto-detection
If neither `--project` nor `--global` is specified, skillshare auto-detects: if `.skillshare/config.yaml` exists in the current directory, it defaults to project mode; otherwise global mode.
:::

## JSON Output

```bash
skillshare check --json
```

```json
{
  "tracked_repos": [
    {"name": "_team-skills", "status": "up_to_date", "behind": 0, "branch": "main"},
    {"name": "_shared-rules", "status": "behind", "behind": 3, "branch": "develop"}
  ],
  "skills": [
    {"name": "pdf", "source": "anthropics/skills", "version": "a1b2c3d",
     "status": "up_to_date", "installed_at": "2024-06-01T10:00:00Z"},
    {"name": "commit", "source": "anthropics/skills", "version": "x9y8z7w",
     "status": "update_available", "installed_at": "2024-05-15T08:30:00Z"},
    {"name": "old-helper", "source": "anthropics/skills", "version": "d4e5f6g",
     "status": "stale", "installed_at": "2024-03-10T09:00:00Z"},
    {"name": "my-review", "source": "anthropics/skills", "version": "a1b2c3d",
     "status": "modified", "installed_at": "2024-06-01T10:00:00Z",
     "message": "local changes since install"},
    {"name": "local-skill", "source": "", "version": "",
     "status": "local", "installed_at": "2024-04-20T12:00:00Z"}
  ]
}
```

Skills with `"status": "error"` include a `message` field when the reason is known, for example `"message": "local source not found: /path/to/skill"`.

## Status Indicators

| Icon | Meaning |
|------|---------|
| `✓` | Up to date |
| `⬇` | Update available (tracked repo: commits behind; skill: newer version) |
| `⚠` | Stale — subdirectory deleted or renamed upstream |
| `!` | Has uncommitted changes (tracked repo) or local changes since install (skill) |
| `•` | Local source (no install metadata to compare) |

:::info Stale skills
When a skill's subdirectory was renamed or deleted upstream, `check` reports it as **stale**. Use `update --prune` to clean up stale skills.
:::

:::info Locally modified skills
A skill you edited after installing it is reported as **modified**, even when upstream has a newer version; its `message` then ends with `update available upstream`. `update` leaves it alone until you run `skillshare update <name> --force`, which discards your changes and reinstalls it.

To see the changes first, run `skillshare check <name> --diff`. It fetches the commit the skill was installed at, so the diff shows only your edits even when upstream has moved on:

```
! my-skill  2 files changed since install
            modified SKILL.md
                # My Skill
              - Run the tests.
              + Run the tests with -race.
            added NOTES.md
              + my notes
```
:::

:::tip Monorepo awareness
For skills installed from a subdirectory, `check` only reports "update available" when that specific directory changed — not when unrelated parts of the repo have new commits.
:::

## Project Mode

```bash
skillshare check -p                    # Check all project skills
skillshare check -p my-skill           # Check specific project skill
skillshare check -p --group frontend   # Check project group
skillshare check -p --json             # JSON output for project
```

## Agent Support

`skillshare check agents` scopes the check to agents only, reporting drift and update status for `.md` files in the agents source directory:

```bash
skillshare check agents              # Check all agents
skillshare check agents --json       # JSON output for agents
skillshare check agents -p           # Check project agents
```

Without the `agents` argument, `check` operates on skills only (default behavior). See [Agents](/docs/understand/agents) for background.

## See Also

- [update](/docs/reference/commands/update) — Apply updates
- [list](/docs/reference/commands/list) — View installed skills
- [status](/docs/reference/commands/status) — Show sync status
- [Agents](/docs/understand/agents) — Agent concepts
