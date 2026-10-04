# #274 step 4 (CLI half): `follow` and `unfollow`

Branch `runkids/follow-cmd`. Proposal: `proposals/274-skillfollow.md` §1, §4 "Allowed policy", §5 "Who writes it", §6, §8 "Step 4". The dashboard `.skillfollow` tab and followed single skills are not part of this slice.

## Command grammar

```text
skillshare follow   <name> [--to <dir>] [--local] [-g|-p] [--json]
skillshare unfollow <name> [--local] [--keep-link] [-g|-p] [--json]
```

- `<name>` must pass `sourcewalk.ValidEntryName` (exported wrapper of `validEntryName`). One name per call. A `_` prefix with `.git` follows as a tracked repository; anything else as a group. Path, glob, and absolute names are refused before any write, which also covers "refuse inside a followed tree": a name can only be a first-level entry.
- `-g`/`-p` and auto-detection go through `parseModeArgs` and `resolveAutoMode`, like `enable`. Global mode uses `globalSkillFollowSet(cfg)`; project mode uses `skillFollowSet(runtime.sourcePath, runtime.targets, cwd)` from `loadProjectRuntime`, the same as `status_project.go`. No discovery call site was touched.
- `--to` is accepted only by `follow`; `--keep-link` only by `unfollow`. Other flags are refused as unknown.

### follow

1. With `--to <dir>`: `<dir>` must exist and be a directory; its canonical path must not be the source or inside it. If `<source>/<name>` does not exist, it is created with `sync.CreateSymlink(link, abs(dir), "")`: an absolute symlink on Unix, a junction first on Windows (symlink fallback). If it exists, it must be a link whose canonical target equals the canonical `<dir>`; otherwise the command refuses.
2. Without `--to`: `<source>/<name>` must exist as a link (dangling allowed; it then reports `missing`) or a real directory (reported `not-link`). Anything else is refused.
3. `skillfollow.AddEntry` appends the name to `.skillfollow` (or `.skillfollow.local`), keeping every line, comment, blank line, order, and CRLF style. Already declared: `already in <file>`, exit 0, file untouched. If this write fails after `--to` created the link, the new link is removed again with `sourcefs.Root.Unlink`.
4. The FollowSet is rebuilt. When `git rev-parse --git-dir` succeeds in the source, `git.FollowedLinksStaged(source, set)` finds the entry's staged location, which gives the same ignore file and line text that doctor and commit print. If `git check-ignore` does not already ignore the path, the line goes into the managed block of `<source>/.gitignore`. An indexed link is never untracked; the result carries `git.UntrackCommand(path)`. With `--local`, `/.skillfollow.local` is added the same way unless Git already ignores that file.
5. The entry's state, reason, and resolved target are printed from the rebuilt set. No sync runs; a change prints `Next  skillshare sync  apply the change`.

### unfollow

1. `skillfollow.RemoveEntry` drops every line declaring the name from `.skillfollow` and `.skillfollow.local` (`--local`: only the local file), keeping all other lines. Each edited file is listed.
2. `--local`: if `.skillfollow` still declares the name, the result lists it in `still_declared_in`, prints `still declared in .skillfollow; it remains followed`, and stops: the link and ignore line stay.
3. Not declared anywhere: `not declared; nothing changed`, exit 0, the link is not touched.
4. Otherwise the link is removed with `sourcefs.Root.Unlink` unless `--keep-link`. A real directory is kept (`link kept: not a link; a real directory is never removed`). The target is never touched.
5. The managed-block ignore line is removed only when the link was removed. When the link stays and `.gitignore` contains the line, it is reported as kept.
6. Any failed write returns an error. A failure after an earlier file was edited says so: `failed to update .skillfollow.local: …; .skillfollow was already edited, and _dev is still declared`.
7. The `Next  skillshare sync  prune the entry's managed links` hint prints only when the entry left discovery (declarations edited, none remaining, and it was not a real directory).

Both commands write an oplog entry (`follow`/`unfollow`, args `name`, `local`, optional `to`/`keep_link`) only when something changed. `--json` keeps stdout to one JSON object; errors use `writeJSONError`.

