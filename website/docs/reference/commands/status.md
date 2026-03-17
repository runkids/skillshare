---
sidebar_position: 7
---

# status

Show the current state of skillshare: source, tracked repositories, targets, and versions.

```bash
skillshare status
```

## When to Use

- Check if all targets are in sync after making changes
- See which targets need a `sync` run
- Verify tracked repos are up to date
- Verify the active audit policy (profile, threshold, dedupe mode)
- Check for CLI or skill updates

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
  ✓ copilot   [copy] ~/.copilot/skills (3 managed, 0 local)
  ! windsurf  [merge->needs sync] ~/.windsurf/skills
  ⚠ 2 skill(s) not synced — run 'skillshare sync'

Audit
  Profile:    DEFAULT
  Block:      severity >= CRITICAL
  Dedupe:     GLOBAL
  Analyzers:  ALL

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
- **Sync mode**: `merge`, `copy`, or `symlink`
- **Path**: Target directory location
- **Status**: `merged`, `linked`, `unlinked`, or `needs sync`
- **Shared/local counts**: In merge and copy modes, counts use that target's expected set (after `include`/`exclude` filters). Copy mode shows "managed" instead of "shared".

If a target is in symlink mode, `include`/`exclude` is ignored.

| Status | Meaning |
|--------|---------|
| `merged` | Skills are symlinked individually |
| `copied` | Skills are copied as real files (with manifest) |
| `linked` | Entire directory is symlinked |
| `unlinked` | Not yet synced |
| `needs sync` | Mode changed, run `sync` to apply |
| `not synced` | Some expected skills (after filters) are missing — run `sync` |

### Extras

When extras are configured, shows each extra's sync status:

```
Extras
  ✓ rules       2 files → ~/.claude/rules (merge)
  ✓ commands    1 file  → ~/.cursor/commands (copy)
```

Each entry shows the file count, target path, and sync mode.

### Audit

Shows the active audit policy configuration (resolved from CLI flags, project config, or global config):

- **Profile**: `DEFAULT`, `STRICT`, or `PERMISSIVE`
- **Block**: severity threshold for blocking (`CRITICAL` by default)
- **Dedupe**: deduplication mode (`GLOBAL` or `LEGACY`)
- **Analyzers**: enabled analyzers (`ALL` or a filtered list)

```
Audit
  Profile:    DEFAULT
  Block:      severity >= CRITICAL
  Dedupe:     GLOBAL
  Analyzers:  ALL
```

### Version

Compares your CLI and skill versions against the latest releases.

## Options

| Flag | Description |
|------|-------------|
| `--json` | Output as JSON (for scripting/CI; global mode only) |
| `--project, -p` | Use project mode |
| `--global, -g` | Use global mode |
| `--help, -h` | Show help |

## JSON Output

```bash
skillshare status --json
```

```json
{
  "source": {
    "path": "~/.config/skillshare/skills",
    "exists": true,
    "skillignore": {
      "active": true,
      "files": [".skillignore", "_team-skills/.skillignore"],
      "patterns": ["test-*", "vendor/"],
      "ignored_count": 2,
      "ignored_skills": ["test-draft", "vendor/lib"]
    }
  },
  "skill_count": 12,
  "tracked_repos": [
    {"name": "_team-skills", "skill_count": 5, "dirty": false},
    {"name": "_personal-repo", "skill_count": 3, "dirty": true}
  ],
  "targets": [
    {
      "name": "claude",
      "path": "~/.claude/skills",
      "mode": "merge",
      "status": "merged",
      "synced_count": 8,
      "include": [],
      "exclude": []
    }
  ],
  "audit": {
    "profile": "DEFAULT",
    "threshold": "CRITICAL",
    "dedupe": "GLOBAL",
    "analyzers": []
  },
  "version": "0.16.12"
}
```

The `source.skillignore` field is present only when at least one `.skillignore` file exists. When absent: `"skillignore": { "active": false }`.

:::note
`--json` is only supported in global mode. In project mode, it returns an error.
:::

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

## See Also

- [sync](/docs/reference/commands/sync) — Sync skills to targets
- [diff](/docs/reference/commands/diff) — Show detailed differences
- [doctor](/docs/reference/commands/doctor) — Diagnose issues
- [Project Skills](/docs/understand/project-skills) — Project mode concepts
