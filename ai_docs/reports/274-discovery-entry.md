# Proposal 274: discovery entry hardening

Follow-up to PR #390. Twelve Codex review rounds kept finding the same defect: a command or handler reached source discovery without the operation's `.skillfollow` snapshot, or walked with it and never checked `FollowSet.Err()`. This change closes that class with code structure and a ratchet test. It does not add more call-site patches. All work is on `runkids/discovery-entry`. Nothing was pushed.

## Inventory

The table covers every call in `cmd/skillshare` and `internal/server` (non-test) that discovered skills, tracked repos, or updatable skills, or walked the source, without receiving the operation's snapshot. The new ratchet found the last six rows. "Not observable" means the CLI and API output cannot differ: the decision is recorded and no regression test applies.

| Call | Decision | Test |
|---|---|---|
| `install.GetUpdatableSkills(sourceDir)` in `handler_check.go:65`, `handler_check_stream.go:48`, `check.go:280`, `update_resolve.go:39`, `:78` | Threaded through the new `GetUpdatableSkillsWithOptions`, which returns `Err()` itself. Not observable: the list comes from metadata, and every caller first runs `GetTrackedReposWithOptions` with the same snapshot and stops on its error. | Existing `TestServerSkillfollowCheckUnreadableDeclaration` and the CLI/server behavior matrices |
| `ssync.DiscoverSourceSkillsAll(source)` in `disabledByRelPath` (`handler_toggle.go:147`), single and batch toggle re-check | Threaded with the request snapshot. Not observable: toggles of skills inside a followed entry are refused (409) before the write, and both walks agree on physical skills. | Existing matrix rows `toggle`, `toggle-repo`, `toggle-batch` |
| `collectInstalledSkillPaths(sourcePath)` (no snapshot) in `discoverForKind` (`audit.go:515`), used by the audit TUI's second tab | Threaded through `auditTUIContext.follow`. **Gap closed:** skills below a followed group were missing from that tab. TUI only. | `TestDiscoverForKindSkillsUsesFollowSnapshot` fails without the fix |
| `sourcewalk.ReadDir(source, sourcewalk.Options{})` in `checkSkillsValidity` (`doctor.go:939`) | Threaded. **Gap closed, user-visible:** a followed entry with no skills was never validated; `doctor` now reports it under `skills_validity` like a physical directory. | `TestCheckSkillsValidityIncludesFollowedEntries` fails without the fix |
| `sourcewalk.ReadDir(..., sourcewalk.Options{})` in `checkSource` (`doctor.go:459`) | Kept and allowlisted. It is the error-path count after follow-aware discovery already failed; the skillfollow checks report that failure. Threading it would only turn the count to 0 in that state. | Ratchet allowance |
| `sourcewalk.ReadDir(root, sourcewalk.Options{})` in `checkUndeclaredSourceLinksWithFollow` (`doctor.go:424`) | Kept and allowlisted. It runs only when the snapshot is nil and lists first-level links as links on purpose. | Ratchet allowance |
| `s.resolveTrackedRepo(name)` in `updateSingleByKind` (`handler_update.go:185`) | The variadic fallback called `s.skillFollowSet()` inside the helper; the snapshot is now passed explicitly. No change. | Existing `TestServerFollowedNamedUpdateUnreadableDeclaration` |
| `resolveGroupUpdatable` (`update_resolve.go:92`), `buildTrackedRepos` (`handler_overview.go:110`) | Follow-less wrappers with no caller: deleted. | Compiler |
| `checkUndeclaredSourceLinks`, `buildTargetSkillSyncSummary`, `extractTrackedRepos` (forwarded `nil` to their `*WithFollow` forms) | The first two had only test callers and moved into `cmd/skillshare/discovery_follow_test.go`; `extractTrackedRepos` had none and is deleted. | Ratchet `argument` rule found them |
| `install.InstallOptions{Update: true, ...}` without `Follow` in `makeInstallOpts` (`update_batch.go:54`) and `updateRegularSkill` (`handler_update.go:458`) | Threaded. Not observable: `InstallOptions.Follow` is read only for tracked-repository updates and rehydrate. | Existing update tests |
| `install.InstallOptions{Kind: "agent", Update: true, ...}` in `batchUpdateAgents`, `reinstallAgent`, `(*Server).updateAgent` | Kept and allowlisted: agents source, outside `.skillfollow`. | Ratchet allowance |
| `auditOptions{}` in `parseAuditArgs` | Kept and allowlisted: a flag-parse result; `cmdAudit` sets `Follow` before any scan. | Ratchet allowance |

Searches that found nothing in `cmd/skillshare` or `internal/server`: `DiscoverSourceSkills(`, `DiscoverSourceSkillsLite(`, `GetTrackedRepos(`, `GetMissingTrackedRepos(`, `Follow: nil`, `resource.SkillKind{}.Discover`. Every `sourcewalk.Walk`/`WalkDir` with a snapshot already checked `Err()`.

`tryPullAfterRemoteSetup` (`init.go`) still reads a fresh snapshot with `configuredSkillFollowSet` after the pull, because the pull can bring a new `.skillfollow`. The variable is renamed `pulled` so it no longer shadows the parameter.

