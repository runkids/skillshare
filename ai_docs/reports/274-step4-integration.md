# Proposal 274 step 4: integration

Merges the three step-4 branches into the PR #390 line so the whole `.skillfollow` feature ships in one PR. Work is on `runkids/integrate-step4`, which started at `runkids/274-step3` `10da0eb2`. Nothing was pushed, and no source branch or worktree was deleted.

## Commits

```text
df19b5de docs(skillfollow): reconcile the reference after integrating step 4
b2ad3dc7 refactor(skillfollow): share the prune-pause sentence
4bfa25e9 merge: integrate runkids/skillfollow-ui into 274-step3
5d4f2cde fix(skillfollow): match declaration lines with Windows name semantics
35364cd0 merge: integrate runkids/follow-cmd into 274-step3
1cff93ff fix(skillfollow): pass the snapshot to collect's skill pull explicitly
97f63b46 merge: integrate runkids/discovery-entry into 274-step3
```

The order was discovery-entry, then follow-cmd, then skillfollow-ui. Each merge used `--no-ff`, so all 15 branch commits are preserved. `git branch --merged` lists all three branches. After each merge, the build and the touched packages' tests ran in the container before the next merge started.

## Resolved conflicts

| Merge | File | This branch wanted | Feature branch wanted | Kept |
|---|---|---|---|---|
| discovery-entry | `cmd/skillshare/doctor.go` `runDoctorChecks` | Skip `skills_validity`, `skill_integrity`, and `skill_targets_field` when discovery failed (round 13+) | `checkSkillsValidity(..., follow)` | The skip block, with the snapshot passed to `checkSkillsValidity` inside it |
| follow-cmd | `website/docs/reference/skillfollow.md` Limits, plus ja/ko/zh-Hans/zh-Hant | Windows 11 ARM64 verification record; `follow`/`unfollow` listed as future work | `follow`/`unfollow` removed from future work; note that `follow --to`'s junction path is unverified | The verification record, `follow`/`unfollow` removed from future work, and the `--to` note |
| skillfollow-ui | `internal/sourcewalk/parse.go` | `ValidEntryName` and the new `SameEntryName` wrapper (`5d4f2cde`) | `ValidEntryName` with a different comment | Both wrappers |
| skillfollow-ui | `skillfollow.md` Limits (5 languages), `skills/skillshare/references/skillfollow.md` | Verification record, "dashboard declaration editor" as future work | Editor removed from future work; older Windows wording | The verification record, with the editor removed from future work |

## Compile and ratchet fallout

The discovery-entry merge compiled without changes. None of the code that rounds 13-21 added on this branch called a removed follow-less API or a `follows ...` variadic. The only variadics left are the five in `internal/git`, which the discovery-entry report deliberately kept. `TestFollowSnapshotGuard` found two gaps, both from code this branch added after the discovery-entry base:

```text
{File:internal/server/handler_collect.go Function:(*Server).handleCollect Rule:options Expr:ssync.PullOptions{Force: body.Force}}
{File:internal/server/handler_update.go Function:(*Server).updateAgent Rule:argument Expr:s.updateTrackedRepo}
```

Both are agents-source operations, the same exception as the existing agent reinstall rows. In `1cff93ff`:

- `handleCollect` built one `PullOptions` for both kinds, and the skill pull copied it and then set `Follow`. The skill pull now builds its own literal with `Follow: s.skillFollowSet()`. Only the agent pull's literal omits `Follow`, and it is allowlisted with the reason "Agent pull into the agents source".
- `updateAgent` passes `nil` to `updateTrackedRepo` for agent repositories (round 18: "keep agent repo updates outside skillfollow policy"). It is allowlisted under the `argument` rule.

`follow_allowlist.json` now has 8 entries, and every one is an agents-source write or an entry from the original report. The discovery-entry call sites needed no change for `TestFollowWriteGuard`. `TestRawWalkGuard` and `TestRawWriteRatchet` stayed green, and their allowlists did not change.

## API renames applied to follow-cmd and skillfollow-ui

None were needed. Neither branch called a renamed or removed discovery or parse API: follow-cmd uses `globalSkillFollowSet(cfg)` and `skillFollowSet(runtime.sourcePath, runtime.targets, cwd)`, and skillfollow-ui uses `s.skillFollowSet()`. Both build, and both pass `TestFollowSnapshotGuard` and `TestFollowWriteGuard` unchanged.

One semantic mismatch did need a fix. Round 21 made declared names match case-insensitively on Windows, but follow-cmd's `Declares`, `AddEntry`, and `RemoveEntry` still compared lines exactly. On Windows, `unfollow team` against a `Team` line reported "not declared" while the entry stayed followed, and `follow team` appended a duplicate. `5d4f2cde` exports `sourcewalk.SameEntryName` and uses it for those line edits. The test is `TestEntries_MatchCaseInsensitivelyWhenNamesFold`.

## Small follow-ups fixed

