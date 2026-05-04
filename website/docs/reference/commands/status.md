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
- See whether plugin and hook bundles are present in the current source root

![status demo](/img/status-demo.png)

## Example Output

```
Source
✓ ~/.config/skillshare/skills (12 skills, 2026-01-20 15:30)
→ .skillignore: 5 patterns, 2 skills ignored
✓ ~/.config/skillshare/agents (8 agents, 2026-01-20 15:30)

Tracked Repositories
_team-skills    ✓  5 skills, up-to-date
_personal-repo  !  3 skills, has uncommitted changes

Targets
claude
  skills   merged       [merge] ~/.claude/skills (8 shared, 2 local)
  agents   merged       [merge] 8/8 linked
cursor
  skills   merged       [merge] ~/.cursor/skills (3 shared, 0 local)
  agents   merged       [merge] 8/8 linked
windsurf
  skills   has files    [merge->needs sync] ~/.windsurf/skills
⚠ 2 skill(s) not synced — run 'skillshare sync'

Extras
rules        has files  [merge] .cursor/rules (4 files)
commands     has files  [merge] .claude/commands (3 files)

Plugins
demo         plugin      claude=true codex=true

Hooks
audit        hook        claude=2 codex=1

Audit
→ Profile:    DEFAULT
→ Block:      severity >= CRITICAL
→ Dedupe:     GLOBAL
→ Analyzers:  ALL

Version
✓ CLI: 0.17.0
✓ Skill: 0.17.0 (up to date)
```

## Sections

### Source

Shows the source directory location, skill count, and last modified time. When agents are configured, the agents source is shown on a separate line.

### Tracked Repositories

Lists git repositories installed with `--track`. Shows:
- Skill count per repository
- Git status (up-to-date or has changes)

### Targets

Each target is shown as a header with sub-items for **skills** and **agents**:

```
claude
  skills   merged       [merge] ~/.claude/skills (8 shared, 2 local)
  agents   merged       [merge] 8/8 linked
```

**Skills sub-item** shows:
- **Sync mode**: `merge`, `copy`, or `symlink`
- **Path**: Target directory location
- **Status**: `merged`, `copied`, `linked`, `has files`, or `needs sync`
- **Shared/local counts**: In merge and copy modes, counts use that target's expected set (after `include`/`exclude` filters). Copy mode shows "managed" instead of "shared".

**Agents sub-item** shows:
- **Status**: `synced` or `drift`
- **Linked count**: e.g. `8/8 linked`

If agents source does not exist or the target has no agent path configured, the agents sub-item is omitted.

| Status | Meaning |
|--------|---------|
| `merged` | Skills/agents are symlinked individually |
| `copied` | Skills are copied as real files (with manifest) |
| `linked` | Entire directory is symlinked |
| `has files` | Not yet synced |
| `needs sync` | Mode changed, run `sync` to apply |
| `drift` | Some agents are missing — run `sync agents` |

### Extras

When extras are configured, shows each extra's sync status:

```
Extras
rules        has files  [merge] .cursor/rules (4 files)
commands     has files  [merge] .claude/commands (3 files)
```

Each entry shows the name, status, sync mode, target path, and file count.

### Plugins

When plugin bundles exist in the current source root, `status` lists each bundle and whether it has Claude and/or Codex manifests available.

### Hooks

When hook bundles exist in the current source root, `status` lists each bundle and the number of hook entries defined for Claude and Codex.

### Audit

Shows the active audit policy configuration (resolved from CLI flags, project config, or global config):

- **Profile**: `DEFAULT`, `STRICT`, or `PERMISSIVE`
- **Block**: severity threshold for blocking (`CRITICAL` by default)
- **Dedupe**: deduplication mode (`GLOBAL` or `LEGACY`)
- **Analyzers**: enabled analyzers (`ALL` or a filtered list)

### Version

Compares your CLI and skill versions against the latest releases. (Global mode only.)

## Options

| Flag | Description |
|------|-------------|
| `--json` | Output as JSON (for scripting/CI) |
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
  "agents": {
    "source": "~/.config/skillshare/agents",
    "exists": true,
    "count": 8,
    "targets": [
      {"name": "claude", "path": "~/.claude/agents", "expected": 8, "linked": 8, "drift": false}
    ]
  },
  "plugins": [
    {
      "name": "demo",
      "source_dir": "/home/user/.config/skillshare/plugins/demo",
      "has_claude": true,
      "has_codex": true
    }
  ],
  "hooks": [
    {
      "name": "audit",
      "source_dir": "/home/user/.config/skillshare/hooks/audit",
      "targets": {
        "claude": 2,
        "codex": 1
      }
    }
  ],
  "audit": {
    "profile": "DEFAULT",
    "threshold": "CRITICAL",
    "dedupe": "GLOBAL",
    "analyzers": []
  },
  "version": "0.17.0"
}
```

The `source.skillignore` field is present only when at least one `.skillignore` or `.skillignore.local` file exists. When absent: `"skillignore": { "active": false }`. The `files` array includes `.skillignore.local` paths when present. In text mode, the source line shows `.local active` when any `.skillignore.local` is in effect.

JSON output is supported in both global and project mode. The top-level `plugins` and `hooks` arrays are omitted when no plugin or hook bundles are discovered. Plugin bundles report generated-target capability, and hook bundles report only the target sections they actually define.

## Project Mode

In a project directory, status shows project-specific information. The first section header shows `Source (project)` to indicate project mode:

```bash
skillshare status        # Auto-detected if .skillshare/ exists
skillshare status -p     # Explicit project mode
```

### Example Output

```
Source (project)
✓ .skillshare/skills/ (3 skills, 2026-04-08 12:43)
→ .skillignore: 3 patterns, 0 skills ignored
✓ .skillshare/agents/ (4 agents, 2026-04-08 12:43)

Targets
claude
  skills   merged       [merge] .claude/skills (3 shared, 0 local)
  agents   merged       [merge] 4/4 linked
cursor
  skills   merged       [merge] .cursor/skills (3 shared, 0 local)
  agents   merged       [merge] 4/4 linked

Extras
rules        has files  [merge] .cursor/rules (4 files)
commands     has files  [merge] .claude/commands (3 files)

Audit
→ Profile:    DEFAULT
→ Block:      severity >= CRITICAL
→ Dedupe:     GLOBAL
→ Analyzers:  ALL
```

Project status does not show Tracked Repositories or Version sections (these are global-only features).

## See Also

- [sync](/docs/reference/commands/sync) — Sync skills to targets
- [diff](/docs/reference/commands/diff) — Show detailed differences
- [doctor](/docs/reference/commands/doctor) — Diagnose issues
- [Project Skills](/docs/understand/project-skills) — Project mode concepts