## Structural changes

1. **Plain parameter.** 23 helpers in `cmd/skillshare` and `internal/server` took `follows ...*sourcewalk.FollowSet` only to keep old call sites compiling. They now take `follow *sourcewalk.FollowSet`, so omitting the snapshot fails to compile, and passing `nil` is a visible choice that the ratchet flags. `firstFollowSet` is gone. Commands still obtain the snapshot once from their loaded config (`globalSkillFollowSet(cfg)`, or `skillFollowSet(source, targets, root)` in project mode). The server takes one per request with `s.skillFollowSet()`.
2. **No follow-less public twins.** The short discovery forms that only tests still used moved into `_test.go` files under the same names, so plain-behavior tests compile unchanged and production code cannot call them. Unused ones are deleted. Discovery now has one entry per shape, and each takes options carrying `Follow`.
3. **Errors surface from discovery.** `DiscoverSourceSkills*WithOptions`, `GetTrackedReposWithOptions`, `GetMissingTrackedReposWithOptions`, the new `GetUpdatableSkillsWithOptions`, `getServerUpdatableSkills`, and `buildTrackedReposWithOptions` all return `Err()` themselves, and a nil snapshot returns nil. `sourcewalk.ReadDir` already returns its read failures. `sourcewalk.Walk`/`WalkDir` keep `filepath.Walk` semantics, because reconcile deliberately tolerates a failed followed subtree: it marks the entry missing and keeps its metadata. The ratchet's `err-check` rule therefore makes every command or handler that uses them call `Err()`.

Not converted: the `follows ...` variadics in `internal/git` (`StageAll`, `FirstPull`, `Checkout`, `PullWithEnv`, `PullWithResolution`). `git.Pull` legitimately calls them without a snapshot for tracked-repo pulls. The ratchet flags any call from `cmd/skillshare` or `internal/server` that omits the argument; none do today.

## Ratchet

`TestFollowSnapshotGuard` (`internal/sourcewalk/follow_guard_test.go`) mirrors `TestRawWalkGuard`: it parses with `go/parser`, uses import-aware matching, fingerprints each candidate, and fails on new and on stale allowances (`follow_allowlist.json`). It learns from declarations in all of `cmd/` and `internal/`: types with a `Follow *sourcewalk.FollowSet` field, and functions and methods with `*FollowSet` parameters. It then checks `cmd/skillshare` and `internal/server` for four rules:

| Rule | Flags |
|---|---|
| `options` | A literal of such a type that omits `Follow` or sets it to nil (keyed or positional). `install.InstallOptions` counts only when it sets `Update`. |
| `argument` | `nil` for a `*FollowSet` parameter, or an omitted `...*FollowSet` variadic |
| `api` | A follow-less discovery API with no options form: `resource.SkillKind{}.Discover` |
| `err-check` | A follow-aware `sourcewalk.Walk`/`WalkDir` in a function with no `.Err()` call |

A new option type or snapshot-taking function is covered without editing the test. Like the raw-walk guard, the check is syntactic: methods match by name within their package, and data flow is not traced.

Final allowlist (6 entries):

| File | Function | Rule | Reason |
|---|---|---|---|
| `cmd/skillshare/audit.go` | `parseAuditArgs` | options | Flag parsing result; `cmdAudit` sets `Follow` from the loaded config before any scan |
| `cmd/skillshare/doctor.go` | `checkSource` | options | Error-path count of physical first-level directories after follow-aware discovery failed |
| `cmd/skillshare/doctor.go` | `checkUndeclaredSourceLinksWithFollow` | options | Runs only when the snapshot is nil, to list first-level links without following them |
| `cmd/skillshare/update_agents.go` | `batchUpdateAgents` | options | Agent reinstall into the agents source |
| `cmd/skillshare/update_agents.go` | `reinstallAgent` | options | Agent reinstall into the agents source |
| `internal/server/handler_update.go` | `(*Server).updateAgent` | options | Agent reinstall into the agents source |

Negative tests: `TestFollowGuardRejectsFollowLessCalls` injects one call per rule (missing `Follow`, `Follow: nil`, positional nil, `Update` without `Follow`, nil argument, omitted variadic, `SkillKind{}.Discover`, walk without `Err()`). Each must produce exactly one gap and an unclassified failure. `TestFollowGuardAcceptsThreadedSnapshot` shows that threaded calls and files outside the two directories pass. `TestFollowGuardRejectsStaleAndDuplicateAllowances` covers stale, duplicated, unreasoned, and unknown-rule entries. As a live check, changing `check.go`'s `GetUpdatableSkillsWithOptions(sourceDir, sourcewalk.Options{Follow: follow})` to `sourcewalk.Options{}` failed the guard with:

```text
discovery without the follow snapshot (1, a literal of a type with a Follow *FollowSet field that omits Follow or sets it to nil): {File:cmd/skillshare/check.go Function:runCheck Rule:options Expr:sourcewalk.Options{}}
```

Both existing ratchets (`TestRawWalkGuard`, `TestRawWriteRatchet`) stay green with unchanged allowlists.

