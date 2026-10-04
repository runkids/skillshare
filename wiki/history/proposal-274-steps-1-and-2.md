# Proposal 274 — steps 1 and 2

## Outcome

PR [#385](https://github.com/runkids/skillshare/pull/385) shipped the visibility,
walker and write-boundary foundation of [proposal 274](../../proposals/274-skillfollow.md).
It merged to main as squash commit `8c60f51b9f36939ef2e5233bd21c3aa39054319a`.
This milestone does not parse `.skillfollow` or follow child links:
`internal/sourcewalk/walk.go` still has an empty `Options` and resolves only the
source root.

- `checkUndeclaredSourceLinks` (`cmd/skillshare/doctor.go`) reads through
  `sourcewalk.ReadDir` and reports first-level symlinks and junctions without
  following them. Text uses an info row named "Source link"; JSON checks use
  `undeclared_source_links`. They do not add warnings or errors.
- Thirteen raw source scans moved onto `sourcewalk.Walk`, `WalkDir` or `ReadDir`,
  including the originally omitted `(SkillKind).Discover`
  (`internal/resource/skill.go`). Traversal preserves the existing non-following
  behavior; `internal/sourcewalk/walk_test.go` contains parity tests.
- `internal/sourcefs/sourcefs.go` wraps an `os.Root` and checks path components
  for links before writes. Migrated callers cover `--into` and config-install
  directories, install overwrite and staged replacement, dashboard skill content
  edits, uninstall edge checks, and the `init` built-in fallback. The entry points
  include `sourcefs.MkdirAllIn`, `swapStagedIntoSource`
  (`internal/install/source_write.go`), `writeSourceFileAtomic`
  (`internal/server/handler_skill_content.go`), `sourcefs.CheckMoveOut`, and
  `installBuiltinSkill` (`cmd/skillshare/init_apply.go`).

The one intended behavior change is refusal of an explicit source write path
crossing a link at migrated sites, including replacing a final-component link,
instead of writing through or replacing it. The source root may itself be a link;
ordinary paths remain writable. `CheckMoveOut` checks parents but permits moving
the final link itself without following it (`internal/sourcefs/sourcefs.go`).

## Ratchets and review

Allowlist sizes are the merge-time snapshot at `8c60f51b`, not a claim about later
migration progress:

| Guard | Allowlist | Rows |
|---|---|---:|
| `TestRawWalkGuard` (`internal/sourcewalk/guard_test.go`) | `internal/sourcewalk/allowlist.json` | 123 |
| `TestRawWriteRatchet` (`internal/sourcefs/ratchet_test.go`) | `internal/sourcefs/testdata/raw_writes.tsv` | 509 |

Both use `go/parser`, classify individual occurrences, and reject new unclassified
calls and stale allowances. The walk guard also requires classification notes.
Their reason vocabularies differ and are documented alongside the allowlists in
`walkReasons` and `validReasons`. Both are syntactic checks, not data-flow proofs.

The Codex review fix closed the cross-device fallback escape:
`swapStagedIntoSource` falls back from `sourcefs.Root.MoveIn` only when
`sourcefs.IsCrossDevice` matches `EXDEV` on Unix or `ERROR_NOT_SAME_DEVICE` on
Windows (`internal/sourcefs/crossdevice_unix.go`, `crossdevice_windows.go`).
`sourcefs.Root.CopyIn` creates every destination directory and file through the
root; other rename failures are returned, not retried with a raw copy.
`TestSwapStagedIntoSourceCrossDevice` (`internal/install/source_write_linux_test.go`)
contains the `/dev/shm` cross-filesystem regression fixture.

The squash commit records the **Release-As: 0.24.3** decision. This milestone
records that requested version, not evidence that a release was published.

## Residual and verification limits

At merge, 69 of the 509 write allowances still used `source-unmigrated`: skills
source writes not yet moved onto the handle. A follow-up branch is migrating them;
this log does not claim completion. Step 3 must not start until that list is empty
or every remaining row has a documented reason for staying raw. The allowlist and
`validReasons` are the evidence for this residual, rather than an assertion that
all source mutations are already bounded.

Parsing, classification, followed-entry identity and ownership, unavailable-entry
pauses, repo-root audit fixes, Git seams and followed-update policy remain step 3
design. The followed-entry per-command behavior matrix also remains pending;
`tests/integration/source_write_boundary_test.go` covers the shipped link-write
refusals, not following.

This documentation reconciliation inspected the merged source and commit message
and counted both allowlists. The host context-router check passed; Go tests and
the CLI were not run for this documentation-only task.
