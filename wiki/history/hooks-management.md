# Native hooks management

## Outcome

Skillshare manages named native hooks for Claude, Codex, Gemini, Copilot,
Cursor, Droid, Qwen, Pi, Amp and OpenCode. Global and project scopes use the
same source format and domain service. CLI and dashboard workflows cover
import, editing, receiving Agents, enable/disable, preview, sync, removal and
selective native backup/restore.

The dashboard includes Hooks in the resource library, Targets, Projects,
Sync and Settings > Backups. Project mutation and sync affect only that root.
Source-only saving remains available when native synchronization conflicts.
Each hook's row menu previews its native configuration, code, auxiliary files
and destination paths. Permanent synchronization/trust banners are omitted
from the list; actual conflicts and unmanaged content remain visible.

## Design decisions

- Preserve native event names, matchers, handler types, timeout units and
  supplied extension/plugin code rather than inventing a shared runtime.
- Keep ownership outside native files; identical content alone does not
  establish ownership. Preserve unrelated settings, comments and hooks.
- Require preview revisions for writes. Compare hook-relevant fingerprints
  when other resource operations require a refreshed full revision.
- Back up native writes and restore selectively, preserving later unrelated
  edits. Report conflicts, stale previews and partial writes explicitly.
- Keep native loading and trust with each Agent. Management never executes
  supplied hooks or changes native trust.

## Evidence and limits

The [Hooks command reference](../../website/docs/reference/commands/hooks.md)
describes native formats and discovery sources. The
[domain regressions](../../internal/hooks/acceptance_test.go) and
[isolated CLI lifecycle script](../../scripts/hooks/e2e.sh) verify configuration
management with inert fixtures. Builds and tests ran in the dedicated devcontainer.

Native configuration generation was tested with inert fixtures. Native hook
execution is unverified for all ten Agents. Pi, Amp and OpenCode code must match
the installed native APIs. Multi-file writes are separate operations rather
than a filesystem transaction.

No release, push, native user configuration change or Codex daemon
change is part of this milestone.

## Actual UI acceptance and final main integration

The coordinator operated the isolated dashboard through real Chrome/CUA UI,
while supervised Claude-auto Tasks reviewed screenshots/source and repaired
confirmed defects. Actual interactions covered editing, native preview/import,
project sync, disable, selective restore, removal, target/global plans and tooltips.

Repaired defects include dropped matcher-only rows, busy-state stale warnings,
English native/conflict messages, inaccessible conflict recovery, ambiguous
settings/script labels, incorrect removal scope and global target/rail counts.
Source-only edits and scoped native writes retain their existing contracts.

The final rebase onto main `df72e98a` preserved the complete feature without
conflicts. Full `make check`, dashboard tests, TypeScript/build and scoped ESLint
passed afterward; the rebuilt backend passed actual UI smoke. Mobile at 390px
remains limited by the shared fixed sidebar. Native hook execution remains
unverified for all ten Agents. React Doctor reports remaining complexity and
semantic markup warnings.

## Account targets

Global hook bindings can now name another Claude, Codex or Pi account declared
under `targets` with `agent` and `config_dir`. This lets one source and the
existing Version 1 ownership ledger manage multiple Codex homes. Native format
selection uses the account's Agent; inventory, changes, imports and backups keep
the account name. Missing homes are skipped, and directory collisions are refused.

Two rules protect other homes. An Agent environment override pointing at a
declared account uses the plain Agent's default home instead (with a warning).
Outputs from a former home, under a removed target, or in a
missing account home are parked with ownership intact. An explicit entry-level
`--replace` can release parked ownership without changing native files.

Native regression tests cover separate homes, in-place adoption, binding removal,
Claude settings, Pi code and scripts, symlink collisions, backups, environment
shadowing, parking, API contracts and CLI sync-all. Config-aware validation also
has built-in Go fuzz coverage. The English command/config references and their
Japanese, Korean and Chinese translations describe these rules.

Lifecycle review added regressions for another account reusing a parked path,
explicit ownership release, moving a home to its ancestor, and symlinks introduced
before cleanup or backup restore. Parked registrations remain protected during
planning, and collision checks include owned cleanup outputs. Restore refuses a
backup while its target resolves to a different home. The dashboard's full native
suite passed 706 tests, including Pi account template guidance; the account picker
and source-only save were also checked in Clean/Playful light and dark modes.

Verification used the installed native toolchain with isolated test homes rather
than Docker. Docker/ssenv-only `scripts/hooks/e2e.sh`, `scripts/hooks/real-agents.sh`
and mdproof runbooks were not run. Native hook execution and trust remain the
Agent's responsibility; no user configuration rollout is part of this change.
