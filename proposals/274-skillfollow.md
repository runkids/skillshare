# Feature Proposal: Opt-in `.skillfollow` for declared first-level links in the skills source

Issue: [#274](https://github.com/runkids/skillshare/issues/274). Related: [#314](https://github.com/runkids/skillshare/issues/314) (fixed by #315), [#253](https://github.com/runkids/skillshare/issues/253), [#206](https://github.com/runkids/skillshare/issues/206) (closed by #334).

Credits: the `.skillfollow` design, the call-site list, and the reference implementation come from @hhdebb in #274. The Windows junction analysis and the provenance suggestion come from @star-nebula's comment on #274 and from #314/#315.

All file and line references below were checked against `runkids/proposal-274-skillfollow` at `475b1769`. Line numbers will drift. The function names are the stable reference.

## Problem

A user keeps a shared team skills repo with their other projects (`~/code/work/dev/dev-skills`). They want skillshare to treat it as a tracked repo: discover its skills, sync them, and `update` it with `git pull`. Today the only layout that works puts the real repo inside the source and a link back outside it:

```
~/.config/skillshare/skills/_dev-skills/   <- real repo
~/code/work/dev/dev-skills                 <- symlink back to it
```

Editors, agent CLIs, and session managers canonicalize the working directory. They all show the `~/.config/...` path, so the repo has two names, and the name the user types is never the one that is displayed. The user wants the reverse layout:

```
~/code/work/dev/dev-skills                 <- real repo
~/.config/skillshare/skills/_dev-skills    <- link to it
```

In that layout the repo is invisible. Discovery runs `filepath.Walk` in `discoverSourceSkillsInternal` (`internal/sync/discover_walk.go:162`). Walk uses `Lstat`, so a linked child reports `IsDir() == false`. It fails every `info.IsDir()` branch (`:168`, `:176`, `:187`, `:216`) and the `SKILL.md` branch (`:234`), and drops out of the walk without a skip notice. Only the source root is resolved first (`utils.ResolveSymlink`, `:141`).

The same applies to Windows, with one extra detail from @star-nebula. Without Developer Mode, the only directory link a user can create is a junction (`mklink /J`). `createLink` in `internal/sync/symlink_windows.go` falls back to one for exactly that reason. Since Go 1.23 a junction reports `ModeIrregular`, not `ModeSymlink`, and not a directory. `utils.IsLinkMode` (`internal/utils/link.go`) exists to handle this. star-nebula measured it on 0.22.0: two skills behind a junction in the source were not discovered.

This is a deliberate boundary, not a bug. The source root may be a link because dotfiles managers need it (`website/docs/reference/commands/sync.md#dotfiles-manager-compatibility`), but links inside the source are not followed. The maintainer confirmed this in #207: following them would affect discovery, list, doctor, backup, the audit gate, and cross-machine portability. This proposal defines an explicit opt-in that addresses each of those.

## Proposed Solution

### 1. The declaration files

Add a file at the skills source root, symmetric with `.skillignore`:

```
# ~/.config/skillshare/skills/.skillfollow
# first-level links (symlinks or junctions) that discovery may follow
_dev-skills
```

The rules below are @hhdebb's design, plus the target-overlap rule:

- **Entry names, never target paths.** Each line names a direct child of the source root. The link resolves to a different absolute path on each machine, but the declaration stays the same, so `.skillfollow` can be committed with the source.
- **`.skillfollow.local`** holds machine-local additions. It is merged after `.skillfollow`, the same way `skillignore.ReadMatcher` merges `.skillignore.local` (`internal/skillignore/skillignore.go`). Blank lines and `#` comments are ignored. Glob patterns, negation, and paths containing `/` are rejected with a warning, so the meaning of each entry stays exact.
- **First level only.** Only direct children of the source root are followed. Links inside a followed tree stay unfollowed, the same as links in the source today, so recursion is bounded at one hop.
- **The entry must be a link.** It must satisfy `utils.IsLinkMode(path, lstat.Mode())`, so symlinks and junctions both qualify. A declared name that is a real directory is reported as a no-op, not an error. It is discovered anyway.
- **The target must exist and be a directory.** A missing or dangling target is skipped with a warning. Its skills are not reported as uninstalled, its metadata is not pruned (see §2), and its links in targets are not treated as orphans.
- **Cycles and overlaps are rejected.** The resolved target must not be the source root, an ancestor of it, or a path inside it, compared after `EvalSymlinks` on both sides. It must also not be inside any configured skills target path. Without the target rule, following `~/.agents/skills`, which is the built-in universal target (`internal/config/targets.yaml`), would make sync write links into the directory it is reading from. Rejected entries are skipped with a warning naming the rule.
- **Same name rules as a real directory.** A followed `_dev-skills` that contains `.git` is a tracked repo, through `utils.IsTrackedRepoDir` plus the `.git` check. A followed entry without `_` is a group or a single skill. Repo-level `.skillignore` inside a followed repo applies as it does today.
- **Logical paths everywhere.** Skills are reported as `<source>/_dev-skills/<skill>` with `RelPath` `_dev-skills/<skill>` and `FlatName` `_dev-skills__<skill>`. This matches the existing `SourcePath: filepath.Join(sourcePath, relPath)` convention at `discover_walk.go:305`, so target link names and the manifest stay stable.
- **Default unchanged.** With no `.skillfollow`, an undeclared link stays invisible exactly as today. The existing `TestUninstallGroup_ExternalSymlinkRejected` and `TestUpdateGroup_ExternalSymlinkRejected` (`tests/integration/sync_symlinked_dir_test.go:207`, `:239`) must keep passing unchanged.

Project mode reads the same files from the project skills source (`.skillshare/skills/` or `sources.skills`). This proposal does not add `.agentfollow` or extras support. See Open Questions.

### 2. Future-proofing: one source walker and a guard test

*Maintainer requirement (a).*

The reference implementation swaps a wrapper into each call site, one line per site. That fixes the sites that exist today, but nothing stops the next new walk from skipping declared entries. I counted the walks of the skills source on this branch. Twelve call sites read the source directly instead of going through `discoverSourceSkillsInternal`. The issue names four of them.

| Call site | Function | Today | Effect with a followed entry if not migrated |
|---|---|---|---|
| `internal/install/install_queries.go:223` | `getTrackedReposImpl` (backs `install.GetTrackedRepos`, 12 callers) | Walk, `info.IsDir()` | Followed repo is missing from status, check, `update <name>`, server update/check/overview |
| `internal/config/reconcile_core.go:26` | `reconcileSkillsWalk` | WalkDir, `!d.IsDir()` returns | **Data loss:** `pruneStaleEntries` (`:132`) deletes metadata for every followed skill |
| `cmd/skillshare/update.go:217` | `cmdUpdate` (`--all`) | Walk | Followed repo is not updated |
| `cmd/skillshare/update_project.go:179` | `updateAllProjectSkills` | Walk (root already resolved at `:38`) | Same, in project mode |
| `internal/server/handler_update.go:552` | `getServerUpdatableSkills` | WalkDir | Dashboard "update all" skips it |
| `cmd/skillshare/uninstall.go:280` | `resolveNestedSkillDir` | Walk | `uninstall <skill>` cannot find a followed skill |
| `cmd/skillshare/uninstall.go:184` | `resolveUninstallByGlob` | ReadDir, `e.IsDir()` | Glob uninstall skips followed entries |
| `cmd/skillshare/audit.go:414` | `collectInstalledSkillPaths` | ReadDir, `e.IsDir()` | `audit` misses followed skills |
| `cmd/skillshare/doctor.go:328`, `:808` | `checkSource`, `checkSkillsValidity` | ReadDir, `e.IsDir()` | Counts and validity checks skip them |
| `internal/server/handler_overview.go:49` | `handleOverview` | ReadDir, `e.IsDir()` | Overview count is wrong |
| `internal/install/metadata_migrate.go:136`, `:163` | `migrateSkillSidecars`, `walkSkillDir` | ReadDir, `de.IsDir()` | Legacy sidecars in a followed tree are not migrated |
| `internal/git/scope.go:399` | `NestedRepos` | WalkDir, `!d.IsDir()` | See §4 |

Corrections to the issue's list: `FindLocalSkills` (`internal/sync/pull.go:43`) walks a *target*, not the source, so it needs no change. `update_project.go` now resolves the source root (`:38`), which matches the maintainer's note that `update --all` with a symlinked root already works.

There is a second class of walk the issue does not mention. These walk a *single skill or group directory* by its logical path. A Walk only fails when the walked root is itself a link, because `Lstat` sees the link. Link components in the middle of a path are resolved by the OS. So they fail only when a followed entry is itself a skill, which is the #206 shape (`surge -> /Applications/...`). That case affects:

- `internal/install/meta.go:93` `ComputeFileHashes`, which also feeds `internal/check/local_check.go`
- `internal/audit/audit_scan_skill.go:121` `scanSkillImpl`
- `internal/audit/content_integrity.go:124`
- `internal/server/handler_skills.go:221` `handleGetSkill`
- `cmd/skillshare/list_tui_content.go:59`
- `cmd/skillshare/diff_file.go:92`, `cmd/skillshare/diff.go:175`
- `cmd/skillshare/init_apply.go:311`
- `internal/sync/copy.go:372` `DirMaxMtimeWithIgnore`

`collectChecksumEntries` and `copyDirectoryWithState` already resolve their root with `EvalSymlinks`, so they are safe. The guarded group walks are `update_resolve.go:106` `resolveGroupUpdatable` and `uninstall.go:231` `resolveGroupSkills`. Both reject a group that resolves outside the source (`update_resolve.go:98-99`, `uninstall.go:226-227`). A declared entry must be exempt. An undeclared one stays rejected.

**Proposed mechanism:**

1. **One package owns source traversal.** `internal/sourcewalk` (name open) exposes:
   - `Follow(root) (*FollowSet, []Warning)`: parses the two files and validates each entry against the rules in §1.
   - `Walk(root, fn)` and `WalkDir(root, fn)`: walk `ResolveSymlink(root)`. At depth 1, a declared entry that passes `IsLinkMode` is reported to `fn` as a directory at its logical path. The wrapper then reads the resolved directory itself. As @star-nebula noted, `filepath.Walk` will not descend through either link kind, so the one-hop recursion guard has to live in the wrapper, not in a `filepath.Walk` callback.
   - `ReadDir(root)`: returns first-level entries with followed links reported as directories.
   - `WalkSkill(dir, fn)`: resolves `dir` before walking, which covers the single-skill class above.
   - `Owns(root, resolvedPath) bool`: used by the group guards and by link ownership (§3).

   `discoverSourceSkillsInternal` and `getTrackedReposImpl` move onto it first, because 40+ callers already funnel through those two. Each raw site in the table becomes a one-line swap.
2. **A guard test that fails on raw walks.** The repository has no golangci-lint, and `make lint` is `go vet ./...` (`Makefile:134-135`). So the guard is an ordinary Go test, for example `internal/sourcewalk/guard_test.go`. It parses every non-test `.go` file under `cmd/` and `internal/` with `go/parser` and finds calls to `filepath.Walk`, `filepath.WalkDir`, `fs.WalkDir`, `os.ReadDir`, and `(*os.File).Readdir`. It fails unless the enclosing `file:function` appears in an allowlist with a one-word reason (`target`, `backup`, `trash`, `clone`, `agents`, `extras`, `skill-resolved`, `sourcewalk`). A new walk must be classified to get past `make check`. That is the "test that catches it" the maintainer asked for. It runs inside `make test`, so CI needs no new tool.
3. **A behavior test.** One integration fixture contains a followed tracked repo, a followed group, and a followed single skill. It runs every public read path against that fixture and asserts that the same skill set appears: `list --json`, `status --json`, `check --json`, `update --all --dry-run`, `audit`, `doctor --json`, `uninstall --dry-run`, `diff`, and the server endpoints for skills, overview, update, and check. When a future command is added, adding it to this table is the second line of defense.

### 3. Link ownership for prune and status

*Maintainer requirement (b).*

**Where it breaks today.** Absolute links are created from the logical `skill.SourcePath` (`createLink` in `internal/sync/symlink_unix.go`). That path starts with the source, so every prefix test passes. Global mode always creates absolute links, because `shouldUseRelative` (`internal/sync/relative.go:16-31`) returns true only when the source and the target both sit under the project root.

Relative links break the prefix test. `createLink` computes them from `evalOrClean(sourcePath)` (`symlink_unix.go`, and the Developer Mode branch of `symlink_windows.go`). That resolves *every* link component, including the followed entry, so the stored target points straight at `~/code/work/dev/dev-skills/<skill>`. Every reader then sees an external path:

- `SyncTargetMergeWithSkills` compares the link target to the logical source path with `utils.PathsEqual(absLink, absSource)` (`internal/sync/sync.go:595-601`). For a relative followed link the compare fails, so the link is removed and recreated on every sync and reported as Updated. Sync never converges.
- `CheckStatusMerge` (`sync.go:960-1029`) counts the link as local, not linked. `doctor`'s `checkSyncDrift` then reports drift.
- `PruneOrphanLinksWithSkills` (`sync.go:736-880`) sends it to the external branch. A live link is kept with the "symlink to external location" warning, so it is never pruned when filters change. A dangling one is removed as a "broken external symlink" with no manifest check.
- `unlinkMergeMode` (`cmd/skillshare/target.go:544`), `unlinkMergeSymlinks` (`internal/server/handler_targets.go:700`), and `computeTargetDiff` (`internal/server/handler_diff_stream.go:222`) all skip it.

Windows junctions are created by `createJunction(absTarget, absSource)` with the logical `absSource`. `mklink /J` stores the path it is given. `utils.ResolveLinkTarget` reads it back with `os.Readlink` and only falls back to `filepath.EvalSymlinks` when Readlink fails. The fallback resolves fully and *would* produce an external path. I could not verify on a Windows host whether Go 1.23+ `os.Readlink` returns the stored logical path for a junction whose target itself crosses a junction. See the test plan and Open Questions.

**Proposed fix, in two parts:**

1. **Create links through the source, not around it.** Relative link creation canonicalizes only the source root and the link's parent directory, and keeps the logical tail. In practice that is `rel(evalOrClean(linkDir), evalOrClean(sourceRoot) + "/" + relPath)`. The OS resolves the followed entry when it opens the path, so the link still works. It also stays under the source, so every existing prefix test and `PathsEqual` compare holds without special cases. `createLink` needs the source root alongside the skill path. `reformatLink` (`relative.go:90`) and `CreateSymlink` (`sync.go:236`) share the change.
2. **Classify ownership by provenance, using a set of source roots.** Links created before the fix, or read back through the `EvalSymlinks` fallback, can still resolve outside the source. Ownership therefore uses an owned-roots set: the logical source, `ResolveSymlink(source)`, and the resolved target of each valid followed entry. This is `sourcewalk.Owns`.

**On @star-nebula's suggestion to use `manifest.Managed` instead of the prefix test.** Part of it has already landed. Since #315 (`5f3e79f1`), the in-source live-link branch of `PruneOrphanLinksWithSkills` requires `manifest.Managed[name]` or `force` (`sync.go:819`), so the #314 misclassification of hand-made links is fixed. The remaining question is whether provenance alone can replace the prefix test. I recommend combining the two, not substituting one for the other:

- **Prune:** remove a live link only if it is in the manifest **and** it resolves into an owned root. The manifest stores names, not link targets (`internal/sync/manifest.go:16-20`). Manifest alone would therefore let prune delete a user's own link that reuses a name skillshare once managed. The owned-roots test keeps #314's guarantee, and the manifest keeps #315's.
- **Broken link that resolves under the source or an owned root:** removed, as today. A link that resolves under a followed root whose target is *missing* is kept, because §1 says a missing followed target is a warning, not an uninstall.
- **Broken external link that is not in the manifest:** unchanged by this proposal. `sync.go:828` still removes it without a manifest check. That is the same class as #314 and is listed as a follow-up below.
- **Status:** `CheckStatusMerge` uses the owned-roots test. Reading the manifest in status is optional, because status only counts and never deletes.

`PruneOrphanAgentLinks` and `pruneExtraOrphans` are out of scope, because `.skillfollow` covers skills only. They already use `prunableLink` (`internal/sync/agent_sync.go:94-117`), which resolves both sides.

**Test cases** (unit tests in `internal/sync`, extending `merge_test.go`, `status_test.go`, `relative_test.go`):

| # | Setup | Expected |
|---|---|---|
| 1 | Global, absolute link to `<src>/_f/a`, `_f` followed | sync: Linked on 2nd run (idempotent); status: linked |
| 2 | Project mode, relative link, `_f` followed | link text goes through `<src>/_f/a`; 2nd sync reports 0 updated; status linked |
| 3 | Pre-fix relative link resolving to `/ext/a`, in manifest | status linked (owned root); prune removes it when `a` is filtered out |
| 4 | Same as 3, not in manifest | kept and counted `local` (#314 rule) |
| 5 | `_f` removed from `.skillfollow`, link in manifest | pruned as orphan |
| 6 | `_f` target temporarily missing | warning; target links kept; metadata kept |
| 7 | Hand-made link to `/ext/a` with `_f` *not* declared | kept with external warning (default unchanged) |
| 8 | Windows junction in source (simulated via `isJunction` var, as `TestIsLinkMode` does) | discovered when declared, invisible when not |
| 9 | Windows junction target link to followed skill | Real Windows only (`skillshare-windows-utm`): junction created, `ResolveLinkTarget` result is owned, 2nd sync idempotent, prune with filter change removes it |
| 10 | Windows with Developer Mode, relative symlink | same as 2 |

### 4. Git operations

*Maintainer requirement (c).*

**The nested-repo gap.** `NestedRepos` (`internal/git/scope.go:396-418`) uses `filepath.WalkDir` and returns early on `!d.IsDir()`, so it never sees a followed repo. That is correct for its purpose: the followed repo is not nested in the git root, and git does not traverse links. The real risk is the link entry itself:

- Tracked repos are ignored through `install.UpdateGitIgnore`, which always writes the pattern with a trailing slash (`internal/install/gitignore.go:26-28`, called from `install_tracked.go:144`).
- A pattern ending in `/` matches only directories, and git stores a symlink as a file (mode 120000).
- `push` and `commit` stage with `git add -A` (`cmd/skillshare/push.go:91`), the server stages with `git.StageAll` (`internal/git/info.go:599-603`), and `init --remote` uses `git add .` (`cmd/skillshare/init.go:573`).

Any of these would commit `_dev-skills` as a link that holds a machine-specific absolute path. The maintainer raised exactly this in #207.

**Proposal:**

- Following an entry writes `/_dev-skills` (no trailing slash, anchored to the source) into the managed `.gitignore` block in the skills source. A no-slash pattern matches the link as well as a directory. Because `UpdateGitIgnore` always appends a slash, it needs a variant or a flag for this.
- `.skillfollow` is committed. `.skillfollow.local` is treated like `.skillignore.local`. Whether skillshare should auto-ignore `.local` files is an open question, since it does not do so for `.skillignore.local` today.
- **Guard at every staging point.** Add a `FollowedEntriesTracked(gitRoot, followSet)` check that runs `git check-ignore` for each declared entry. It runs in `rootScopeSafetySweep` (`cmd/skillshare/gitroot.go:106-121`) and the server `rootScopeGuard` (`internal/server/handler_git.go:563`). `rootScopeSafetySweep` currently runs only when `git_root: root`, so the new check runs for every scope. If an entry is not ignored, `commit` and `push` refuse with the same shape as `errNestedRepos` (`gitroot.go:159`) and print the exact `.gitignore` line to add. A dry run reports it.
- **Root scope (`git_root: root`).** `ScopeDir("root")` is `BaseDir()`. The skills source lives under it, so the source's nested `.gitignore` still applies. A followed external repo can never be committed into the root repo: git does not follow the link, and the guard keeps the link out. If a followed entry resolves to a path *inside* `BaseDir()`, its content is ordinary root-repo content. A `.git` there is already caught by `NestedRepos` when it is reached through the real path. `.skillfollow` should not add a second route to that content, and §1's overlap rule could extend to reject it. See Open Questions.

**Tracked-repo update and pull on a followed repo.** Once `getTrackedReposImpl` uses the walker (§2), the followed repo appears in `status`, `check`, `update <name>`, `update --all`, and the server update and check handlers. `install.IsGitRepo` already uses `os.Stat`, which follows the link. The server's `resolveTrackedRepo` (`internal/server/handler_skills.go:547-563`) already resolves `_dev-skills` directly. So the dashboard can pull a followed repo today, while the CLI `update dev-skills` reports "not found". This inconsistency should be fixed either way.

A followed repo is the user's working copy in their projects directory, not a clone that skillshare owns:

- **Plain `update`** keeps today's behavior: `git.IsDirty` refuses when the tree is dirty, then `PullWithProgress` runs `git pull --no-rebase --ff --no-edit` (`internal/git/info.go:238-254`).
- **`update --force`** runs `git restore .` and then `fetch` plus `reset --hard origin/<branch>` (`cmd/skillshare/update_handlers.go:113`, `info.go:566`, `:1049-1073`). That would destroy uncommitted work in the user's repo. **Proposal:** refuse `--force` for followed repos and report "followed repo: resolve local changes in <resolved path>".
- **Audit gate.** `update` audits the new revision as it does today. When content changes because the user pulled outside skillshare, there is no audit gate, the same as editing a skill in the source. `audit` (§2) and `check` make that visible. The maintainer's #207 concern about bypassing the audit gate applies only when content changes without `update`. This proposal treats that as the user's own edit.

**Other git and trash interactions:**

- **`uninstall _dev-skills`** must not move the link into trash. `trash.MoveToTrash` renames the link (`internal/trash/trash.go:215`). `trash.List` shows only directories (`:237`), so the trashed link would be invisible. **Proposal:** uninstalling a followed entry removes the link and its declaration, never touches the target, and says so. Removing the declaration counts as an "unfollow".
- **`collect --force`**: `PullSkill` (`internal/sync/pull.go:127-143`) runs `os.RemoveAll(dest)` on the link and copies a real directory in its place, which silently replaces the followed entry. **Proposal:** refuse when the destination is a followed entry, matching the group guard.
- **Metadata.** Following does not create a `tracked: true` metadata entry. `GetMissingTrackedRepos` (`install_queries.go:42`) would report an existing entry with that name as missing if the walker were not used, so step 2 is a prerequisite here too.

### 5. Windows

- Detect declared entries with `utils.IsLinkMode(path, info.Mode())`, never `ModeSymlink` alone, and never `info.IsDir()`. A junction is `ModeIrregular` and not a directory (`internal/utils/link.go`).
- The wrapper must read the resolved directory itself with `ReadDir(ResolveSymlink(entry))` or a Walk of the resolved path, then rewrite each reported path back to the logical `<source>/<entry>/...` form before calling `fn`. This is where the one-hop rule is enforced. It is also why `discover_walk.go`'s `filepath.Rel(walkRoot, path)` must use the logical path for followed subtrees, not the resolved one.
- Cycle checks compare `EvalSymlinks` results with `utils.PathsEqual` and `PathHasPrefix`, which are case-insensitive on Windows (`internal/utils/path.go`).
- Real-Windows coverage goes in an `ai_docs/tests/` runbook run with `skillshare-windows-utm`, using both a basic-user token (junction only) and Developer Mode (relative symlink). It covers discovery, sync idempotency, status, prune after a filter change, and uninstall.

### 6. Common base for #253 and #206

- **#206 (live link to an app-bundled skill).** This was closed by #334. `check` now detects drift at local install sources, and `update` re-copies through the audit. The maintainer chose that over `--link` in #207 to keep the audit gate, Windows parity, and backup and git behavior. `.skillfollow` gives a cleaner base if `--link` comes back: `install --link <dir>` would create the link (junction on Windows), add the name to `.skillfollow.local` (machine-specific, because the target path is), and add the anchored `.gitignore` line. Every downstream command then works through §2 without `--link`-specific code. I recommend not reopening it in this proposal. The copy-plus-`check` flow already serves the bundle case.
- **#253 (`include_sources` for `~/.agents/skills`, `~/.cursor/skills`).** A followed entry `_agents -> ~/.agents/skills` would expose those skills as `_agents/<skill>`. `~/.agents/skills` is the universal target in `internal/config/targets.yaml`, though, so §1's target-overlap rule must reject it. Otherwise sync writes links back into the directory it reads. That makes `.skillfollow` a *partial* base for #253: it covers external directories that are not targets (a team repo, another tool's private store) but not the main example in the issue. A real #253 design also has to decide name-collision and read-only semantics, which `.skillfollow` avoids by namespacing everything under the entry name. **Recommendation:** build #253 on the same `sourcewalk` package (an `include_sources` root becomes another owned root), but keep the config surface separate.

### 7. CLI and dashboard surface

- **Phase 1: no new command.** The file is hand-edited like `.skillignore` before `enable`/`disable` existed.
- **Phase 2:** `skillshare follow <name>` / `unfollow <name>` with `-p`/`-g` and `--local`. These write the declaration and the anchored `.gitignore` line together. A follow-up could take `follow <name> --to <dir>` to create the link too (junction on Windows). This mirrors `enable`/`disable` (`cmd/skillshare/enable.go`), which are the CLI writers of `.skillignore`.
- **`status`.** Add a line next to `printSkillignoreLine` (`cmd/skillshare/status_render.go`): `.skillfollow (.local active): 2 entries, 1 skipped`. JSON gets `source.skillfollow` next to `source.skillignore` (`buildSkillignoreJSON`, `cmd/skillshare/status.go:216`).
- **`doctor`.** Add a `checkSkillfollow` after `checkSkillignore` (`cmd/skillshare/doctor.go:282`). It reports each declared entry as followed, missing, not-a-link, cycle, target-overlap, or not-gitignored. It also adds an info line for *undeclared* first-level links in the source, which are invisible today with no notice (star-nebula's point). That is the most useful single addition even without following.
- **`list`.** Followed tracked repos show as tracked. The suffix could add `→ <resolved>`, following #206's suggestion.
- **Dashboard.** `.skillignore` has `GET/PUT /api/skillignore` (`internal/server/server.go:627-633`, `handler_skillignore.go`) and a tab in `ui/src/pages/ConfigPage.tsx`. A `.skillfollow` tab could reuse that pattern in phase 2. It is not required for phase 1. The handlers only edit `.skillignore`, not `.local`, and `.skillfollow` would follow suit. The doctor and status data above reach the dashboard through the existing doctor and overview endpoints.
- **Docs to update (English, then translations through the normal flow):**
  - `website/docs/reference/filtering.md` (new section next to `.skillignore`)
  - `reference/appendix/file-structure.md`
  - `understand/source-and-targets.md` (the symlinked-source tip)
  - `reference/commands/sync.md#dotfiles-manager-compatibility`
  - `reference/commands/{status,doctor,update,uninstall,collect,push,commit,list}.md`
  - `reference/targets/configuration.md#git-root`
  - `how-to/sharing/cross-machine-sync.md`
  - `troubleshooting/faq.md` (dotfiles question)
  - the built-in skill `skills/skillshare/references/`
  - `reference/commands/backup.md` only to confirm that backup still never follows source links. Backup copies only target content, and `copyDir` in `internal/backup` skips links.

## Alternatives Considered

- **Follow every link in the source.** This breaks the deliberate boundary and changes behavior for every existing user with a stray link. It would also have repeated #314's problem class on the discovery side. Rejected. Opt-in keeps the default unchanged.
- **Target paths in config (`include_sources`, #253).** Absolute paths differ per machine, so the config cannot be shared. Skills would also have no logical location in the source, which every link and manifest path assumes. A good fit for #253, not for this case.
- **Keep the repo inside the source with a link outside it.** This is what works today, and the issue explains why it fails the user.
- **Wrapper swaps without a guard test.** This is the reference implementation's shape. It is the smallest diff, but it fails the maintainer's future-proofing requirement.
- **`manifest.Managed` as the only ownership test.** See §3. Names alone cannot tell skillshare's link from the user's link of the same name.
- **Commit the link instead of ignoring it.** Absolute targets break on other machines, which the maintainer raised in #207. Relative link targets outside the repo are no better.

## Scope

- [ ] Small (1-3 files, < 200 lines)
- [ ] Medium (3-10 files, 200-500 lines)
- [x] Large (10+ files, 500+ lines)

Rough split. The reference implementation was about 26 insertions and 17 deletions across 11 upstream files, plus a new package. This proposal adds the guard test, the link-creation change, git guards, doctor/status, and more call sites.

- Phase 1 core: new package and guard test (~400 lines with tests); about 20 call-site swaps; link creation and ownership (~150); git ignore and guard (~120); integration fixture and tests (~400).
- Phase 2: CLI `follow`/`unfollow`, dashboard tab, docs.

## Test Plan

- **Unit tests:**
  - parser: comments, `.local` merge, rejected globs and slashes
  - validation: missing, not-a-link, cycle to the root, ancestor, inside the source, inside a target
  - walker: one hop only, logical paths, junction simulation through the `isJunction` var
  - the guard test (§2)
  - the ownership table (§3)
- **Integration** (`tests/integration/`, devcontainer): the all-commands fixture from §2.3. It also covers:
  - default-unchanged regression: no `.skillfollow` gives identical output
  - `update --force` refused on a followed repo
  - `uninstall` of a followed entry leaves the target intact
  - `collect --force` refused
  - `commit`/`push` refused when the link is not ignored, in both `git_root: skills` and `git_root: root`
  - reconcile keeps metadata for followed skills
- **Windows:** `skillshare-windows-utm` runbook for cases 8-10 of §3 and §5.
- `make check` in the devcontainer.

## Phased Rollout

1. **Visibility first, no behavior change:** a `doctor` info line for undeclared first-level links in the source. It can ship on its own.
2. **Walker and guard test.** Move the existing walks onto `sourcewalk` with follow disabled. That is a pure refactor and existing tests must pass unchanged. Add the guard test.
3. **Enable `.skillfollow`:** parsing, the git ignore and guard, link creation and ownership, update `--force` refusal, uninstall and collect rules, and status and doctor output. Docs ship in the same release, marked experimental.
4. **CLI `follow`/`unfollow`, then the dashboard tab.** Revisit `install --link` (#206) and `include_sources` (#253) on the same base if they are still wanted.

## Open Questions

1. **Windows junction read-back.** Does `os.Readlink` in Go 1.23+ return the stored logical path for a junction created with `mklink /J <target> <source>\_f\skill`, where `_f` is itself a junction? If it falls back to `EvalSymlinks`, ownership relies on the owned-roots set (§3 part 2). This needs a real Windows run.
2. **`update --force` on a followed repo.** Should it refuse outright, as proposed, or require an extra confirmation flag?
3. **Auto-ignoring `.local` files.** Should skillshare add `.skillfollow.local` to the managed `.gitignore` block? It does not do this for `.skillignore.local` today.
4. **Followed targets inside the git root.** Should a followed entry whose target lies inside the git root (for example under `BaseDir()` at root scope) be rejected, or only warned?
5. **Agents and extras.** Is `.agentfollow` wanted? The agent and extras walkers (`internal/resource/agent.go`, `internal/sync/extras.go:49`) have the same blind spot.
6. **Followed single skills.** Should a followed entry that is itself a skill (no `_` prefix, `SKILL.md` at its root) be allowed in phase 1? It is the #206 shape and needs every `WalkSkill` site migrated.
7. **Out-of-scope follow-up.** Should the broken-external-link branch of `PruneOrphanLinksWithSkills` (`sync.go:828`), which removes without a manifest check, get the #314 provenance rule? It is independent of this proposal.
8. **Package name and API.** Is `internal/sourcewalk` acceptable, or should this live in `internal/sync` next to `discoverSourceSkillsInternal`, which most callers already reach?
