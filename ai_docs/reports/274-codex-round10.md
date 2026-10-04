# PR 390 Codex round 10 fixes

All four findings held against reviewed commit `e63548fb`. Each new regression test failed before its fix and passes after it. The fixes are three commits on `runkids/274-codex-round10` (findings 3 and 4 share one collection change). Nothing was pushed and no GitHub thread was changed.

## Replies ready for the coordinator

### P1: Fail closed on incomplete follow snapshots

Fixed in `929516b4`. The described path is confirmed. The dashboard named route (`updateSingleByKind` → `resolveTrackedRepo`) and the named stream route both find `_dev` with `install.IsGitRepo` (an `os.Stat`) before any follow-aware walk. With `.skillfollow` unreadable, the follow set has an error but no entries, so `PrepareFollowedUpdate` returned a nil policy and the request fell through to `git.ForcePullWithAuth`. Before the fix, both routes reported `"action":"updated"` with `force=true` and moved the external checkout's HEAD. `install --update` reached the same pull through `install.updateTrackedRepo` and also moved HEAD.

`PrepareFollowedUpdate` and `RefuseFollowedSkillUpdate` now return `follow.Err()` before classifying ownership, wrapped as `followed repository update refused: skillfollow discovery is incomplete: read skillfollow declaration <path>: ...`. Because it wraps `ErrFollowedUpdate`, batch callers report it as a per-item failure. This covers the CLI, the dashboard (named, update-all, stream), and `install --update`, with and without force. A nil follow set still returns before the check.

Coverage:

- `TestServerFollowedNamedUpdateUnreadableDeclaration` (internal/server): `POST /api/update {"name":"_dev","force":true}` and `GET /api/update/stream?names=_dev&force=true` both return an error item naming the declaration read failure. The external HEAD does not move.
- `TestSkillfollowUpdateUnreadableDeclarationRefuses` (integration): `install <url> --track --name dev --update` fails with `read skillfollow declaration` and leaves HEAD unchanged. Its `update _dev --force` case already passed before this fix, because CLI name resolution (`resolveByBasename`) runs a follow-aware walk that returns the declaration error. It is kept as coverage.

### P1: Pass the follow snapshot through the initial sync

Fixed in `186d2caa`. Confirmed. `firstSync` used the snapshot for discovery but built every `syncTargetEntry` without `follow`, so `runParallelSyncQuiet` pruned without the unavailable-entry pause. Before the fix, `init --remote` against a repo whose `.skillfollow` declares `_off` (absent on the new machine) removed an existing target link `_off__c` that points into the source. Init printed no pause.

`firstSync` now keeps one snapshot (the one it already computed for discovery) and attaches it to every entry, as `sync` and `sync -p` do. After the quiet run, it prints the same `<target>: prune paused; unavailable .skillfollow entry: ...` and `kept N managed copies ...` warnings that `sync` reports. It prints no other sync warnings, so output without `.skillfollow` is unchanged.

Coverage: `TestInit_Headless_RemoteWithMissingFollowEntry_PausesPrune` (integration). The pulled repo declares `_off`. The claude target already holds `_off__c`. After init, the link survives, `prune paused` is printed, and the repo's `tdd` skill is still linked.

### P2: Apply the refusal during dashboard update-all

Fixed in `7c8a62c9`. Confirmed. `getServerUpdatableSkills` walked with `sourcewalk.Options{}`, so the first-level `group` link was not followed. A metadata-backed `group/g` never reached `updateRegularSkill`, and `POST /api/update {all:true}` and the stream returned only `_dev` and `_ordinary`.

`getServerUpdatableSkills` now takes `sourcewalk.Options`. With a snapshot, it walks the logical source root (as `GetTrackedReposWithOptions` does) and returns `follow.Err()` after the walk. A new `Server.collectUpdateAll` collects repos and skills with one snapshot, and both the JSON and SSE update-all paths use it. Each skill below a followed group now appears as `{"name":"group/g","action":"error","message":"followed repository update refused: skill group/g is inside followed entry group; ..."}`, and other items still update.

Coverage: `TestServerFollowedGroupSkillUpdateAllRefused` (JSON and SSE). It checks the per-item refusal for `group/g` and that `_ordinary` still updates.

### P2: Report declaration errors from dashboard update-all

Fixed in `7c8a62c9` (same collection change). Confirmed. Before the fix, an unreadable `.skillfollow` gave `200 {"results":null}` from the JSON route, and a stream with `start {"total":0}` and `done`.

