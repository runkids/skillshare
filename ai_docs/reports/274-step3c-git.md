# Skillfollow step 3c: Git, update, and audit handoff

## Scope and implementation

This slice implements the proposal's followed-update policy, resolved-root audit gates, staging checks, source-repository mutation checks, source-change refusal, and doctor diagnostics. No sync ownership or configuration semantics were changed, and existing tests were not modified.

The shared update policy lives in `internal/install/followed_update.go`; `internal/git/followed_update.go` adapts it to `UpdateInfo` without reversing the existing git-to-install dependency. `PrepareFollowedUpdate` checks the operation's FollowSet before dry-run output or mutation, resolves the repository, rejects force, and fails closed on dirty/status errors. Pull uses `--ff-only --no-rebase` without restore, merge-abort, or cleanup recovery. Ordinary repositories retain the legacy path. CLI single/quick/batch/project, dashboard single/all/SSE, and tracked `install --update` consume this policy. Batch errors remain per-item failures and independent ordinary repositories continue. JSON install's implicit overwrite flag is distinguished from explicit force; followed refusals include an `error` field.

Audit rollback remains a hard reset to the pre-pull hash in all three existing audit gates. The update help explicitly documents this exception to the force refusal. `audit.ScanResolvedSkill` scans the canonical root but reports the logical root; private `Result.scannedFiles` supports a zero-coverage check. Eligibility matches the scanner's depth, hidden-directory, size, metadata, regular-file, extension, and binary filters; it does not introduce a new Git-ignore interpretation. CLI single/quick, dashboard including SSE, install audit/re-discovery, and fallback-group audit use the resolved root. Normal scans retain their prior behavior.

`git.FollowedLinksStaged` uses an additive `FollowSet.SourceRoot()` query to locate a declaration's physical parent without resolving its final link. It checks containment and intervening links, then staging guards reject indexed or unignored entries with exact anchored no-slash ignore lines and shell-quoted untrack commands. Commit, push, their dry runs, both dashboard staging paths, and init's source commit use the guard. Doctor reports missing ignore entries and unsafe `.skillfollow.local` tracking without writing ignore files. `install.FollowedIgnoreLine` escapes Git pattern metacharacters; existing `RemoveFromGitIgnore` remains the removal API.

`git.CheckSourceMutation` pins the incoming commit, refuses reachable indexed declarations (including missing indexed links), and checks NUL-delimited incoming paths with Lstat for declared or undeclared link components. Source pull fetches once and applies the checked hash; first pull, init reset/legacy remote setup, and dashboard checkout use the same check. Discard is unchanged. Checkout checks the selected existing local or remote-tracking revision; it does not add an implicit fetch to the existing checkout contract.

## Discovery callers and integration seams

`GetTrackedReposWithOptions` now receives operation-owned FollowSets from CLI check (global/project), update single/multiple/all resolution, dashboard check/check-stream, update-all/SSE, missing-tracked-repository queries, and rehydrate. `GetMissingTrackedReposWithOptions` is additive; no-option wrappers remain compatible. Global/project audit discovery also receives the set. CLI uses `skillFollowSet`/`globalSkillFollowSet`; server uses `serverSkillFollowSet` and `(*Server).skillFollowSet` with configured targets and the effective global/project Git root.

`InstallOptions.Follow` carries policy across install/query seams. The server source-change refusal runs before metadata lookup, so a followed repository cannot evade the refusal by lacking metadata. Slice 3d independently owns the same `internal/server/skillfollow.go` helper signatures: keep its equivalent helper when integrating and retain this slice's caller changes. The coordinator approved the install-handler plumbing and fallback audit ownership during implementation.

## Verification

All Go commands ran in the throwaway devcontainer, never on the host. Added tests cover:

- Physical staging reachability across skill/agent/extra/custom roots, alias-in/source-out layouts, indexed/unignored/trailing-slash cases, doctor, CLI/dashboard staging, and dry runs.
- Declared/undeclared incoming link paths, indexed and missing indexed declarations, NUL/newline paths, CLI pull/init and dashboard pull/checkout refusals, unchanged links, safe pulls, and discard preservation.
- Dirty, force, divergence, and status-error refusals in mixed CLI global/project batches and dashboard single/all/SSE, with ordinary items continuing.
- Malicious pulled child content and rollback through CLI single/quick, dashboard/SSE, and install; zero-eligible scan rejection and rollback; logical fallback-group audit paths.
- Actual CLI `update --all --json` global/project output and `install --update --json` implicit-versus-explicit force behavior.

The initial linked-root audit regression failed before the scanner adapter and passed afterward. The JSON install regression exposed missing refusal text and passed after the scoped output fix. Existing tests remain unchanged. Linux `make check` (including both ratchets and integration tests), the context-router check, and Windows amd64 cross-compilation of `internal/git` and `internal/sourcewalk` passed.

Final `make check` tail:

```text
=== RUN   TestXDG_StatusWorksWithXDGPath
--- PASS: TestXDG_StatusWorksWithXDGPath (0.04s)
PASS
ok  skillshare/tests/integration 67.968s

✓ All tests passed!
```

Implementation commits:

- `37e6a4b7`: shared followed-update policy, audit gates, discovery plumbing, and corresponding tests.
- `508d9cdf`: staging/source-mutation guards, additive SourceRoot query, doctor diagnostics, and corresponding tests.

## Limits and follow-up

Real Windows junction execution was not available; cross-compilation is not runtime evidence. FollowSet and Git revision checks are operation snapshots, not a filesystem lock against concurrent external edits. No public website or translated documentation was changed; update help now states that audit rollback still hard-resets. No push or pull request is part of this slice.
