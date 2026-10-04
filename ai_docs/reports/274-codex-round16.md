# PR 390 Codex round 16 fix

Both findings held against reviewed commit `0d7779a8`. They share one root cause: when a declared entry is unavailable, its link is absent, so nothing rejects a write at that path, and the write creates the declared name as a real directory. The next snapshot classifies the entry as `not-link`, the prune pause ends, and the external tree is masked when it returns. Auditing the other write paths turned up the same gap in bare `install`, in `install --into` (CLI and dashboard), in tracked installs and in `skillshare new`. The fixes are three commits on `runkids/codex-round16`. Every new test failed before its fix and passes after it. Nothing was pushed and no GitHub thread was changed.

| Commit | Scope |
|---|---|
| `b52636ce` | P1: missing tracked repos and rehydration |
| `ffe3518f` | Bare `install` from metadata or project config (found during the P1 audit) |
| `a6a71ed4` | P2: dashboard create, plus the install and `new` paths found during the audit |

## Replies ready for the coordinator

### P1: Exclude unavailable followed prefixes from rehydration

Fixed in `b52636ce`, with `ffe3518f` closing the CLI equivalent. Confirmed as described. With `.skillfollow` declaring `group`, no `group` link, and metadata holding tracked `group/_repo`, `GetMissingTrackedReposWithOptions` listed `group/_repo`. `POST /api/update/rehydrate` returned `{"results":[{"name":"group/_repo","action":"rehydrated"}]}` after creating a real `group/` and cloning into it. With the link present, the clone was refused only because the source handle rejected the link (`failed to create --into directory: … is a link`).

What changed:

- `GetMissingTrackedReposWithOptions` skips metadata paths that the operation's snapshot places inside a declared entry (`InFollowed`, live or unavailable). That content belongs to the entry's external owner. A live followed repo is not "missing" to skillshare, and an offline one is for its owner to restore.
- `RehydrateMissingTrackedRepos` now reads its work list from that query instead of repeating the loop. Every caller therefore shares the rule: `status`, `doctor` (`checkMissingTrackedRepos`), `check`, `update` (global and project), the dashboard banner (`GET /api/update/missing-tracked-repos`) and `POST /api/update/rehydrate`.
- Bare `skillshare install` (global from metadata, project from `.skillshare/config.yaml`) does not call `RehydrateMissingTrackedRepos`. It goes through `InstallFromConfig`, which reinstalled both tracked repos and plain skills recorded below an offline entry, creating a real `group/`. It now skips those entries and counts them as skipped (`skipped (inside followed entry group)` when not quiet). Both callers pass their snapshot.
- Status and doctor no longer mention such an entry as a missing repo. The followed entry already appears with state `missing` and the prune-pause message. A second "missing repository, run install" row would recommend exactly the recreation this fix forbids.

Coverage:

- `TestMissingTrackedReposSkipFollowedEntries` (`internal/install`): offline and live `group`. `group/_repo` is neither listed nor rehydrated. An undeclared first-level `_plain` is still listed and rehydrated. No `group/_repo` is written, and the `group` link (or its absence) is unchanged.
- `TestServerRehydrateSkipsUnavailableFollowedEntry` (`internal/server`): nothing is offered, the rehydrate response does not name `group/_repo`, and no `group/` exists afterwards.
- `TestSkillfollowInstallFromConfigSkipsFollowedEntry` (integration, global and project): bare install creates no `group/`, and a skill outside the declaration is still installed.
- Two existing integration tests encoded the old behavior and were updated. `TestSkillfollowStatusTrackedRepoNotDuplicated` now expects no tracked-repo row for an offline followed `_repo` (it expected `missing`). `TestSkillfollowDoctorTrackedRepoNotMissing` now expects no `tracked_repos` diagnostic for it, and still requires one for an undeclared missing tracked repo `_other`.

### P2: Refuse creation beneath unavailable followed entries

