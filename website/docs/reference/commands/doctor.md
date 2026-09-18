---
sidebar_position: 1
---

# doctor

Check environment and diagnose issues with your skillshare setup.

```bash
skillshare doctor
skillshare doctor -p        # Project mode (.skillshare/config.yaml)
skillshare doctor -g        # Force global mode
skillshare doctor --json    # Structured JSON output for CI
```

![doctor demo](/img/doctor-demo.png)

## When to Use

- Something isn't working and you don't know why
- After upgrading skillshare or your OS
- Verify all targets, git, and symlinks are healthy
- First diagnostic step before filing a bug report

## What It Checks

```text
skillshare doctor

Checking environment
✓ Config: ~/.config/skillshare/config.yaml
→ Config directory: ~/.config/skillshare
→ Data directory:   ~/.local/share/skillshare
→ State directory:  ~/.local/state/skillshare

✓ Source: ~/.config/skillshare/skills (12 skills)
✓ Agents source: ~/.config/skillshare/agents (8 agents)
✓ Skillignore: 2 patterns, 1 skills ignored
✓ Link support: OK
✓ Git: initialized with remote

✓ Skill integrity: 12/12 verified

Checking targets
claude
  skills   [merge] merged (8 shared, 2 local)
  agents   [merge] merged (8/8 linked)
cursor
  skills   [copy] copied (8 managed, 0 local)
  agents   [merge] merged (8/8 linked)
codex
  skills   [merge] needs sync

Extras
✓ rules: 4 files, 1/1 targets OK
✓ commands: 3 files, 1/1 targets OK

Version
✓ CLI: 0.17.0
✓ Skill: 0.17.0

Summary
✓ All checks passed!
```

## Checks Performed

### Environment

| Check | What It Verifies |
|-------|-----------------|
| Config | Config file exists and is valid |
| Source | Source directory exists and is readable |
| Agents source | Agents source directory exists (if configured) |
| Skillignore | `.skillignore` (and `.skillignore.local`) active patterns and ignored skill count |
| Link support | System can create symlinks |
| Git | Repository status and remote configuration |

### Targets

Each target shows sub-items for **skills** and **agents** (when agents are configured):
- Skills: path, sync mode, sync state, shared/local counts
- Agents: linked count, drift detection
- No broken symlinks
- Duplicate-skill checks for unintended local collisions:
  - `merge` mode: skipped (local skills are expected)
  - `copy` mode: manifest-managed copies are ignored; only local colliding copies are warned
- Valid include/exclude glob patterns
- Info-level per-target compatibility hint when applicable (example target priority: `cursor` → `antigravity` → `copilot` → `opencode`; no hint when these targets are absent)

### Path Overlap

Doctor flags two classes of duplicate-skill risk before they reach the runtime picker:

**`shared_target_paths`** — fires when two or more enabled targets resolve to the same primary path. Common cause: enabling both `universal` and a tool that writes to `~/.agents/skills` (e.g. `warp`, `witsy`).

```text
! Shared path ~/.agents/skills ← universal, warp
```

Resolution: disable one of the overlapping targets, or set a distinct path with `skillshare target <name> --path <dir>`.

**`cross_target_discovery`** — fires when an enabled target's runtime is documented to also scan a directory another enabled target writes to. For example, Codex Desktop reads `~/.agents/skills` in addition to `~/.codex/skills`, so enabling both `codex` and `universal` causes Codex to see universal's content.

```text
! codex will see content from: universal
    ~/.agents/skills ← universal
```

Resolution: pick one target as the writer for the shared content, or accept the overlap if duplicate listings in the runtime picker are acceptable.

Both checks are pure metadata — they read configured paths and the built-in `also_scans` table, no filesystem probing.

#### Codex: keep shared targets, disable redundant entries

If another tool needs the overlapping target, removing that writer is not always
the right fix. Codex can disable an individual skill entry without deleting its
files or changing another tool's configuration.

First distinguish the possible causes:

- **Multiple discovery roots:** a copy in `~/.codex/skills` and a symlink under
  `~/.agents/skills` can expose the same source skill twice.
- **Multiple source locations:** a standalone skill and a bundled or archived
  copy can have the same frontmatter `name`. Codex does not merge entries just
  because their names match.
