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

## Maintainer review follow-up — 2026-10-01

Rebased onto main `bcea0b0f`, preserving account-target hooks. Git is resolved
before account lookup, including a configured account named `git`; account-home
parking and restore checks do not reinterpret Git destinations. Global and
project schema bindings both accept Git commands, and Git is excluded from the
account-key pattern. Account and Git canonical-path helpers have distinct names.

Inactive Git output keeps its actual file action. `inactiveReason` and warnings
explain activation separately, so a successful sync settles to `unchanged` while
the dashboard still shows why Git cannot execute it. Skipped outputs never
count as pending writes. Windows ignores unavailable POSIX execute bits when
comparing unchanged helpers; POSIX sync still repairs permission drift.
Successful config rename consumes the native lock, and cleanup does not remove
a later writer's lock. Failed commits still release their own lock.

## Verification and limits

Verification for the review fixes ran inside the devcontainer on an owned
Crabbox runner, with normal test timeouts and assertions:

- `make check` passed with Git 2.39.5, including the full Go unit and integration
  suites. Native assertions requiring config hooks or orphan worktrees are
  version-gated; separate inactive-output tests run on older Git. The named-user
  include test explicitly skips for UID 1001 without a passwd entry.
- Git 2.55.0 hooks race tests passed. The guarded `ssenv` real-hook proof passed
  for commit/amend/push, ordering, arguments/stdin, no-verify and linked worktrees.
- The dashboard passed 749 tests across 93 files with default budgets, lint and
  production build. Chromium confirmed inactive status without a pending sync
  button in Clean/Playful light/dark at desktop width. Website typecheck and production builds passed in all five
  locales. Context-router and diff checks passed.
- JSON Schema draft 2020 validation accepted Git command bindings in global and
  project scope and rejected Agent event maps in a Git binding.
- Canonical parent-alias behavior has a Linux regression. Native macOS
  confirmation requires PR CI. Windows hooks tests cross-compiled successfully;
  permission settling is exercised with the Windows service platform on Linux.
  Native Windows hook execution and a second-machine runtime proof were not run.

The execution runbook is `ai_docs/tests/git_hooks_runbook.md`; management itself
never executes hooks. The native submission's earlier timeout overrides are
superseded by the devcontainer verification above.

Import/adoption, hooks-directory discovery/recognizers and double-run warnings
are Phase 2; script-mode bindings are Phase 3; structured Git cards and doctor
checks are Phase 4. These phases are sequential and await review. Config mode
coexists with hooks-directory hooks, so users must inspect duplicates manually.

See `website/docs/reference/commands/hooks.md` and the portable Roborev recipe
at `website/docs/how-to/recipes/git-hooks.md` for the public behavior.
