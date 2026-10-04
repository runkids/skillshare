# PR 390 Codex round 18 fix

Both findings held against reviewed commit `67d0561a`. They belong to the round-16/17 family: a declared entry is never created by skillshare, live or offline. When an entry is offline, its link is absent and `sourcefs` has nothing to refuse, so a write at that name creates a real directory. The next snapshot classifies the entry as `not-link`, the prune pause ends, and the external tree is masked when it returns. The final sweep found three more members of the family (symlink-mode `sync` migration, `init` imports and the `init` built-in skill fallback). The fixes are three commits on `runkids/codex-round18`. Every new test that covers an offline entry failed before its fix and passes after it. Nothing was pushed and no GitHub thread was changed.

| Commit | Scope |
|---|---|
| `a972ad33` | P2: collect (CLI global and project, dashboard) |
| `120bd8e6` | P2: trash restore (CLI, trash TUI, dashboard) |
| `ad70251c` | Sweep: symlink-mode `sync` migration, `init` imports, `init` built-in skill fallback |

## Replies ready for the coordinator

### P2: Refuse collecting into an offline followed entry

Fixed in `a972ad33`. Confirmed as described. With `.skillfollow` declaring `team`, no `team` link, and a target holding local skills `team` and `keep`, `skillshare collect --force` (global and project) and `POST /api/collect` copied `team` into the source and created a real `source/team`. With the link live, the forced copy was already refused by `sourcefs` while removing the existing destination (`… is a link; edit its target directly`); without `--force`, a live entry is reported as already existing and skipped, as before.

What changed: `PullOptions` gains `Follow *sourcewalk.FollowSet`. `PullSkills` checks each skill name with `InFollowed` (live or offline) before copying or the dry-run listing, and records `<source>/<entry> is a link; edit its target directly` as that skill's failure, so the other skills are still collected. Callers pass their snapshot: CLI global `globalSkillFollowSet(cfg)`, CLI project `skillFollowSet(runtime.sourcePath, runtime.targets, root)`, dashboard `s.skillFollowSet()`. Agent collect (`PullAgents`) writes the agents source and is unchanged. With no `.skillfollow` the snapshot is nil and nothing changes.

Coverage:

- `TestSkillfollowCollectRefusesDeclaredEntry` (integration; global and project, entry missing and live): `collect --force` reports the link error for `team`, collects `keep`, leaves no `source/team` when missing, and keeps the link and the external tree when live.
- `TestHandleCollect_RefusesDeclaredEntry` (`internal/server`; missing and live): `POST /api/collect` returns 200 with `failed.team` naming the entry, `pulled` is `[keep]`, and nothing is created or replaced.

### P2: Refuse restoring trash beneath an offline followed entry

Fixed in `120bd8e6`. Confirmed as described. With `team` declared and its link missing, restoring a trash entry named `team` or `team/skill` succeeded and created a real `source/team`. With the link live, `team` failed as `'team' already exists` (500 on the dashboard) and `team/skill` was refused by `sourcefs` (500 on the dashboard).

What changed: `trash.Restore` (the skills-source restore) checks `entry.Name` against the source's declarations with `InFollowed` before creating anything, and refuses with `<source>/<entry> is a link; edit its target directly` (`errors.Is(err, sourcefs.ErrLink)`). The check uses a bare snapshot, `sourcewalk.Follow(destDir, FollowOptions{})`, inside `Restore` instead of a new parameter: whether a name is declared depends only on the declaration files, not on targets or the Git root (the same reasoning as `followedDestError`), and the trash TUI does not hold a config to build a full snapshot from. All three callers (CLI `trash restore`, the trash TUI, the dashboard) are therefore covered without signature changes. The dashboard maps the link error to 409, like the other handlers. `RestoreAgent` writes the agents source and is unchanged. `internal/trash` importing `sourcewalk` adds no cycle (`sourcewalk` depends only on `utils`).

Coverage:

- `TestRestore_RefusesDeclaredEntry` (`internal/trash`; names `team` and `team/skill`, entry missing and live): `ErrLink`, the trash entry stays, no `team` is created, and a live link and its external tree are unchanged.
- `TestHandleRestoreTrash_RefusesDeclaredEntry` (`internal/server`; missing and live): 409 with the link error, the trash entry stays, nothing is created or replaced.

## Failures before the fix

The tests were written and run in the container before each fix:

```text
--- FAIL: TestRestore_RefusesDeclaredEntry
    restore_follow_test.go:45: restore into a declared entry: got <nil>, want ErrLink                       (team/missing)
    restore_follow_test.go:45: restore into a declared entry: got 'team' already exists in …, want ErrLink    (team/live)
    restore_follow_test.go:45: restore into a declared entry: got <nil>, want ErrLink                       (team/skill/missing)
--- FAIL: TestHandleCollect_RefusesDeclaredEntry/missing
    failed[team] = "", want "…/skills/team is a link; edit its target directly"
    pulled = [team keep], want [keep]
    request created the declared entry: <nil>
--- FAIL: TestHandleRestoreTrash_RefusesDeclaredEntry
    expected 409, got 200: {"success":true}                                                               (missing)
    trash entry should stay after a refused restore
    request created the declared entry: <nil>
    expected 409, got 500: {"error":"failed to restore: 'team' already exists in …"}                       (live)
--- FAIL: TestSkillfollowCollectRefusesDeclaredEntry/global/missing
    expected output to contain "…/skills/team is a link; edit its target directly"
    collect created the declared entry: <nil>
--- FAIL: TestSkillfollowCollectRefusesDeclaredEntry/project/missing   (same two assertions)
--- FAIL: TestSkillfollowSymlinkSyncRefusesDeclaredEntry
    expected output or error to contain "…/skills/team is a link; edit its target directly", got:
        stdout: ✓ claude    files migrated and linked
    sync created the declared entry: <nil>
--- FAIL: TestSkillfollowInitRefusesDeclaredEntry/import
    expected output to contain "…/skills/team is a link; edit its target directly" (output: ✓ Skills   2 imported)
    init created the declared entry: <nil>
--- FAIL: TestSkillfollowInitRefusesDeclaredEntry/builtin-skill
    expected output to contain "…/skills/skillshare is a link; edit its target directly"
```

Passing before the fix by design (they pin the existing refusal of a live link): `TestRestore_RefusesDeclaredEntry/team/skill/live`, `TestHandleCollect_RefusesDeclaredEntry/live`, and `TestSkillfollowCollectRefusesDeclaredEntry/{global,project}/live` (all with `--force`). The sweep tests were shown failing by reverse-applying the sweep diff (`git apply -R`), running them, and re-applying it.

## Sweep: writes into the skills source

Every write below the skills source whose destination comes from a user-, target- or remote-supplied name, or a fixed name a user could declare:

| Path | Status |
|---|---|
| Collect (CLI global/project, `POST /api/collect`) | Fixed in `a972ad33` (`PullOptions.Follow`, `InFollowed`) |
| Trash restore (CLI, TUI, `POST /api/trash/{name}/restore`) | Fixed in `120bd8e6` (`InFollowed` in `trash.Restore`) |
| `sync` of a symlink-mode target holding files (`MigrateToSource`; CLI `sync` and `POST /api/sync`) | Was unguarded: merged a target folder named like an offline entry into the source, then linked the target to the source. Fixed in `ad70251c` (`followedMergeError`, checked before any write). When the source does not exist yet, there is no declaration to protect |
| `init` importing detected skills (`--copy-from`, default import) into an existing source | Was unguarded. Fixed in `ad70251c`: skipped with a warning naming the entry |
| `init` built-in skill (`--skill`) | `install.Install` already refused an offline `skillshare` entry, but the minimal-copy fallback then created it. Fixed in `ad70251c`: a link refusal ends the step |
| `upgrade` (built-in skill download) | Already guarded: goes through `install.Install` → `followedDestError` |
| Install funnel (CLI `install` incl. hub/search installs, `--into`, `--track`, dashboard install and batch install, rehydrate, bare install) | Already guarded (round 16: `followedDestError`, `InstallTrackedRepo`, `InstallFromConfig` skip) |
| `skillshare new`, dashboard create skill | Already guarded (round 16) |
| `pull`, `init --remote`, branch checkout (Git) | Already guarded (round 17: `CheckSourceMutation`) |
| Dashboard content save, toggles, target edits, source URL edit, uninstall, `list` TUI frontmatter toggle | Write only inside an existing discovered skill (an offline entry has none) or are refused through `followedSkillWriteError`; unchanged |
| `.metadata.json`, `registry.yaml`, `.skillignore`, hub index | Fixed names at the source root, never a declared entry; unchanged |
| `update` (`swapStagedIntoSource`, `removeInSource`), install cleanup (`removeAll`), Git cleanup (`removeInRepo`, `.git.disabled` rename, metadata conflict merge) | Replace or remove existing paths; followed updates go through `PrepareFollowedUpdate`; unchanged |
| `backup restore`, file-backup restore | Write targets or config files, not the skills source; unchanged |
| Agents (create, install, collect, trash `RestoreAgent`), extras, MCP, hooks, AGENTS.md instructions | Write their own sources or config files, which `.skillfollow` does not cover; unchanged |
| Hub drafts | Read-only on the source; unchanged |

The family should now be closed: every path that can create a first-level name in the skills source either checks `InFollowed` (directly, via `followedDestError`/`followedSkillWriteError`, or via `CheckSourceMutation`) or only writes inside paths that already exist.

## User-visible behavior changes (for the docs update)

These apply only when `.skillfollow` or `.skillfollow.local` declares the entry. With no declaration, all output is unchanged.

1. `skillshare collect` (global and project) and dashboard collect report a target skill named like a declared entry as failed with `<source>/<entry> is a link; edit its target directly`, live or offline; the other skills are still collected. Before, an offline entry was created, and a live one was skipped as "already exists" (refused only with `--force`).
2. `skillshare trash restore`, the trash TUI and dashboard restore refuse a trashed skill whose name is a declared entry or is below one, live or offline, with the same error; the item stays in the trash. The dashboard answers 409 (before: 200 offline, 500 live).
3. `skillshare sync` (and dashboard sync) of a symlink-mode target that still holds a folder named like a declared entry fails for that target with the same error, before migrating anything; the target keeps its files and is not replaced by a link.
4. `skillshare init` skips importing a tool's skill named like a declared entry in an existing source, with the warning `Skipped <name>: <source>/<entry> is a link; edit its target directly`.
5. `skillshare init --skill` with an offline entry declared as `skillshare` reports `Failed to install the skillshare skill: … is a link; edit its target directly` instead of writing the minimal fallback.

## Evidence

All Go commands ran in a throwaway container from the devcontainer image (compose project `skillshare_wt_round18`, this worktree at `/workspace`). Nothing ran on the host. `make check` inside the container exited 0 after `120bd8e6` and again after `ad70251c`. Tail of the final run:

```text
=== RUN   TestXDG_StatusWorksWithXDGPath
--- PASS: TestXDG_StatusWorksWithXDGPath (0.03s)
PASS
ok  	skillshare/tests/integration	56.852s

✓ All tests passed!
```

## Notes and limits

- `collect --dry-run` (CLI) still lists a skill named like a declared entry under "Would collect": the CLI dry run builds its preview without calling `PullSkills`. Nothing is written; the real run reports the failure. Left as is to keep the fix minimal.
- With an unreadable declaration the snapshot knows no entries, so these checks do not apply. This is unchanged from rounds 16 and 17.
- `followedMergeError` lists the target's top-level entries with `os.ReadDir`, so `ad70251c` adds one `target` allowance to `internal/sourcewalk/allowlist.json`; `TestRawWalkGuard` and `TestRawWriteRatchet` pass.
- No website docs, README or built-in skill edits.