## Output samples

Captured from `bin/skillshare` in the devcontainer (global mode, Git source, `NO_COLOR=1`).

```text
$ skillshare follow _dev --to /tmp/fsmoke/ext
✓ _dev        linked to /tmp/fsmoke/ext
✓ _dev        added to .skillfollow
✓ .gitignore  added /_dev
✓ _dev        followed — following directory

Next
  skillshare sync  apply the change

$ skillshare follow tgt --to /tmp/fsmoke/home/.claude
✓ tgt         linked to /tmp/fsmoke/home/.claude
✓ tgt         added to .skillfollow
✓ .gitignore  added /tgt
! tgt         target-overlap — target overlaps active skills target /tmp/fsmoke/home/.claude/skills

$ skillshare follow realdir
✓ realdir     added to .skillfollow
✓ realdir     not-link — real directory; discovered normally

$ skillshare follow _dev          # link already indexed
✓ _dev        added to .skillfollow
✓ .gitignore  added /_dev
! _dev        indexed in Git; run git rm --cached -- '_dev'
✓ _dev        followed — following directory

$ skillshare follow _dev --to /tmp
✗ /tmp/fsmoke/src/_dev already exists and does not link to /tmp; remove it, or run follow without --to

$ skillshare follow x --to /tmp/fsmoke/src/realdir
✗ --to: /tmp/fsmoke/src/realdir is inside the skills source

$ skillshare follow nope
✗ /tmp/fsmoke/src/nope does not exist; pass --to <dir> to create the link

$ skillshare follow a/b
✗ "a/b" is not a first-level entry name: no path separators, globs, or absolute paths

$ skillshare unfollow _dev
✓ _dev        removed from .skillfollow
✓ _dev        link removed; its target was not touched
✓ .gitignore  removed /_dev

Next
  skillshare sync  prune the entry's managed links

$ skillshare unfollow _dev --keep-link
✓ _dev        removed from .skillfollow
  _dev        link kept: --keep-link
  .gitignore  kept /_dev

$ skillshare unfollow _dev --local
✓ _dev        removed from .skillfollow.local
! _dev        still declared in .skillfollow; it remains followed

$ skillshare unfollow realdir
✓ realdir     removed from .skillfollow
  realdir     link kept: not a link; a real directory is never removed

$ skillshare unfollow zzz
! zzz         not declared; nothing changed
```

JSON (`follow _dev --json`, already declared; `unfollow _dev --json` after `follow --local`):

```json
{
  "name": "_dev",
  "source": "/…/skills",
  "file": ".skillfollow",
  "added": false,
  "link": "/…/skills/_dev",
  "link_created": false,
  "ignore_file": "/…/skills/.gitignore",
  "ignore_lines_added": [],
  "state": "followed",
  "reason": "following directory",
  "resolved_target": "/tmp/fsmoke/ext"
}
```

```json
{
  "name": "_dev",
  "source": "/…/skills",
  "files_edited": [".skillfollow.local"],
  "still_declared_in": [],
  "not_declared": false,
  "link": "/…/skills/_dev",
  "link_removed": true,
  "ignore_line": "/_dev",
  "ignore_line_removed": true,
  "ignore_line_kept": false
}
```

Optional fields: `link_target` (follow with `--to`), `untrack_command` (indexed link), `link_kept` (unfollow reason).

## Code

| File | Role |
|---|---|
| `cmd/skillshare/follow.go` | Flag parsing, mode/source resolution (`followContext`), dispatch, oplog, JSON vs rows, help |
| `cmd/skillshare/follow_handlers.go` | `runFollow`, `prepareFollowLink`, `followIgnores`, `runUnfollow`, result types, row rendering |
| `internal/skillfollow/declare.go` | `Declares`, `AddEntry`, `RemoveEntry`, `AddIgnoreLine`, `RemoveIgnoreLine`; every write is `sourcefs.Root.WriteFileAtomic` |
| `internal/install/gitignore.go` | Pure `AddGitIgnoreLines`/`RemoveGitIgnoreLines`; the existing file writers now call the same extracted block helpers (behavior unchanged, `internal/install` tests pass) |
| `internal/sourcewalk/parse.go` | Exported `ValidEntryName` |
| `cmd/skillshare/main.go` | Command map and help listing (Skills and agents group) |
| `cmd/skillshare/completion_{bash,zsh,fish,powershell,nushell}.go` | Both commands and their flags |
| `internal/sourcefs/testdata/raw_writes.tsv` | Notes on the `sync.CreateSymlink`/`createLinkAs` rows (see deviations) |

