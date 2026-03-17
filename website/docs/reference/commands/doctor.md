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
  ✓ Source: ~/.config/skillshare/skills (12 skills)
  ✓ Link support: OK
  ✓ Git: initialized with remote

Checking targets
  ✓ claude    [merge]: merged (8 shared, 2 local)
  ✓ cursor    [copy]: copied (8 managed, 0 local)
  ⚠ codex     [merge]: needs sync

Version
  ✓ CLI: 1.2.0
  ✓ Skill: 1.1.0

Summary
  ✓ All checks passed!
```

## Checks Performed

### Environment

| Check | What It Verifies |
|-------|-----------------|
| Config | Config file exists and is valid |
| Source | Source directory exists and is readable |
| Link support | System can create symlinks |
| Git | Repository status and remote configuration |

### Targets

For each target:
- Path exists and is writable
- Sync mode matches actual state
- Sync drift (linked/managed count vs target expected count after `include`/`exclude`)
- No broken symlinks
- Duplicate-skill checks for unintended local collisions:
  - `merge` mode: skipped (local skills are expected)
  - `copy` mode: manifest-managed copies are ignored; only local colliding copies are warned
- Valid include/exclude glob patterns
- Info-level per-target compatibility hint when applicable (example target priority: `cursor` → `antigravity` → `copilot` → `opencode`; no hint when these targets are absent)

### Version

- CLI version
- skillshare skill version
- Checks for available updates

### Skill Integrity

For tracked skills with `.skillshare-meta.json` file hashes, doctor verifies that no files have been tampered with since installation:

- Compares current SHA-256 hashes against stored hashes
- Reports modified, missing, and added files per skill
- Local skills (no `.skillshare-meta.json`) are silently skipped — this is expected
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
  ✓ Link support: OK
  ⚠ Git: 3 uncommitted change(s)

⚠ Skills without SKILL.md: test-dir, temp
⚠ _team-repo__api-helper: 1 modified
✓ Skill integrity: 5/6 verified

Checking targets
  ✓ claude    [merge]: merged (8 shared, 2 local)
  ✗ cursor    [merge]: 2 broken symlink(s): old-skill, removed-skill
  ⚠ codex     [merge->needs sync]: linked (needs sync to apply merge mode)
  ⚠ claude: 1 skill(s) not synced (2/3 linked)

Version
  ✓ CLI: 1.2.0
  ⚠ Skill: 1.0.0 (update available: 1.1.0)
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
    { "name": "sync_drift", "status": "warning", "message": "claude: 1 skill(s) not synced (7/8 linked)", "details": ["new-skill"] },
    { "name": "broken_symlinks", "status": "error", "message": "cursor: 1 broken symlink(s)", "details": ["old-skill"] }
  ],
  "summary": { "total": 13, "pass": 11, "warnings": 1, "errors": 1 },
  "version": { "current": "0.17.4", "latest": "0.18.0", "update_available": true }
}
```

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
