# PR 390 Codex round 6 fixes

Both findings held against reviewed commit `62d51cfc`. Each new regression test failed before its fix and passes after it. The fixes are two commits on `runkids/274-codex-round6`; nothing was pushed and no GitHub thread was changed.

## Replies ready for the coordinator

### P1: Reject regular-skill updates below followed groups

Fixed in `bcbcd524`. The `gitPull` claim is confirmed. A metadata-backed skill whose source is a git URL without a subdir, and whose directory is a git checkout, reaches `install.handleUpdate`, which runs `gitPull` on the checkout. That path skips the followed-repository clean-tree and fast-forward-only policy. Before the fix, `update group/g` and `update --all` both moved the user's checkout HEAD in the tests.

The new `install.RefuseFollowedSkillUpdate` refuses every non-repository target for which `InFollowed` succeeds. It runs where targets are executed, in `updateRegularSkill` (the single-target path) and `executeBatchUpdate`. So `--all`, names, globs, group expansion, and project mode share one guard. The refusal happens before any fetch, audit, or write, and also in dry run. Each refused item is reported as a failure with `followed repository update refused: skill <path> is inside followed entry <name>; skillshare does not reinstall skills in a followed tree`, in text and in `--json` (`type: skill`, `status: failed`). Other items in the batch still update. Repository targets still go through `PrepareFollowedUpdate`.

Coverage:

- `TestFollowedGroupSkillUpdateRefused` covers real and dry-run modes. It checks the single path on a skill that is a git checkout, and a batch that holds that skill, a grouped subdir skill, and an ordinary checkout. Refused items are reported, the user's checkout HEAD does not move, and the ordinary checkout updates.
- `TestFollowedGroupSkillUpdateRefusedProjectAll` covers project `--all`.
- `TestSkillfollowUpdateRefusesFollowedGroupSkill` is an integration test in global and project modes. It covers `--all --dry-run`, explicit names with and without `--dry-run`, a glob (global only, because project names are exact paths), `--group group`, positional `group`, and `--all --json`. The JSON test shows both followed-group skills as refused items, `updated: 1` for the ordinary skill, and the checkout HEAD unchanged.

### P2: Fail staging when declarations could not be read

Fixed in `68963e37`. `declaredLinkLocations` now returns `follow.Err()` before it trusts `ParsedEntries()`. It is the single choke point for `FollowedLinksStaged` and `CheckFollowedLinks`. Those cover `commit`, `push`, `init`'s source commit, `StageAll`, the dashboard git handlers, and the source pull/checkout guard (`source_mutation.go`). `doctor` already reports a `FollowedLinksStaged` error as a Skillfollow warning, so it now shows the read error instead of showing no links.

Coverage: `TestFollowedLinksUnreadableDeclarationFailsClosed` (internal/git) builds an ignored `.skillfollow.local` with mode `000`, a link it would declare, and no `.skillfollow`. `FollowedLinksStaged`, `CheckFollowedLinks`, and `StageAll` all return the read error, and nothing is staged. `TestFollowedStagingUnreadableDeclaration` (cmd) covers `commit --dry-run`, `push --dry-run`, `stageAndCommit`, and `commitSourceFiles`. Each route fails with `read skillfollow declaration ...` and `git ls-files` stays empty. Both tests skip on Windows and as root. They ran as UID 1000 here, so neither skip applied.

## Evidence

All Go commands ran in a throwaway container from the devcontainer image (compose project `skillshare_wt_codex_round6`, this worktree at `/workspace`). Nothing ran on the host.

Before the fix (implementation reverted, tests kept):

```text
TestFollowedGroupSkillUpdateRefused/real:     single: {updated:1 ... items:[]} <nil>   (checkout pulled)
TestFollowedGroupSkillUpdateRefused/dry-run:  single: {updated:0 skipped:1 ... items:[]} <nil>
TestFollowedGroupSkillUpdateRefusedProjectAll: missing refusal: &{updated:2 skipped:3 ...} <nil>
TestSkillfollowUpdateRefusesFollowedGroupSkill/{global,project}:
  expected failure, but command succeeded (every route)
  [--all --json] pulled the user's checkout
TestFollowedLinksUnreadableDeclarationFailsClosed: FollowedLinksStaged accepted an unread declaration: []
TestFollowedStagingUnreadableDeclaration/{commit,push,stage,init}: missing refusal: <nil>
```

After the fix, `make check` passed (exit 0) as UID 1000. It covered docs-check, fmt-check, vet/lint, unit tests, and integration tests. `TestRawWriteRatchet` and `TestRawWalkGuard` passed, and every new test ran and passed. Tail:

```text
=== RUN   TestXDG_StatusWorksWithXDGPath
--- PASS: TestXDG_StatusWorksWithXDGPath (0.03s)
PASS
ok  	skillshare/tests/integration	58.407s

✓ All tests passed!
```

Default behavior is unchanged. With no `.skillfollow` the follow set is nil: `RefuseFollowedSkillUpdate` returns nil and `declaredLinkLocations` returns before the new check. No existing test changed.

## Notes and limits

- **Group expansion and project names classify any git checkout as a repository.** This includes checkouts without the `_` prefix, and the behavior predates this work. Under a followed group, such a checkout is therefore pulled through `PrepareFollowedUpdate`, with the clean-tree and fast-forward-only policy. It is not refused. `--all` and global names see the same directory as a regular skill and refuse it. Both outcomes avoid the unsafe pull. The fix follows Codex's wording ("reject non-repository update targets") and keeps round 2's `group/sub/_repo` support.
- **Dashboard follow-up (not fixed).** The dashboard's single update route (`Server.updateRegularSkill`, reached from `updateSingleByKind` through a metadata entry) has no followed-tree check. A metadata entry under a followed group therefore reaches the same `install.handleUpdate` direct pull. `install.RefuseFollowedSkillUpdate` can be reused there. That is outside this finding's CLI scope.
- **Docs unchanged.** The reference pages do not describe either case, and their current text stays accurate.
- **Not run:** real Windows, and frontend tests (no UI change).