## Tests

Unit, `internal/skillfollow/declare_test.go`:

| Test | Covers |
|---|---|
| `TestAddEntry_PreservesCommentsAndOrder` | Comments, blank lines, order kept; missing trailing newline handled |
| `TestAddEntry_CreatesMissingFile` | First write creates `.skillfollow.local` |
| `TestAddEntry_Idempotent` | Whitespace-padded existing line counts; file byte-identical |
| `TestAddEntry_KeepsCRLF` | Appends with the file's CRLF style |
| `TestRemoveEntry_KeepsOtherLines` | Removes every duplicate line, keeps comments/blank lines |
| `TestRemoveEntry_NotDeclared` | A commented name is not a declaration; missing file is a no-op |
| `TestDeclares` | Exact-name match, comments ignored |
| `TestAddEntry_FailedWriteLeavesFileUntouched` | Linked `.skillfollow` is refused with `sourcefs.ErrLink`; link target unchanged; no temp file left |
| `TestIgnoreLine_AddAndRemove` | Managed block added once; removal leaves the user's own line outside the block |

Integration, `tests/integration/skillfollow_follow_cmd_test.go` (global and project unless noted; each source is a Git work tree with one active target):

| Test | Covers |
|---|---|
| `TestFollow_ToCreatesLinkDeclaresAndIgnores` | `--to` link text, declaration appended after comments, ignore line, `git check-ignore` passes, `followed` row, sync hint |
| `TestFollow_ExistingLinkIsIdempotent` | Without `--to`; second run says `already in .skillfollow`, no hint, files unchanged |
| `TestFollow_LocalWritesLocalFileAndIgnore` | `--local` writes only `.skillfollow.local`, adds `/.skillfollow.local`, Git ignores it |
| `TestFollow_PrintsRejectedState` | `target-overlap` row and JSON `state`/`reason`/`added` |
| `TestFollow_IndexedLinkIsNotUntracked` (global) | Prints the `git rm --cached` command; the link stays indexed |
| `TestFollow_Refusals` (global) | Path and glob names, missing entry, `--to` conflict, `--to` inside source, missing `--to` dir, `unfollow ../x`; no declaration written |
| `TestUnfollow_RemovesLinkAndEveryDeclaration` | Both files edited and listed, comments kept, link gone, target intact, ignore line removed, prune hint |
| `TestUnfollow_KeepLink` | Link and ignore line kept and reported |
| `TestUnfollow_LocalReportsStillFollowed` | Only `.skillfollow.local` edited; still-followed row; link kept; no prune hint |
| `TestUnfollow_RealDirectoryIsKept` | `follow` reports `not-link`; `unfollow` keeps the directory |
| `TestUnfollow_JSON` | `files_edited`, `still_declared_in`, `link_removed`, `ignore_line_removed`; JSON error object on a bad name |
| `TestUnfollow_PartialWriteFailureReportsFailure` (global) | Linked `.skillfollow.local` fails the second write: non-zero exit, message names the already-edited file, link not removed, write not followed through the link |

`tests/integration/completion_test.go`: `TestCompletion_FollowCommands_AllShells` checks literal content in all five scripts.

E2E: `ai_docs/tests/skillfollow_follow_cmd_runbook.md` (7 steps: follow `--to` JSON, files and `git check-ignore`, sync links `_ext__a`, unfollow JSON, target pruned and external tree intact, partial `--local`, cleanup). Result in a fresh ssenv: `{"total":7,"passed":7,"failed":0,"skipped":0}`.

## Verification