## Closed gaps

| Gap | User-visible | Commit |
|---|---|---|
| `doctor` never validated followed entries: a followed entry without any skill is now reported under `skills_validity` (text and `--json`) | **Yes**, `doctor` only, and only with `.skillfollow` | `78770ec4` |
| The audit TUI's second-tab skills scan omitted skills below followed groups | TUI only (see the pre-existing issue below) | `16b873d2` |
| Toggle re-check discovered without the snapshot | No (refused before the write) | `d21c9b61` |

No other output changes. Without `.skillfollow` every snapshot is nil, and every path is unchanged.

## Renamed or removed functions

For the workers rebasing `follow`/`unfollow` and the dashboard `.skillfollow` tab:

| Package | Before | After |
|---|---|---|
| `internal/sync` | `DiscoverSourceSkills`, `DiscoverSourceSkillsLite`, `DiscoverSourceSkillsAll`, `DiscoverSourceSkillsForAnalyze`, `DiscoverSourceSkillsWithStats` | Test-only. Use `DiscoverSourceSkillsWithOptions` (`CollectIgnored` for stats), `DiscoverSourceSkillsLiteWithOptions`, `DiscoverSourceSkillsAllWithOptions`, `DiscoverSourceSkillsForAnalyzeWithOptions` |
| `internal/sync` | `SyncTargetMerge`, `PruneOrphanLinks` | Test-only. Use `SyncTargetMergeWithSkills` or `SyncTargetMergeWithSkillsOptions`, and `PruneOrphanLinksWithSkills` |
| `internal/sync` | `DiscoverSourceSkillsWithStatsAndContext`, `PruneOrphanCopies` | Deleted (no callers). Use the options forms, `PruneOrphanCopiesWithOptions` |
| `internal/install` | `GetUpdatableSkills(sourceDir)` | `GetUpdatableSkillsWithOptions(sourceDir, sourcewalk.Options)` |
| `internal/install` | `GetTrackedRepos`, `GetMissingTrackedRepos` | Test-only. Use `GetTrackedReposWithOptions`, `GetMissingTrackedReposWithOptions` |
| `internal/server` | `buildTrackedRepos` | Deleted. Use `buildTrackedReposWithOptions` |
| `internal/server` | `resolveTrackedRepo(input, follows...)`, `searchBuiltinIndex(..., follows...)`, `discoverAuditSkills(source, follows...)` | Plain `follow *sourcewalk.FollowSet` parameter |
| `internal/server` | `disabledByRelPath(kind, source, agentsSource)` | Adds `follow *sourcewalk.FollowSet` |
| `cmd/skillshare` | `firstFollowSet`, `resolveGroupUpdatable`, `extractTrackedRepos` | Deleted |
| `cmd/skillshare` | `checkUndeclaredSourceLinks`, `buildTargetSkillSyncSummary` | Test-only. Use the `*WithFollow` forms |
| `cmd/skillshare` | `runCheck`, `runCheckFiltered`, `displayTrackedRepos`, `collectInstalledSkillPaths`, `resolveSkillPath`, `auditSkillByName`, `integrateRemote`, `resolveByBasename`, `resolveByGlob`, `pullRemote`, `resetToRemoteBranch`, `resolveUninstallTarget`, `resolveUninstallByGlob`, `resolveNestedSkillDir`, `stageAndCommit`, `cmdUpdateProjectBatch`, `commitSourceFiles`, `setupGitRemote`, `addRemote`, `tryPullAfterRemoteSetup` | `follows ...*sourcewalk.FollowSet` → `follow *sourcewalk.FollowSet` (pass `nil` explicitly where a caller has no snapshot) |
| `cmd/skillshare` | `discoverForKind(kind, sourcePath)`, `checkSkillsValidity(source, result, discovered)` | Add a trailing `follow *sourcewalk.FollowSet` |

New code should call the `*WithOptions` / `*WithFollow` forms with the command's snapshot. A follow-less call will fail `TestFollowSnapshotGuard`.

## Pre-existing issue, not fixed

`cmdAudit` replaces `sourcePath` with the agents source when the kind is agents (`audit.go:212-214`). `auditTUIContext.sourcePath` therefore holds the agents source, and the TUI's Skills tab under `audit --agents` scans the agents directory as skills. This happens with or without `.skillfollow`. The snapshot now reaches that call, but the tab stays wrong until the context keeps the skills source separately.

## Verification

All commands ran in a throwaway container from the devcontainer image (`skillshare_wt_discovery_entry`), with this worktree at `/workspace`.

- Every commit: `go build ./...`, `go vet` on the touched packages, and the touched packages' tests (`./cmd/skillshare`, `./internal/server`, `./internal/sync`, `./internal/install`, `./internal/sourcewalk`), all passing.
- Each new regression test failed with its fix reverted and passed with it.
- `make check` at `45b3a46c` (fmt, vet, unit, and integration tests) passed. Tail:

```text
--- PASS: TestXDG_StatusWorksWithXDGPath (0.03s)
PASS
ok  	skillshare/tests/integration	64.842s

✓ All tests passed!
```
