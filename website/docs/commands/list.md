---
sidebar_position: 4
---

# list

List all installed skills in the source directory.

```bash
skillshare list              # Compact view
skillshare list --verbose    # Detailed view
```

![list demo](/img/list-demo.png)

## Example Output

### Compact View

```
Installed skills
  → my-skill              local
  → commit-commands       github.com/user/skills
  → _team-skills:review   tracked: _team-skills

Tracked repositories
  ✓ _team-skills          3 skills, up-to-date
```

### Verbose View

```bash
skillshare list --verbose
```

```
Installed skills
  my-skill
    Source:      (local - no metadata)

  commit-commands
    Source:      github.com/user/skills
    Type:        github
    Installed:   2026-01-15

  _team-skills:review
    Tracked repo: _team-skills
    Source:      github.com/team/skills
    Type:        github
    Installed:   2026-01-10

Tracked repositories
  ✓ _team-skills          3 skills, up-to-date
  ! _other-repo           5 skills, has changes
```

## Options

| Flag | Description |
|------|-------------|
| `--verbose, -v` | Show detailed information (source, type, install date) |
| `--help, -h` | Show help |

## Understanding the Output

### Skill Sources

| Label | Meaning |
|-------|---------|
| `local` | Created locally, no metadata |
| `github.com/...` | Installed from GitHub |
| `tracked: <repo>` | Part of a tracked repository |

### Repository Status

| Icon | Meaning |
|------|---------|
| `✓` | Up-to-date, no local changes |
| `!` | Has uncommitted changes |

## Related

- [install](/docs/commands/install) — Install skills
- [uninstall](/docs/commands/uninstall) — Remove skills
- [status](/docs/commands/status) — Show sync status
