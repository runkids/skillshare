# Profiles

Profiles select deployed skills without uninstalling source skills. They are separate from `audit --profile` security policies.

```bash
skillshare profile list --json
skillshare profile create writing --add-include 'docs__*'
skillshare profile show writing --json
skillshare profile add writing docs/review
skillshare profile remove writing docs__review
skillshare profile use writing --dry-run --json
skillshare profile use writing
skillshare profile use --none
```

Use `-g` or `-p` to choose a config. Definitions are shared YAML under `profiles`; active state and pending are local to the canonical config identity. No `skills` key is unrestricted; `skills: {}` or empty/blank includes select nothing. Match flat source names with Go globs. Excludes require an include key; `include: ['*']` means everything before exclusions. New profiles without patterns are empty.

`edit` accepts `--description`, `--add-include`, `--remove-include`, `--add-exclude`, `--remove-exclude`; pattern flags repeat. `rename` follows the locally active name; `delete` refuses an active definition. Definition changes affecting active selection require a later sync.

`add <profile> <skill>...` / `remove <profile> <skill>...` resolve flat names,
source-relative paths or unique base names. Ambiguity fails with candidates.
Add removes an exact exclude and adds an exact include when needed. A matching
glob exclude refuses the whole batch, naming the pattern, with nothing saved.
Remove drops an exact include and adds an exact exclude if a broader include
still selects the skill. For an unrestricted definition it first writes
`include: ['*']`; the last removed include leaves `include: []` and preserves
existing excludes. Already-in/not-in assignments are no-ops. `--json` reports
`profile`, `changed` and `skills: [{name, result}]`; active changes need sync.

Diff explains managed prune rows with `not in profile <name>`. The sync matrix
and resource detail use `not_in_profile` before target-filter reasons. Doctor
checks unavailable active state, pending application, kept modified copies,
unmatched includes, inert excludes and symlink targets. Sync warns once about
unmatched active-profile patterns, even with an empty desired selection.

Successful install/new operations hint with actual final flat names outside
the usable active profile. Install JSON keeps hints on stderr; dry runs,
skips, failures and agents get none. Partial installs name only successes.
Move warns on membership changes in all usable profiles, including inactive
ones, without rewriting patterns. Folder moves check each contained skill.

`use` accepts `--dry-run`, `--no-sync`, `--none`, `--allow-empty`, `--discard-edits`, `--json`. It syncs valid enabled merge/copy skill targets, excluding registered projects and every other resource kind. Profiles narrow target filters and metadata routing. Symlink-mode roots remain unchanged.

A refused switch changes no state or target. An incomplete switch leaves the new active name pending and exits nonzero; retry `skillshare sync`. Modified managed copies and unmanaged collisions are kept. `--discard-edits` snapshots and verifies each listed modified copy and rechecks before deletion; it cannot discard unmanaged content. Ordinary `sync --discard-edits` uses the same rule while a profile or pending switch exists. Text and JSON report snapshot paths, retention and restore commands. Collect authored changes when they should become source content.

Project `sync --discard-edits` also covers selected eligible targets’ declared former skills folders. Use each reported moved restore command to recover that folder while preserving the current one. Profile switching does not run former-folder cleanup.

Reads/dry runs create no lock. Non-dry operations retain an empty local lock file and its directories even on refusal; never delete it to bypass a running command. Unavailable active state refuses skill operations; repair with `profile use <name>` or `profile use --none`. Older binaries ignore the local selection and may drop definitions when saving. No-profile behavior stays unchanged except copy output fingerprint bookkeeping.

## Dashboard workflow

Start `skillshare ui -g`, or `skillshare ui -p` from the intended project.
The Library's Profiles page creates, edits, renames and deletes definitions.
Pattern edits use a server preview of selected flat names and unmatched patterns;
an empty include is intentional, and an omitted skills section stays unrestricted
until the user chooses specific skills. Failed saves retain the draft for retry.

List, card and tree skill selections offer Add to profile / Remove from profile.
Folders expand to exact source flat names and overlapping selections deduplicate.
Glob-blocked additions refuse atomically; agents have no membership action.
Definition changes never activate or automatically sync. Active edits offer an
explicit skills preview and apply action; inactive deletion requires confirmation,
and active deletion is refused even if it became active after the page loaded.

The header picker previews switches and requires separate consent for an empty
selection and snapshot-first discard. Preserve returned restore commands and
retention details. Repair incomplete targets, then explicitly sync from the pending
banner, including when the preview shows no changes. Missing active definitions
require choosing a usable profile or All skills; no silent fallback occurs.