- Prune-pause sentence (`b2ad3dc7`): `FollowSet.PrunePauses(source)` in `sourcewalk` now builds the sentence once. `cmd/skillshare` `skillfollowPauses` and `/api/skillfollow` both call it, and the output is byte-identical. The test is `TestPrunePausesNamesUnavailableEntries`.
- `/api/skillignore` source: a comment in `handleGetSkillignore` records that `s.cfg.EffectiveSkillsSource()` equals `s.skillsSource()` in project mode, and points to `TestServerProjectUpdateAllUsesProjectSource`. Behavior is unchanged.

## Docs

`df19b5de` updates English plus ja, ko, zh-Hans, and zh-Hant, following each locale's existing wording:

- `reference/skillfollow.md`:
  - Setup still leads with `follow --to`, and the dashboard tab is in the Dashboard bullet.
  - "Only `follow` and `unfollow` write declaration files" now includes the dashboard tab. Ignore lines are still written only by the commands.
  - New File format bullet: on Windows, names match entries case-insensitively, and case-only duplicates collapse to the first spelling.
  - Doctor bullet: a followed group with no skill is reported under `skills_validity`.
  - Dashboard bullet: a skill's own audit fails rather than reporting clean when its followed directory or a declaration file cannot be read.
  - Limits: the Windows list said "unfollow" was verified, but `scripts/windows/e2e-skillfollow.ps1` §8 removes a declaration by hand. It now says that, and lists `follow`/`unfollow` (including the `--to` junction) and case-insensitive matching as not verified on Windows.
  - All Codex-round sentences remain: declared-entry write refusals, unreadable declaration, doctor skipped checks, and the Git guards.
- `reference/commands/doctor.md`: "Skills without `SKILL.md`" now includes a followed group with no skill below it.
- `reference/commands/ui.md`: the Followed sources box describes the single-skill audit fail-closed behavior.
- `reference/commands/audit.md`: no change. It has no dashboard skillfollow content, and the dashboard audit behavior is in `ui.md`.
- `skills/skillshare/references/skillfollow.md`: same facts as above.

A site-wide search found no remaining "no follow/unfollow command" or "no dedicated editor tab" wording. The only match is `wiki/history/proposal-274-step-3.md`, a history file, which was left as written. `python3 scripts/ai-context.py check` printed `ok`.

Report index: `ai_docs/reports/` has no index, and `wiki/README.md` indexes only `wiki/history/` milestone logs. A step-4 milestone log belongs there once the PR lands. None applies now.

## Verification

All checks ran in the throwaway container `skillshare_wt_integrate_step4` (devcontainer image, this worktree at `/workspace`) at `df19b5de`.

`make check`:

```text
--- PASS: TestXDG_StatusWorksWithXDGPath (0.03s)
PASS
ok  	skillshare/tests/integration	68.622s

✓ All tests passed!
exit=0
```

This includes `ok skillshare/internal/sourcewalk` (`TestFollowSnapshotGuard`, `TestFollowWriteGuard`, `TestRawWalkGuard`) and `ok skillshare/internal/sourcefs` (`TestRawWriteRatchet`).

`cd ui && pnpm exec vitest run`:

```text
Test Files  99 passed (99)
     Tests  891 passed (891)
```

`cd website && ./node_modules/.bin/docusaurus build` (output written to `/tmp` inside the container):

```text
[SUCCESS] Generated static files in "../../tmp/site-build".
[SUCCESS] Generated static files in "../../tmp/site-build/ja".
[SUCCESS] Generated static files in "../../tmp/site-build/ko".
[SUCCESS] Generated static files in "../../tmp/site-build/zh-Hans".
[SUCCESS] Generated static files in "../../tmp/site-build/zh-Hant".
```

`GOOS=windows go vet ./...`: exit 0.

`ai_docs/tests/skillfollow_follow_cmd_runbook.md` in a fresh ssenv (`step4-followcmd`), run through mdproof v0.0.9: `{"total":7,"passed":7,"failed":0,"skipped":0}`. No runbook or code change was needed.

The `website/pnpm-workspace.yaml` that `pnpm install` created was deleted.

Not run:

- Real Windows, including the new name-matching fix for follow/unfollow.
- Real-shell zsh/fish completion tests, which skip in the container.
- `ui` `pnpm run build`.

## Open follow-ups

- The audit TUI's Skills tab under `audit --agents` scans the agents source as skills. This is pre-existing and unrelated to `.skillfollow` (see the discovery-entry report).
- follow-cmd deviation 4: `follow --to` pointing at an ancestor of the source or at an active target links and declares the entry, prints `cycle`/`target-overlap`, and exits 0.
- follow-cmd deviation 5: `unfollow` of an undeclared name is a no-op with a warning and exits 0.
- follow-cmd deviation 6: `follow` and `unfollow` have no `--dry-run`.
- On Windows, `unfollow team` against a `Team` declaration now removes the line. The managed `.gitignore` line is still compared exactly, so a `/Team` line written under the other spelling can stay behind in `.gitignore`. Not run on real Windows.
- `ai_docs/tests/windows_skillfollow_runbook.md` still needs a step that declares `Team` against a junction named `team` (round 21).
