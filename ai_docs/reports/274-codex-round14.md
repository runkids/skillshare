# PR 390 Codex round 14 fix

The finding held against reviewed commit `411c5bcf`. The new regression tests failed before the fix and pass after it. The fix is one commit on `runkids/codex-round14`. Nothing was pushed and no GitHub thread was changed.

## Reply ready for the coordinator

### P2: Classify skills under nested repositories as repo members

Fixed in `1e5ecfaf`. Confirmed, and it is not limited to `.skillfollow`: a repo installed with `install --track --into group/sub` sits at `group/sub/_repo` and hit the same path through the ordinary walk. Discovery set `IsInRepo` only when the first path component was `_`-prefixed, while `.skillignore` matching already used the innermost tracked repo at any depth. A skill at `group/sub/_repo/a` was therefore emitted with `IsInRepo == false`, and every consumer that keys off it treated the skill as a plain one.

Discovery now records the innermost tracked repo for each skill with the same lookup the ignore matcher uses (the tracked repos recorded during the walk), stores its path in a new `DiscoveredSkill.RepoRelPath`, and sets `IsInRepo` from it. A `_`-prefixed first component still counts without a `.git`, as before, so first-level repos behave exactly as they did. `internal/resource/skill.go` applies the same rule and now fills `RepoRelPath` too.

Target overrides are keyed by the skill's own relative path (`group/sub/_repo/a`), as written by the dashboard target edit. `applyTargetOverrides` only consults them for `IsInRepo` skills, so before the fix it skipped the override and the skill synced to its frontmatter targets.

Consumers changed:

- `list`: `RepoName` is the recorded repo path instead of the first path component.
- `status --json`: per-repo `skill_count` is counted by recorded repo path instead of the first path component.
- `list` TUI `group:` filter (`skillGroup`): skips the repo directory only when the repo is the first component, so a skill in a nested repo keeps its outer group as before.

Consumers that needed no change, because they already match the full repo path or walk up for `.git`, and only lacked a correct `IsInRepo`: text `status` and `list` per-repo counts (`strings.HasPrefix(RelPath, repo+"/")` against the full tracked-repo path), dashboard overview repo counts, dashboard batch uninstall and single uninstall guards, dashboard single and batch target edits (they write `.metadata.json` overrides for `IsInRepo` skills), hub draft/index `isInRepo`, CLI and dashboard analyze `is_tracked`, dashboard skill branch fallback, `handler_skill_content` `findRepoRoot`, and `applyTargetOverrides`. CLI `uninstall` has no per-skill tracked-repo guard for first-level repos either, so there is nothing to align there.

Coverage:

- `TestDiscoverSourceSkillsNestedRepoOwnership` (`internal/sync`), subtests `plain` and `followed`: `group/sub/_repo/a` with frontmatter targets `cursor` and a `.metadata.json` override `[claude]`; asserts `IsInRepo`, `RepoRelPath == "group/sub/_repo"`, and `Targets == [claude]`.
- `TestSkillKind_Discover_NestedTrackedRepo` (`internal/resource`): the resource-kind walk reports the same repo.
- `internal/server`: `TestHandleBatchSetTargets_NestedRepoSkillOverrides` (batch edit on `group/sub` sets the override and leaves the clone clean), `TestHandleBatchUninstall_NestedRepoSkillRefused` (refused with the tracked-repo error, skill kept), `TestHandleOverview_NestedRepoSkillCount` (repo `group/sub/_repo` counts 1 skill).
- `TestSkillfollowGroupNestedRepoOwnership` (`tests/integration`), subtests `plain` and `followed`, mirroring `TestSkillfollowGroupNestedRepoSkillignore`: `list --json` reports `repoName: group/sub/_repo`, `status --json` counts 1 skill for that repo, and `sync` links the skill to the override target `cursor` and not to `claude`.
- `TestSkillGroup_TrackedRepoInGroup` (`cmd/skillshare`): the TUI group of a nested-repo skill stays `group`.

## User-visible behavior changes (for the docs update)

These are fixes to pre-existing `install --track --into` behavior and apply equally to repos inside a followed group. Without a repo below the first level, every output is unchanged.