Fixed in `a6a71ed4`. Confirmed as described. `POST /api/resources` with `{"into":"missing"}` returned 201 and created `missing/fresh`. With a live `group` link the source handle refused, but as a 500 (`failed to create directory: … is a link`).

What changed: `handleCreateSkill` calls `followedSkillWriteError(source, into/name, snapshot)` before any `Stat` or `MkdirAll`. It answers 409 with the wording every other dashboard write uses for followed paths: `<source>/<entry> is a link; edit its target directly`. Writes inside a followed entry are refused whether its link is live or offline.

Audit of the other writes that take a path under the skills source:

| Path | Before | Now |
|---|---|---|
| Dashboard create skill (`POST /api/resources`) | Offline: created the entry. Live: 500 from `sourcefs` | 409 via `followedSkillWriteError` on `into/name` |
| Dashboard install (`POST /api/install`) with `into` | Offline: created the entry and copied into it | 409 before `MkdirAllIn` |
| Dashboard batch install (`POST /api/install/batch`) with `into` (skills) | Offline: created the entry and installed | 409 before discovery or clone |
| Dashboard tracked install (`track: true`) | Did not pass a snapshot | Passes the snapshot. The link error maps to 409 |
| CLI `install --into` (all plain flows share `ensureIntoDirExists`) | Offline: created the entry | Link error before the directory is created |
| Tracked install (`InstallTrackedRepo`: CLI `--track`, dashboard, rehydrate) | Offline: created `group/` or `_repo` | Refuses when the destination (`into/_name`, or `_name` itself) is inside a declared entry. An update of an existing followed repo (`Update` with the repo present) still goes through `PrepareFollowedUpdate` |
| CLI `skillshare new <name>` | Offline: created the entry | Link error before `MkdirAll` |
| Bare `install` (`InstallFromConfig`) | Offline: created the entry | Skipped (`ffe3518f`) |
| Rehydrate (CLI no-arg and dashboard) | Offline: created the entry | Not listed (`b52636ce`) |
| Content save (`PUT /api/resources/{name}/content`) | Already safe: resolves an existing discovered skill (offline: 404) and writes only through the source handle | Unchanged |
| Source URL edit (`PATCH …/source`) | Already refused through `InFollowed` / `followedSkillWriteError` | Unchanged |
| Toggle, toggle batch, target edits, batch target edits, uninstall, uninstall batch, repo uninstall | Already refused through `followedSkillWriteError` | Unchanged |
| `.skillignore` save | Writes the source root file only | Unchanged |
| Create agent, agent installs (`kind: agent`) | Write the agents source, which `.skillfollow` does not cover | Unchanged |
| Hub drafts | Read-only on the source | Unchanged |
| Project `init` | Creates the source root, not a path below an entry | Unchanged |

The CLI checks (`ensureIntoDirExists`, `new`) take a bare snapshot, `sourcewalk.Follow(source, FollowOptions{})`. Targets and the Git root only decide an entry's state, never whether it is declared, and `InFollowed` returns every declared state except `not-link`.

Coverage:

- New rows in `TestServerSkillfollowBehaviorMatrix`: `create` (live `group`), `create-offline`, `install-offline` and `install-batch-offline` (all 409). Every row now also asserts that the offline `missing` entry was not created.
- `TestInstallTrackedRepoRefusesDeclaredEntry` (`internal/install`): a repo named `_repo`, `Into: group`, and `Into: group` with `Update`. All three are refused and neither `group` nor `_repo` is created.
- `TestSkillfollowInstallIntoDeclaredEntryRefused` and `TestSkillfollowNewRefusesOfflineEntry` (integration).

## Failures before the fixes

Each test ran in the container against the unfixed code.