When a snapshot is active and collection fails, the JSON route now returns HTTP 500 with the error. It writes an `update` ops-log entry with status `error`. The stream sends `event: error` with `{"error": ...}` and stops before `start`. Without a snapshot (no declaration files), a walk error keeps the old behavior: the affected list is empty and the request continues.

Coverage: `TestServerFollowedUpdateAllUnreadableDeclaration` (JSON and SSE, with `force=true`). It checks status 500 or `event: error`, the declaration path in the message, no `done` event, and that neither the external nor the ordinary checkout moved.

## User-visible behavior changes (for the docs update)

1. **Unreadable declaration refuses every update.** When `.skillfollow` or `.skillfollow.local` exists but cannot be read, these all refuse with `followed repository update refused: skillfollow discovery is incomplete: read skillfollow declaration <path>: ...`: updating any tracked repo or metadata-backed skill in that source, `install --update` of a tracked repo, and dashboard named or streamed updates. `force` does not bypass the refusal.
2. **Dashboard update-all fails on incomplete discovery.** With declarations present, an unreadable declaration (or a traversal read error) makes `POST /api/update {all:true}` return HTTP 500, and makes `/api/update/stream` emit `event: error`. It no longer returns an empty success.
3. **Dashboard update-all lists skills inside followed groups.** Metadata-backed skills below a followed non-`_` group now appear in update-all results as per-item errors (`skillshare does not reinstall skills in a followed tree`), matching CLI `update --all`. Before, they were silently omitted.
4. **`init` honors the prune pause.** When the source that `init` pulls declares an entry missing on this machine, init's first sync pauses prune like `skillshare sync`, keeps existing links into that entry, and prints `<target>: prune paused; unavailable .skillfollow entry: <name>` (plus `kept N managed copies ...` for copy-mode targets).

Without any `.skillfollow` file, all output is unchanged. The follow set is nil, so every new check is skipped, and the new init warnings need a non-empty pause or kept list.

## Evidence

All Go commands ran in a throwaway container from the devcontainer image (compose project `skillshare_wt_r10`, this worktree at `/workspace`), as UID 1000 so the `chmod 000` tests did not skip. `-v` output shows each new test as `PASS`, not `SKIP`. Nothing ran on the host.

Before each fix (tests added first, implementation unchanged):

```text
TestServerFollowedNamedUpdateUnreadableDeclaration/json: {"results":[{"name":"_dev","action":"updated","message":"1 commits, 1 files changed","isRepo":true}]}
TestServerFollowedNamedUpdateUnreadableDeclaration/sse:  event: result ... "name":"_dev","action":"updated"
TestSkillfollowUpdateUnreadableDeclarationRefuses/install_--update: expected failure, but command succeeded; update moved the followed checkout
TestInit_Headless_RemoteWithMissingFollowEntry_PausesPrune: no "prune paused"; lstat .../.claude/skills/_off__c: no such file or directory
TestServerFollowedGroupSkillUpdateAllRefused/{json,sse}: results only _dev and _ordinary (group/g missing)
TestServerFollowedUpdateAllUnreadableDeclaration/json: 200 {"missingTrackedRepos":null,"results":null}
TestServerFollowedUpdateAllUnreadableDeclaration/sse:  event: start ... event: done (no error event)
```

After the fixes, `make check` passed (exit 0) as UID 1000. It covered docs-check, fmt-check, vet/lint, unit tests, and integration tests. The `internal/sourcewalk` and raw-write ratchets passed, and none of the new tests was skipped. Tail:

```text
=== RUN   TestXDG_StatusWorksWithXDGPath
--- PASS: TestXDG_StatusWorksWithXDGPath (0.03s)
PASS
ok  	skillshare/tests/integration	55.311s

✓ All tests passed!
```

## Notes and limits

- `updateTrackedRepo` and `updateRegularSkill` in the server still build a fresh follow set per item, as before. Update-all now builds one more snapshot to collect items. The per-item sets see the same files unless the source changes during the request.
- **Dashboard display.** `ui/src/api/http.ts` `createSSEStream` sends the stream's `event: error` to its generic `error` listener. That listener closes the stream and shows the caller's fixed error message, not the server's text, as for the check, audit, and diff streams. The JSON route's 500 becomes an `ApiError` carrying the server message. No UI code changed.
- **Not run:** real Windows, and frontend tests (no UI change).
