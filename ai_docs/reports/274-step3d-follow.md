# Skillfollow step 3d handoff

## Result

Connected every assigned CLI, hub, and server discovery caller except CLI `audit.go:400`, which the coordinator explicitly reassigned to slice 3c to avoid overlapping edits to `collectInstalledSkillPaths`. No UI, sourcewalk semantics, config, git, update/check files, audit scanners, or sync implementation files were changed. The worktree was initially clean; all changes belong to this task.

Implementation commits:

- `a5ae2c31`: CLI consumers, uninstall resolution/refusal, list suffix, and discovery wrapper twins.
- `080266ea`: options-bearing hub index/candidate APIs and CLI hub caller.
- `09360503`: server policy snapshots, caller plumbing, and mutation refusals.
- The final matrix/report commit is identified in `worker_done` and the branch log.

No push or pull request was performed.

## APIs and policy

Added in `internal/sync/discover_options.go`:

- `DiscoverSourceSkillsLiteWithOptions`: preserves Lite's no-frontmatter, tracked-repository collection behavior.
- `DiscoverSourceSkillsAllWithOptions`: sets `IncludeIgnored`.
- `DiscoverSourceSkillsForAnalyzeWithOptions`: sets `IncludeIgnored` and `CollectContext`.

The existing nil-follow discovery functions remain unchanged. Stats and stats/context callers use the already-existing `DiscoverSourceSkillsWithOptions` with their respective `CollectIgnored`/`CollectContext` flags.

Added hub twins `BuildIndexWithOptions` and `DraftCandidatesWithOptions`; their original entry points delegate with nil options. Hub tests verify logical discovery, legacy invisibility, and refusal of partial results after a followed tree disappears.

Server policy lives in `internal/server/skillfollow.go`:

```go
func serverSkillFollowSet(cfg *config.Config) *sourcewalk.FollowSet
func (s *Server) skillFollowSet() *sourcewalk.FollowSet
```

The method snapshots while the caller holds `s.mu`; global mode delegates to the pure helper, and project mode passes the project root directly through `serverSkillFollowSetAtGitRoot`. `Config.GitRoot` is a scope keyword, so assigning a project path to that field is not a valid override. Only enabled skills target paths participate, sorted deterministically. Overview shares one set across discovery, top-level counting, and tracked-repository lookup; all consumer discovery APIs reject `FollowSet.Err()` rather than return partial skills. Older error-swallowing consumers now propagate followed-read failures while keeping nil-follow behavior.

`resolveTrackedRepo` accepts an optional existing set so nested consumers can share their request snapshot; old callers remain compatible. Slice 3c can pass its existing set as the second argument.

## Switched callers

CLI: global/project analyze, global/project diff, the assigned init count/remote/first-sync calls, uninstall discovery, named/glob nested uninstall resolution, and hub index. Plain global/project list renders followed tracked repos with `→ <resolved>`; JSON is unchanged in shape.

Hub: `draft_candidates.go` and `index.go`.

Server: `handler_analyze.go`, `handler_audit.go`, `handler_audit_stream.go`, `handler_diff_stream.go`, both overview lookups and its top-level count, both skill-content discovery helpers, skillignore stats, all skills discovery/repository lookup sites, both skills-batch calls, both sync discovery calls, both sync-matrix calls, targets discovery, single/batch toggles, batch uninstall discovery, hub index/candidates, and builtin search.

A final search found no legacy discovery/tracked-repo calls in the assigned files. Skills and overview return followed skills under logical source-relative paths.

### Refusal boundaries

CLI uninstall refuses followed roots and descendants before dry-run planning or actual mutation, including basename, flat-name, glob, and `--all` resolution. Existing group containment refusal is unchanged.

Server tests cover actual single skill/repo uninstall, batch uninstall, single/batch toggle, single/batch target edits, and content writes. All report the sourcefs link message, retain link entries, and leave external file bytes unchanged. Followed tracked-repo target overrides are refused before metadata changes too. Sync handlers only gained discovery policy; target ownership/pruning remains slice 3b.

