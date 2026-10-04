# Proposal 274: internal review fixes

The last fix round on PR #390 before merge. Three read-only reviewers (Go correctness, docs/code audit, tests/ratchets) covered `e4a3d8a8..3470ad73`. They reported no P1 findings. Work is on `runkids/review-fixes`, which started at `runkids/274-step3` `3470ad73`. Nothing was pushed.

## Commits

```text
d1b46c76 docs(skillfollow): document follow/unfollow as the normal setup and match real output
06e7de21 test(skillfollow): cover follow --to rollback when the declaration write fails
52241fa5 test(skillfollow): skip the read-only source test on Windows
588e45df fix(skillfollow): print an untrack command that runs from any directory
0a6799f0 test(skillfollow): pin the empty JSON arrays of follow and unfollow
40b86470 fix(skillfollow): match follow and unfollow names with Windows semantics
c611f81e fix(skillfollow): fold entry names the same way when matching and deduplicating
```

## Per finding

| # | Finding | Commit | What changed | Covering test | Fails before? |
|---|---|---|---|---|---|
| 1 | P2: no test reaches the `follow --to` rollback | `06e7de21` | Test only | `TestFollow_ToRemovesNewLinkWhenDeclarationFails` (integration). `.skillfollow` is a link to an outside file, so `follow x --to <ext> --json` fails. The test checks the JSON `error` object, that `<source>/x` is gone, and that the outside file is byte-identical. | Yes. With the `Unlink` call removed, the test fails with `the new link was left in place: <nil>`. |
| 2 | P3: follow/unfollow compare names exactly | `40b86470` | `skillfollow.RemoveEntry` now returns the first removed line's spelling (`""` when none) instead of a bool. `unfollow` sets `ignore_line` from that spelling and falls back to the typed spelling when the declared line was not in the block. `follow` matches its state row and the staged link through `sameFollowName` (`sourcewalk.SameEntryName`), then adds that link's own `IgnoreLine`. The misplaced comment is back on `TestAddEntry_FailedWriteLeavesFileUntouched`. | `TestEntries_MatchCaseInsensitivelyWhenNamesFold` now also checks `Declares(TEAM)` and that `RemoveEntry` returns `Team`. The new `TestFollowIgnores_UsesDeclaredSpellingWhenNamesFold` in `cmd/skillshare` checks that `follow team` with a declared `Team` link adds `/Team`. | Yes for the handler test (`ignore lines = [], want ["/Team"]` with the old match). The skillfollow spelling assertion tests the new return value. |
| 3 | P3: `sameEntryName` and `entryNameKey` fold differently | `c611f81e` | `sameEntryName(a, b)` is now `entryNameKey(a) == entryNameKey(b)`. `entryNameKey` uses `strings.ToLower(strings.ToUpper(name))` when names fold and the exact name otherwise. `utils.PathsEqual` is unchanged. | `TestSameEntryNameFolding` (table, both semantics) checks that matching and the key agree, including `ſkill`/`SKILL`. | Yes. With the old key: `entryNameKey("ſkill") == entryNameKey("SKILL") is false, want true`. |
| 4 | P3: empty JSON arrays serialize as `null` | `0a6799f0` | Test only. The finding does not reproduce: `writeJSON` calls `ensureEmptySlices`, which already turns every nil slice field into `[]`. The handlers were not changed. | `TestFollow_PrintsRejectedState` asserts `"ignore_lines_added": []`. `TestUnfollow_JSON` asserts `"still_declared_in": []`, and `"files_edited": []` on a second unfollow. | No. They passed on the unchanged handlers, which shows the finding was wrong. They stay as regression guards. |
| 5 | P3: `untrack_command` is relative to the source | `588e45df` | The new `gitops.UntrackCommandIn(dir, path)` prints `git -C '<source>' rm --cached -- '<rel>'` and shares its quoting with `UntrackCommand`. Only `follow` uses it. `doctor`, `CheckFollowedLinks`, and the incoming-commit guard keep the relative form, and their docs already say to run it "from the source". `follow.md` (5 languages) shows the new string and explains the difference. | `TestFollow_IndexedLinkIsNotUntracked` asserts the full `git -C '<source>' rm --cached -- '_dev'` row. `internal/git` tests still pass. | Yes. With `UntrackCommand` back in place, the row assertion fails. |
| 6 | P3: a permission test does not skip on Windows | `52241fa5` | `TestHandlePutSkillfollow_FailedWriteKeepsFile` now skips when `runtime.GOOS == "windows" \|\| os.Geteuid() == 0`, like its neighbors. | Itself. `GOOS=windows go vet ./internal/server/` passes. | Not applicable (a guard change). |
| 7 | Docs | `d1b46c76` | The README `.skillfollow` section (5 languages) now leads with `follow --to` / `unfollow` and keeps `ln -s` as the manual path. It replaces "Windows junction verification pending" with the current state: global-mode junction cases pass `scripts/windows/e2e-skillfollow.ps1`, while the `follow --to` junction and case-insensitive names have not run on a real Windows machine. The built-in skill reference says "Not supported yet" and lists update refusal as verified. The `follow` target-overlap and indexed samples and the `unfollow --keep-link` sample now include the `Next` block. These samples were copied from the binary's output in the container. `follow.md` and `unfollow.md` (5 languages) say that argument errors print plain text even with `--json`. | Captured output: `follow out --to ~/.claude`, `follow _dev` (indexed), and `unfollow --keep-link` each print `Next / skillshare sync …`. `follow --bogus --json` and `unfollow --json` print `✗ unknown flag: --bogus` and `✗ usage: skillshare unfollow <name>` as plain text. | Not applicable. |

