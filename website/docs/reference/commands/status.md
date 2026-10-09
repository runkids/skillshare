---
sidebar_position: 7
---

# status

Show the current state of skillshare: source, tracked repositories, targets, and versions.

```bash
skillshare status
```

With `follow_source_links: true`, unavailable first-level source links produce a warning naming the link while healthy skills remain listed. With `--json`, these warnings go to stderr and stdout remains valid JSON.

## When to Use

- Check if all targets are in sync after making changes
- See which targets need a `sync` run
- Verify tracked repos are up to date
- Verify the active audit policy (profile, threshold, dedupe mode)
- Check for CLI or skill updates

## Example Output

```
Source
  skills    ~/.config/skillshare/skills  43 skills
  agents    ~/.config/skillshare/agents  2 agents

Tracked repositories
✓ _superpowers  15 skills

Targets                        skills                 agents
  claude     ~/.claude/skills  ✓ 43 linked            ✓ 2
  cursor     ~/.cursor/skills  ✓ 43 linked · 1 local  ✓ 2
  gemini     ~/.gemini/skills  ✓ 43 linked            —
  universal  ~/.agents/skills  ✓ 43 linked            —
  all use merge

Extras
  rules     ~/.claude/rules     2 files · merge
  rules     ~/.cursor/rules     2 files · merge
  commands  ~/.claude/commands  1 file · merge
  team      ~/.codex            1 file · symlink
  team      ~/.claude           1 file · import

Audit    default · blocks critical
Version  CLI 0.24.0 · skill 0.21.12
! Skill 0.21.13 is available — run skillshare upgrade --skill && skillshare sync
```

## Sections

### Source

Shows the skills folder and how many skills it holds. When an agents folder exists, it is shown on a second line with its agent count. An active `.skillignore` adds a line with its pattern and ignored-skill counts.

### Tracked Repositories

Lists git repositories installed with `--track`, each with its skill count. `✓` means the repository is clean; `!` adds `uncommitted changes`, or the git error when its status cannot be read.

### Targets

Each target is one row: its name, its skills folder, and how its skills and agents stand. A line under the table names the sync modes in use.

```
Targets                     skills                agents
  claude  ~/.claude/skills  ✓ 8 linked · 2 local  ✓ 8
  cursor  ~/.cursor/skills  ! 6/8 copied          ! 7/8
  copy: cursor · merge: claude
! 2 skills not synced — run skillshare sync
```

**skills column:**

| Shows | Meaning |
|-------|---------|
| `✓ 8 linked` / `✓ 8 copied` | Every expected skill is in place. Merge and copy targets count the skills left after `include`/`exclude` |
| `· 2 local` | Skills of your own in that folder, which sync leaves alone |
| `! 6/8 linked` | Some skills are not synced yet; status ends with how many and the `sync` command. Skills that `sync` skips on purpose (an invalid name under `standard` or `prefixed` naming, or a name collision) are not counted, because running `sync` again cannot add them; `sync` names them |
| `✓ symlinked` | Symlink mode: the whole folder links to the source |
| `! needs sync` | The mode changed; run `sync` to apply it |
| `! has files` / `! not synced yet` | The target has never been synced |
| `✗ links to …` / `✗ broken link` | The folder is a link to somewhere else, or to nothing |
| `skills off` | Skills are turned off for this target |

**agents column:** `✓ 8` is the number of linked agents (up-to-date copies count as linked). `! 7/8` means some are missing; run `skillshare sync agents`. Only the agents this target syncs count: those left after `.agentignore`, the target's agents include/exclude and each agent's `targets` frontmatter. In copy fallback, identical local files that skillshare does not own are kept and counted as `· 1 local`. `—` means the target has no agents folder. The column is left out when there is no agents source.

### Extras

When extras are configured, each extra target gets a row:

```
Extras
  rules     .cursor/rules     4 files · merge
  commands  .claude/commands  3 files · merge
```

Each row shows the extra, its target folder, the file count, and the mode files are synced with: on Windows without Developer Mode, a target that links files shows `copy`.

### Audit

One line with the active audit policy (from CLI flags, project config, or global config): the profile (`default`, `strict` or `permissive`) and the lowest severity that blocks an install (`critical` by default). The dedupe mode and the analyzers are named only when they differ from the defaults (`global` and all analyzers).

### Version

Shows the CLI and skill versions. When a newer skill is released, a line says how to update it. (Global mode only.)

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
  "audit": {
    "profile": "DEFAULT",
    "threshold": "CRITICAL",
    "dedupe": "GLOBAL",
    "analyzers": []
  },
  "version": "0.17.0"
}
```

A tracked repo whose git status cannot be read has `"status": "unknown"` and `message` holding the error; `dirty` is then false and not meaningful.

The `source.skillignore` field is present only when at least one `.skillignore` or `.skillignore.local` file exists. When absent: `"skillignore": { "active": false }`. The `files` array includes `.skillignore.local` paths when present. In text mode, the `.skillignore` line shows `(.local active)` when any `.skillignore.local` is in effect.

JSON output is supported in both global and project mode.

## Project Mode

In a project directory, status shows the project's source, targets and extras, with paths relative to the project root:

```bash
skillshare status        # Auto-detected if .skillshare/ exists
skillshare status -p     # Explicit project mode
```

### Example Output

```
Source
  skills    .skillshare/skills  3 skills
  agents    .skillshare/agents  4 agents
  .skillignore: 3 patterns, 0 skills ignored

Targets                   skills      agents
  claude  .claude/skills  ✓ 3 linked  ✓ 4
  cursor  .cursor/skills  ✓ 3 linked  ✓ 4
  all use merge

Extras
  rules     .cursor/rules     4 files · merge
  commands  .claude/commands  3 files · merge

Audit    default · blocks critical
```

Project status does not show Tracked Repositories or Version sections (these are global-only features).

## See Also

- [sync](/docs/reference/commands/sync) — Sync skills to targets
- [diff](/docs/reference/commands/diff) — Show detailed differences
- [doctor](/docs/reference/commands/doctor) — Diagnose issues
- [Project Skills](/docs/understand/project-skills) — Project mode concepts
