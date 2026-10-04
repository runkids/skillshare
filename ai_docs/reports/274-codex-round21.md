# PR 390 Codex round 21 fix

Reviewed commit `e4a3d8a8`. Both P1 findings held. Finding 1, the single-skill audit route, reproduced exactly as described: an unreadable followed skill scanned clean, and an unreadable declaration was ignored. Finding 2, case-sensitive matching of declared names, is confirmed in the code. It was proven on Linux by switching the name semantics in tests; it was not run on real Windows. The work is three commits on `runkids/codex-round21`. Nothing was pushed and no GitHub thread was changed.

| Commit | Scope |
|---|---|
| `513c488d` | P1 (1): `GET /api/audit/{name}` scans followed skills through the resolved root and refuses an unreadable declaration |
| `08d248a5` | P1 (2): declared entry names match with Windows path semantics |

## Replies ready for the coordinator

### P1: Mark followed inputs in the single-skill audit route

Fixed in `513c488d`. Confirmed as described. `handleAuditSkill` called `ScanSkill` or `ScanSkillForProject` on `source/<name>` directly. The dashboard passes the skill's `relPath` (`group/c` URL-encoded), so a skill below a followed group was found, because `os.Stat` follows the link, but the follow snapshot was never consulted. Before the fix:

- With the external skill directory `chmod 000`, the route returned `200` with `"totalBytes":0` and `"riskLabel":"clean"`.
- With `.skillfollow` unreadable (a directory), the route returned `200` and scanned the skill as if nothing were declared.

What changed: the handler snapshots `s.skillFollowSet()` with the rest of the config. It marks the named skill with `skillsToAuditInputs`, the same helper the bulk route uses, so no new `InFollowed` call is added and `TestFollowWriteGuard` is unchanged. Then:

- An unreadable declaration returns `500` with the declaration read error. Discovery does the same in the bulk route, and the CLI fails the same way.
- A followed skill is scanned through `audit.ScanResolvedSkill`, with the same mode-specific `scan` closure that `auditGateTrackedRepo` uses. The coverage check therefore fails closed, and `scanTarget` stays the logical path.
- Any other skill, and every skill when no `.skillfollow` exists, keeps the plain scan.

Coverage: `TestServerSkillfollowAuditSkillRoute` (`internal/server/handler_skillfollow_test.go`):

- `findings`: a followed skill with risky content returns `200` with HIGH findings, and `scanTarget` is `source/group/c`; the external path never appears. This passed before and after the fix and guards the normal followed scan.
- `unreadable external skill`: `chmod 000` on the external `group/c` returns `500` with `cannot verify followed audit coverage`. It failed before the fix with the clean `200` result. The test skips as root, and the container runs as root, so the compiled test binary was also run as `nobody` (`setpriv --reuid=65534`) before and after the fix.
- `unreadable declaration`: `.skillfollow` as a directory returns `500` naming `.skillfollow`. It failed before the fix with a `200` scan.

### P1: Compare declared names with Windows path semantics

Fixed in `08d248a5`. Confirmed in the code; not run on real Windows. Every declared-name comparison in `sourcewalk` was an exact string match:

- `InFollowed` (`entry.Name == name`)
- `markMissing`
- `followedChild`
- the undeclared-link check (`declared[child.Name()]`)
- declaration dedup (`seen[name]`)

On Windows, a declaration `Team` and an on-disk junction `team` are the same entry. The effects:

- `InFollowed("team/…")` missed the offline entry, so `WriteBoundary` allowed writes below it.
- `followedChild("team")` missed the live link, so the walk treated it as a leaf link and omitted its skills while the snapshot still reported `Followed`.
- The same link was also reported as an undeclared link.
- `Team` and `team` in the declaration files were kept as two entries.

What changed: `sourcewalk` has one switch, `caseInsensitiveNames = runtime.GOOS == "windows"`, and two helpers built on it. `sameEntryName` uses `strings.EqualFold`, the same comparison `utils.PathsEqual` makes. `entryNameKey` lowercases map keys. All five places above use them. Declaration dedup keeps the first spelling and drops the later one silently, as exact duplicates already were. On Unix, matching stays exact. The `cycle`, `target-overlap` and `entry-overlap` checks already compare resolved targets with `PathsEqual` and `PathHasPrefix` (`containsPath`/`overlaps`), so they needed no change. Callers outside `sourcewalk` compare paths, not names (`internal/sync` with `PathsEqual`/`PathHasPrefix`, `internal/git` with `PathHasPrefix`), so they needed no change either.

Coverage (`internal/sourcewalk/follow_names_test.go`): each test runs both semantics on every host by switching `caseInsensitiveNames`. The exact cases pass before and after. Each folded case failed before the fix:

- `TestParseDeclarationsFoldsCaseInsensitiveNames`: `Team`, `team` becomes `[Team]` when folded and stays as two names when exact.
- `TestInFollowedMatchesDeclaredNameCase`: with an offline declaration `Team`, `InFollowed("team/skill")` matches and `WriteBoundary(source, "team")` refuses.
- `TestUndeclaredLinkMatchesDeclaredNameCase`: a link `team` under declaration `Team` is not reported as undeclared.
- `TestFollowWalkMatchesDeclaredNameCase`: with a `Followed` entry `Team` and an on-disk link `team` (the snapshot Windows produces), `Walk` reaches `team/skill/SKILL.md` and `ReadDir` reports `team` as a directory.
- `TestMarkMissingMatchesDeclaredNameCase`: `markMissing("team", …)` marks the `Team` entry missing.

Real Windows was not run. `ai_docs/tests/windows_skillfollow_runbook.md` (`scripts/windows/e2e-skillfollow.ps1`) is the runbook that would verify this on a guest, after adding a step that declares `Team` against a junction named `team`. `GOOS=windows go vet ./internal/sourcewalk/ ./internal/server/` passes.

## Verification

All checks ran in a throwaway container (`skillshare_wt_codex_round21`) built from the devcontainer image, with this worktree mounted at `/workspace`:

- The new tests: before and after each fix, as described above.
- `go test ./internal/sourcewalk/... ./internal/sync/... ./internal/git/... ./internal/server/... ./internal/audit/...`: pass.
- `GOOS=windows go vet ./internal/sourcewalk/ ./internal/server/`: pass.
- `make check`: pass. It includes `TestRawWalkGuard`, `TestRawWriteRatchet` and `TestFollowWriteGuard`.

## User-visible behavior changes

- Dashboard single-skill audit (`GET /api/audit/{name}`, the audit panel on a skill's page): a skill under a followed entry is scanned through its resolved directory. If that directory cannot be read, the request fails with a coverage error instead of reporting a clean result.
- Dashboard single-skill audit: when `.skillfollow` or `.skillfollow.local` cannot be read, the route returns the declaration read error for every skill instead of scanning.
- Windows only: declared names in `.skillfollow` and `.skillfollow.local` match on-disk entries regardless of case. `Team` follows a junction named `team`, write guards refuse writes below it, and `status` and `doctor` no longer list it as an undeclared link. Names that differ only in case collapse to the first spelling. Linux and macOS behavior is unchanged.
- No change without a `.skillfollow`.
