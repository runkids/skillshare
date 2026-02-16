---
sidebar_position: 7
---

# status

Show the current state of skillshare: source, tracked repositories, targets, and versions.

```bash
skillshare status
```

![status demo](/img/status-demo.png)

## Example Output

```
Source
  ✓ ~/.config/skillshare/skills (12 skills, 2026-01-20 15:30)

Tracked Repositories
  ✓ _team-skills          5 skills, up-to-date
  ! _personal-repo        3 skills, has uncommitted changes

Targets
  ✓ claude    [merge] ~/.claude/skills (8 shared, 2 local)
  ✓ cursor    [merge] ~/.cursor/skills (3 shared, 0 local)
  ✓ codex     [merge] ~/.codex/skills (3 shared, 0 local)
  ! windsurf  [merge->needs sync] ~/.windsurf/skills
  ⚠ 2 skill(s) not synced — run 'skillshare sync'

Version
  ✓ CLI: 1.2.0
  ✓ Skill: 1.1.0 (up to date)
```

## Sections

### Source

Shows the source directory location, skill count, and last modified time.

### Tracked Repositories

Lists git repositories installed with `--track`. Shows:
- Skill count per repository
- Git status (up-to-date or has changes)

### Targets

Shows each configured target with:
- **Sync mode**: `merge` or `symlink`
- **Path**: Target directory location
- **Status**: `merged`, `linked`, `unlinked`, or `needs sync`
- **Shared/local counts**: In merge mode, counts use that target's expected set (after `include`/`exclude` filters)

If a target is in symlink mode, `include`/`exclude` is ignored.

| Status | Meaning |
|--------|---------|
| `merged` | Skills are symlinked individually |
| `linked` | Entire directory is symlinked |
| `unlinked` | Not yet synced |
| `needs sync` | Mode changed, run `sync` to apply |
| `not synced` | Some expected skills (after filters) are missing — run `sync` |

### Version

Compares your CLI and skill versions against the latest releases.

## Project Mode

In a project directory, status shows project-specific information:

```bash
skillshare status        # Auto-detected if .skillshare/ exists
skillshare status -p     # Explicit project mode
```

### Example Output

```
Project Skills (.skillshare/)

Source
  ✓ .skillshare/skills (3 skills)

Targets
  ✓ claude       [merge] .claude/skills (3 synced)
  ✓ cursor       [merge] .cursor/skills (3 synced)

Remote Skills
  ✓ pdf          anthropic/skills/pdf
  ✓ review       github.com/team/tools
```

Project status does not show Tracked Repositories or Version sections (these are global-only features).

## Related

- [sync](/docs/commands/sync) — Sync skills to targets
- [diff](/docs/commands/diff) — Show detailed differences
- [doctor](/docs/commands/doctor) — Diagnose issues
- [Project Skills](/docs/concepts/project-skills) — Project mode concepts
