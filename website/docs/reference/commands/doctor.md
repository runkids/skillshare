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

```text
skillshare doctor

Checking environment
─────────────────────────────────────────
✓ Config: ~/.config/skillshare/config.yaml
→ Config directory: ~/.config/skillshare
→ Data directory:   ~/.local/share/skillshare
→ State directory:  ~/.local/state/skillshare

✓ Source: ~/.config/skillshare/skills (43 skills)
✓ Agents source: ~/.config/skillshare/agents (2 agents)
→ Skillignore: not configured
✓ Link support: OK
! Git: not initialized (recommended for backup)

✓ Skill integrity: 27/27 verified

Checking targets
─────────────────────────────────────────
claude
  skills   [merge] merged (43 shared, 0 local)
  agents   [merge] synced (2/2 linked)
cursor
  skills   [merge] merged (43 shared, 1 local)
  agents   [merge] synced (2/2 linked)
gemini
  skills   [merge] merged (43 shared, 0 local)
…
! gemini will see content from: universal
    ~/.agents/skills ← universal
…
✗ claude: 1 broken symlink(s): frontend__css-review
…

Extras
─────────────────────────────────────────
✓ rules: 2 files, 2/2 targets OK
✓ commands: 1 files, 1/1 targets OK
✓ team: 1 files, 4/4 targets OK

Storage
─────────────────────────────────────────
→ Backups: last backup 2026-09-28_12-41-50 (10 minutes ago)
→ Trash: 1 item(s) (247 B), oldest <1 day

Summary
─────────────────────────────────────────
✗ 6 error(s), 4 warning(s)
```

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
  agents   [merge] synced (8/8 linked)
cursor
  skills   [copy] copied (8 managed, 0 local)
  agents   [merge] synced (8/8 linked)
codex
  skills   [merge] needs sync

Extras
✓ rules: 4 files, 1/1 targets OK
✓ commands: 3 files, 1/1 targets OK

MCP
✓ All 2 MCP server(s) OK

Hooks
✓ All 1 hook(s) in sync

Plugins
→ Plugins: none configured

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
- Agents: sync mode, linked count, drift detection. On Windows without Developer Mode, `merge` shows as `[copy]`; up-to-date managed copies count as linked. Identical local files that skillshare does not own are preserved. In copy fallback, agent counts show them separately as `local preserved`, for example `0/1 linked, 1 local preserved`.
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

When the targets sharing a path have different `include` or `exclude` filters, each sync adds what one target wants and prunes what another filters out, so the folder never settles and `sync` keeps showing the same pending changes. Doctor marks these and suggests turning skills off for all but one target (`universal` when it is one of them) instead of removing a target:

```text
! Shared path ~/.agents/skills ← codex, universal (different filters, so they undo each other on every sync)
    suggestion: Keep universal syncing skills to ~/.agents/skills and stop the rest with `skillshare target codex --skills=false`.
```

`sync` prints the same targets with the command to run, and the dashboard's Sync page has a button that stops syncing skills for the target to drop. Targets sharing a path with identical settings keep the resolution above.

**`cross_target_discovery`** — fires when an enabled target's runtime is documented to also scan a directory another enabled target writes to. For example, a config left over from an older setup still points `codex` at the legacy `~/.codex/skills`, while `universal` writes to `~/.agents/skills` — which Codex also reads. Enabling both makes Codex see universal's content on top of its own.

```text
! codex will see content from: universal
    ~/.agents/skills ← universal
```

Resolution: start by removing the scanning target (`codex` above). Its runtime already reads the shared directory, and no other tool is affected. Preview with `skillshare target remove codex --dry-run`. Removing the writer (`universal`) instead also hides those skills from every other tool that reads `~/.agents/skills`. Keep both targets only if the scanning target carries skills the writer filters out, and accept the duplicate listings in the runtime picker.

Both checks are pure metadata — they read configured paths and the built-in `also_scans` table, no filesystem probing.

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
- Each extra's settings are valid (mode, `flatten`, `as`) and no two extras claim the same file
- Source directory exists for each extra
- Target directories are reachable
- Broken symlinks in each directory target (error)
- Files that differ from the source, as `skillshare diff` reports them (warning). Targets with `flatten` or `extension` are not compared.

```text
! rules → ~/.claude/rules: 1 file(s) out of sync (a.md missing in target)
✗ rules → ~/.claude/rules: broken symlink gone.md
```

### MCP

Runs the static part of [`mcp check`](./mcp.md): referenced environment variables are set, `command` resolves on `PATH`, client rules accept the server, and every entry is synced. Doctor does not resolve hosts or start servers; run `skillshare mcp check` or `skillshare mcp check --live` for that. Shows `info` when no server is configured.

```text
✗ docs: command no-such-mcp-binary was not found on PATH
! docs → claude: not synced yet; run skillshare sync mcp
```

### Hooks

Previews `skillshare sync hooks` without writing. A failed preview is an error. Entries that sync would still add, update or remove, conflicts with native hooks, and advisory warnings (such as an event name the Agent does not document) are warnings. Shows `info` when no hook is configured.

```text
! bash-log → claude: not synced (add)
```

### Plugins

Previews `skillshare sync plugins` without fetching any source. Doctor only asks each bound Agent's native CLI what is installed, and only when a plugin package exists. Blocked bindings (for example, the Agent's CLI is not installed) and bindings that still need a sync are warnings. Checking a source for new releases stays in `skillshare plugin check`. Shows `info` when no package is configured.

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

`doctor` links a test folder inside the system temp directory (`%TEMP%` on Windows, `$TMPDIR` or `/tmp` elsewhere). On Windows that link is an NTFS junction, which needs neither Administrator nor Developer Mode, so turning on Developer Mode does not fix this error. The `junction error:` line in the message shows why Windows refused. Check that the temp directory:

1. Is on a local NTFS drive, not FAT32, exFAT, or a network share (junctions only work on NTFS)
2. Is writable by your account, and not blocked by antivirus or security software

This check does not test file links. Without Developer Mode, agents and extras that link single files are copied instead; see [Windows troubleshooting](../../troubleshooting/windows.md#file-links-need-windows-developer-mode-copying-instead).

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
  agents   [merge] synced (8/8 linked)
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
    { "name": "shared_target_paths", "status": "warning", "message": "1 shared target path(s) — enabled targets writing to the same directory may produce duplicate skills in runtime pickers", "details": ["~/.agents/skills ← universal, warp"], "suggestions": ["Choose one authoritative target for ~/.agents/skills; preview removing duplicate targets with `skillshare target remove <name> --global --dry-run` (currently: universal, warp)."] },
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
