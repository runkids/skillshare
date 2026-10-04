# PR 390 Codex round 17 fix

The finding held against reviewed commit `cfa38c1d`. It is the Git-side member of the round-16 family: a declared entry is never created by skillshare, live or offline. The new tests failed before the fix and pass after it. The fix is one commit on `runkids/codex-round17`. Nothing was pushed and no GitHub thread was changed.

## Reply ready for the coordinator

### P1: Reject incoming paths beneath missing declarations

Fixed in `c3a46ce9`. Confirmed as described. With a source Git work tree whose `.skillfollow` declares `group`, no `group` link on disk (entry `missing`), and a remote commit adding `group/a/SKILL.md`, `CheckSourceMutation` accepted the commit and `skillshare pull` printed `✓ Pull  1 commit, 1 file changed (+1 −0)`, leaving a real `source/group/` directory. With the link present and ignored, the same path was already refused by `linkComponent` (`touches link "group"`); with the link indexed, the first guard already refused (`declared entry "group" is indexed`).

What changed: `CheckSourceMutation` already loaded the declared locations with `declaredLinkLocations(dir, follow, true)`, which includes missing entries, but only used them for the indexed check. It now also compares every incoming path against them: a path equal to a declared entry or below it (component match, so `group-other/` and `groupx/` are unaffected) is refused with `refusing incoming commit <commit>: path "<path>" is inside declared entry "<entry>"; remove it from the remote, or remove the entry from .skillfollow or .skillfollow.local`. Declared paths are relative to the canonical `dir`, while diff paths are relative to the Git toplevel, so the check prefixes the entry with `dir`'s path below the toplevel (empty when `dir` is the root). The existing `linkComponent` check still runs first for every path, so a live link keeps its current wording, and undeclared links are still refused as before.

Callers covered: every Git operation that moves the source work tree to a remote revision goes through `CheckSourceMutation`, so the one change covers all of them:

- `PullWithEnv` / `PullWithResolution` (`skillshare pull` with an upstream) via `sourcePullRevision`
- `FirstPull` (`skillshare pull` without an upstream, via `integrateRemote`)
- `Checkout` (local and remote-tracking branch)
- `resetToRemoteBranch` and `pullRemote` (`cmd/skillshare/init_remote.go`, `init --remote`)
- `tryPullAfterRemoteSetup` (`cmd/skillshare/init.go`)

Coverage:

- `TestSourceMutationRefusesPathsBelowMissingDeclaration` (`internal/git/source_mutation_test.go`): `missing` (refused, naming the entry; HEAD unchanged and no `group` created), `nested-source` (the source is `skills/` below the Git root and the incoming path is `skills/group/a/SKILL.md`; refused), `live-ignored` (link present; the existing `touches link "group"` refusal still wins and the link is kept), and `sibling` (`group-other/a/SKILL.md` and `groupx/a/SKILL.md` are accepted).
- `TestSkillfollowPullRefusesPathBelowMissingEntry` (`tests/integration/skillfollow_git_test.go`): `skillshare pull` exits non-zero, the output names `inside declared entry "group"`, and `source/group` does not exist afterwards.

## Failures before the fix

Both tests were written and run in the container before the fix:

```text
--- FAIL: TestSourceMutationRefusesPathsBelowMissingDeclaration/missing
    source_mutation_test.go:185: err = <nil>, want "is inside declared entry \"group\""
--- FAIL: TestSourceMutationRefusesPathsBelowMissingDeclaration/nested-source
    source_mutation_test.go:185: err = <nil>, want "is inside declared entry \"group\""
--- PASS: TestSourceMutationRefusesPathsBelowMissingDeclaration/live-ignored
--- PASS: TestSourceMutationRefusesPathsBelowMissingDeclaration/sibling
--- FAIL: TestSkillfollowPullRefusesPathBelowMissingEntry
    skillfollow_git_test.go:189: expected failure, but command succeeded
        stdout: ✓ Pull      1 commit, 1 file changed (+1 −0)
    skillfollow_git_test.go:190: expected output or error to contain "inside declared entry \"group\""
    skillfollow_git_test.go:192: pull created the declared entry: <nil>
```

`live-ignored` and `sibling` passed before the fix by design: they pin the existing refusal and the component-level match.

## User-visible behavior changes (for the docs update)

These apply only when `.skillfollow` or `.skillfollow.local` declares the entry. With no declaration, output is unchanged.

1. `skillshare pull` (with or without an upstream), `init --remote` and the first pull after remote setup, and source branch checkouts refuse an incoming commit that adds, changes or deletes a path inside a declared entry whose link is currently missing. The error names the commit, the path and the entry, and suggests removing the path from the remote or the entry from `.skillfollow` / `.skillfollow.local`. The work tree and HEAD are unchanged.
2. A live declared link keeps its existing refusal (`touches link`), and an indexed declared entry keeps its existing refusal (`is indexed`).

## Evidence

All Go commands ran in a throwaway container from the devcontainer image (compose project `skillshare_wt_round17`, this worktree at `/workspace`). Nothing ran on the host. `make check` inside the container exited 0. Tail:

```text
=== RUN   TestXDG_StatusWorksWithXDGPath
--- PASS: TestXDG_StatusWorksWithXDGPath (0.03s)
PASS
ok  	skillshare/tests/integration	60.986s

✓ All tests passed!
```

## Notes and limits

- With an unreadable declaration, `declaredLinkLocations` returns the read error and `CheckSourceMutation` already refuses every incoming revision. Unchanged.
- No website docs, README or built-in skill edits.
