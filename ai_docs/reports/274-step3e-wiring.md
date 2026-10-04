# Skillfollow step 3e wiring handoff

## Result and scope

Finished rollout step 3 plumbing: every reconcile, merge-status, and dashboard sync caller reachable from a command or handler now receives the operation's follow set. No policy was added. `internal/sourcewalk`, `internal/sync`, `internal/config`, `internal/git`, and `internal/install` are unchanged; the only new function is a cmd-local options twin for the target list summary. No push or pull request was performed.

Base: `3a926517` on `runkids/274-step3e`.

## Commits

- `1d32fea8` reconcile: install-time reconcile receives the set.
- `ab2e44db` status: target info, target list, and dashboard targets count followed links as linked.
- `1154fde6` server sync: dashboard sync pauses prune and reports `prune_paused` and `kept`.
- `589076f3` matrix: CLI and server rows for the wired paths.

## Callers switched

Reconcile (`ReconcileGlobalSkillsWithOptions` / `ReconcileProjectSkillsWithOptions`):

- `cmd/skillshare/install.go` (both the metadata-resolved and dispatched paths), `install_context.go` (`globalInstallContext.Reconcile`), `search.go`, and the global branch of `search_batch.go` use `globalSkillFollowSet(cfg)`.
- `cmd/skillshare/project_skills.go` (`reconcileProjectRemoteSkills`, which also serves `projectInstallContext.Reconcile` and the project branch of `search_batch.go`) uses `skillFollowSet(runtime.sourcePath, runtime.targets, runtime.root)`, the same set `sync -p` and `status -p` build.
- `internal/server/handler_install.go` `reconcileSkillsConfig` builds `s.skillFollowSet()`. Every caller (install, batch install, update, batch uninstall) holds `s.mu.Lock()`, so the snapshot follows the existing rule.

The set is built at reconcile time, after the install wrote its files, so it reflects the post-install source.

Merge status (`CheckStatusMergeWithOptions`):

- `cmd/skillshare/target.go` `showTargetInfo`: `globalSkillFollowSet(cfg)`.
- `cmd/skillshare/target_project.go`: `skillFollowSet(sourcePath, targets, root)` with the targets it already resolves through `config.ResolveProjectTargets`.
- `cmd/skillshare/target_list_tui.go`: one set per list build (global and project), passed through `targetSkillSyncSummary`. `buildTargetSkillSyncSummary` keeps its signature as the nil-follow wrapper for its existing test; `buildTargetSkillSyncSummaryWithFollow` carries the set.
- `internal/server/handler_targets.go`: the `follow` snapshot the handler already took under `s.mu.RLock()` for discovery.

Dashboard sync (`internal/server/handler_sync.go`): `syncResources` builds one set, shares it between discovery and `SkillRunOptions.Follow`, which `SyncSkillTarget` forwards to `MergeOptions`, `CopyOptions.Follow`, and both `PruneOptions.Follow` calls, as `cmd/skillshare/sync_parallel.go` does. Each result row gains `prune_paused` and `kept` (omitempty, same keys as CLI `sync --json`), and the warnings gain the CLI's two lines: `<target>: prune paused; unavailable .skillfollow entry: ...` and `<target>: kept N managed copies whose origin cannot be proven: ...`. The dashboard UI was not changed; it already lists warnings.

`internal/server/handler_sync_matrix.go` needed nothing: both handlers only discover and classify include/exclude, and 3d already passes the set to discovery.

## Left nil, with reasons

`rg -n 'ReconcileGlobalSkills\(|ReconcileProjectSkills\(|CheckStatusMerge\(' cmd internal --glob '!*_test.go'` now returns only the three wrapper definitions:

```text
internal/sync/sync.go:1014:func CheckStatusMerge(targetPath, sourcePath string) (TargetStatus, int, int) {
internal/config/project_reconcile.go:17:func ReconcileProjectSkills(projectRoot string, projectCfg *ProjectConfig, store *install.MetadataStore, sourcePath string) error {
internal/config/reconcile.go:13:func ReconcileGlobalSkills(cfg *Config, store *install.MetadataStore) error {
```

Other sync-family calls deliberately left without a set:

- `SyncTargetMerge`, `PruneOrphanLinks`, `PruneOrphanCopies`, and `SyncTargetCopy` (the wrappers that discover internally with nil follow, `internal/sync/sync.go:541,752` and `copy.go:259`) have no non-test callers.
- `CheckStatus` (symlink mode) in status, doctor, target info/list, dashboard targets, `targetsummary`, and `SyncSkillTarget`: the whole target is one link to the source root, and no follow-aware twin exists or is needed.
- `DetachSkills` (skills off cleanup, `cmd/skillshare/target_skills.go:79` and `internal/server/handler_targets.go:581,638`) has no options parameter. It removes only links whose text, or canonical destination, lies under the source. Links sync creates for followed skills have logical source text, so they are removed as today; a link whose text is the resolved followed target is kept, which errs toward keeping. Teaching it followed ownership would be new policy, so it stays as is.

## Tests added

- `tests/integration/skillfollow_matrix_test.go`: rows `sync` (`sync --json` reports `"prune_paused": [` with `missing (missing)`), `target-info` (`2 linked`, no `local`), and `target-list` (`2 shared`, no `local`). A per-row setup map places a managed-to-be link `group__c -> <external>/group/c` (link text is the resolved target) before `sync`. The rows run after `diff`, so earlier rows see the unchanged fixture; the source and external snapshot check still runs on every row. No existing row changed.
- `internal/server/handler_skillfollow_test.go` `TestServerSkillfollowSyncAndTargets`: the same five-entry fixture with one merge target, a stale link into `missing`, and the resolved-text `group__c` link. `POST /api/sync` reports `prune_paused` for `missing (missing)` and `rejected (invalid-target)`, prunes nothing, keeps the stale link, and adds the CLI warning; `GET /api/targets` then reports `linkedCount: 3` and `localCount: 0`. The shared `TestServerSkillfollowBehaviorMatrix` table has no targets, and adding one would change its other rows' classification, so the sync and targets rows live in this separate test on the same fixture.

Against the pre-3e branch (`3a926517`, run from a `git archive` copy inside the container): the CLI `target-info` and `target-list` rows failed (`claude` counted the followed link as local), and the server test failed because the dashboard sync pruned the stale link and targets reported `linkedCount: 1, localCount: 1`. The CLI `sync` row already passed there, since 3b wired CLI sync; it pins parity with the dashboard row.

Reconcile wiring has no new matrix row: the fixture has no install metadata under a followed or unavailable prefix, and an install row would need network or a local repo fixture outside the matrix layout. The behavior itself is covered by the 3b unit tests in `internal/config/reconcile_follow_test.go`.

## Verification

All Go commands ran in the throwaway container `skillshare-274-step3e` (devcontainer image, this worktree at `/workspace`). See `worker_done` for the final `make check` tail.

Not run: Windows build or runtime, frontend tests (no UI change).
