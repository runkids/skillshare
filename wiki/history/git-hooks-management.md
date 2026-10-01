# Git hooks management — config mode

Phase 1 adds the `git` hooks destination alongside Agent hooks. Git 2.54+ reads
named `hook.<name>` sections from a generated include; `parallel` requires 2.55+.
Source uses `bindings.git.commands` and optional inline executable `files`.
The dashboard edits the whole binding as YAML and preserves commands/helpers.

## Destinations and ownership

Global files live under `$XDG_CONFIG_HOME/git/skillshare`; project files live
under the canonical Git common directory, shared by linked worktrees. Declare
one source root per common destination. Include targets use absolute
`GIT_CONFIG_GLOBAL`, existing HOME config, existing XDG config, then HOME config.
Manual/conditional includes stay user-owned and retain their scope, including
inactive nested parents. Structural inspection retains lexical symlink paths
and fails closed on unreadable or excessively nested declarations. Symlinked or
unwritable targets produce inactive status with exact manual include lines;
older Git is inactive without a fallback dispatcher. Missing Git/roots skip
outputs without discarding ownership.

The generated file is owned whole, with per-name records and helper records.
Explicit replacement regenerates the file after backup. Collision replacement
is limited to the writable regular target config, using native locking, narrow
section fragments and restore anchors. Includes use exact value operations.
Combined global/project sync acknowledges only its own changed rows in other
destination guards. Planned commands use distinct names across those scopes to
prevent merged registrations before the first write.
Unrelated private settings are absent from previews and section backups, and
later unrelated edits survive restore. Journal recovery preserves completed
writes; backups retain executable modes. Git `--keep-files` is refused because
entries share one generated file.

Foreign section restore also checks generated names after ledger loss and
invalidates previews when even an inactive generated file changes.

Review follow-up strengthens the schema guard to require an explicit
`additionalProperties: false`, gives same-file diff regions distinct indexed
accessible names, and warns on `false`, `no`, `off`, `0` and explicitly empty
`enabled` values. Bare `enabled` keys remain true. If native Git parsing blocks
journal recovery, the error names the journal and instructs users to keep it
intact, restore Git or repair the config, then retry; config and ownership stay
unchanged until safe recovery succeeds.
Whole-file/include restore also checks live managed names across global/project
scope while keeping different projects isolated. Sync repairs permission drift
on unchanged owned Git helpers without altering their contents.

Management invokes only allowlisted Git version, rev-parse, config and read-only
hook-list commands. It never executes hooks or installs a dispatcher.

## Verification and limits

Regression coverage includes validation, Git-parser quoting, destinations and
linked identity, ownership decision tables, scopes/conditional includes,
collisions/overrides, stale previews, locks, partial failure/recovery, ordered
apply/remove, private backups, modes, CLI lifecycle and API JSON contracts.
UI tests cover YAML round trips, new/edit saves, inactive status, localization
and linked backup grouping; Chromium checks cover Clean/Playful light/dark and
mobile layout. Native Go builds/tests and non-Docker checks are used for this
contribution under the explicit execution-environment override.

Final native evidence: the complete hooks suite passed (139.219s), and the hooks
race suite passed (191.183s). The complete UI suite passed 704 tests across 91
files with a local-only ten-second async-render wait budget and thirty-second
test budget; cases and assertions were unchanged. Default native UI waits
previously failed two existing tests, with the MCP wait also reproduced at the
base revision. The affected Git/editor/localization suite passed with normal
async waits. Go integration uses a local thirty-minute process budget after the
default ten-minute budget expired; no cases or assertions are removed. UI production build and lint passed (existing warnings remain),
and website production builds passed in all five locales.

The guarded real-hook proof is `scripts/hooks/git-e2e.sh`; its runbook is
`ai_docs/tests/git_hooks_runbook.md`. It is authored and syntax-checked, but its
execution is skipped because it requires a devcontainer. Mac/Windows and a
second-machine runtime proof are not claimed. Windows-style paths are rendered;
native Windows execution remains unverified.

Import/adoption, hooks-directory discovery/recognizers and double-run warnings
are Phase 2; script-mode bindings are Phase 3; structured Git cards and doctor
checks are Phase 4. These phases are sequential and await review. Config mode
coexists with hooks-directory hooks, so users must inspect duplicates manually.

See `website/docs/reference/commands/hooks.md` and the portable Roborev recipe
at `website/docs/how-to/recipes/git-hooks.md` for the public behavior.
