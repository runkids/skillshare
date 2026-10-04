# PR 390 Codex round 3 fix

The P2 finding held against reviewed commit `3fae6812`. The new regression tests failed before the fix and pass after it. The fix is one commit on `runkids/274-codex-round3`; nothing was pushed and no GitHub thread was changed.

## Reply ready for the coordinator

### P2: Pass the follow snapshot into diff calculations

Fixed in `387a79e6`. Diff now keeps the operation's follow set and makes the same prune and identity decisions as sync, using the sync-side helpers instead of copies of the rules. `internal/sync` exports `PrunePaused`, `KeepsManagedCopies` (copy sync now calls it too), and `SameSkillLink`; nil-follow callers behave exactly as before.

- CLI `diff` (global and project): while an entry is unavailable, managed copies no longer show as `remove`, standard-naming managed copies that sync would keep show as `keep` ("Kept"), each target carries `prune_paused` in `--json`, and the text output prints `<target>: prune paused; unavailable .skillfollow entry: <name> (<state>)`.
- Dashboard `GET /api/diff` and `GET /api/diff/stream`: the same set reaches `computeTargetDiff`. While paused, no `prune` items (orphan links, manifest copies, flat-name heuristics) are reported, kept copies show as `skip`, and each target carries `prune_paused`. With a set, merge link identity uses `SameSkillLink`, so a managed link whose text is the fully resolved followed path no longer shows as "symlink points elsewhere".

Coverage: `TestSkillfollowDiffPreviewsPausedPrune` (integration, global and project) and `TestServerSkillfollowDiffPreviewsPausedPrune` (GET and SSE). Both use a followed group, a missing entry, a copy target with standard naming and a managed copy from the missing entry, and a merge target whose managed link text is the resolved followed path. After the entry is restored (CLI) or removed from `.skillfollow` (server), diff reports the orphan removal and content update, and CLI sync then does exactly that.

## Evidence

Before the fix (same tests, fix not applied):

```text
TestServerSkillfollowDiffPreviewsPausedPrune/diff:
  claude prune_paused = []
  claude: update group__c (symlink points elsewhere) while prune is paused
  claude: prune missing__x (orphan symlink) while prune is paused
  cursor: prune x (orphan copy) while prune is paused
  cursor c = "update: content changed"
  (the same for diff/stream)
TestSkillfollowDiffPreviewsPausedPrune/{global,project}:
  claude prune_paused = [] / copyt prune_paused = []
  copyt reports remove x (orphan (will be pruned)) while prune is paused
  copyt c = {Action:modify Name:c Reason:content changed}, want keep
  missing "prune paused; unavailable .skillfollow entry: missing (missing)"
```

After the fix, both pass. `make check` passed in a throwaway devcontainer for this worktree (docs-check, formatting, vet, both ratchets `TestRawWalkGuard` and `TestRawWriteRatchet`, unit and integration tests). Tail:

```text
=== RUN   TestXDG_StatusWorksWithXDGPath
--- PASS: TestXDG_StatusWorksWithXDGPath (0.03s)
PASS
ok  	skillshare/tests/integration	56.713s

✓ All tests passed!
```

Default behavior is unchanged: with no `.skillfollow` the set is nil, `PrunePaused` returns nil (the `prune_paused` keys are `omitempty`), `KeepsManagedCopies` is false, and the legacy link comparison stays. No existing test changed.

Docs: added a `diff` bullet to `website/docs/reference/skillfollow.md`, its four translations, and `skills/skillshare/references/skillfollow.md`. The website build passed.

## Limits

- CLI merge diff has never checked link identity or reported orphan links; it only distinguishes links from local directories. It now names the pause but still does not preview link pruning. Adding that would be new behavior, not part of this finding.
- The dashboard merge orphan check keeps its own containment rule rather than sync's manifest-based ownership; only the pause gate is shared. Making it match `ownsLink` would change default output.
- The interactive diff TUI shows kept copies but not the pause line. The dashboard UI does not render `prune_paused` yet; the API returns it.
- Not run: Windows, frontend tests (no UI change).
