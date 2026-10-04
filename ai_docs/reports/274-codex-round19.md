# PR 390 Codex round 19 fix

Both findings held against reviewed commit `ad7fa76e`. The first is structural: every write guard added in rounds 16-18 asked `InFollowed` on a snapshot that, when `.skillfollow` or `.skillfollow.local` exists but cannot be read, carries the read error but no entries. Each guard therefore allowed the write, and a real directory could appear at an entry hidden by the read failure. The fix is one fail-closed helper, `FollowSet.WriteBoundary`, used by every write guard, plus a ratchet that keeps new guards from calling `InFollowed` directly. The second finding is the `collect --dry-run` shortcut noted as a limit in round 18. The work is three commits on `runkids/codex-round19`. Every new behavior test failed before its fix and passes after it. Nothing was pushed and no GitHub thread was changed.

| Commit | Scope |
|---|---|
| `42223e54` | P2: fail closed on an unreadable declaration (`WriteBoundary`, all write guards routed through it) |
| `b17de9ed` | Ratchet: `TestFollowWriteGuard` flags any direct `InFollowed` outside an allowlist of read-only uses |
| `68041fdb` | P2: `collect --dry-run` goes through the follow-aware pull |

## Replies ready for the coordinator

### P2: Abort installs when follow declarations are unreadable

Fixed in `42223e54`. Confirmed as described, and not only for installs. With `.skillfollow` unreadable (a directory in the tests, so it is unreadable for every user including root), these all created `source/team`: a plain install named `team` (bare and full snapshot), `install --into team`, a tracked install into `team`, a bare config install of a skill grouped under `team` (the group directory was created before the per-skill guard ran), dashboard create (`POST /api/resources`, with and without `into`), dashboard install (with and without `into`), dashboard and CLI collect, trash restore (CLI and dashboard), `new`, and `init` imports.

What changed: `sourcewalk.FollowSet` gains `WriteBoundary(source, rel string) error`. When the declaration cannot be read, it returns the read error (`read skillfollow declaration <path>: <err>`), because an unread declaration may hide any entry. When `rel` is at or below a declared entry, live or offline, it returns `&sourcefs.LinkError{Path: <source>/<entry>}`, as before. Otherwise, and for a nil `*FollowSet` (no declaration), it returns nil. The helper checks only the declaration read error, not `Err()`'s traversal errors: those already mark their entry `missing`, which the entry match refuses, and `sourcewalk.WalkDir` fails on the same declaration error, so writes and discovery follow one rule. `sourcewalk` now imports `sourcefs`, which imports only `utils`, so there is no cycle and the `LinkError` mapping lives in one place.

Every write guard now goes through the helper:

| Guard | Caller | Unreadable declaration now |
|---|---|---|
| `followedSkillWriteError` (`internal/server/skillfollow.go`) | dashboard create, install `into`, batch target edit, repo uninstall, batch toggle, uninstall, skills batch | Refused with the read error. Handlers that answered a link refusal with a fixed 409 now use `followWriteStatus`: 409 for `ErrLink`, 500 otherwise, as `handler_update`/`handler_overview` do for an unreadable declaration |
| `followedDestError` (`internal/install/install_apply.go`) | every plain install (CLI, search/hub, dashboard, `upgrade`) | Refused; the update exception stays limited to a link refusal of an existing path |
| `installTrackedRepoImpl` (`internal/install/install_tracked.go`) | `install --track`, dashboard tracked install, rehydrate | Refused; same update exception |
| `InstallFromConfig` (`internal/install/install_config.go`) | bare `install` (global, project) | The item fails with the read error (a declared entry is still skipped) |
| `ensureIntoDirExists` (`cmd/skillshare/install.go`) | `install --into` | Refused before `MkdirAll` |
| `cmdNew` (`cmd/skillshare/new.go`) | `new` | Refused |
| `applyInitPlan` (`cmd/skillshare/init_apply.go`) | `init` imports | Skipped with `Skipped <name>: read skillfollow declaration …` |
| `trash.Restore` (`internal/trash/trash.go`) | CLI `trash restore`, trash TUI, dashboard (500) | Refused; the item stays in the trash |
| `followedMergeError` (`internal/sync/sync.go`) | symlink-mode `sync` migration (CLI, dashboard) | Refused before any merge |
| `PullSkills` (`internal/sync/pull.go`) | `collect` (CLI global/project, dashboard) | Each item fails with the read error |
| `runUninstallSkills` (`cmd/skillshare/uninstall_handlers.go`) | CLI `uninstall` | Refused, matching dashboard uninstall (also through `followedSkillWriteError`) |

Coverage:

