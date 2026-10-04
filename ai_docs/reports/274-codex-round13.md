# PR 390 Codex round 13 fix

The finding held against reviewed commit `0da66fde`. The new regression test failed before the fix and passes after it. The fix is one commit on `runkids/codex-round13`. Nothing was pushed and no GitHub thread was changed.

## Reply ready for the coordinator

### P2: Keep agent repository updates outside skillfollow policy

Fixed in `ea816195`. Confirmed. `updateTrackedRepo` always built its policy with `PrepareFollowedUpdate(s.cfg.EffectiveSkillsSource(), repoPath, s.skillFollowSet(), force)`. `updateAgent` calls it for a repo-backed agent with a `repoPath` under the agents source. With a non-nil skills snapshot, that path reached two failures:

- On Windows, with the agents and skills sources on different volumes, `filepath.Rel` returns an error and the agent update reports it.
- On every platform, when the skills `.skillfollow` cannot be read, `FollowSet.Err()` is set and `PrepareFollowedUpdate` fails closed before `Rel`. The agent update then fails with `followed repository update refused: skillfollow discovery is incomplete`. This is the Unix-observable form of the same bug.

With a readable `.skillfollow` on Unix the bug was latent: `Rel(skillsSource, agentRepo)` yields `../…`, which `InFollowed` never matches, so the agent update took the ordinary path.

`updateTrackedRepo` now takes the resource's source directory and follow snapshot as parameters instead of reading the skills ones. Callers:

- `updateSingleByKind` (skills, JSON route): `s.cfg.EffectiveSkillsSource()` and `s.skillFollowSet()`, as before.
- `updateAll` (skills, JSON `all`): the same source and the snapshot it already took for `collectUpdateAll`.
- `handleUpdateStream` (skills, SSE): `s.cfg.EffectiveSkillsSource()` and `s.skillFollowSet()`, as before. The stream route has no agent path: it resolves names under the skills source only.
- `updateAgent` (agents, JSON `kind: "agent"`): the agents source and a nil snapshot. This matches the CLI, whose `update_agents.go` already builds its `updateContext` with `sourcePath: agentsDir` and no `follow`.

Project mode was checked as a possible second bug on the same line. It is not one: `NewProject` receives a synthetic `config.Config` whose `Source` is the project skills source, and `reloadConfig` keeps `s.cfg.Source` in sync with it. In project mode `s.cfg.EffectiveSkillsSource()` therefore equals `s.skillsSource()`, the root the project-scoped snapshot was built from.

Coverage:

- `TestServerAgentRepoUpdateIgnoresSkillFollow` (`internal/server`): a tracked agent clone `agents/_team` is updated through `POST /api/update` with `kind: "agent"` after its remote advanced. Subtest `valid` has a readable `.skillfollow` in the skills source; subtest `unreadable` makes `.skillfollow` a directory so the read fails. Both assert `"action":"updated"` and that the clone's HEAD is the remote HEAD.
- Existing skills tests are unchanged and still pass, including `TestServerFollowedNamedUpdateUnreadableDeclaration` and `TestServerFollowedUpdateAllUnreadableDeclaration`, so skills updates still fail closed on an unreadable declaration.

## User-visible behavior changes (for the docs update)

1. **Dashboard agent repo updates ignore the skills `.skillfollow`.** Updating a repo-backed agent from the dashboard no longer fails when the skills `.skillfollow` (or `.skillfollow.local`) cannot be read, and no longer fails on Windows when the agents and skills sources are on different volumes. Skills updates are unchanged and still refuse in those cases.

Nothing else changes. Without `.skillfollow`, every output is unchanged.

## Evidence

All Go commands ran in a throwaway container from the devcontainer image (compose project `skillshare_wt_codexr13`, this worktree at `/workspace`). Nothing ran on the host.

Before the fix (new test added, `handler_update.go` and `handler_update_stream.go` at `0da66fde`):

```text
--- PASS: TestServerAgentRepoUpdateIgnoresSkillFollow/valid (0.06s)
--- FAIL: TestServerAgentRepoUpdateIgnoresSkillFollow/unreadable (0.04s)
handler_update_follow_test.go:340: agent repo update failed: {"results":[{"name":"_team/reviewer","action":"error","message":"followed repository update refused: skillfollow discovery is incomplete: read skillfollow declaration .../skills/.skillfollow: read .../skills/.skillfollow: is a directory","isRepo":true}]}
```

After the fix, `go test ./internal/server -count=1` passed, and `make check` passed. Tail:

```text
=== RUN   TestXDG_StatusWorksWithXDGPath
--- PASS: TestXDG_StatusWorksWithXDGPath (0.04s)
PASS
ok  	skillshare/tests/integration	81.102s

✓ All tests passed!
```