### Limit of item 2's coverage

The unexported `caseInsensitiveNames` switch in `sourcewalk` can be flipped only by that package's tests. A CLI-level test with Windows folding is therefore not possible without exporting a test hook, and none was added. The handler logic is covered by injecting the comparison (`sameFollowName` in `cmd/skillshare`, `sameName` in `internal/skillfollow`). Unfollow's use of the returned spelling is covered only at the `skillfollow` level. Under exact (Unix) matching the declared and typed spellings are always equal, so the existing integration tests confirm that Unix behavior did not change. A Windows run on real hardware is still pending (see the `skillfollow.md` Limits section).

## Verification

All commands ran in a throwaway container from the devcontainer image, with this worktree mounted at `/workspace`.

- `make check`: exit 0. It runs docs-check, gofmt, `go vet`, lint, unit, and integration tests, and ends with `ok skillshare/tests/integration 59.851s` and `✓ All tests passed!`. It had 0 `--- FAIL` lines. The container runs as root, so tests that need Unix permission enforcement skipped as before.
- Ratchets: `TestRawWriteRatchet`, `TestFollowSnapshotGuard`, `TestRawWalkGuard`, and `TestFollowWriteGuard` all pass. No allowlist changed.
- `GOOS=windows go vet ./...`: pass.
- `python3 scripts/ai-context.py check`: ok.
- `cd website && ./node_modules/.bin/docusaurus build` ×5: every run exited 0, each with 5 `[SUCCESS]` lines (en, ja, ko, zh-Hans, zh-Hant) and 0 broken-link or warning lines.
- Follow-cmd runbook: not re-run. It asserts no string that these commits changed. Its only JSON check is `.still_declared_in == [".skillfollow"]`.

## Not changed (recorded follow-ups)

- An oplog entry for a failed follow/unfollow. Current behavior matches `enable`.
- `--to` pointing at an ancestor or a target. It reports `cycle`/`target-overlap`, by design.
- Path traversal in `/api/audit/{name}`. This is pre-existing and needs a separate fix.
- The audit TUI under `--agents`.