- `TestWriteBoundary` (`internal/sourcewalk`): nil set allows; a declared entry gives `ErrLink` naming it; an undeclared name passes; an unreadable declaration gives the read error.
- `TestInstallRefusesUnreadableDeclaration` (`internal/install`; plain with bare and full snapshot, tracked, config): refused with the read error and nothing created; the config item is listed as failed.
- `TestHandleWrites_RefuseUnreadableDeclaration` (`internal/server`; create, create into, install, install into, collect, trash restore): read error with 500 (collect: 200 with `failed.team`), nothing created.
- `TestSkillfollowWritesRefuseUnreadableDeclaration` (integration; `new`, `install`, `install --into`, `collect --force`, `trash restore`, `init --copy-from`): output names the read error, nothing created.

### P2: Apply follow refusals to collect dry runs

Fixed in `68041fdb`. Confirmed as described. With `team` declared (link missing or live) and target skills `team` and `keep`, `collect -g --dry-run` listed both under "Would collect 2 skills", and `--json` reported `pulled: [keep team]`, `failed: {}`.

What changed: `runCollectPlan` no longer answers a dry run from the scan. It calls `res.pull` with `DryRun: true`. `PullSkills` refuses before its dry-run listing, so `pulled` and `failed` match execution for text and `--json`. The text preview prints failed items as the real run does (`✗ team  <source>/team is a link; edit its target directly`), then `Would collect N` and the dry-run note. Without a declaration nothing fails and the output is byte-for-byte unchanged. The confirmation prompt is still skipped for dry runs. The orphaned `collectResources.names` field was removed. `PullAgents` already lists every agent in a dry run and creates no directory, so agent collect previews are unchanged.

Coverage:

- `TestSkillfollowCollectDryRunRefusesDeclaredEntry` (integration; missing and live, text and `--json`): `team` under failed with the link error, `pulled` is `[keep]`, `Would collect 1 skill`, nothing is written.

## Ratchet

`TestFollowWriteGuard` (`internal/sourcewalk/write_guard_test.go`) parses every non-test Go file under `cmd` and `internal` except `internal/sourcewalk`. It fails on any `InFollowed(` call whose enclosing function (closures count toward it) is not allowlisted, and on a stale allowance. `TestFollowWriteGuardRejectsBareCheck` shows that an injected bare write guard, including one inside a closure, is flagged. The file and test names differ from `TestFollowSnapshotGuard` in `follow_guard_test.go` on `runkids/discovery-entry`, so the two merge side by side.

Allowlist (read-only uses, none of which decides whether a new path may be created):

| Function | Reason |
|---|---|
| `cmd/skillshare/list.go:displayTrackedRepos` | Display: marks a tracked repo as followed |
| `cmd/skillshare/audit.go:auditPathFollowed` | Audit: picks the resolved-path scan |
| `cmd/skillshare/update_resolve.go:resolveGroupUpdatableWithOptions` | Update target resolution; update refuses an incomplete snapshot (`followDiscoveryError`) |
| `cmd/skillshare/update_handlers.go:refreshTrackedRootSkillMetadata` | Skips a metadata refresh for a followed repo; update refuses an incomplete snapshot |
| `cmd/skillshare/update_batch.go:auditScanFn` | Audit: picks the resolved-path scan |
| `internal/install/install_audit.go:auditTrackedRepoUpdate` | Audit: picks the resolved-path scan |
| `internal/install/followed_update.go:PrepareFollowedUpdate` | Update policy; checks `Err()` first |
| `internal/install/followed_update.go:RefuseFollowedSkillUpdate` | Update policy; checks `Err()` first |
| `internal/install/install_queries.go:GetMissingTrackedReposWithOptions` | Rehydrate query; the preceding walk returns the declaration error |
| `internal/config/reconcile_core.go:reconcileSkillsWalk` | Metadata reconcile; the walk returns the declaration error |
| `internal/config/reconcile_core.go:pruneStaleEntries` | Metadata reconcile; runs only after a successful walk |
| `internal/server/handler_update.go:auditGateTrackedRepo` | Audit: picks the resolved-path scan |
| `internal/server/handler_skill_content.go:handlePatchSkillSource` | Metadata edit of an existing skill; its refusal names the entry's resolved target, and an unreadable declaration already fails `findMetadataEntry`'s discovery (404) before any write |
| `internal/audit/audit_follow.go:MarkFollowedInputs` | Audit: marks followed inputs |

## Failures before the fix

The tests were written first and run in the container against `ad7fa76e` plus the unused helper:

