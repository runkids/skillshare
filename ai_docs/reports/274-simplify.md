# #274 `.skillfollow`: simplification batch

PR #390 (`runkids/274-step3`). This batch applies the four read-only reviews (reuse, simplification, efficiency, altitude) of the production diff against `main`. None of the commits is meant to change user-visible behavior: CLI text, JSON, and HTTP status codes stay the same. Test edits are mechanical signature updates, with one exception (item 3), which the coordinator approved.

## Items

Line counts cover non-test files only (`git show --shortstat -- ':!*_test.go'`).

| # | Commit | What changed | +/− |
|---|---|---|---|
| 1 | `719bcb325` refactor(skillfollow): make FollowSet queries nil-safe | Every `FollowSet` query method now has a pointer receiver and returns the zero value on a nil set, as `WriteBoundary` already did. 14 guard-only `follow != nil &&` prefixes are gone, and so are the `skillfollowPauses` and `followScope.unavailable` wrappers. Nil checks that decide something else stay: `handler_overview.go:51` (it also gates `readErr`), `sync.go:793/1020`, `install_handlers.go:34`, `uninstall_handlers.go:242`, and `handler_diff_stream.go:255`. | +71 −52 |
| 2 | `05c9a694a` refactor(skillfollow): share one snapshot constructor | The body now lives once, in `sync.FollowSetFor`. cmd's `skillFollowSet` is a one-line call to it, kept so its ~30 callers don't change. The server's `followSetFor` is deleted. | +25 −35 |
| 3 | `4f869a4ec` perf(skillfollow): return early from Follow without declarations | With no declaration file and no read error, `Follow` returns right after `readDeclarations`. It skips canonicalizing the targets and git root, and skips listing the root. | +3 −0 |
| 4 | `10a27bc7e` refactor(skillfollow): name the declared and available entry states | Adds `State.Declared()`, `State.Available()`, and `FollowSet.Declared()`, and uses them in sourcewalk, doctor, status, follow, and the server's skillfollow view. | +19 −20 |
| 5 | `4b2ab649f` refactor(skillfollow): drop the unused source path from prune helpers | `PrunePaused(set)` and `KeepsManagedCopies(naming, set)`. `followScope.paused` is folded into `PrunePaused`, which merge and copy sync now call directly. | +13 −18 |
| 6 | `b69e9617f` refactor(skillfollow): call WriteBoundary directly from dashboard handlers | `followedSkillWriteError` is deleted and its 10 callers use `WriteBoundary`. Also removes 13 redundant `filepath.ToSlash` calls in front of `InFollowed`/`WriteBoundary`. | +22 −29 |
| 7 | `56bcd77b0` refactor(skillfollow): reuse followWriteStatus in the install handler | Both hand-written 409/500 mappings in `handler_install.go` now call `followWriteStatus`. Their fallback was already 500. | +2 −10 |
| 8 | `840e731a9` refactor(git): merge UntrackCommand and UntrackCommandIn | Now one `UntrackCommand(dir, path)`, which omits `-C` when `dir` is empty. The printed strings are unchanged. | +10 −12 |
| 9 | `1ae6b4a56` perf(skillfollow): reuse the request snapshot in dashboard updates | The update stream passes the request snapshot to each item instead of building one per item under `s.mu.Lock()`. `updateRegularSkill` and `auditGateTrackedRepo` take the snapshot as a parameter, and `updateAll` passes its own. | +7 −9 |
| 10 | `3f12b2d3c` perf(skillfollow): look up followed children only at the source root | Both walkers compute `atRoot` once per directory and call `followedChild` only at the root. | +12 −2 |
| 11 | `7907910f3` refactor(skillfollow): keep one followed-target containment check | `Owns` is renamed `FollowedTargetContains` and keeps only its followed-target loop. `inFollowedTarget` calls it. | +5 −15 |
| 12 | `3dffd0fea` refactor(ui): share the discovery query invalidation in ConfigPage | Adds `invalidateDiscovery(queryClient)` to `ui/src/lib/sync.ts`, next to `invalidateAfterSync`, and calls it from the three save paths. | +12 −12 |

### Deviations from the brief

- **Item 3, test change (approved by the coordinator).** `junction_walk_test.go` read an `UndeclaredLink` entry from a set that had no declaration file. That assertion now runs on a set that declares an unrelated name (`other`). The not-traversed check still runs on the inactive set. No production caller reads entries from an inactive set: the constructors return nil for one, and the bare `Follow` callers only use `WriteBoundary`.
- **Item 4: the predicates are on `State`, not `Entry`.** `follow_handlers.go:335` checks `followResult.State`, a bare `State`. Methods on `State` cover that site and the `Entry` sites (`entry.State.Available()`). `InFollowed`'s "declared and not a real directory" test matches neither predicate, so it stays as written.
- **Item 6: `WriteBoundary` itself is unchanged.** `InFollowed` already applies `filepath.ToSlash(filepath.Clean(...))`, and `WriteBoundary` passes `logicalRel` straight to it. The wrapper's `ToSlash` was redundant. The `TestFollowWriteGuard` allowlist did not need changes.
- **Item 9, one theoretical difference.** For agent repositories, `updateTrackedRepo` receives a nil snapshot (`handler_update.go:214`, unchanged), so `auditGateTrackedRepo` now sees nil for them too. Before, it built the skills snapshot and asked `InFollowed` about the agent repo's path relative to the skills source. That path starts with `..` unless the agents source sits inside a declared entry of the skills source, so the answer is the same in every supported layout. Sharing the snapshot across items is safe: neither helper walks the source root with it, so no item can mark an entry missing for a later one.
- **Item 11: `inFollowedTarget` cannot call `Owns` directly.** `Owns` also counted the canonical source root. `followedLink` (merge sync's managed-link prune) must not count a link into the source itself. Only the followed-target loop was kept, and its existing test assertions carry over unchanged.

## Verification

All of this ran in a throwaway container (`skillshare_wt_simplify274`, devcontainer image, this worktree at `/workspace`). Nothing ran on the host.

- `make check`: exit 0. `TestRawWalkGuard`, `TestRawWriteRatchet`, `TestFollowSnapshotGuard`, and `TestFollowWriteGuard` pass.
- `GOOS=windows go vet ./...`: pass.
- `cd ui && pnpm exec vitest run`: 99 files, 891 tests passed. `pnpm run build`: pass.
- `git diff --stat origin/main...HEAD -- '*.go' ':!*_test.go' | tail -1`:
  - before: `143 files changed, 4971 insertions(+), 909 deletions(-)` (net +4062)
  - after: `143 files changed, 4963 insertions(+), 914 deletions(-)` (net +4049)

The batch removes 13 net production Go lines. Item 1 accounts for most of the offset: each nil-safe receiver adds a 3-line guard.

## Follow-ups (out of scope for this batch)

- Store the logical source root in `FollowSet` (about 33 signatures change).
- Have the walker return traversal errors.
- Move the write boundary into `sourcefs.Root`.
- Batch the `git ls-files` and `check-ignore` calls.
- Cache canonical parent directories in merge sync.
- Define non-nil empty snapshot semantics in `install_apply.go`.
- Unify the doctor and git followed-link checks.
- Derive `unfollowResult` fields (changes the JSON shape).
- Export `ParseDeclarations`.
- Move the declaration-file constants.
- Remove the double checks in commit and push.
- Remove the repeated snapshots in `hub.go` and `init.go`.