```text
install_follow_test.go:57: missing = [{Name:_plain …} {Name:group/_repo …}], <nil>; want only _plain      (missing and followed)
install_follow_test.go:61: results = [{Name:_plain Action:rehydrated} {Name:group/_repo Action:rehydrated}]  (missing)
install_follow_test.go:61: results = [… {Name:group/_repo Action:error Error:failed to create --into directory: …}]  (followed)
install_follow_test.go:64: rehydrate wrote below the followed entry: <nil>
handler_update_follow_test.go:363: offered for rehydration: [{Name:group/_repo …}]
handler_update_follow_test.go:368: status 200: {"results":[{"name":"group/_repo","action":"rehydrated"}]}
skillfollow_install_test.go:54: install created the declared entry: <nil>    (bare install, global and project)
TestServerSkillfollowBehaviorMatrix/create: status 500, want 409: {"error":"failed to create directory: …/skills/group is a link; edit its target directly"}
TestServerSkillfollowBehaviorMatrix/create-offline: status 201, want 409: {"createdFiles":["SKILL.md"],…"relPath":"missing/fresh"…}
TestServerSkillfollowBehaviorMatrix/install-offline: status 200, want 409: {"action":"copied","skillName":"a"}
TestServerSkillfollowBehaviorMatrix/install-batch-offline: status 200, want 409: {"results":[{"name":"a","action":"installed"}]}
install_follow_test.go:89: {Name:repo …} / {Name:other Into:group} / {… Update:true}: err = <nil>, want link refusal
install_follow_test.go:94: tracked install created declared entry group / _repo
skillfollow_install_test.go:74: expected failure, but command succeeded        (install --into group/sub)
skillfollow_install_test.go:90: expected failure, but command succeeded        (new group)
```

## User-visible behavior changes (for the docs update)

These apply only when `.skillfollow` or `.skillfollow.local` declares the entry. With no declaration, all output is unchanged.

1. `status`, `status --json`, `doctor`, `check`, `update` and the dashboard "missing tracked repos" banner no longer list a tracked repo inside a declared entry as missing, whether the entry is live or offline. Its state appears only in the skillfollow section.
2. Rehydration (`POST /api/update/rehydrate`) never clones into a declared entry.
3. Bare `skillshare install` (global and project) skips skills and tracked repos recorded inside a declared entry and counts them as skipped.
4. `install --into <path inside a declared entry>` fails with `<source>/<entry> is a link; edit its target directly` (CLI) or 409 (dashboard install and batch install). This includes offline entries.
5. Tracked installs whose destination is inside a declared entry, or is the entry's own name (such as `_repo`), fail with the same error. Updating an existing followed repo is unchanged.
6. `skillshare new <name>` fails with the same error when `<name>` is a declared entry.
7. Dashboard create skill into a declared entry returns 409 with that error. Before, a live entry gave a 500 and an offline entry created the directory.

## Evidence

All Go commands ran in a throwaway container from the devcontainer image (compose project `skillshare_wt_round16`, this worktree at `/workspace`). Nothing ran on the host. `make check` inside the container exited 0. Tail:

```text
=== RUN   TestXDG_StatusWorksWithXDGPath
--- PASS: TestXDG_StatusWorksWithXDGPath (0.03s)
PASS
ok  	skillshare/tests/integration	60.126s

✓ All tests passed!
```

## Notes and limits

- **Not fixed:** a plain install with no `--into` whose skill name equals a declared entry name (CLI single and multi-skill installs, dashboard `POST /api/install` and batch per-skill names). With the entry offline, this still creates the entry as a real directory. It is a rarer variant: the user must install a skill whose name equals the declaration. The checks above cover every `into` path and the tracked and `new` destinations. Covering this case would need a check at each per-skill destination. It is left for a decision rather than widened here.
- With an unreadable declaration the snapshot knows no entries, so the bare-install skip and the `into` refusals do not apply. The missing-repo query already fails in that case, because `GetTrackedReposWithOptions` returns the snapshot error. This is unchanged from before.
- The test for `skillshare new` was shown failing by temporarily restoring `cmd/skillshare/new.go` from `HEAD`, then putting the fix back. The other new tests were written and run before their fix.
- No dependency on `runkids/discovery-entry`. No website docs, README or built-in skill edits.
