# PR 390 Codex round 20 fix

Reviewed commit `c4f5d8a8`. Of the three P2 findings, two held and one did not. Finding 1, the project update-all source mismatch, does not reproduce on the path the dashboard actually runs. A test now proves the pairing, and the handlers are unchanged. Finding 2, the built-in skill fallback under an unreadable declaration, reproduced exactly as described. Finding 3, a declared regular file missing from the mutation guard, holds in the guard logic. On Linux and macOS the transition was already refused, but only by accident, through an `ENOTDIR` inspect error that gave no guidance. On Windows nothing refused it. The work is four commits on `runkids/codex-round20`. Nothing was pushed and no GitHub thread was changed.

| Commit | Scope |
|---|---|
| `3a20fa5b` | P2 (1): proving test only; handlers unchanged |
| `542c6d83` | P2 (2): `init` consults the declaration before the built-in skill download and fallback |
| `6918d791` | P2 (3): the mutation guard keeps declared non-directory non-links |

## Replies ready for the coordinator

### P2: Pair project follow snapshots with the project source

Not reproduced; no handler change. A proving test was added in `3a20fa5b`.

In a project dashboard, `s.cfg` is not the global config. `cmd/skillshare/ui.go` (project branch) builds a synthetic `config.Config{Source: rt.sourcePath, Targets: rt.targets, Mode: "merge"}`, where `rt.sourcePath` is the project skills source, and passes it to `server.NewProject`. `Sources.Skills` is never set on it. Every `/api/` request goes through `withConfigAutoReload` and then `reloadConfig`, and in project mode that sets `s.cfg.Source = pcfg.EffectiveSkillsSource(projectRoot)`. `Config.EffectiveSkillsSource()` returns `Sources.Skills` first and then `Source`. So in project mode `s.cfg.EffectiveSkillsSource()` returns the same path as `s.skillsSource()`, which is the source `skillFollowSet()` is bound to. `updateAll`, the SSE update-all collect (`handler_update_stream.go:34-35`) and the SSE per-item call (`:127`) therefore pair the project source with the project snapshot.

The only way to get the mismatch is to hand `NewProject` a config whose `Sources.Skills` names another tree. Some test fixtures do this (for example `TestServerSkillfollowProjectSourceDiffersFromGlobal`). No production caller does.

Coverage: `TestServerProjectUpdateAllUsesProjectSource` (`internal/server/handler_update_follow_test.go`) builds the synthetic config the way `ui.go` does. It places a project root whose `.skillshare/skills` differs from the global source, and makes the global source's `.skillfollow` a directory so that any read of the global tree would fail. It drives `POST /api/update {all:true}` and `GET /api/update/stream` through the server handler, including the auto-reload:

- `all/followed`, `sse/followed`: the followed project entry `_dev` is updated through the project snapshot, and its external HEAD moves to the remote commit.
- `all/unreadable`, `sse/unreadable`: with the project `.skillfollow` unreadable, the request is refused with `read skillfollow declaration <project source>/.skillfollow`, the response never names the global source, and the external HEAD does not change.

Sensitivity check: changing only the test's config to `Sources.Skills: <global>` made `all/followed` and `sse/followed` fail (`{"results":null}`: the global tree was walked with the project snapshot). This shows the test detects the mismatch the finding describes. The `unreadable` cases cannot tell the two apart, because the error comes from the project snapshot either way.

Inventory: no handler pairs a global-only path with `skillFollowSet()` in project mode, because `s.cfg.EffectiveSkillsSource()` is the project source on every request there. The other `s.cfg.EffectiveSkillsSource()` uses were left as they are for the same reason.

### P2: Abort the built-in fallback on declaration read errors

Fixed in `542c6d83`. Confirmed as described. With `.skillfollow` unreadable (a directory), `installBuiltinSkill` returned `fallback=true, err=nil` and `init --skill` created `skillshare/SKILL.md`. `install.Install` returned the declaration read error, which is not `sourcefs.ErrLink`, so the fallback ran.

What changed: `installBuiltinSkill` now builds `sourcewalk.Follow(sourcePath, FollowOptions{})` and calls `WriteBoundary(sourcePath, "skillshare")` after the existing link check and before either install path. Any refusal is returned without downloading. That covers the read error and a link refusal for a declared `skillshare`, live or offline. `init` reports it as the existing `Failed to install the skillshare skill: …` warning. Any other download failure still writes the fallback.

Coverage:

- `TestInstallBuiltinSkill_RefusesUnreadableDeclaration` (`cmd/skillshare`): returns `read skillfollow declaration …`, and no `skillshare/` is created. Before the fix it failed with `installBuiltinSkill = <nil>`.
- `TestSkillfollowInitRefusesDeclaredEntry/builtin-skill-unreadable-declaration` (integration, downloads pointed at a closed proxy port): the output names `read skillfollow declaration <source>/.skillfollow`, and no `skillshare/` is created. Before the fix it failed on both assertions.
- `.../builtin-skill-offline-fallback`: with no declaration and a failed download, `skillshare/SKILL.md` holds the fallback content. This passed before and after the fix and pins the fallback.
- `.../builtin-skill` (existing): a declared offline `skillshare` is still refused with the link error.

### P2: Keep invalid declared entries in the mutation guard

Fixed in `6918d791`. The guard gap is confirmed: `declaredLinkLocations` skipped every existing non-link even with `includeMissing`, so a declared entry that exists as a regular file gave `CheckSourceMutation` no declared prefix. What happened next depended on the platform:

