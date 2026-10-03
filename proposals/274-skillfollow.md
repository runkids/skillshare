# Feature Proposal: Opt-in `.skillfollow` for declared first-level links in the skills source

Issue: [#274](https://github.com/runkids/skillshare/issues/274). Related: [#314](https://github.com/runkids/skillshare/issues/314) (fixed by #315), [#253](https://github.com/runkids/skillshare/issues/253), [#206](https://github.com/runkids/skillshare/issues/206) (closed by #334).

Credits: the `.skillfollow` design, the call-site list, and the reference implementation come from @hhdebb in #274. The Windows junction analysis and the provenance suggestion come from @star-nebula's comment on #274 and from #314/#315.

Source baseline: every file and line reference was checked against `main` at `475b1769`. Line numbers will drift; function names are the stable reference. Nothing here was run: the analysis comes from reading the source, and the Windows claims still need a real Windows host (see Test Plan).

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

Windows has one extra detail, from @star-nebula. Creating a junction (`mklink /J`) needs no symlink privilege. A directory symlink needs Developer Mode or elevation. For that reason `createLink` in `internal/sync/symlink_windows.go` falls back to a junction, and many Windows users can only create junctions. Since Go 1.23 a junction reports `ModeIrregular`, not `ModeSymlink`, and not a directory. `utils.IsLinkMode` (`internal/utils/link.go`) exists to handle this. star-nebula measured it on 0.22.0: two skills behind a junction in the source were not discovered.

This is a deliberate boundary, not a bug. The source root may be a link because dotfiles managers need it (`website/docs/reference/commands/sync.md#dotfiles-manager-compatibility`). Links inside the source are not followed. In #207 the maintainer listed what following them would touch: discovery, list, doctor, backup, the audit gate, and cross-machine portability. This proposal defines an explicit opt-in and answers each of those.

## Proposed Solution

### 1. The declaration files

Add a file at the skills source root, symmetric with `.skillignore`:

```
# ~/.config/skillshare/skills/.skillfollow
# first-level links (symlinks or junctions) that discovery may follow
_dev-skills
```

The rules below are @hhdebb's design, plus the overlap rules:

- **Entry names, never target paths.** Each line names a direct child of the source root. The link resolves to a different absolute path on each machine, but the declaration stays the same, so `.skillfollow` can be committed with the source.
- **`.skillfollow.local`** holds machine-local additions. It is merged after `.skillfollow`, the same way `skillignore.ReadMatcher` merges `.skillignore.local` (`internal/skillignore/skillignore.go`).
- **Strict names.** Blank lines and `#` comments are ignored. Entries are rejected with a warning in any of these cases:
  - the name is empty, `.`, or `..`
  - it contains `/` or `\`
  - it is an absolute path, a volume (`C:`), or a UNC path
  - `filepath.Clean` changes it
  - it uses glob or negation syntax
- **First level only.** Only direct children of the source root are followed. Links inside a followed tree stay unfollowed, the same as links in the source today, so recursion is bounded at one hop.
- **The entry must be a link.** It must satisfy `utils.IsLinkMode(path, lstat.Mode())`, so symlinks and junctions both qualify. A declared name that is a real directory is reported as a no-op, not an error. It is discovered anyway.
- **Phase 1 follows groups and tracked repos only.** If the resolved target has a `SKILL.md` at its root, it is a single skill (the #206 shape), and phase 1 skips it with a warning. With this restriction, every per-skill walk starts at a real directory below the link. The OS resolves the link component in the middle of the path, so the per-skill walks listed in §2 need no change in phase 1. Followed single skills are deferred to phase 4.
- **Cycles and overlaps are rejected.** Paths are compared after canonicalization. A missing path is resolved through its nearest existing ancestor, the way `evalOrClean` does it (`internal/sync/relative.go:41-59`). Comparisons use `utils.PathsEqual` and `PathHasPrefix`, which are case-insensitive on Windows. An entry is rejected when its resolved target:
  - is the source root, an ancestor of it, or a path inside it
  - is equal to an active skills target path (global or project, from the current config), or is an ancestor or descendant of one. The check runs in both directions. Without it, a followed `/ext` with a target at `/ext/out` would make sync write links inside the tree that discovery reads (`SyncTargetMergeWithSkills`, `internal/sync/sync.go:587`, `:625`, `:639`).
  - is equal to, or an ancestor or descendant of, another followed entry's target. *Both* entries are rejected, and the warning names the pair. Merge order of `.local` must not decide which one wins.
- **Per-entry state.** Each declared entry ends up in exactly one state:
  - `followed`
  - `missing`: dangling link or unreadable target
  - `not-link`
  - `single-skill`: phase 1 only
  - `invalid-name`
  - `cycle`
  - `target-overlap`
  - `entry-overlap`

  A separate state, `undeclared-link`, covers first-level links in the source that no file names. Commands consume these states, not warning text (see §3 for `missing`).
- **Same name rules as a real directory.** A followed `_dev-skills` that contains `.git` is a tracked repo, through `utils.IsTrackedRepoDir` plus the `.git` check. A followed entry without `_` is a group. Repo-level `.skillignore` inside a followed repo applies as it does today.
- **Logical paths everywhere.** Skills are reported as `<source>/_dev-skills/<skill>` with `RelPath` `_dev-skills/<skill>` and `FlatName` `_dev-skills__<skill>`. This matches the existing `SourcePath: filepath.Join(sourcePath, relPath)` convention at `discover_walk.go:305`, so target link names and the manifest stay stable.
- **Default unchanged.** With no `.skillfollow`, an undeclared link stays invisible exactly as today, and every command behaves as today. The existing `TestUninstallGroup_ExternalSymlinkRejected` and `TestUpdateGroup_ExternalSymlinkRejected` (`tests/integration/sync_symlinked_dir_test.go:207`, `:239`) must keep passing unchanged.

Project mode reads the same files from the project skills source (`.skillshare/skills/` or `sources.skills`). This proposal does not add `.agentfollow` or extras support. See Open Questions.

### 2. Future-proofing: one source walker, a ratchet, and a behavior contract

*Maintainer requirement (a).*

The reference implementation swaps a wrapper into each call site. That fixes the sites that exist today, but nothing stops the next new walk from skipping declared entries.

Several scanner families read the skills source directly instead of going through `discoverSourceSkillsInternal`. Some families contain more than one function. The issue names four of them.

| Call site | Function | Today | Effect with a followed entry if not migrated |
|---|---|---|---|
| `internal/install/install_queries.go:223` | `getTrackedReposImpl` (backs `install.GetTrackedRepos`, 12 callers) | Walk, `info.IsDir()` | Followed repo is missing from status, check, `update <name>`, server update/check/overview |
| `internal/config/reconcile_core.go:26` | `reconcileSkillsWalk` | WalkDir, `!d.IsDir()` returns | **Data loss:** `pruneStaleEntries` (`:132`) deletes metadata for every followed skill |
| `cmd/skillshare/update.go:217` | `cmdUpdate` (`--all`) | Walk | Followed repo is not updated |
| `cmd/skillshare/update_project.go:179` | `updateAllProjectSkills` | Walk (root already resolved at `:38`) | Same, in project mode |
| `internal/server/handler_update.go:552` | `getServerUpdatableSkills` | WalkDir | Dashboard "update all" skips it |
| `cmd/skillshare/uninstall.go:280` | `resolveNestedSkillDir` | Walk | `uninstall <skill>` cannot find a followed skill (it must then refuse; see §4) |
| `cmd/skillshare/uninstall.go:184` | `resolveUninstallByGlob` | ReadDir, `e.IsDir()` | Glob uninstall skips followed entries |
| `cmd/skillshare/audit.go:414` | `collectInstalledSkillPaths` | ReadDir, `e.IsDir()` | `audit` misses followed skills |
| `cmd/skillshare/doctor.go:328`, `:808` | `checkSource`, `checkSkillsValidity` | ReadDir, `e.IsDir()` | Counts and validity checks skip them |
| `internal/server/handler_overview.go:49` | `handleOverview` | ReadDir, `e.IsDir()` | Overview count is wrong |
| `internal/install/metadata_migrate.go:136`, `:163` | `migrateSkillSidecars`, `walkSkillDir` | ReadDir, `de.IsDir()` | Stays raw on purpose: migration deletes sidecars (`:158`), which would be a write into the external repo (§4) |
| `internal/git/scope.go:399` | `NestedRepos` | WalkDir, `!d.IsDir()` | Stays raw on purpose: it must describe what git stages (§5) |

Corrections to the issue's list:

- `FindLocalSkills` (`internal/sync/pull.go:43`) walks a *target*, not the source, so it needs no change.
- `update_project.go` now resolves the source root (`:38`). This matches the maintainer's note that `update --all` with a symlinked root already works.

A second class of walks reads a *single skill directory* by its logical path. Examples:

- `ComputeFileHashes` (`internal/install/meta.go:93`)
- `scanSkillImpl` (`internal/audit/audit_scan_skill.go:121`)
- `handleGetSkill` (`internal/server/handler_skills.go:221`)
- `DirMaxMtimeWithIgnore` (`internal/sync/copy.go:372`)
- the diff and TUI walkers

These walks only fail when the walked root is itself a link, and phase 1 rules that out (§1). Phase 4 (followed single skills) has to add a `WalkSkill` that resolves the root and reports paths back under the logical directory. Callers compute relative paths from the original directory (`meta.go:117`, `handler_skills.go:229`), so those relative paths must stay unchanged.

The group walks `resolveGroupUpdatable` (`cmd/skillshare/update_resolve.go:106`) and `resolveGroupSkills` (`cmd/skillshare/uninstall.go:231`) reject a group that resolves outside the source (`update_resolve.go:98-99`, `uninstall.go:226-227`). Exempting declared entries is not enough. These walks run on physical paths, and the later `Rel` checks (`update_resolve.go:117-119`, `uninstall.go:250-251`) would still discard the external relative paths. The walker therefore remaps physical paths back to logical `<source>/<entry>/...` paths, and the containment checks stay in place on those logical paths.

**Proposed mechanism:**

1. **One package owns source traversal.** `internal/sourcewalk` (name open) exposes:
   - `Follow(root, Options{TargetPaths []string}) FollowSet`: parses the two files, validates each entry against §1, and returns the per-entry states. Options are plain values, so `sourcewalk` imports neither `config` nor `sync`, and no import cycle forms between `config`, `install`, and `sync`.
   - `(FollowSet) Walk(fn)`, `WalkDir(fn)`, `ReadDir()`: these walk `ResolveSymlink(root)`. At depth 1, a `followed` entry is reported to `fn` as a directory at its logical path. The wrapper then reads the resolved directory itself and maps every path back to the logical form. As @star-nebula noted, `filepath.Walk` will not descend through either link kind, so the one-hop rule has to live in the wrapper. `SkipDir` and error semantics match `filepath.Walk` and `WalkDir`.
   - `(FollowSet) Owns(resolved) bool` and `(FollowSet) InFollowed(logicalRel) (entry, state)`: used by the ownership rules (§3) and the mutation boundary (§4).

   One `FollowSet` is built per operation and passed down. Walkers do not re-read declarations relative to a child directory. `discoverSourceSkillsInternal` and `getTrackedReposImpl` move first, because 40+ callers already go through those two.

   Most rows in the table become small swaps. These rows have their own semantics, defined in §3 and §4: reconcile, update `--all`, uninstall, and the group walks.
2. **A ratchet test against raw walks.** The repository has no golangci-lint, and `make lint` is `go vet ./...` (`Makefile:134-135`), so the guard is an ordinary Go test (`internal/sourcewalk/guard_test.go`). It runs in `make test`, so CI needs no new tool. It works like this:
   - It parses non-test `.go` files under `cmd/` and `internal/` with `go/parser`, using import-aware matching. Calls are recognized through aliased imports.
   - It finds calls to `filepath.Walk`, `filepath.WalkDir`, `fs.WalkDir`, and `os.ReadDir`. It also finds `.ReadDir`/`.Readdir` method calls on values that are syntactically `*os.File`, which is a heuristic.
   - Each allowed call is recorded at call level: the file, the enclosing function, the callee, a fingerprint of the root-argument expression, and a reason. Reasons are `target`, `backup`, `trash`, `clone`, `agents`, `extras`, `git-root`, `migration`, `skill-dir`, or `sourcewalk`.
   - A new call fails the test, including a new call inside a function that is already on the list. A stale allowance whose call no longer exists also fails.

   **Limitation:** this is a syntactic check. It cannot follow data flow, so a source path passed through a variable into an allowed walk can still slip past it. It makes a raw walk a deliberate, reviewed choice. It is not a guarantee.
3. **The behavior contract.** The real "test that catches it" is an integration fixture with a followed tracked repo, a followed group, a missing entry, and an undeclared link. Every public read path runs against it and must report the same skill set:
   - CLI: `list --json`, `status --json`, `check --json`, `update --all --dry-run`, `audit`, `doctor --json`, `uninstall --dry-run`, `diff`
   - server: the skills, overview, update, and check endpoints

   A new command that reads the source gets a row in this table. The ratchet only flags candidates for that review.

### 3. Link ownership for prune and status

*Maintainer requirement (b).*

**Where it breaks today.** Absolute links are created from the logical `skill.SourcePath` (`createLink`, `internal/sync/symlink_unix.go`). That path starts with the source, so every prefix test passes. Global mode always creates absolute links, because `shouldUseRelative` (`internal/sync/relative.go:16-31`) returns true only when the source and the target both sit under the project root.

Relative links break this. `createLink` computes them from `evalOrClean(sourcePath)` (`symlink_unix.go:14-23`, and the Developer Mode branch of `symlink_windows.go:45-53`). That resolves *every* link component, including the followed entry, so the stored target points straight at `~/code/work/dev/dev-skills/<skill>`. Every reader then sees an external path:

- `SyncTargetMergeWithSkills` compares the link target with the logical path using `utils.PathsEqual(absLink, absSource)` (`sync.go:595-601`). For a relative followed link the compare fails, so the link is removed and recreated on every sync and reported as Updated. Sync never converges.
- `CheckStatusMerge` (`sync.go:960-1029`) counts the link as local, so `doctor`'s `checkSyncDrift` reports drift.
- `PruneOrphanLinksWithSkills` (`sync.go:736-880`) sends it to the external branch. A live link is kept with the "symlink to external location" warning, so it is never pruned when filters change.
- `unlinkMergeMode` (`cmd/skillshare/target.go:544`), `unlinkMergeSymlinks` (`internal/server/handler_targets.go:700`), and `computeTargetDiff` (`internal/server/handler_diff_stream.go:222`) all skip it.

Windows junctions are created by `createJunction(absTarget, absSource)` with the logical `absSource`, and `mklink /J` stores the path it is given. `utils.ResolveLinkTarget` reads it back with `os.Readlink`, and falls back to `filepath.EvalSymlinks` only when Readlink fails. The fallback resolves fully and returns an external path. Whether Go 1.23+ `os.Readlink` returns the stored logical path for a junction whose target crosses another junction is unverified (Open Questions).

**1. Exact link identity, shared by sync and status.** A logical tail alone is not enough. When the source root is itself a link, discovery keeps the caller's logical root (`discover_walk.go:302-305`). A link created from `evalOrClean(sourceRoot) + tail` still fails the lexical compare at `sync.go:599-601`, and so does the Windows fallback. Proposal:

- **Creation.** Relative skill links are computed from `evalOrClean(linkDir)` to `evalOrClean(sourceRoot) + "/" + relPath`. Only the source root and the link's parent are canonicalized. The followed entry stays in the link text, and the OS resolves it on open. `createLink` takes an explicit skills-only root/tail input. Its agents and extras callers keep their current behavior unchanged (`agent_sync.go:153-184`, `:317-349`, `extras.go`, `extras_file.go`). `reformatLink` (`relative.go:90`) and `CreateSymlink` (`sync.go:236`) follow the skills path.
- **Comparison.** A skills-only `sameSkillLink(linkPath, skill, followSet)` replaces the lexical compare in `SyncTargetMergeWithSkills` and the linked test in `CheckStatusMerge`. It reads relative link text against the link's real parent, the same way `prunableLink` already does (`agent_sync.go:111-114`). It accepts exactly one of three forms: the logical skill path, canonical source root plus logical tail, or the fully resolved path of *this* skill. Being somewhere under an owned root is never enough for equality.

**2. Ownership for prune.** Since #315 (`5f3e79f1`), the in-source live-link branch already requires `manifest.Managed[name]` or `force` (`sync.go:819`). @star-nebula's suggestion has therefore partly landed, and the #314 case of hand-made links is fixed.

This proposal keeps the manifest schema (names and modes only, `internal/sync/manifest.go:16-20`) and combines both tests. Provenance alone is not used. A live link is pruned only when two conditions hold:

- it is in `manifest.Managed`
- it resolves under the logical source, or under the resolved target of an entry that is currently `followed`

A user's own link that reuses a name skillshare once managed is therefore never deleted. `--force` keeps #315's escape hatch unchanged (`sync.go:819`, `:832`).

**3. After unfollow.** Links created by this proposal point through `<src>/_f/...`. After `_f` leaves `.skillfollow`, they still resolve under the logical source, and they are pruned like any other orphan. A managed link that resolves to a physical external path can only come from a hand-made link or the Windows read-back fallback. Once its root is no longer followed, prune keeps it with the warning "managed link resolves outside the source after unfollow; remove it or re-run with --force". Remembering per-link targets in the manifest was rejected as extra schema for this edge case.

**4. Missing entries: prune stays conservative.** A followed entry can be `missing` because a drive is unmounted or a repo is being re-cloned. In that case discovery is incomplete, and no naming mode can safely tell which absent names belonged to it. Standard naming uses the skill basename (`internal/sync/target_naming.go:181`), and the manifest has no source mapping. So while any entry is `missing`, these steps skip removal of managed entries that are absent from discovery, and report "skipped: followed entry `<name>` is missing":

- merge prune (`PruneOrphanLinksWithSkills`)
- copy prune (`PruneOrphanCopiesWithSkills`, `internal/sync/copy.go:273-291`)
- reconcile's `pruneStaleEntries` (`reconcile_core.go:132-138`), which keeps metadata under `<entry>/`

Broken links that resolve under the logical source are also kept while any entry is `missing`. Links still to be created are created as usual.

**5. Status provenance.** Today `CheckStatusMerge` reads the manifest only to count hidden entries (`sync.go:996-1015`). Under this proposal, a link that matches only through a followed root counts as linked only when it is in the manifest. Otherwise it counts as local. In-source links keep today's prefix rule, so the result with no `.skillfollow` is unchanged. Sync already records linked and updated entries in the manifest (`sync.go:669-675`). A hand-made link to a followed skill that is in the active selection is therefore adopted on the next sync. The "local" case applies only to links outside the selection.

`PruneOrphanAgentLinks` and `pruneExtraOrphans` are out of scope, because `.skillfollow` covers skills only. They use `prunableLink` (`agent_sync.go:94-117`), which resolves both sides but is not a provenance check: it also removes proven-broken links.

**Test cases** (unit tests in `internal/sync`, extending `merge_test.go`, `status_test.go`, `relative_test.go`):

| # | Setup | Expected |
|---|---|---|
| 1 | Global, absolute link to `<src>/_f/a`, `_f` followed | 2nd sync: 0 updated; status linked |
| 2 | Project mode, relative link, `_f` followed | link text goes through `<src>/_f/a`; 2nd sync 0 updated; status linked |
| 3 | 2 plus a symlinked source root and a symlinked target parent | same as 2 (exact identity, no churn) |
| 4 | Managed link whose read-back is fully resolved `/ext/_f/a` (simulated fallback) | status linked; sync idempotent; prune removes it when `a` is filtered out |
| 5 | Link to `/ext/_f/a`, not in manifest, `a` outside the selection | kept, counted `local` |
| 6 | `_f` removed from `.skillfollow`, managed link through `<src>/_f/a` | pruned as orphan |
| 7 | `_f` removed, managed physical link `/ext/_f/a` | kept with the after-unfollow warning; removed with `--force` |
| 8 | `_f` target missing (merge and copy mode, flat and standard naming) | no removals of managed absent names; metadata kept; warning names `_f` |
| 9 | Hand-made link to `/ext/a`, nothing declared | kept with external warning (default unchanged) |
| 10 | Windows junction in the source (simulated) | discovered when declared, invisible when not |
| 11 | Real Windows junction to a followed skill (`skillshare-windows-utm`) | created, owned, 2nd sync idempotent, pruned on filter change |
| 12 | Windows Developer Mode, relative symlink | same as 2 |

The junction simulation for case 10 needs a seam. `utils.isJunction` is unexported, so `sourcewalk` tests cannot assign it. Either add a small exported test hook in `utils`, or inject the link check into `sourcewalk`.

### 4. External ownership: what may change a followed tree

Everything under `<src>/<followed>/` belongs to the user, not to skillshare. Today's guards are not enough. For a followed *group*, `handleBatchUninstallSkills` (`internal/server/handler_uninstall.go:239-251`) refuses only tracked-repo members and would move a group child into trash. `resolveGroupSkills` (`uninstall.go:221-255`) passes children to `MoveToTrash` (`uninstall_handlers.go:113`, `:129`). The proposal therefore defines one boundary. It lives in shared domain code that the CLI and server both call, not in each handler.

**Allowed:**

- Unlink and unfollow of the root entry. The link is removed and never trashed. The target is never touched. The name is removed from every declaration file that contains it, and the command reports which file it edited. `trash.MoveToTrash` would rename the link into trash (`internal/trash/trash.go:215`), where `trash.List` cannot show it (`:237`).
- Pull of a followed tracked repo, under the git rules in §5.

**Refused**, with "inside followed entry `<name>`; hide it with the root `.skillignore`":

| Operation | Seam |
|---|---|
| CLI uninstall of a descendant | `performUninstallQuiet`, `performUninstall` (`cmd/skillshare/uninstall_handlers.go:107`, `:128`) |
| Server uninstall of a descendant | `handleBatchUninstallSkills` (`internal/server/handler_uninstall.go:147`, trash at `:204`, `:251`) |
| Collect into or over a followed entry | `PullSkill` (`internal/sync/pull.go:127`, `RemoveAll` at `:136`) |
| Install overwrite or `--into` a followed tree | `installImpl` (`internal/install/install_apply.go:177`, `RemoveAll` at `:202`); `installFromDiscoveryInternal` (`:348`, replace at `:449-452`) |
| Tracked install over a followed repo | `installTrackedRepoImpl` (`internal/install/install_tracked.go:10`, overwrite at `:72`) |
| Standalone or local update inside a followed tree | `handleUpdate` (`internal/install/install_update.go:11`, replace at `:146-150`, `:260-263`) |
| Legacy sidecar migration inside a followed tree | `migrateSkillSidecars`, `walkSkillDir` (`internal/install/metadata_migrate.go:135`, `:153`, delete at `:158`). Skipped; `doctor` reports unmigrated sidecars. |

`enable`/`disable` (`cmd/skillshare/enable.go`) and `PUT /api/skillignore` (`internal/server/handler_skillignore.go:80-89`) already write the root ignore files only, so they need no change.

Reconcile marks names under a followed entry as live. It never creates or updates metadata for them (no `store.Set` at `reconcile_core.go:97-105` and no `RefreshTrackedRootSkillHashes`). Following never creates a `tracked: true` entry.

### 5. Git operations

*Maintainer requirement (c).*

**The nested-repo gap.** `NestedRepos` (`internal/git/scope.go:396-418`) never sees a followed repo, and it should stay that way. A followed repo is not nested in the git root, and git does not traverse links. If `NestedRepos` followed it, the existing guard would advise disabling the *external* `.git`. It stays a raw walk with the `git-root` allowance. The real risk is the link entry itself:

- `install.UpdateGitIgnore` writes tracked-repo patterns with a trailing slash (`internal/install/gitignore.go:26-28`, called from `install_tracked.go:144`).
- A pattern ending in `/` matches only directories, and git stores a symlink as a file (mode 120000).
- Several paths stage everything:
  - `stageAndCommit` (`cmd/skillshare/push.go:91`), used by `push` and `commit`, runs `git add -A`
  - the server runs `git.StageAll` (`internal/git/info.go:599-603`) from `handler_git.go:418` and `:515`
  - `commitSourceFiles` (`cmd/skillshare/init.go:567-573`), used by `init --remote`, runs `git add .`

Any of these would commit `_dev-skills` as a link that holds a machine-specific absolute path, the portability problem the maintainer raised in #207.

**Proposal:**

- **Ignore line.** The correct entry is `/_dev-skills`: anchored, no trailing slash, written into the managed block of the skills source `.gitignore`. Reuse `install.UpdateGitIgnoreFiles` (`gitignore.go:62-70`), which already writes no-slash entries.
- **Who writes it.** In phase 1, nothing writes it automatically. Discovery, `list`, `status`, `doctor`, and every `--dry-run` stay read-only. The guard below refuses and prints the exact line, and `doctor` reports `not-ignored`. In phase 2, `follow`/`unfollow` write or remove the line together with the declaration.
- **Guard at every staging point.** A new check, `FollowedLinksStaged(gitRoot, followSet)`:
  - runs only when the effective git root (`EffectiveGitRoot`, `internal/config/config.go:409-425`) contains the skills source
  - checks only declared links that lie inside that staging tree, so agents and extras scopes, and a custom skills source outside the git root, are unaffected
  - is called from `stageAndCommit`, both server staging paths (beside `rootScopeGuard`, `handler_git.go:563`), and `commitSourceFiles`

  Each declared link is checked read-only:
  - Already indexed (`git ls-files --error-unmatch <path>` succeeds): refuse and tell the user to run `git rm --cached <path>` themselves. skillshare never untracks automatically.
  - Not indexed and not ignored (`git check-ignore` fails): refuse with the exact ignore line.
  - The refusal has the same shape as `errNestedRepos` (`cmd/skillshare/gitroot.go:159`), and a dry run reports it.
- **`.skillfollow` is committed.** `.skillfollow.local` is treated like `.skillignore.local`, which skillshare does not auto-ignore today (Open Questions).
- **Root scope (`git_root: root`).** `ScopeDir("root")` is `BaseDir()`, which contains the skills source, so the source's nested `.gitignore` applies. Git does not follow links, and the guard keeps the link out. So a followed external repo can never be committed into the root repo.

**Tracked-repo update on a followed repo.** Once `getTrackedReposImpl` uses the walker, the followed repo appears in `status`, `check`, `update <name>`, `update --all`, and the server update and check handlers. `install.IsGitRepo` uses `os.Stat`, which follows the link. The server's `resolveTrackedRepo` (`internal/server/handler_skills.go:547-564`) already stats `_dev-skills` directly. So the server update route can resolve a followed repo today, while CLI `update dev-skills` reports "not found". The walker makes them consistent.

A followed repo is the user's working copy, so the update rules change:

- **Fast-forward only.** `pullWithResolution` (`internal/git/info.go:254`) runs `git pull --no-rebase --ff`, which creates a merge commit when histories diverge. For followed repos, use `--ff-only`. On divergence, refuse with "resolve in `<resolved path>`". skillshare must not create commits in the user's repo.
- **The dirty check stays.** `git.IsDirty` refuses a dirty tree before the pull.
- **`--force` is refused** on every route:
  - CLI `updateTrackedRepo`, where `git.Restore` runs at `cmd/skillshare/update_handlers.go:143`
  - CLI `updateTrackedRepoQuick`: `:317` and `:329`
  - server `updateTrackedRepo`: `internal/server/handler_update.go:304` and `:317`
  - the streaming endpoint delegates to the server function (`handler_update_stream.go:127`)

  The refusal reads "followed repo: resolve local changes in `<resolved path>`".
- **`install --update`** reaches a separate pull and audit path, `install.updateTrackedRepo` (`install_tracked.go:157-182`), which has no dirty check. For followed repos it is routed through the same clean-tree, ff-only policy.
- **Audit rollback is kept, and documented.** When the audit fails after a pull, `auditGateAfterPull` (`update_handlers.go:56`, `:96`, `:107`) and the server `auditGateTrackedRepo` (`handler_update.go:375`, `:392`) run `git reset --hard <beforeHash>` even without `--force`. The tree was clean, and `beforeHash` is the user's own HEAD, so this only undoes what skillshare pulled. The docs must say so plainly: refusing `--force` does not mean skillshare never hard-resets a followed repo.
- **Known limitation: concurrency.** The clean-tree check happens once. An editor or another git process could change the working copy during fetch, pull, or audit. The rollback could then discard those changes. The test plan covers the intended failure message. The docs say not to edit the repo while `update` runs on it.
- **Audit gate.** `update` audits each new revision. Content that changes because the user pulled outside skillshare is not audited, the same as editing a skill in the source. `audit` and `check` make it visible. The #207 concern about bypassing the audit gate therefore applies only to changes made outside `update`, and this proposal treats those as the user's own edits.

### 6. Windows

- Detect declared entries with `utils.IsLinkMode(path, info.Mode())`, never `ModeSymlink` alone, and never `info.IsDir()`. A junction is `ModeIrregular` and not a directory (`internal/utils/link.go`).
- The wrapper reads the resolved directory itself and maps every path back to the logical `<source>/<entry>/...` form before calling `fn`. This is where the one-hop rule is enforced. It is also why `discover_walk.go`'s `filepath.Rel(walkRoot, path)` must use the logical path for followed subtrees, not the resolved one.
- Canonical comparisons use `utils.PathsEqual` and `PathHasPrefix`, which are case-insensitive on Windows (`internal/utils/path.go`).
- Real-Windows coverage goes in an `ai_docs/tests/` runbook, run with `skillshare-windows-utm`. It uses both a basic-user token (junction only) and Developer Mode (relative symlink), and covers discovery, sync idempotency, status, prune after a filter change, and unfollow/uninstall.

### 7. Common base for #253 and #206

- **#206 (live link to an app-bundled skill).** Closed by #334: `check` now detects drift at local install sources, and `update` re-copies through the audit. In #207 the maintainer chose that over `--link`, to keep the audit gate, Windows parity, and backup and git behavior. If `--link` comes back, it could sit on this base:
  - `install --link <dir>` creates the link (a junction on Windows)
  - it adds the name to `.skillfollow.local`, which is machine-specific like the target path
  - it adds the anchored `.gitignore` line

  It would need followed single skills (phase 4). This proposal does not reopen it.
- **#253 (`include_sources` for `~/.agents/skills`, `~/.cursor/skills`).** A followed entry `_agents -> ~/.agents/skills` would expose those skills as `_agents/<skill>`. If `~/.agents/skills` is an active configured target (it is the universal target in `internal/config/targets.yaml`), §1's overlap rule rejects it. Otherwise sync would write into the directory it reads. A path that is not an active target can be followed.

  `.skillfollow` is therefore a partial answer to #253. #253 could reuse the walker for discovery. Read-only inputs give no authority to prune, though, so #253 would need its own ownership rule, as well as its own answers on name collisions and read-only semantics. Its config surface stays separate.

### 8. CLI and dashboard surface

- **Phase 1: no new command.** The file is hand-edited, as `.skillignore` was before `enable`/`disable` existed.
- **Phase 2:** `skillshare follow <name>` / `unfollow <name>` with `-p`/`-g` and `--local`. They write the declaration and the anchored `.gitignore` line together. A later `follow <name> --to <dir>` could also create the link (a junction on Windows). This mirrors `enable`/`disable` (`cmd/skillshare/enable.go`).
- **`status`.** Add a line next to `printSkillignoreLine` (`cmd/skillshare/status_render.go`), for example `.skillfollow (.local active): 2 entries, 1 skipped`. JSON gets `source.skillfollow` next to `source.skillignore` (`buildSkillignoreJSON`, `cmd/skillshare/status.go:216`).
- **`doctor`.** Add `checkSkillfollow` after `checkSkillignore` (`cmd/skillshare/doctor.go:282`). It reports each entry's state from §1, plus `not-ignored` and unmigrated sidecars. It also prints an info line for each *undeclared* first-level link in the source. Those links are invisible today with no notice (star-nebula's point), so this line is useful even without following.
- **`list`.** Followed tracked repos show as tracked. The suffix could add `→ <resolved>`, as #206 suggested.
- **Dashboard.** `.skillignore` has `GET/PUT /api/skillignore` (`internal/server/server.go:627-633`, `handler_skillignore.go`) and a tab in `ui/src/pages/ConfigPage.tsx`. A `.skillfollow` tab could reuse that pattern in phase 2; phase 1 does not need it. Doctor and status data already reach the dashboard through the existing endpoints.
- **Docs to update** (English first, then translations through the normal flow):
  - `website/docs/reference/filtering.md` (new section next to `.skillignore`)
  - `reference/appendix/file-structure.md`
  - `understand/source-and-targets.md` (the symlinked-source tip)
  - `reference/commands/sync.md#dotfiles-manager-compatibility`
  - `reference/commands/{status,doctor,update,uninstall,collect,install,push,commit,list}.md`, including the ff-only, `--force`, and audit-rollback rules
  - `reference/targets/configuration.md#git-root`
  - `how-to/sharing/cross-machine-sync.md`
  - `troubleshooting/faq.md` (dotfiles question)
  - the built-in skill `skills/skillshare/references/`

  `reference/commands/backup.md` stays as is. Backup copies only target content and skips links (`internal/backup/backup.go:30`, `:56`, `:308`, `:330`).

## Alternatives Considered

- **Follow every link in the source.** This breaks the deliberate boundary and changes behavior for every existing user with a stray link. Rejected. Opt-in keeps the default unchanged.
- **Target paths in config (`include_sources`, #253).** Absolute paths differ per machine, so the config cannot be shared. Skills would also have no logical location in the source, which every link and manifest path assumes.
- **Keep the repo inside the source with a link outside it.** This is what works today, and the issue explains why it fails the user.
- **Wrapper swaps without a ratchet or a behavior fixture.** This is the reference implementation's shape. It is the smallest diff, but it does not meet the future-proofing requirement.
- **A `file:function` allowlist.** A new walk inside an allowed function would pass silently. The call-level ratchet catches it.
- **`manifest.Managed` as the only ownership test.** Names alone cannot tell skillshare's link from a user's link with the same name.
- **Store each managed link's target in the manifest.** This would let prune clean up physical links after unfollow. It is extra schema for an edge case that only hand-made links and the Windows fallback produce. Rejected for now; such links are kept with a warning.
- **Allow mutations of descendants because the entry is declared.** Declaring an entry gives discovery read access. It does not give skillshare ownership of the user's tree.
- **Block commits in every git scope.** That would let a skills declaration block unrelated agents and extras repos. The guard only checks the tree that is being staged.
- **Follow external repos in `NestedRepos`.** That would report what git does not stage and advise disabling the external `.git`.
- **Commit the link instead of ignoring it.** Absolute targets break on other machines (#207). Relative targets outside the repo are no better.

## Scope

- [ ] Small (1-3 files, < 200 lines)
- [ ] Medium (3-10 files, 200-500 lines)
- [x] Large (10+ files, 500+ lines)

The reference implementation was about 26 insertions and 17 deletions across 11 upstream files, plus a new package. This proposal is substantially larger, because it adds:

- the ratchet and the behavior fixture
- exact link identity
- the missing-entry prune policy
- the external mutation boundary across CLI and server
- the git staging guard and the ff-only rule
- doctor and status output

Treat any line estimate as rough. It is not a commitment.

## Test Plan

- **Unit tests:**
  - parser: comments, `.local` merge, every rejected name form
  - validation: every per-entry state, overlaps in both directions against targets and between entries, canonicalization through a missing ancestor
  - walker: one hop only, logical paths, `SkipDir`, the junction seam
  - the ratchet test, including stale allowances
  - the ownership table (§3) in flat and standard naming
- **Integration** (`tests/integration/`, devcontainer): the behavior fixture from §2.3. It also covers:
  - default unchanged: no `.skillfollow` gives identical output
  - each refusal in §4, through the CLI and the server
  - `uninstall`/unfollow of a followed entry removes only the link and the declaration
  - `update` on a followed repo: `--force` refused on every route, divergence refused (ff-only), `install --update` routed through the same policy, audit failure rolls back to `beforeHash` with the documented message
  - `commit`/`push`/`init --remote` refused when a declared link is indexed or not ignored, under `git_root: skills` and `git_root: root`; an agents-scope repo unaffected
  - a missing entry keeps metadata, links, and copies
- **Windows:** `skillshare-windows-utm` runbook for cases 10-12 of §3 and the items in §6.
- `make check` in the devcontainer.

## Phased Rollout

1. **Visibility first, no behavior change:** the `doctor` info line for undeclared first-level links in the source. It can ship on its own.
2. **Walker, ratchet, and fixture.** Move the existing walks onto `sourcewalk` with following disabled. This is a pure refactor, and existing tests must pass unchanged.
3. **Enable `.skillfollow` for groups and tracked repos.** This phase ships:
   - parsing and the per-entry states
   - exact link identity and the ownership rules
   - the missing-entry policy
   - the mutation boundary
   - the git guard and the ff-only and `--force` rules
   - status and doctor output

   Docs ship in the same release, marked experimental.
4. **CLI `follow`/`unfollow`, the dashboard tab, and followed single skills** (with `WalkSkill`). Revisit `install --link` (#206) and `include_sources` (#253) on this base if they are still wanted.

## Open Questions

1. **Windows junction read-back.** Does `os.Readlink` in Go 1.23+ return the stored logical path for a junction created as `mklink /J <target> <source>\_f\skill`, where `_f` is itself a junction? If not, sync relies on `sameSkillLink`'s fully resolved form, and unfollow cleanup on the warning path in §3. This needs a real Windows run.
2. **`.local` files and git.** Should skillshare add `.skillfollow.local` to the managed `.gitignore` block? It does not do this for `.skillignore.local` today.
3. **Followed targets inside the git root.** Should a followed entry whose target lies inside the git root (for example under `BaseDir()` at root scope) be rejected, or only warned? Its content would be ordinary root-repo content reached through two paths.
4. **Agents and extras.** Is `.agentfollow` wanted? The agent and extras walkers (`internal/resource/agent.go`, `internal/sync/extras.go:49`) have the same blind spot.
5. **A `followed` metadata kind.** A later phase might want metadata for followed repos (for example to show their branch in `list`) without marking them as installed. Is that worth a schema change?
6. **Broken external links.** Should the broken-external-link branch of `PruneOrphanLinksWithSkills` (`sync.go:828`), which removes without a manifest check, also get the #314 provenance rule? This is independent of this proposal.
7. **Package name and placement.** Is `internal/sourcewalk` acceptable, or should the walker live in `internal/sync` next to `discoverSourceSkillsInternal`? The import direction must stay free of `config`/`sync` cycles.
