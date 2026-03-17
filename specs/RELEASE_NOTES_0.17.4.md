# skillshare v0.17.4 Release Notes

Release date: 2026-03-17

## TL;DR

v0.17.4 adds **machine-readable health checks** and **source-level skill filtering**:

1. **`doctor --json`** — structured JSON output for CI pipelines, with per-check status and exit code 1 on errors
2. **Web UI Health Check page** — visual dashboard for `doctor` results with filtering, expandable details, and re-check
3. **Root-level `.skillignore`** — hide skills and directories from all commands using a single file at the source root
4. **Full gitignore syntax for `.skillignore`** — `**`, `?`, `[abc]`, `!negation`, anchored `/pattern`, directory-only `pattern/`, and escaped `\#`/`\!` now all work
5. **`.skillignore` visibility** — `status` and `doctor` now report active patterns and ignored skills; the web UI Config page has a `.skillignore` editor tab

---

## Doctor JSON Output

### The problem

`skillshare doctor` was text-only — useful for humans but impossible to integrate into CI pipelines, automation scripts, or the web dashboard. There was no way to programmatically check if a skillshare setup was healthy.

### Solution

`doctor --json` outputs structured JSON with every check result:

```bash
skillshare doctor --json
```

```json
{
  "checks": [
    { "name": "source", "status": "pass", "message": "Source: ~/.config/skillshare/skills (12 skills)" },
    { "name": "sync_drift", "status": "warning", "message": "claude: 1 skill(s) not synced", "details": ["new-skill"] }
  ],
  "summary": { "total": 13, "pass": 12, "warnings": 1, "errors": 0 },
  "version": { "current": "0.17.4", "update_available": false }
}
```

### Design decisions

- **Exit code semantics** — errors produce exit 1 (via `jsonSilentError`), warnings exit 0. This lets CI gate on errors while allowing warnings to pass
- **Flat checks array** — each check is an independent entry with `name`, `status`, `message`, and optional `details`. Same check name can appear multiple times (e.g., one `targets` entry per configured target)
- **Version info** — included as a top-level field, populated from the async update check that already runs during doctor

### Usage patterns

```bash
# CI gate — fail if any errors
skillshare doctor --json | jq -e '.summary.errors == 0'

# Extract warnings for Slack notification
skillshare doctor --json | jq '[.checks[] | select(.status == "warning") | .message]'

# Quick health summary
skillshare doctor --json | jq '.summary'
```

---

## Web UI — Health Check Page

### The problem

The web dashboard had no equivalent of `skillshare doctor`. Users had to switch to the terminal to run diagnostics.

### Solution

New **Health Check** page at `/doctor` in the sidebar under "System":

- **Summary cards** — pass/warnings/errors counts with color-coded indicators
- **Filter toggles** — show All, Errors, Warnings, or Pass checks
- **Check list** — each check shows a human-readable label, status icon, and message. Checks with details (e.g., list of unsynced skills) are expandable
- **Version section** — CLI version with update-available badge
- **Re-check button** — re-run all checks without leaving the page

### Design decisions

- **Server handler shells out** — `GET /api/doctor` runs `skillshare doctor --json` as a subprocess rather than importing check functions directly. The check logic lives in `package main` and would require a large refactor to extract. Shelling out is zero-refactoring and stays in sync automatically
- **Human-readable labels** — check names are mapped from identifiers (`sync_drift`) to labels ("Sync Status") in the frontend, not the backend, keeping the JSON API stable

---

## Root-Level .skillignore

### The problem

`.skillignore` only worked inside tracked repos (`_repo/.skillignore`). There was no way to hide skills at the source root level — you couldn't exclude draft skills, archived directories, or test fixtures from discovery without uninstalling them.

### Solution

Place a `.skillignore` at the source root to hide skills from all commands:

```bash
# ~/.config/skillshare/skills/.skillignore
draft-*          # Hide all draft skills
_archived/       # Hide entire directory
test-fixture     # Hide specific skill
```

Both levels now work:
- **Root-level** (`<source>/.skillignore`) — affects all skills in the source
- **Repo-level** (`<source>/_repo/.skillignore`) — scoped to that tracked repo

### Design decisions

- **SkipDir optimization** — when a directory matches a `.skillignore` pattern, the walker skips the entire directory tree (returns `fastwalk.SkipDir`) instead of entering it and filtering individual files. This meaningfully improves discovery time for source trees with large ignored directories
- **Gitignore syntax** — uses the same glob patterns as `.gitignore` for familiarity

---

## Full Gitignore Syntax for .skillignore

### The problem