- `make check` in a throwaway devcontainer-image container (worktree at `/workspace`), after all code changes:

  ```text
  test -z "$(gofmt -l ./cmd ./internal ./tests)"
  go vet ./...
  …
  --- PASS: TestXDG_StatusWorksWithXDGPath (0.03s)
  PASS
  ok  	skillshare/tests/integration	143.046s

  ✓ All tests passed!
  exit=0
  ```

- Ratchets: `go test ./internal/sourcefs/ ./internal/sourcewalk/` pass (the sourcewalk allowlist needed no change; `raw_writes.tsv` only gained notes).
- Website: `./node_modules/.bin/docusaurus build` built `en`, `ja`, `ko`, `zh-Hans`, `zh-Hant` (`[SUCCESS] Generated static files` ×5, exit 0). The stray `website/pnpm-workspace.yaml` that `pnpm install` created was deleted.
- `python3 scripts/ai-context.py check`: ok.
- Not run: real Windows. The junction path is the existing `sync.createJunction` used by sync; no Windows runtime verification is claimed. Real-shell zsh/fish completion tests skip in the container as usual.

## Docs touched

- New: `website/docs/reference/commands/follow.md`, `unfollow.md`, and their `ja`, `ko`, `zh-Hans`, `zh-Hant` copies; sidebar entries under Skill Management.
- `website/docs/reference/skillfollow.md` (+4 locales): setup leads with `follow --to`, manual steps kept under "By hand", "Check and sync" subheading, "no follow/unfollow command" and hand-edit wording replaced, `undeclared-link` row and cleanup sentence point at the commands, Limits no longer lists follow/unfollow and notes the unverified Windows junction path.
- `reference/commands/index.md`, `getting-started/quick-reference.md`, `reference/appendix/file-structure.md`, `reference/commands/uninstall.md` (+4 locales each).
- `skills/skillshare/SKILL.md` (description unchanged, 515 characters) and `skills/skillshare/references/skillfollow.md`.
- `wiki/cli-development.md` (no longer calls `unfollow` future), `proposals/274-skillfollow.md` (status line and step-4 rollout note link here).

## Deviations from the proposal and brief

1. **Link creation is not a sourcefs write.** `os.Root` cannot create a Windows junction, and the brief asks to reuse the existing helper, so `follow --to` calls `sync.CreateSymlink` after its own checks (valid first-level name, entry does not exist, target outside the source). The ratchet rows for `CreateSymlink`'s `os.MkdirAll` and `createLinkAs`'s `os.Symlink` keep reason `target` and now carry a note naming this second use, instead of a new tag that would be wrong for their main use.
2. **`.gitignore` edits go through sourcefs, not `install.UpdateGitIgnoreFiles`.** The proposal names `UpdateGitIgnoreFiles`/`RemoveFromGitIgnore`, which write with raw `os.WriteFile` (a `source-unmigrated` row). The new writer reuses their managed-block logic through extracted pure helpers and writes atomically through the source root, so a linked `.gitignore` is refused rather than written through.
3. **Ignore line skipped when Git already ignores the path.** This keeps `follow` idempotent for users who followed the setup docs and wrote `/_dev` outside the managed block. `unfollow` only removes the managed-block line; a user's own line stays.
4. **`--to` refuses only targets inside the source.** An ancestor of the source or an active target is linked and declared, then reported as `cycle`/`target-overlap`, matching the brief's "print the state so a bad declaration is visible immediately". `follow` exits 0 in that case; scripts should read `state` from `--json`.
5. **Undeclared name on `unfollow`** is a no-op with a warning (exit 0); it does not remove an undeclared link. `unfollow --local` on a name declared only in `.skillfollow.local` is a full unfollow (link and ignore line removed unless `--keep-link`).
6. **No `--dry-run`.** Neither the brief nor the proposal asks for it; `enable` has one. Easy to add later.
7. **Sourcefs link errors are unchanged** and still do not advise `unfollow` (step-3 rule kept; the wiki line now says so without "future").

## Rebase notes

Everything is in new files except the command map/help rows in `main.go`, the five completion scripts, `parse.go` (one exported wrapper), the `install/gitignore.go` helper extraction, and the TSV notes. No discovery call site or `firstFollowSet` user changed.