- Linux and macOS (run): the incoming `group/a/SKILL.md` was refused anyway, because `linkComponent` hit `lstat group/a: not a directory`, and that error fails closed. The refusal was incidental and offered no guidance: `git pull failed: inspect incoming path "group/a/SKILL.md": lstat …/group/a: not a directory`. HEAD and the file were unchanged.
- Windows (inferred from Go's `syscall.Errno.Is`, not run on Windows): the same lstat returns `ERROR_PATH_NOT_FOUND`, which `os.IsNotExist` accepts. `linkComponent` therefore returns no link and no error, and the transition is accepted as Codex describes.

What changed:

- `internal/git/followed_links.go`: with `includeMissing` (only `CheckSourceMutation` passes it), a declared entry that exists as a non-directory non-link stays in the list. An existing real directory (`not-link`) stays exempt. `FollowedLinksStaged` (`includeMissing=false`) is unchanged.
- `internal/git/source_mutation.go`: the diff loop checks the declared prefixes before it returns a `linkComponent` inspect error. A path below a declared regular file now gets the declared-entry refusal with its guidance on every platform. A live link still gets the `touches link` refusal first.
- Indexed check, decided: a tracked regular file at a declared path is now refused by the first guard (`declared entry "group" is indexed; run git rm --cached …`). An indexed missing entry was already refused the same way. A declared entry must be untracked and ignored, and Follow already reports this file as `InvalidTarget`.

Coverage:

- `TestSourceMutationRefusesPathsBelowMissingDeclaration` (`internal/git`) gains three cases:
  - `regular-file`: refused with `is inside declared entry "group"`; HEAD and file unchanged.
  - `indexed-file`: refused with `declared entry "group" is indexed`; HEAD and file unchanged.
  - `real-directory`: the same incoming change is accepted, which pins the exemption.

  Before the fix, `regular-file` and `indexed-file` failed on the refusal message (`inspect incoming path … not a directory`). The HEAD and file assertions already held on Linux.
- `TestSkillfollowPullRefusesPathBelowMissingEntry/regular-file` (integration, `skillshare pull`): the output contains `inside declared entry "group"`, and HEAD and the file are unchanged. Before the fix it failed on the message. The `missing` subtest is the existing case.

## User-visible behavior changes (for the docs update)

- `init --skill`: when `.skillfollow` or `.skillfollow.local` cannot be read, the built-in `skillshare` skill is neither downloaded nor written as the offline fallback. `init` warns `Failed to install the skillshare skill: read skillfollow declaration <path>: …`.
- `pull` and other source-repository updates that go through the mutation guard (`pull`, conflict resolution, first pull, checkout, `init --remote`, dashboard git pull): an incoming path at or below a declared entry that exists as a regular file is refused with `path "…" is inside declared entry "<name>"; remove it from the remote, or remove the entry from .skillfollow or .skillfollow.local`. Previously this was an `inspect incoming path … not a directory` error on Linux and macOS, and it was accepted on Windows.
- A tracked regular file at a declared path now blocks every source pull with `declared entry "<name>" is indexed; run git rm --cached …`, the same as an indexed missing entry.
- No change without `.skillfollow`: the new snapshot in `installBuiltinSkill` is nil, and the mutation guard's list is empty.

## Evidence

All commands ran in a throwaway container (`skillshare_wt_codexr20`) from the devcontainer image, with this worktree at `/workspace`.

```text
$ go test ./internal/server -run TestServerProjectUpdateAllUsesProjectSource -count=1 -v
--- PASS: TestServerProjectUpdateAllUsesProjectSource (0.19s)
    --- PASS: .../all/followed  --- PASS: .../all/unreadable
    --- PASS: .../sse/followed  --- PASS: .../sse/unreadable

$ go test ./internal/git ./internal/sourcewalk -count=1
ok  skillshare/internal/git
ok  skillshare/internal/sourcewalk

$ make check   (tail)
ok  	skillshare/tests/integration	56.202s
✓ All tests passed!
```

## Failures before the fix

```text
init_test.go:136: installBuiltinSkill = <nil>, want the declaration read error
skillfollow_sweep_test.go:85: expected output to contain "read skillfollow declaration …/skills/.skillfollow"
skillfollow_sweep_test.go:87: init created skillshare under an unreadable declaration: <nil>
source_mutation_test.go:204: err = inspect incoming path "group/a/SKILL.md": lstat …/source/group/a: not a directory, want "is inside declared entry \"group\""
source_mutation_test.go:204: err = inspect incoming path "group/a/SKILL.md": lstat …/source/group/a: not a directory, want "declared entry \"group\" is indexed"
skillfollow_git_test.go:201: expected output or error to contain "inside declared entry \"group\"", got: … not a directory
```

For finding 3, the pre-fix run swapped only the two production files back to `HEAD` and rebuilt.

## Notes and limits

- Finding 1 stays correct only while project mode builds `s.cfg` without `Sources.Skills`. If a future change ever passes a loaded global config to `NewProject`, every `s.cfg.EffectiveSkillsSource()` use in project-mode handlers would need `s.skillsSource()`. That swap was not made here because no production path needs it and the brief asked for no handler change when the finding does not reproduce.
- The Windows half of finding 3 is inferred from Go's errno mapping and was not run on a Windows guest. `ai_docs/tests/windows_skillfollow_runbook.md` does not cover a regular file at a declared path.
- Ratchets `TestRawWalkGuard`, `TestRawWriteRatchet` and `TestFollowWriteGuard` pass within `make check`.