```text
--- FAIL: TestFollowWriteGuard
    write guard must use FollowSet.WriteBoundary, not InFollowed: cmd/skillshare/init_apply.go:applyInitPlan
    … cmd/skillshare/install.go:ensureIntoDirExists, cmd/skillshare/new.go:cmdNew,
      cmd/skillshare/uninstall_handlers.go:runUninstallSkills, internal/install/install_apply.go:followedDestError,
      internal/install/install_config.go:InstallFromConfig, internal/install/install_tracked.go:installTrackedRepoImpl,
      internal/server/skillfollow.go:followedSkillWriteError, internal/sync/pull.go:PullSkills,
      internal/sync/sync.go:followedMergeError, internal/trash/trash.go:Restore
--- FAIL: TestInstallRefusesUnreadableDeclaration/{plain,tracked,config}
    err = <nil>, want the declaration read error
    install created a possibly declared entry: <nil>
    failed = [], want [team/demo]
--- FAIL: TestHandleWrites_RefuseUnreadableDeclaration/{create,create-into,install,install-into,collect,restore}
    got 200 {"action":"copied","skillName":"team",…}, want 500 with the declaration read error
    got 200 {"failed":{},"pulled":["team"],…}, want 200 with the declaration read error
    got 200 {"success":true}, want 500 with the declaration read error
    request created a possibly declared entry: <nil>
--- FAIL: TestSkillfollowWritesRefuseUnreadableDeclaration/{new,install-into,install,collect,trash-restore,init-import}
    <command> created a possibly declared entry: <nil>
--- FAIL: TestSkillfollowCollectDryRunRefusesDeclaredEntry/{missing,live}/{text,json}
    expected output to contain "…/skills/team is a link; edit its target directly"
    expected output to contain "Would collect 1 skill"
    pulled = [keep team], failed = map[], dry_run = true; want [keep], team refused, true
```

`TestWriteBoundary` and `TestFollowWriteGuardRejectsBareCheck` exercise new code and have no pre-fix state. After `42223e54`, everything except the dry-run test passed. After `68041fdb`, that passed too.

## User-visible behavior changes (for the docs update)

With no `.skillfollow`, every output is unchanged. With a readable declaration, behavior is as rounds 16-18 left it, except item 6.

1. When `.skillfollow` or `.skillfollow.local` exists but cannot be read, every command that could create a path in the skills source refuses with `read skillfollow declaration <path>: <err>` and creates nothing: `install` (plain, `--into`, `--track`, hub/search installs, `upgrade`'s built-in skill), `new`, `trash restore` (CLI, TUI; the item stays in the trash), and symlink-mode `sync` migration. Tested: plain and `--into` install, `new`, `trash restore` (integration) and tracked install (unit); `--track` through the CLI, `upgrade` and the `sync` migration share the same guards but are not separately tested. This extends the documented "unreadable" rule, which so far listed `sync`, `check`, `status`, `doctor`, `update` and Git staging.
2. Same condition: `collect` (CLI and dashboard) reports each skill as failed with the read error. Bare `install` reports each config skill as failed (a declared entry is still "skipped (inside followed entry …)"). `init` skips each import with `Skipped <name>: read skillfollow declaration …`.
3. Same condition, dashboard: create skill, install (single, with or without `into`) and trash restore answer 500 with the read error (tested). A declared entry still answers 409.
4. Same condition, not separately tested: CLI `uninstall`, the dashboard target-edit, repo-uninstall and batch toggle/target/uninstall endpoints now also refuse with the read error at their write guard. These paths operate on discovered skills, and discovery already fails on an unreadable declaration, so in practice they most likely fail earlier with the same error.
5. Existing refusals for a readable declaration keep their text and status.
6. `skillshare collect --dry-run` (and `--json`) lists a target skill named like a declared entry, live or offline, as failed with `<source>/<entry> is a link; edit its target directly`, as the real run does, and `Would collect N` counts only the skills that would be copied. Before, the preview listed it as collected.

## Evidence

All Go commands ran in a throwaway container from the devcontainer image (compose project `skillshare_wt_round19`, this worktree at `/workspace`). Nothing ran on the host. `make check` inside the container after `68041fdb`:

```text
=== RUN   TestXDG_StatusWorksWithXDGPath
--- PASS: TestXDG_StatusWorksWithXDGPath (0.03s)
PASS
ok  	skillshare/tests/integration	58.016s

✓ All tests passed!
exit=0
```

## Notes and limits

- `internal/sourcewalk` now imports `internal/sourcefs` for `LinkError`. `sourcefs` imports only `internal/utils`, so there is no cycle.
- The ratchet works at function granularity: a second `InFollowed` call added to an allowlisted read-only function is not flagged. The allowlist reasons say why each function is read-only.
- `handlePatchSkillSource` stays on `InFollowed` because its 400 message names the entry's resolved target. It is fail-closed through discovery, but the response for an unreadable declaration is 404 (`resource not found`), not the read error.
- No website docs, README or built-in skill edits.