1. **Target overrides apply to skills in nested repos.** A dashboard target edit on such a skill is stored in `.metadata.json` (the clone stays clean), and `sync` honors it. Before, the edit rewrote the repo's `SKILL.md`, and an override already in `.metadata.json` was ignored.
2. **The dashboard refuses to uninstall a single skill from a nested repo**, with the same "uninstall the repo instead" error as for a first-level repo. Before, it moved the skill to trash and left the clone dirty.
3. **`list` shows the repo.** `list --json` `repoName` and the text "tracked: …" label name the nested repo, and the skill's type is `tracked` in the list TUI. The branch shown is the repo's branch.
4. **Repo skill counts.** `status`, `status --json`, `list`, and the dashboard overview count a nested repo's skills under that repo instead of 0.
5. **Tracked flags.** Dashboard skill list `isInRepo`, analyze `is_tracked`, and hub index `isInRepo` are true for nested-repo skills. Hub drafts look up the repo's tracked metadata entry for them, like first-level repo skills.
6. **A tracked repo inside another tracked repo** now owns its own skills: `status --json` counts them under the inner repo instead of the outer one, and `list` names the inner repo. Text `status`, text `list`, and the dashboard overview match by path prefix and still count them under both repos, as before.

## Evidence

All Go commands ran in a throwaway container from the devcontainer image (compose project `skillshare_wt_round14`, this worktree at `/workspace`). Nothing ran on the host.

Before the fix (new tests added, `DiscoveredSkill.RepoRelPath` declared but not set, discovery and consumers at `411c5bcf`):

```text
--- FAIL: TestDiscoverSourceSkillsNestedRepoOwnership/plain (0.02s)
    discover_follow_test.go:132: IsInRepo=false RepoRelPath="" Targets=[cursor]
--- FAIL: TestDiscoverSourceSkillsNestedRepoOwnership/followed (0.01s)
    discover_follow_test.go:132: IsInRepo=false RepoRelPath="" Targets=[cursor]
--- FAIL: TestHandleBatchSetTargets_NestedRepoSkillOverrides (0.02s)
    handler_nested_repo_test.go:44: expected clean tracked repo, got:
         M nested/SKILL.md
--- FAIL: TestHandleBatchUninstall_NestedRepoSkillRefused (0.02s)
    handler_nested_repo_test.go:58: expected tracked-repo refusal, got 200: {"results":[{"name":"nested","kind":"skill","success":true,"movedToTrash":true}],"summary":{"succeeded":1,"failed":0}}
--- FAIL: TestHandleOverview_NestedRepoSkillCount (0.01s)
    handler_nested_repo_test.go:81: expected group/sub/_repo with 1 skill, got [{Name:group/sub/_repo SkillCount:0 Dirty:false}]
--- FAIL: TestSkillfollowGroupNestedRepoOwnership/plain (0.06s)
    skillfollow_group_test.go:220: list: want repoName group/sub/_repo, got [{RelPath:group/sub/_repo/a RepoName:}]
    skillfollow_group_test.go:235: status: want group/sub/_repo with 1 skill, got [{Name:group/sub/_repo SkillCount:0}]
    skillfollow_group_test.go:243: skill linked to claude despite override: <nil>
--- FAIL: TestSkillfollowGroupNestedRepoOwnership/followed (0.05s)
    (same three assertions)
--- FAIL: TestSkillKind_Discover_NestedTrackedRepo (0.00s)
    kind_test.go:122: IsInRepo=false RepoRelPath="", want true group/sub/_repo
```

`TestSkillGroup_TrackedRepoInGroup` was added with the `skillGroup` change and was not run against the old code. It guards the regression the discovery fix would otherwise cause: with `RepoName` now set, the old rule returns `sub`.

After the fix, the new tests pass and `make check` passed, including `TestRawWalkGuard` and `TestRawWriteRatchet`. Tail:

```text
=== RUN   TestXDG_StatusWorksWithXDGPath
--- PASS: TestXDG_StatusWorksWithXDGPath (0.03s)
PASS
ok  	skillshare/tests/integration	59.854s

✓ All tests passed!
```

## Notes and limits

- Repo ownership is the innermost tracked repo, matching `.skillignore`. Agents (`internal/resource/agent.go`) still use the outermost tracked repo; that walk is unchanged.
- CLI `uninstall` resolves names by path and has no per-skill tracked-repo guard, for first-level and nested repos alike. It is unchanged.