- **Nested skills:** a synchronized parent skill may contain another `SKILL.md`
  under `references/`. Excluding the child's flattened target name does not
  remove that file from the parent directory or stop Codex from discovering it.

For example, a standalone `gem/SKILL.md` and an identical
`archive/references/originals/gem/SKILL.md` can produce four `gem` entries when
both a copy target and a shared symlink target are visible to Codex. Excluding
`archive__*` still leaves the nested entry reachable through `archive/`.

To resolve this for Codex only:

1. Inspect Codex's skill entries and identify the paths it actually loads. For
   a structured inventory, the [Codex app-server `skills/list`
   method](https://developers.openai.com/codex/app-server) returns each entry's
   `name`, `path`, and `enabled` state. Symlink paths may resolve into
   Skillshare's source directory.
2. Choose which entry to retain. Compare instructions, supporting files and
   `agents/openai.yaml`, not just the name or `SKILL.md` hash. Keep any local
   customizations and the relative references needed by the retained version.
3. Back up Codex's user configuration, normally `~/.codex/config.toml`, and add
   a path override for each redundant entry, preserving existing settings:

   ```toml
   [[skills.config]]
   path = "/absolute/path/to/redundant/skill/SKILL.md"
   enabled = false
   ```

   Use the redundant entry's exact path from Codex. Disabling a canonical path
   can affect every symlink alias of that file in Codex; do not disable the path
   of the entry you intend to keep. These overrides are Codex configuration,
   even when their paths point into Skillshare's shared source.
4. Restart Codex, then check its picker or reload `skills/list` with
   `forceReload: true`. Disabled entries may remain in the API inventory; count
   entries with `enabled: true`. Confirm one enabled entry for each name you
   chose to deduplicate and that every skill you need is still available.

See [OpenAI's skill configuration documentation](https://developers.openai.com/codex/skills)
for the supported setting. Remove the specific override, or set it to
`enabled = true`, to restore an entry. Review these path-specific overrides when
installations move, a retained version is removed, or new duplicate paths appear.

:::note What this verifies
Doctor's `duplicate_skills` check looks for unintended local collisions in
targets; it does not inventory Codex's combined runtime catalog. A
`duplicate_skills` pass can coexist with a `cross_target_discovery` warning.
The overlap warning may remain after Codex overrides because the directory
topology has not changed. Likewise, a deduplicated catalog may still exceed
Codex's description budget. Verify enabled entries rather than expecting every
warning to disappear.
:::

### Version

- CLI version
- skillshare skill version
- Checks for available updates

### Skill Integrity

For installed skills with file hash metadata, doctor verifies that no files have been tampered with since installation:

- Compares current SHA-256 hashes against stored hashes
- Reports modified, missing, and added files per skill
- Local skills (not in `.metadata.json`) are silently skipped — this is expected
- Installed skills with metadata but missing `file_hashes` are flagged with their names

```text
⚠ _team-repo__api-helper: 1 modified, 1 missing
✓ Skill integrity: 5/6 verified
⚠ Skill integrity: 1 skill(s) missing file hashes: _old-repo__legacy-skill
```

### Extras

When extras are configured, verifies:
- Source directory exists for each extra
- Target directories are reachable
- Reports missing source directories or unreachable targets

### Other

- Skills without `SKILL.md` files
- Skill-level `targets:` field validation (warns on unknown target names)
- Last backup timestamp (global mode)
- Trash status (item count, total size, oldest item age)
- Broken symlinks in targets

:::note Project Mode
When a project has `.skillshare/config.yaml`, `skillshare doctor` auto-runs in project mode.

In project mode:
- Config/source checks use `.skillshare/config.yaml` and `.skillshare/skills`
- Trash status uses `.skillshare/trash`
- Backups show `not used in project mode`
:::

## Common Issues

### "Needs sync"

Target mode was changed but not applied:

```bash
skillshare sync
```

### "Not synced"

Target has fewer linked skills than source (e.g. after installing new skills):

```bash
skillshare sync
```

### "Has uncommitted changes"

Tracked repo has local changes:

```bash
cd ~/.config/skillshare/skills/_team-repo
git status
# Commit or discard changes
```

### "Broken symlink"

A skill was removed from source but symlink remains:

```bash
skillshare sync  # Will prune orphaned symlinks
```

### "Skills without SKILL.md"

Skill folders missing required file:

```bash
# Add SKILL.md to each skill, or remove the folder
skillshare new my-skill  # Creates proper structure
```

### "Link not supported"

On Windows without Developer Mode:

1. Enable Developer Mode in Settings
2. Or run as Administrator

## Example Output with Issues

```
Checking environment
✓ Config: ~/.config/skillshare/config.yaml
✓ Source: ~/.config/skillshare/skills (12 skills)
✓ Agents source: ~/.config/skillshare/agents (8 agents)
✓ Link support: OK
⚠ Git: 3 uncommitted change(s)

⚠ Skills without SKILL.md: test-dir, temp
⚠ _team-repo__api-helper: 1 modified
✓ Skill integrity: 5/6 verified

Checking targets
claude
  skills   [merge] merged (8 shared, 2 local)
  agents   [merge] merged (8/8 linked)
cursor
  skills   [merge] 2 broken symlink(s): old-skill, removed-skill
codex
  skills   [merge] needs sync
⚠ claude: 1 skill(s) not synced (2/3 linked)

Version
✓ CLI: 0.17.0
⚠ Skill: 0.16.0 (update available: 0.17.0)
  Run: skillshare upgrade --skill && skillshare sync

Backups: last backup 2026-01-18_09-00-00 (3 days ago)
ℹ Trash: 2 item(s) (45.2 KB), oldest 3 day(s)

ℹ Update available: 1.2.0 -> 1.3.0
  brew upgrade skillshare  OR  curl -fsSL .../install.sh | sh

Summary
  ✗ 1 error(s), 4 warning(s)
```

## JSON Output

Use `--json` for machine-readable output in CI pipelines and automation:

```bash
skillshare doctor --json
```

```json
{
  "checks": [
    { "name": "source", "status": "pass", "message": "Source: ~/.config/skillshare/skills (12 skills)" },
    { "name": "skillignore", "status": "pass", "message": ".skillignore: 3 patterns, 2 skills ignored", "details": ["test-*", "vendor/", "!important", "---", "test-draft", "vendor/lib"] },
    { "name": "sync_drift", "status": "warning", "message": "claude: 1 skill(s) not synced (7/8 linked)", "details": ["new-skill"] },
    { "name": "shared_target_paths", "status": "warning", "message": "1 shared target path(s) — enabled targets writing to the same directory may produce duplicate skills in runtime pickers", "details": ["~/.agents/skills ← universal, warp"], "suggestions": ["Choose one authoritative target for ~/.agents/skills and disable or reconfigure the rest (currently: universal, warp)"] },
    { "name": "broken_symlinks", "status": "error", "message": "cursor: 1 broken symlink(s)", "details": ["old-skill"] }
  ],
  "summary": { "total": 14, "pass": 12, "warnings": 1, "errors": 1, "info": 0 },
  "version": { "current": "0.17.4", "latest": "0.18.0", "update_available": true }
}
```

Check statuses: `pass`, `warning`, `error`, `info`. The `info` status is used for informational checks (e.g., `.skillignore` not found) that are neither passing nor failing. Info checks count toward `total` but not toward `pass`, `warnings`, or `errors`.

Some warning checks (e.g. `shared_target_paths`, `cross_target_discovery`) also include an optional `suggestions` array with actionable remediation steps. The field is omitted when there is nothing to suggest.

### Exit Codes

| Condition | Exit Code |
|-----------|-----------|
| All checks pass (or warnings only) | `0` |
| Any check has `error` status | `1` |

### CI Example

```bash
# Fail pipeline if doctor finds errors
skillshare doctor --json | jq -e '.summary.errors == 0'

# Extract warnings for notification
skillshare doctor --json | jq '[.checks[] | select(.status == "warning")]'
```

:::tip Web Dashboard
The **Health Check** page in the web dashboard (`skillshare ui`) provides a visual version of `doctor --json` with filter toggles and expandable details.
:::

## See Also

- [status](/docs/reference/commands/status) — Quick status check
- [sync](/docs/reference/commands/sync) — Fix sync issues
- [upgrade](/docs/reference/commands/upgrade) — Update CLI and skill