`.skillignore` used a naive string matcher that only supported exact names, directory prefixes, and trailing `*` wildcards. Users expected `.gitignore`-compatible syntax — patterns like `demo/` with trailing slash caused matching failures (#83), and there was no way to use negation (`!important`), character classes (`[Tt]est`), or recursive globs (`**/temp`).

### Solution

The matcher was rewritten to support the full gitignore specification:

```bash
# .skillignore — all gitignore patterns now work
**/temp              # Ignore "temp" at any depth
test-*               # Wildcard prefix
!test-important      # Negation — keep this one
vendor/              # Directory-only (won't match a file named "vendor")
[Dd]raft*            # Character class
/root-only           # Anchored to .skillignore location
\#not-a-comment      # Escaped literal
```

### Design decisions

- **No external dependencies** — uses Go's `path.Match` for per-segment glob matching (`*`, `?`, `[...]`), with `**` and gitignore semantics (negation, anchoring, dir-only) layered on top
- **Parent directory inheritance** — if `vendor` is ignored, `vendor/sub/deep` is automatically ignored too. The matcher checks all parent prefixes of every path
- **Safe CanSkipDir with negation** — when negation patterns exist, the walker does not skip ignored directories, because a descendant might be un-ignored by `!pattern`. Without negation, the SkipDir optimization still applies
- **Backward compatible** — all existing `.skillignore` patterns continue to work. The only semantic change is that `*` no longer crosses `/` (matching gitignore spec), but this is handled transparently through parent-dir inheritance

### Usage patterns

```bash
# Ignore everything under vendor/ except vendor/important
vendor/
!vendor/important

# Ignore all test skills except test-critical
test-*
!test-critical

# Ignore temp directories at any depth
**/temp

# Only ignore build at the root, not nested build/ dirs
/build
```

---

## .skillignore Visibility

### The problem

`.skillignore` filtering was completely invisible. `status` showed post-filter skill counts with no indication that filtering occurred. `doctor` didn't mention `.skillignore` at all. Users had no way to verify which patterns were active or which skills were being excluded — they had to manually inspect the file and cross-reference with `list`.

### Solution

`.skillignore` status is now surfaced across CLI and web UI:

**CLI — `status`:**
```
Source: ~/.config/skillshare/skills (12 skills)
  .skillignore: 5 patterns, 3 skills ignored
```

The extra line only appears when a `.skillignore` file exists. `status --json` includes a `skillignore` object in the `source` field with `active`, `files`, `patterns`, `ignored_count`, and `ignored_skills`.

**CLI — `doctor --json`:**
A new `skillignore` check reports `pass` with pattern/ignored counts, or `info` when no `.skillignore` exists. The `info` status is a new fourth status alongside `pass`/`warning`/`error` for neutral informational checks.

**Web UI — Config page:**
The Config page now has two tabs (`config.yaml` / `.skillignore`) using the same pill toggle as the Skills page. The `.skillignore` tab provides a CodeMirror editor with live stats showing how many skills are currently ignored, plus an "Ignored Skills" summary card below the editor.

**Web UI — Doctor page:**
The Health Check page now shows the `.skillignore` check in its check list. The filter toggles were also unified to use the same `SegmentedControl` component as the Skills page for visual consistency.

### Design decisions

- **`info` status** — informational checks don't inflate error or warning counts. This lets CI pipelines gate on `summary.errors == 0` without being tripped by neutral observations
- **Stats from the same discovery walk** — ignored skill data is collected during the same filesystem walk that discovers skills, so there's no extra I/O cost
- **Visibility only when active** — `status` only shows the `.skillignore` line when the file exists, keeping the default output clean

### Usage patterns

```bash
# See what .skillignore is doing
skillshare status

# Get full details in JSON
skillshare status --json | jq '.source.skillignore'

# CI: check if .skillignore is active
skillshare doctor --json | jq '.checks[] | select(.name == "skillignore")'

# Edit .skillignore from the web UI
skillshare ui   # → Config page → .skillignore tab
```

---

## Bug Fixes

- **`.skillignore` false warnings** — `doctor` reported false "unverifiable (no metadata)" warnings for directories that were supposed to be excluded via `.skillignore` (e.g., `.venv/` inside tracked repos). Source discovery now respects `.skillignore` patterns consistently across all commands ([#83](https://github.com/runkids/skillshare/issues/83))
- **`.skillignore` directory-only patterns during install** — patterns with trailing slash (e.g., `demo/`) now correctly match directories during `skillshare install` discovery, not just during sync
- **Doctor check labels** — the web UI Health Check page displays human-readable labels ("Source Directory", "Symlink Support") instead of raw snake_case identifiers
