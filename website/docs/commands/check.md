---
sidebar_position: 3
---

# check

Check for available updates to tracked repositories and installed skills without applying changes.

```bash
skillshare check             # Check all repos and skills
skillshare check --json      # Machine-readable output
```

## What It Does

`check` inspects your source directory and reports update status for:

1. **Tracked repositories** — Fetches from origin, shows how many commits you're behind
2. **Installed skills (with metadata)** — Compares installed version against the remote HEAD
3. **Local skills** — Marks as "local source" (no remote to compare)

Unlike `update`, `check` never modifies any files.

## Example Output

```
skillshare check

  Tracked Repos
  ─────────────────────────────────────────
  ✓ _team-skills       up to date
  ⬇ _shared-rules      3 commits behind
  ! _design-system     has uncommitted changes

  Installed Skills (remote)
  ─────────────────────────────────────────
  ✓ pdf                up to date          anthropics/skills
  ⬇ commit             update available    anthropics/skills
  • local-skill        local source

  Summary: 1 repo + 1 skill have updates available
  Run 'skillshare update <name>' or 'skillshare update --all'
```

## Options

| Flag | Description |
|------|-------------|
| `--project`, `-p` | Check project-level skills (`.skillshare/`) |
| `--global`, `-g` | Check global skills (`~/.config/skillshare`) |
| `--json` | Output as JSON (for scripting/CI) |
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
    {"name": "_team-skills", "status": "up_to_date", "behind": 0},
    {"name": "_shared-rules", "status": "behind", "behind": 3}
  ],
  "skills": [
    {"name": "pdf", "source": "anthropics/skills", "version": "a1b2c3d",
     "status": "up_to_date", "installed_at": "2024-06-01T10:00:00Z"},
    {"name": "commit", "source": "anthropics/skills", "version": "x9y8z7w",
     "status": "update_available", "installed_at": "2024-05-15T08:30:00Z"},
    {"name": "local-skill", "source": "", "version": "",
     "status": "local", "installed_at": "2024-04-20T12:00:00Z"}
  ]
}
```

## How It Checks

### Tracked Repos

1. Run `git fetch` to get latest refs from origin
2. Compare local HEAD with `origin/<branch>` using `git rev-list --count`
3. Report number of commits behind

### Regular Skills (with metadata)

1. Read `.skillshare-meta.json` for stored version and repo URL
2. Run `git ls-remote <repo_url> HEAD` to get remote HEAD hash
3. Compare with stored version hash

### Local Skills

Skills without metadata or with a local source are shown as "local source" — no remote check is possible.

## Project Mode

```bash
skillshare check -p         # Check project skills
skillshare check -p --json  # JSON output for project
```

## Use Cases

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

## Related

- [update](/docs/commands/update) — Apply updates
- [list](/docs/commands/list) — View installed skills
- [status](/docs/commands/status) — Show sync status
