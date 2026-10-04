# PR 390 Codex round 2 fixes

All three findings held against reviewed commit `609e742fffe297547d00a342c77e8bdd386f5b25`. Each regression was executed and failed before its corresponding implementation change. The fixes are three separate commits on `runkids/274-codex-round2`; nothing was pushed and no GitHub thread was changed.

## Replies ready for the coordinator

### P1: Pause pruning when declaration files cannot be read

Fixed in `6844b11e021c9ae937cfc79e75176547970115ba`. Declaration read failures now survive in `FollowSet.Err()`, and `Walk`, `WalkDir`, and `ReadDir` reject incomplete declarations before invoking discovery callbacks. Missing declaration files keep their previous behavior. This also prevents reconcile from pruning metadata after a declaration read error.

Coverage: `TestFollowUnreadableDeclaration` and `TestSkillfollowSyncUnreadableDeclarationPreservesTargets`. Both declaration filenames were tested under a non-root Linux UID with mode `0000`. Before the fix, `Err()` was nil and sync removed managed targets in all four declaration-file × merge/copy combinations, even with `--force`; after the fix, sync fails with the declaration path and permission error while preserving the managed content. The tests explicitly skip on Windows and root, but neither skip applied to this run.

### P2: Forward the follow policy to group resolution

Fixed in `85d4bc31a707c067ad90c460e47b3327264e88b6`. Added `resolveGroupUpdatableWithOptions` beside the compatible wrapper and forwarded the existing operation snapshot from global/project check and update selectors. Declared groups traverse through `sourcewalk` with logical result paths; undeclared external links and additional nested link escapes remain rejected. Removed the now-obsolete raw-walk allowance, tightening the existing ratchet.

Coverage: `TestSkillfollowGroupCheckAndUpdate` failed before the fix with `resolves outside source directory` for declared groups. It now passes for global/project modes, check/update, explicit/positional selection, and top-level/nested groups, reporting `group/sub/_repo`. It also verifies undeclared-link and nested-link rejection. The unchanged `TestUpdateGroup_ExternalSymlinkRejected` passes.

### P2: Use follow-aware discovery for the doctor's missing-repo check

Fixed in `4eccbd5bc6b95e1394cd053b2c139285d2bd8aa4`. Doctor now passes its existing snapshot to `GetMissingTrackedReposWithOptions`, so source discovery and missing-repository checks agree. No other legacy tracked-repository query remains in the doctor files.

Coverage: `TestSkillfollowDoctorTrackedRepoNotMissing` failed before the fix in both global and project modes: the JSON simultaneously reported `_repo: followed` and a missing-clone warning with a rehydrate suggestion. It now passes, and removing the followed link still produces the true missing-repository diagnostic.

## Verification

All Go builds and tests ran inside the throwaway devcontainer `skillshare-274-codex-round2`, as UID/GID `1000:1000`, using `bash -c`. No product or Go test ran on the host. The container used the devcontainer image and mounted this worktree at `/workspace`.

Command prefix for the checks below:

```sh
docker exec -u 1000:1000 -e HOME=/tmp/round2-home -e GOCACHE=/tmp/round2-cache skillshare-274-codex-round2 bash -c '<command>'
```

Failing-before-fix commands:

```sh
go test ./internal/sourcewalk -run TestFollowUnreadableDeclaration -count=1
make build && go test ./tests/integration -run TestSkillfollowSyncUnreadableDeclarationPreservesTargets -count=1
go test ./tests/integration -run TestSkillfollowGroupCheckAndUpdate -count=1
go test ./tests/integration -run TestSkillfollowDoctorTrackedRepoNotMissing -count=1
```

Passing-after-fix commands:

```sh
go test ./internal/sourcewalk -count=1
make build && go test ./tests/integration -run 'TestSkillfollowSync(UnreadableDeclarationPreservesTargets|PausesPruneForMissingEntry)' -count=1
make build && go test ./tests/integration -run 'TestSkillfollowGroupCheckAndUpdate|TestUpdateGroup_ExternalSymlinkRejected' -count=1
go test ./internal/sourcewalk ./cmd/skillshare -count=1
make build && go test ./tests/integration -run TestSkillfollowDoctorTrackedRepoNotMissing -count=1
```

`make check` passed (exit 0): documentation routing, formatting, `go vet`, unit tests, and integration tests. Both `TestRawWriteRatchet` and `TestRawWalkGuard` passed. All four new top-level regression tests passed without being skipped. Final tail:

```text
=== RUN   TestXDG_StatusWorksWithXDGPath
--- PASS: TestXDG_StatusWorksWithXDGPath (0.03s)
PASS
ok  skillshare/tests/integration 54.921s

✓ All tests passed!
```

## Limits and handoff

The throwaway container was stopped and removed after verification. The context-router check also passed after writing this report.

Real Windows execution was not performed; these results establish Linux behavior only. Follow sets remain operation snapshots rather than filesystem locks. This handoff report is intentionally uncommitted; the three fix commits contain only their implementation and tests. The coordinator owns integration into the PR branch and Codex replies.