Per coordinator agreement, `handlePatchSkillSource` remains untouched at `internal/server/handler_skill_content.go:108`. Its `findMetadataEntry` discovery helper is connected but refuses followed metadata lookup, preventing an external `git.SetRemoteURL`; slice 3c owns the explicit mutation refusal. CLI `collectInstalledSkillPaths` at `cmd/skillshare/audit.go:397` and its legacy discovery call at `:400` remain untouched for 3c, including any necessary `audit_project.go` signature plumbing. `commitSourceFiles`, the init reset call, and `resetToRemoteBranch` also remain untouched.

## Behavior matrix and tests

- `tests/integration/skillfollow_matrix_test.go`: five-entry fixture with followed tracked repo, followed group, missing declaration, rejected regular-file declaration, and undeclared link. Rows cover list JSON/text, status JSON, check JSON, update-all dry-run, audit, doctor JSON, multiple uninstall dry runs, and diff.
- `internal/server/handler_skillfollow_test.go`: the same five-entry layout across routed skills, overview, update, check, hub, candidate, content, and mutation endpoints. Additional tests verify enabled-target classification and project staging boundaries.
- `internal/sync/discover_options_test.go`: Lite repository collection and omitted frontmatter, All disabled skills, and Analyze context fields.
- `internal/hub/follow_test.go`: compatible nil wrappers, logical paths, and incomplete-read rejection.

The CLI status row is marked `pending 3b` for ownership/prune aggregates. CLI check/update/audit and server check/update rows are marked `pending 3c` and pin today's results: followed repos/local groups are absent from check/update, update-all has nothing to update, and CLI audit currently finds no skills in this all-linked fixture. These rows are deliberately not rewritten to future behavior on this branch.

Against baseline `0a077a82`, the CLI matrix reproduces the failures: explicit descendant/root uninstall dry runs incorrectly succeed, basename/glob/all discovery misses followed entries, and diff reports the target in sync instead of showing followed skills. The implemented binary passes those rows.

## Verification

All product builds/tests ran in the isolated `skillshare-274-step3d` Linux devcontainer with this worktree mounted at `/workspace`.

Passed:

```sh
go test ./internal/hub ./internal/sync ./internal/server \
  -run 'TestHubFollowOptions|TestDiscoveryOptionsTwins|TestServerSkillfollow' -count=1
go test ./tests/integration -run TestSkillfollowBehaviorMatrix -count=1
make check
```

`make check` passed docs-check, formatting, vet, both ratchets (`TestRawWalkGuard`, `TestRawWriteRatchet`), all unit tests, and all integration tests. Tail with ANSI color removed:

```text
=== RUN   TestXDG_StatusWorksWithXDGPath
--- PASS: TestXDG_StatusWorksWithXDGPath (0.04s)
PASS
ok  skillshare/tests/integration 88.581s

✓ All tests passed!
```

A separate baseline/current executable comparison with a normal local skill plus an undeclared source link and no `.skillfollow` confirmed byte-for-byte identical stdout, stderr, and exit codes for global list/status/doctor/analyze JSON. Existing tests were not changed or weakened. `python3 scripts/ai-context.py check` also passed after this report was added.

No frontend tests or Windows runtime tests were run: there are no UI or traversal-semantics changes in this slice. The task container was removed after verification; the branch and commits remain.

## Extra supporting files

Outside the original caller inventory, changed supporting files are `cmd/skillshare/{hub.go,skillfollow.go,uninstall.go,uninstall_project.go,list.go,list_project.go}`, `internal/server/{skillfollow.go,handler_audit_stream.go,handler_hub.go,handler_hub_drafts.go,handler_search.go}`, the new discovery wrapper file, the four new test files, and this report. Supporting caller scope was approved by the coordinator; `audit_project.go` needed no changes here.

Git emitted an existing repository maintenance warning about unreachable loose objects during commits. No prune, history rewrite, or GC-log deletion was performed.
