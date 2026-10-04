# Feature Proposal: Opt-in `.skillfollow` for declared first-level links in the skills source

Issue: [#274](https://github.com/runkids/skillshare/issues/274). Related: [#314](https://github.com/runkids/skillshare/issues/314) (fixed by #315), [#253](https://github.com/runkids/skillshare/issues/253), [#206](https://github.com/runkids/skillshare/issues/206) (closed by #334).

Credits: the `.skillfollow` design, the call-site list, and the reference implementation come from @hhdebb in #274. The Windows junction analysis and the provenance suggestion come from @star-nebula's comment on #274 and from #314/#315.

Source baseline: every file and line reference was checked against `main` at `475b1769`. Line numbers will drift; function names are the stable reference. The analysis comes from reading the source, plus two Windows probes (§6, "Windows evidence"; §4, the `os.Root` table). The rest of the Windows behavior still needs the runbook in the Test Plan. This revision includes two rounds of adversarial review.

## Problem

A user keeps a shared team skills repo with their other projects (`~/code/work/dev/dev-skills`). They want skillshare to treat it as a tracked repo: discover its skills, sync them, and `update` it with `git pull`. Today the only layout that works puts the real repo inside the source and a link back outside it:

```
~/.config/skillshare/skills/_dev-skills/   <- real repo
~/code/work/dev/dev-skills                 <- symlink back to it
```

Editors, agent CLIs, and session managers canonicalize the working directory, so they all show the `~/.config/...` path. The repo ends up with two names, and the one the user types is never the one that is displayed. The user wants the reverse layout:

```
~/code/work/dev/dev-skills                 <- real repo
~/.config/skillshare/skills/_dev-skills    <- link to it
```

In that layout the repo is invisible. Discovery runs `filepath.Walk` in `discoverSourceSkillsInternal` (`internal/sync/discover_walk.go:162`), and Walk uses `Lstat`. A linked child therefore reports `IsDir() == false`. It fails every `info.IsDir()` branch (`:168`, `:176`, `:187`, `:216`) and the `SKILL.md` branch (`:234`), and drops out of the walk with no skip notice. Only the source root is resolved first (`utils.ResolveSymlink`, `:141`).

Windows has one extra detail, from @star-nebula. Creating a junction (`mklink /J`) needs no symlink privilege, while a directory symlink needs Developer Mode or elevation. That is why `createLink` in `internal/sync/symlink_windows.go` falls back to a junction, and why many Windows users can only create junctions. Since Go 1.23, a junction reports `ModeIrregular`, not `ModeSymlink`, and not a directory. `utils.IsLinkMode` (`internal/utils/link.go`) exists to handle this. star-nebula measured it on 0.22.0: two skills behind a junction in the source were not discovered.

This is a deliberate boundary, not a bug. The source root may be a link, because dotfiles managers need that (`website/docs/reference/commands/sync.md#dotfiles-manager-compatibility`). Links inside the source are not followed. In #207, the maintainer listed what following them would touch: discovery, list, doctor, backup, the audit gate, and cross-machine portability. This proposal defines an explicit opt-in and answers each of those.

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
- **Strict names.** Blank lines and `#` comments are ignored. Surrounding whitespace is trimmed. Duplicates across both files collapse into one entry. An entry is rejected with a warning if any of these holds:
  - it is empty, `.`, or `..`
  - it contains `/` or `\`
  - it is absolute, a volume such as `C:`, or a UNC path. This check is platform-independent, so a declaration is portable.
  - `filepath.Clean` changes it
  - it uses glob or negation syntax

  When an accepted name goes into a `.gitignore` line, Git pattern metacharacters are escaped.
- **First level only.** Only direct children of the source root are followed. Links inside a followed tree stay unfollowed, the same as links in the source today, so recursion is bounded at one hop.
- **Classification, in order.** Each declared name ends up in exactly one state. The first rule that applies wins:
  1. `missing`: no entry exists, the link dangles, or the resolved root cannot be read. This is temporary. §3 defines how sync behaves while an entry is missing.
  2. `not-link`: the entry is a real directory. This is a no-op; the directory is discovered anyway.
  3. `invalid-target`: the entry is a link that resolves to anything other than a directory, or the entry itself is neither a link nor a directory (a regular file, FIFO, socket, or device).
  4. `cycle`: the resolved target is the source root, an ancestor of it, or inside it.
  5. `target-overlap`: the resolved target equals an active skills target path (global or project, from the current config), or is an ancestor or descendant of one. The check runs in both directions. Without it, a followed `/ext` with a target at `/ext/out` would make sync write links inside the tree that discovery reads (`SyncTargetMergeWithSkills`, `internal/sync/sync.go:587`, `:625`, `:639`).
  6. `inside-git-root`: the resolved target lies physically inside skillshare's effective git staging tree (see §5). Ignoring the link cannot stop the target's content from being staged through its real path. Also, `NestedRepos`/`DisableNestedRepo` (`internal/git/scope.go:396`, `:424`) could then offer to disable the user's own `.git`.
  7. `entry-overlap`: two entries resolve to the same target, or one target is inside the other. *Both* entries are rejected, and the warning names the pair, so the merge order of `.local` never decides which one wins.
  8. `single-skill`: the resolved root contains `SKILL.md`. It comes after every safety check, so a link such as `a -> <active-target>/a` is rejected as `target-overlap` instead of becoming a skill that `sync --force` would delete through `os.RemoveAll` (`internal/sync/sync.go:639`). It still runs before tracked-repo detection, regardless of a `_` prefix. Rollout step 3 skips these entries; step 4 enables them.
  9. `followed`.

  A first-level link that no file declares is reported as `undeclared-link`. Commands act on these states, not on warning text. All comparisons run after canonicalization. A missing path is resolved through its nearest existing ancestor, the way `evalOrClean` does it (`internal/sync/relative.go:41-59`). On Windows, canonicalization cannot rely on `filepath.EvalSymlinks`, because it does not resolve junctions (§6). The entry is resolved with `utils.ResolveLinkTarget`, and every other path goes through a junction-aware canonicalizer. Comparisons use `utils.PathsEqual` and `PathHasPrefix`, which are case-insensitive on Windows.
- **Same name rules as a real directory.** A followed `_dev-skills` that contains `.git` is a tracked repo, through `utils.IsTrackedRepoDir` plus the `.git` check. A followed entry without `_` is a group. A repo-level `.skillignore` inside a followed repo applies as it does today.
- **Logical paths everywhere.** Skills are reported as `<source>/_dev-skills/<skill>`, with `RelPath` `_dev-skills/<skill>` and `FlatName` `_dev-skills__<skill>`. This matches the existing `SourcePath: filepath.Join(sourcePath, relPath)` convention at `discover_walk.go:305`, so target link names and the manifest stay stable.
- **Default unchanged.** With no `.skillfollow`, an undeclared link stays invisible exactly as today, and every command behaves as today. The existing `TestUninstallGroup_ExternalSymlinkRejected` and `TestUpdateGroup_ExternalSymlinkRejected` (`tests/integration/sync_symlinked_dir_test.go:207`, `:239`) must keep passing unchanged.

Project mode reads the same files from the project skills source (`.skillshare/skills/` or `sources.skills`). Agents and extras are out of scope (see Decisions).

### 2. Future-proofing: one source walker, a ratchet, and a behavior contract

*Maintainer requirement (a).*

The reference implementation swaps a wrapper into each call site. That fixes the sites that exist today, but nothing stops the next new walk from skipping declared entries.

**Raw scans of the skills source.** These scanner families read the source directly instead of going through `discoverSourceSkillsInternal`. Some families contain more than one function. The issue names four of them.

| Call site | Function | Today | Effect with a followed entry if not migrated |
|---|---|---|---|
| `internal/install/install_queries.go:223` | `getTrackedReposImpl` (backs `install.GetTrackedRepos`, 12 callers) | Walk, `info.IsDir()` | Followed repo is missing from status, check, `update <name>`, server update/check/overview |
| `internal/config/reconcile_core.go:26` | `reconcileSkillsWalk` | WalkDir, `!d.IsDir()` returns | **Data loss:** `pruneStaleEntries` (`:132`) deletes metadata for every followed skill |
| `cmd/skillshare/update.go:217` | `cmdUpdate` (`--all`) | Walk | Followed repo is not updated |
| `cmd/skillshare/update_project.go:179` | `updateAllProjectSkills` | Walk (root already resolved at `:38`) | Same, in project mode |
| `internal/server/handler_update.go:552` | `getServerUpdatableSkills` | WalkDir | Dashboard "update all" skips it |
| `cmd/skillshare/uninstall.go:280` | `resolveNestedSkillDir` | Walk | `uninstall <skill>` cannot find a followed skill (it must then refuse; see §4) |
| `cmd/skillshare/uninstall.go:184` | `resolveUninstallByGlob` | ReadDir, `e.IsDir()` | Glob uninstall skips followed entries |
| `cmd/skillshare/audit.go:414` | `collectInstalledSkillPaths` | Uses `DiscoverSourceSkillsLite` first (`:399`), then a ReadDir fallback for unmatched first-level groups | The fallback skips followed groups. Once it follows them, the group root it returns can itself be a link (see repo-root consumers below) |
| `cmd/skillshare/doctor.go:328`, `:808` | `checkSource`, `checkSkillsValidity` | ReadDir, `e.IsDir()` | Counts and validity checks skip them |
| `internal/server/handler_overview.go:49` | `handleOverview` | ReadDir, `e.IsDir()` | Overview count is wrong |
| `internal/install/metadata_migrate.go:136`, `:163` | `migrateSkillSidecars`, `walkSkillDir` | ReadDir, `de.IsDir()` | Stays raw on purpose: migration deletes sidecars (`:158`), which would be a write into the external repo (§4) |
| `internal/git/scope.go:399` | `NestedRepos` | WalkDir, `!d.IsDir()` | Stays raw on purpose: it must describe what git stages (§5) |

Corrections to the issue's list:

- `FindLocalSkills` (`internal/sync/pull.go:43`) walks a *target*, not the source, so it needs no change.
- `update_project.go` now resolves the source root (`:38`). This matches the maintainer's note that `update --all` with a symlinked root already works.

**Repo-root consumers must be fixed in rollout step 3.** Some walks are rooted *at* a followed entry. `filepath.Walk` started on a link does not descend into it, but the `os.Stat` checks before these walks pass through the link. So the walk finds nothing, and nothing reports it. For the update audit this is a fail-open security bug: a pulled revision with malicious content would scan clean. Each consumer below resolves the root before walking and reports logical paths:

| Consumer | Evidence |
|---|---|
| CLI update audit (single and quick) | `auditGateAfterPull` (`cmd/skillshare/update_handlers.go:41`, scan at `:50`), called from `:206` and `:343`. The scanner is `scanSkillImpl` (`internal/audit/audit_scan_skill.go:121`). |
| Server update audit, including SSE | `auditGateTrackedRepo` (`internal/server/handler_update.go:362`) |
| `install --update` audit | `auditTrackedRepoUpdate` (`internal/install/install_audit.go:277`) |
| `install --update` re-discovery | `updateTrackedRepo` calls `discoverSkills(repoPath, true)` (`internal/install/install_tracked.go:186`). The result can report zero skills, and agent re-discovery at `:193` depends on that count. |
| Audit of fallback groups | `collectInstalledSkillPaths` (`cmd/skillshare/audit.go:414-424`) |

As a second line of defense, if a scan of a followed root reads zero eligible files while the resolved directory has eligible content, that counts as a scan error. It blocks the update and rolls back. The implementation defines "eligible": `.git`-only, ignored, and non-scannable content does not count, so empty or doc-only repos do not trip a false positive.

`RefreshTrackedRootSkillHashes` (`internal/install/metadata.go:407-420`) also hashes from a repo root, but only when the root has `SKILL.md`. Step 3 classifies that shape as `single-skill`, and reconcile suppresses metadata and hashes for followed entries anyway (§4). So it waits for step 4.

**Child-skill walks are deferred to step 4.** These walks start at a single skill directory *below* the link:

- `ComputeFileHashes` (`internal/install/meta.go:93`)
- `handleGetSkill` (`internal/server/handler_skills.go:221`)
- `DirMaxMtimeWithIgnore` (`internal/sync/copy.go:372`)
- the diff and TUI walkers

The OS resolves the link component in the middle of the path, so these walks work in step 3. Step 4 (followed single skills) adds `WalkSkill`, which resolves the root and reports paths under the logical directory. Callers compute relative paths from the original directory (`meta.go:117`, `handler_skills.go:229`), so those relative paths must not change.

**Group walks keep their containment checks.** `resolveGroupUpdatable` (`cmd/skillshare/update_resolve.go:106`) and `resolveGroupSkills` (`cmd/skillshare/uninstall.go:231`) reject a group that resolves outside the source (`update_resolve.go:98-99`, `uninstall.go:226-227`). Their later `Rel` checks (`update_resolve.go:117-119`, `uninstall.go:250-251`) would also discard external relative paths, so exempting declared entries is not enough. The walker remaps physical paths back to logical `<source>/<entry>/...` paths, and the containment checks run unchanged on those.

**Incomplete discovery must be visible.** `discoverSourceSkillsInternal` currently drops walk errors (`discover_walk.go:163`). Inside a followed subtree, the walker records an unreadable directory as making that entry `missing` for this run. It does not return a partial result as if it were complete. §3's missing-entry rules then apply.

**Proposed mechanism:**

1. **One package owns source traversal.** `internal/sourcewalk` takes plain inputs and imports neither `config` nor `sync`. Today `config` imports `install` (`reconcile_core.go:9`) and `sync` imports `config` (`target_naming.go:9`), so the shared walker cannot live in `sync` without an import cycle. The package exposes:
   - `Follow(root, Options{TargetPaths []string; GitRoot string}) FollowSet`: parses the two files, classifies each entry (§1), and returns the states.
   - `(FollowSet) Walk(fn)`, `WalkDir(fn)`, and `ReadDir()`. These walk `ResolveSymlink(root)`. At depth 1, a `followed` entry is reported to `fn` as a directory at its logical path. The wrapper then reads the resolved directory itself and maps every path back to the logical form. As @star-nebula noted, `filepath.Walk` will not descend through either link kind, so the one-hop rule has to live in the wrapper. `SkipDir` and error semantics match `filepath.Walk`/`WalkDir`. The root and every returned path share one logical base, including when the source root is a link.
   - `(FollowSet) Owns(resolved) bool` and `(FollowSet) InFollowed(logicalRel) (entry, state)`. These serve ownership (§3), the git seams in §4, and error messages for the §4 handle.

   One `FollowSet` is built per operation and passed to nested consumers. It is never rebuilt from a child directory. `discoverSourceSkillsInternal` and `getTrackedReposImpl` move first, because more than 40 callers already go through those two. Most rows in the table become small swaps. Reconcile, update `--all`, uninstall, the group walks, and the repo-root consumers have their own semantics, defined here and in §3–§5.
2. **A ratchet test against raw walks.** The repository has no golangci-lint, and `make lint` is `go vet ./...` (`Makefile:134-135`). So the guard is an ordinary Go test (`internal/sourcewalk/guard_test.go`) that runs in `make test`. It works like this:
   - It parses the non-test `.go` files under `cmd/` and `internal/` with `go/parser`, using import-aware matching so aliased imports count.
   - It finds calls to `filepath.Walk`, `filepath.WalkDir`, `fs.WalkDir`, and `os.ReadDir`. It also finds `.ReadDir`/`.Readdir` on values that are syntactically `*os.File`. That last match is a heuristic.
   - Each allowed call is recorded with its file, enclosing function, callee, a fingerprint of the root-argument expression, and a reason. Valid reasons are `target`, `backup`, `trash`, `clone`, `agents`, `extras`, `git-root`, `migration`, `skill-dir`, and `sourcewalk`.
   - A new call fails the test, even inside a function that is already on the list. An allowance whose call no longer exists also fails.

   **Limitation:** this is a syntactic check. It cannot follow data flow, so it makes raw walks a deliberate, reviewed choice but does not guarantee anything.
3. **The behavior contract.** The real "test that catches it" is an integration fixture. It contains a followed tracked repo, a followed group, a missing entry, a rejected entry, and an undeclared link. The fixture holds a **per-command expectation matrix**. It does not require one shared skill set, because commands legitimately differ: `update` excludes local content, and mutations of descendants must refuse. The matrix covers:
   - CLI: `list --json`, `status --json`, `check --json`, `update --all --dry-run`, `audit`, `doctor --json`, `uninstall --dry-run`, `diff`
   - server: the skills, overview, update, and check endpoints

   For each command, the matrix states what is visible, what is refused (dry runs included), and what is reported. A new command that reads the source gets a row; the ratchet only flags candidates for that review.

### 3. Link identity, ownership, and missing entries

*Maintainer requirement (b).*

**Where it breaks today.** Absolute links are created from the logical `skill.SourcePath` (`createLink`, `internal/sync/symlink_unix.go`). That path starts with the source, so every prefix test passes. Global mode always creates absolute links, because `shouldUseRelative` (`internal/sync/relative.go:16-31`) returns true only when the source and the target both sit under the project root.

Relative links are different. `createLink` computes them from `evalOrClean(sourcePath)` (`symlink_unix.go:14-23`, and the Developer Mode branch of `symlink_windows.go:45-53`). That resolves *every* link component, including the followed entry, so the stored target points straight at `~/code/work/dev/dev-skills/<skill>`. Every reader then sees an external path:

- `SyncTargetMergeWithSkills` compares the link with `utils.PathsEqual(absLink, absSource)` (`sync.go:595-601`). For a relative followed link the compare fails, so the link is removed and recreated, and reported as Updated, on every sync. Sync never converges.
- `CheckStatusMerge` (`sync.go:960-1029`) counts the link as local, so `doctor`'s `checkSyncDrift` reports drift.
- `PruneOrphanLinksWithSkills` (`sync.go:736-880`) sends it to the external branch. A live link there is kept with a warning, so it is never pruned when filters change.
- `unlinkMergeMode` (`cmd/skillshare/target.go:544`), `unlinkMergeSymlinks` (`internal/server/handler_targets.go:700`), and `computeTargetDiff` (`internal/server/handler_diff_stream.go:222`) all skip it.

Windows junctions are created by `createJunction(absTarget, absSource)` with the logical `absSource`, and `mklink /J` stores the path it is given. Measured on Windows 11 ARM64 with Go 1.25.5 (§6): `os.Readlink` on such a junction returns the stored logical path `<src>\_f\a`, even when `_f` is itself a junction. `utils.ResolveLinkTarget` therefore never reaches its `EvalSymlinks` fallback, and `PathHasPrefix`/`PathsEqual` against the logical source both hold. Target junctions created through the logical path need no special handling in identity or ownership. The fully resolved form that `sameSkillLink` accepts covers only hand-made links.

The fix is three separate contracts. They answer different questions and must not be mixed.

**1. Identity: does this link already point at this skill?** This is used only by sync to decide whether to keep a link or recreate it.

- *Creation.* Relative skill links are computed from `canon(linkDir)` to `canon(sourceRoot) + "/" + relPath`, where `canon` is the junction-aware canonicalizer from §1. Only the source root and the link's parent are canonicalized. `evalOrClean` cannot be used here: it does not resolve junctions (§6), so for a target whose parent is a junction it would compute the path from the logical parent while Windows resolves it from the real one. The followed entry stays in the link text, and the OS resolves it when the link is opened. `createLink` gets an explicit skills-only root/tail input. Its agents and extras callers keep their current behavior (`agent_sync.go:153-184`, `:317-349`, `extras.go`, `extras_file.go`). `reformatLink` (`relative.go:90`) and `CreateSymlink` (`sync.go:236`) take the skills path.
- *Comparison.* `sameSkillLink(linkPath, skill, followSet)` replaces the lexical compare in `SyncTargetMergeWithSkills`. It reads relative link text against the link's real parent, resolved with `canon`, as `prunableLink` already does on Unix (`agent_sync.go:111-114`). It accepts exactly three forms: the logical path of this skill, the canonical source root plus the logical tail, or the fully resolved path of *this* skill. A link that merely sits somewhere under an owned root never counts as equal.

**2. Ownership: may prune remove this link?** Since #315 (`5f3e79f1`), the in-source live-link branch already requires `manifest.Managed[name]` or `force` (`sync.go:819`). So @star-nebula's suggestion has partly landed, and the #314 case of hand-made links is fixed. This proposal keeps the manifest schema, which stores names and modes only (`internal/sync/manifest.go:16-20`). It does not switch to provenance alone. A live link is pruned only when both of these hold:

- it is in `manifest.Managed`
- it resolves inside the logical source, the canonical source root, or the resolved target of an entry that is currently `followed`. `prunableLink` checks both sides the same way (`agent_sync.go:102-114`). Including the canonical source root covers relative links created while the source root itself is a link.

`--force` keeps #315's escape hatch (`sync.go:819`, `:832`), except during a unavailable-entry pause (point 4). This rule does not prove who created a link. A user who repoints a managed name by hand to another path inside an owned root can still have that link pruned. In-source links have had the same exposure since #315.

*After unfollow.* Links created by this proposal point through `<src>/_f/...`. After `_f` leaves `.skillfollow`, they still resolve inside the logical or canonical source, so they are pruned like any other orphan. A managed link that resolves to a physical external path can only come from a hand-made link or the Windows read-back fallback. Once its root is no longer followed, prune keeps it and warns: "managed link resolves outside the source after unfollow; remove it or re-run with --force". Storing per-link targets in the manifest was rejected as extra schema for this edge case.

**3. Status: how many links are synced?** `CheckStatusMerge` has no discovery, selection, or naming context, and its four callers do not pass any. So status stays an aggregate count:

- In-source links are classified exactly as today. This is a deliberate compatibility exception, and it keeps output unchanged when there is no `.skillfollow`.
- A link that resolves only through a followed root counts as linked when it is in the manifest **and** inside a currently followed root's resolved target. Otherwise it counts as local. That includes the time before the first sync. Sync then adopts correctly selected links into the manifest (`sync.go:669-675`).
- Limitation: status measures connectivity, not correct per-name wiring. Suppose a managed name is repointed by hand to a different path inside the same followed root. Status still counts it as linked. `doctor`'s `checkSyncDrift` (`cmd/skillshare/doctor.go:715`) only tests `syncedCount < expectedCount`, so it reports no drift. In-source links behave the same way today.

**4. Unavailable entries: no data loss while discovery is incomplete.** A followed entry can be `missing` while a drive is unmounted or a repo is being re-cloned. It can also turn into a rejected state, for example when its link is accidentally replaced by a regular file (`invalid-target`). Either way discovery is incomplete, and absent names cannot be attributed. Standard naming assigns target names from the skill name (`ResolveTargetSkillsForTarget`, `internal/sync/target_naming.go:56`; basename validation at `:181`), and the manifest has no source mapping. An entry is *unavailable* when it is declared and in any state other than `followed` or `not-link`: `missing`, or any rejected state, including `single-skill` before step 4. While any entry is unavailable:

- **Prune is paused on every skills target, including with `--force`.** Merge and copy prune remove nothing they cannot attribute. That covers:
  - managed names that are absent from discovery
  - broken links into the source (`sync.go:816`)
  - broken external links (`:828`)
  - the flat-name/tracked-dir heuristic branch (`:843`)
  - copy-mode orphans (`PruneOrphanCopiesWithSkills`, `copy.go:273-291`)

  `--force` does not override this. In copy mode, the managed copy may be the only remaining copy of the unavailable content. Recovery is one step either way.
- **Copies are not replaced.** In standard naming, an existing managed copy whose origin cannot be proven is not replaced or overwritten, even with `--force`. Example: target `a` was copied from `_offline/a`, `_offline` is now missing, and `other/a` is available. `ResolveTargetSkillsForTarget` only sees `other/a`, so no collision shows up. Without this rule, `SyncTargetCopyWithSkillsOptions` (`copy.go:187-197`) would overwrite `a`. Flat naming can proceed, because the logical prefix (`_offline__a`) identifies the origin. Standard-name reformat and migration paths follow the same rule.
- **Merge links may be replaced, with a warning.** Replacing a link loses no data, and pointing `a` at the only available `a` matches what the source currently contains. The warning says that the name may previously have been served by a missing followed entry and may collide when that entry returns. It must not claim to know the old origin. The existing collision handling then applies.
- **New links and copies are still created** for names that are genuinely absent from the target.
- **Reconcile keeps metadata only under the unavailable entry's logical prefix**, including the entry itself. Keys are already logical paths (`reconcile_core.go:46`). `pruneStaleEntries` (`:132-138`) still prunes everything else normally.
- **Output.** `sync` prints `prune paused`, and reports copies kept from replacement. `status` and `doctor` name the blocking entry and say "restore or fix `<path>`, or remove `<name>` from `.skillfollow[.local]`, to resume cleanup". An abandoned declaration keeps the pause in place indefinitely. This is by design: a visible, recoverable safety state is better than silent data loss. It needs no new override and no schema change.

These rules apply only when at least one entry is unavailable. Prune with no `.skillfollow`, or with every entry valid, is unchanged.

`PruneOrphanAgentLinks` and `pruneExtraOrphans` are out of scope, because `.skillfollow` covers skills only. They use `prunableLink` (`agent_sync.go:94-117`). That function resolves both sides, but it is not a provenance check, and it also removes links it can prove are broken.

**Test cases** (unit tests in `internal/sync`, extending `merge_test.go`, `status_test.go`, `relative_test.go`):

| # | Setup | Expected |
|---|---|---|
| 1 | Global, absolute link to `<src>/_f/a`, `_f` followed | 2nd sync: 0 updated; status linked |
| 2 | Project mode, relative link, `_f` followed | link text goes through `<src>/_f/a`; 2nd sync 0 updated; status linked |
| 3 | 2, plus a symlinked source root and a symlinked target parent | same as 2 (exact identity, no churn) |
| 4 | Managed link read back as fully resolved `/ext/_f/a` (simulated fallback) | status linked; sync idempotent; prune removes it when `a` is filtered out |
| 5 | Link to `/ext/_f/a`, not in manifest, before the first sync | status local; after sync, adopted if `a` is selected |
| 6 | `_f` removed, managed link through `<src>/_f/a`, with and without a symlinked source root | pruned as orphan |
| 7 | `_f` removed, managed physical link `/ext/_f/a` | kept with the after-unfollow warning; removed with `--force` |
| 8 | `_f` missing, merge and copy, flat and standard naming, with and without `--force` | nothing unattributable removed; `prune paused` reported; metadata kept under `_f/` only |
| 8b | `_f` previously followed, its link replaced by a regular file (`invalid-target`), copy mode | same as case 8: managed copies from `_f` are kept, `prune paused` names `_f` |
| 9 | `_offline` missing, `other/a` available, managed `a` from `_offline/a` | copy + standard: `a` untouched and reported; copy + flat: proceeds; merge: relinked with warning |
| 10 | Hand-made link to `/ext/a`, nothing declared | kept with external warning (default unchanged) |
| 11 | Windows junction in the source (simulated) | discovered when declared, invisible when not |
| 12 | Real Windows junction to a followed skill (`skillshare-windows-utm`) | created, owned, 2nd sync idempotent, pruned on filter change |
| 13 | Windows Developer Mode, relative symlink | same as 2 |

The junction simulation in case 11 needs a test seam. `utils.isJunction` is unexported, so `sourcewalk` tests cannot set it. Either add a small exported test hook in `utils`, or inject the link check into `sourcewalk`.

### 4. External ownership: what may change a followed tree

Everything under `<src>/<followed>/` belongs to the user, not to skillshare. Today's guards are not enough. For a followed *group*:

- `handleBatchUninstallSkills` (`internal/server/handler_uninstall.go:239-251`) refuses only tracked-repo members, so it would move a group child into trash.
- `resolveGroupSkills` (`uninstall.go:221-255`) feeds children to `MoveToTrash` (`uninstall_handlers.go:113`, `:129`).

**The boundary is the filesystem handle, not a list of call sites.** Earlier revisions put a followed-entry check in front of each write. Four review rounds kept finding writes that the list missed: dashboard content edits, remote changes, `into` creation, frontmatter edits, `--into` and config-install `MkdirAll`, and the `init` built-in fallback. Each one wrote into the external tree before any check ran. The source has hundreds of raw `os.*` write calls, and any new one would repeat the bug.

So every write to the skills source goes through an `*os.Root` opened at the source root. `os.Root` (Go 1.24; `MkdirAll`, `RemoveAll`, `WriteFile`, and `Rename` since Go 1.25; `go.mod` is `go 1.25.5`) refuses any operation whose path resolves outside the root through a link. A write below a followed entry therefore fails at the handle, whichever command, route, or fallback issues it. `internal/plugin` already uses `os.OpenRoot` this way (`files.go:106`, `rootAtomicWrite` in `pi_extensions_project.go:593`).

*Evidence.* A standalone Go 1.25.5 probe ran in the devcontainer (Linux arm64) and on Windows 11 Home ARM64 (Developer Mode off), with the full token and the basic-user token. The basic token cannot create symlinks, so only its junction rows apply. Results were the same on every platform and token:

| Operation on `src/_f/...`, where `_f` links outside `src` | Symlink | Junction |
|---|---|---|
| `WriteFile`, `MkdirAll`, `Remove`, `RemoveAll`, `Rename`, `Stat` | refused ("path escapes from parent"), external tree unchanged | refused, external tree unchanged |
| `Lstat("_f")`, `Remove("_f")` | allowed; removes only the link | allowed; removes only the link |
| Write through a **relative** link that stays inside `src` | allowed | n/a (junctions store absolute paths) |
| Write through an **absolute** link that stays inside `src` | refused | refused |
| `OpenRoot` on a source root that is itself a link, then write | allowed | allowed |

*The final component.* `os.Root` refuses paths that pass *through* a link, but it still acts on a link that is the *last* component. `Remove("_f")` deletes the link, and `Rename` over a link replaces it. Without a guard, `install --track --force` on an existing followed `_repo` would remove the link at `os.RemoveAll(destPath)` (`internal/install/install_tracked.go:72`) and clone a real directory in its place, so the external repo is silently disconnected. An atomic write (temp file plus rename) would also replace a `SKILL.md` that is a link with a plain file.

So callers do not get the raw `*os.Root`. They get a thin source-write module that wraps it and owns this rule. Before `Remove`, `RemoveAll`, an overwriting `WriteFile` or `OpenFile`, or a `Rename` onto an existing path, it runs `Lstat` through the root on every component of the path, not only the last. `os.Root` alone allows a relative link that stays inside the root, so this is what refuses `install --into alias/new` through an undeclared in-source link. The rule applies to every write, not only overwrites. If any component is a link, it refuses: "`<name>` is a followed entry; use `unfollow`" for a declared entry, and "`<path>` is a link; edit its target directly" otherwise. The only way to remove a link is `Unlink(entry)`, which unfollow alone calls. Removing a directory that *contains* links is unchanged, because removal never follows them. The check and the operation are not atomic. This is the same limitation as §5's concurrency note.

*What callers do:*

- Domain functions that write the source take the source-write module instead of building paths from the source string. That covers install, update, uninstall, collect, `new`, `init`, reconcile, sidecar migration, frontmatter edits, and the server handlers that call them.
- A move across the root's edge cannot use `Root.Rename`, so the in-source side is checked through the root first. For example, trash moves out of the source, and staged installs and updates rename a temp directory in (`install_apply.go:452`, `install_update.go:150`, `:263`). `Root.Stat` of the in-source parent fails on any crossing link, as the table shows.
- Atomic writes use the module's `WriteFileAtomic`, which creates the temp file and renames it inside the root, following `rootAtomicWrite` (`internal/plugin/pi_extensions_project.go:593`). So the first filesystem change is already constrained. Today's `writeFileAtomic` starts with `os.CreateTemp` in the target directory (`internal/server/handler_skill_content.go:347`), which would otherwise create a temp file in the external tree before the rename is refused.
- Fallbacks write through the same handle, so a refused install cannot fall through to a raw write (for example `installBuiltinSkill`, `cmd/skillshare/init_apply.go:185-198`).
- An escape error under a declared entry is reported as "inside followed entry `<name>`; hide it with the root `.skillignore`". Any other escape is reported as "path crosses a link in the skills source". Sidecar migration (`metadata_migrate.go:135`) skips instead of failing, and `doctor` reports the unmigrated sidecars.
- The step 2 ratchet also counts raw `os` write calls (`Create`, `CreateTemp`, `MkdirTemp`, `OpenFile`, `WriteFile`, `Mkdir`, `MkdirAll`, `Remove`, `RemoveAll`, `Rename`, `Symlink`, `Link`, `Truncate`, `Chmod`) in every non-test package under `cmd/` and `internal/`. That includes the shared helpers that write source paths today: `internal/utils/frontmatter_write.go`, `internal/skillignore/write.go`, and `MoveToTrash` in `internal/trash/trash.go`. Each remaining call carries a reason that says it does not write the skills source, such as `target`, `backup`, `trash`, `config`, `agents`, or `extras`. A new unclassified call fails the test. This has the same syntactic limitation as the walk ratchet.

*Behavior change.* Discovered paths never cross a link below the source root today, because links are not followed. So ordinary writes are unaffected. Two things change:

- An explicit path through an **undeclared** link (`install --into <link>/x`, a dashboard `into`) is refused instead of writing into the link's target.
- An absolute link that points inside the source, and every junction, can no longer be written through.
- Overwriting or replacing a link that is the last path component is refused by the module, not only writing through it. Examples are a `SKILL.md` that links to a shared file, or `install --track --force` onto an existing followed or undeclared `_repo` link. Today both silently replace the link. Uninstalling a whole skill directory that contains such a link still works, because removal does not follow it.

A `not-link` entry is a real directory, so it keeps today's ownership rules: naming it in a shared `.skillfollow` never blocks ordinary work below it on a machine where it is not a link.

**Allowed:**

- **Unlink and unfollow of the root entry**, through the module's `Unlink(entry)`, which removes the link itself. The link is never moved to trash; a link in trash would also be invisible, because `trash.List` shows only directories (`internal/trash/trash.go:215`, `:237`). The target is never touched. The name is removed from every declaration file that contains it, because declarations are a union, and the command lists each file it edited. Comments and other entries are preserved. If any write fails, the command reports failure, never a partial success. A `not-link` real directory is never removed by unfollow.
- **Pull of a followed tracked repo**, under the git rules in §5.

**Outside the handle.** Git runs as a subprocess, so the root cannot see its writes. These seams keep an explicit followed-entry check:

| Operation | Seam |
|---|---|
| Pull, audit rollback, and `--force` on a followed repo | §5's followed-update policy |
| Dashboard source change on a followed repo | `handlePatchSkillSource` (`internal/server/handler_skill_content.go:98`), which calls `git.SetRemoteURL` at `:162` |
| Staging | §5's staging guard |
| Source-repo pull, reset, and checkout: `ss pull` (`pullFromRemote`, `cmd/skillshare/pull.go:54`), the dashboard pull (`handlePull`, `internal/server/handler_git.go:677`, via `PullWithResolution` at `:745`), the `init` resets to a remote branch (`resetToRemoteBranch`, `cmd/skillshare/init_remote.go:126`; `cmd/skillshare/init.go:775`), and the dashboard branch switch (`handleGitCheckout`, `handler_git.go:274`, via `git.Checkout` at `:333`). For checkout, the incoming revision is the branch being checked out. | Fetch first. Then refuse while any declared entry in the staging tree is indexed, or while the incoming revision touches a path that has a link component in the working tree, declared or not. Each path from `git diff --name-only -z HEAD <incoming>`, parsed on NUL so that quoted names are read exactly, is checked with `Lstat` on its existing ancestors. Ignoring the link is not enough: Git treats ignored files as expendable, so a remote commit that adds that path would replace the link. Checking undeclared links too keeps this seam consistent with the final-component rule above, and it closes the same exposure that undeclared links have today. The message names the path and the commit, and says to run `git rm --cached` or to fix the remote. |

The dashboard discard (`handleGitDiscard`, `internal/server/handler_git_discard.go:13`, via `git.DiscardChanges`) needs no new seam. `git clean -fd` runs without `-x`, so an ignored link stays, and `git restore --source=HEAD` rewrites only indexed paths, which the staging guard keeps the link out of. An undeclared link that is neither indexed nor ignored is untracked, so discard removes it today, and that behavior is unchanged. The Test Plan pins the followed case.

`enable`/`disable` (`cmd/skillshare/enable.go`) and `PUT /api/skillignore` (`internal/server/handler_skillignore.go:80-89`) write only the root ignore files, which the handle allows.

Reconcile marks names under a followed entry as live. It never creates or updates their metadata: no `store.Set` (`reconcile_core.go:97-105`) and no `RefreshTrackedRootSkillHashes`. Following never creates a `tracked: true` entry.

### 5. Git operations

*Maintainer requirement (c).*

**The nested-repo gap.** `NestedRepos` (`internal/git/scope.go:396-418`) never sees a followed repo, and it should stay that way. A followed repo is not nested in the git root, and git does not traverse links. If `NestedRepos` followed it, the existing guard would advise disabling the *external* `.git`. It stays a raw walk with the `git-root` allowance.

The real risk is the link entry itself:

- `install.UpdateGitIgnore` writes tracked-repo patterns with a trailing slash (`internal/install/gitignore.go:26-28`, called from `install_tracked.go:144`).
- A pattern ending in `/` matches only directories, and git stores a symlink as a file (mode 120000).
- Three code paths stage everything:
  - `stageAndCommit` (`cmd/skillshare/push.go:91`, used by `push` and `commit`) runs `git add -A`.
  - The server's `git.StageAll` (`internal/git/info.go:599-603`, from `handler_git.go:418` and `:515`) also runs `git add -A`.
  - `commitSourceFiles` (`cmd/skillshare/init.go:567-573`, used by `init --remote`) runs `git add .`.

Any of these would commit `_dev-skills` as a link holding a machine-specific absolute path. That is the portability problem the maintainer raised in #207.

**Proposal:**

- **The ignore line** is `/_dev-skills`: anchored, with no trailing slash, written into the managed block of the skills source `.gitignore`. Reuse `install.UpdateGitIgnoreFiles` (`gitignore.go:62-70`), which writes no-slash entries. Removal reuses `RemoveFromGitIgnore` (`:124`), which already matches both forms.
- **Who writes it.** Rollout step 3 writes nothing automatically. Discovery, `list`, `status`, `doctor`, and every `--dry-run` stay read-only. Instead:
  - The setup docs show the full set together: the declaration, the anchored link ignore, and the recommended `/.skillfollow.local` ignore.
  - `doctor` reports `not-ignored`, with the exact file and line, before the user reaches a commit.
  - Step 4's `follow`/`unfollow` write or remove the lines together with the declaration.
- **The staging guard follows Git reachability, not path containment.** A source that is a link out of the git root is not staged. An alias source that points *into* the root is staged. The guard, `FollowedLinksStaged(stagingDir, followSet)`:
  - takes the real staging directory, the canonical form of `EffectiveGitRoot` (`internal/config/config.go:425`);
  - for each declared link, computes its physical location as the canonical link parent plus `/name`, never resolving the final component. "Canonical" means the junction-aware canonicalizer from §1, since `EvalSymlinks` leaves junctions unresolved on Windows (§6);
  - treats the link as staged only if that location is under the staging directory **and** no path component between the two is itself a link, because Git does not descend through one;
  - runs at every staging seam: `stageAndCommit`, both server staging paths (next to `rootScopeGuard`, `handler_git.go:563`), and `commitSourceFiles`.

  The exact algorithm is left to implementation. The required test layouts are listed in the Test Plan. For each staged link:
  - *Already indexed* (`git ls-files --error-unmatch` succeeds): refuse, and tell the user to run `git rm --cached -- <path>` and add the ignore line themselves. skillshare never untracks anything automatically.
  - *Not indexed and not ignored* (`git check-ignore` fails): refuse, and print the exact ignore line.
  - The refusal has the same shape as `errNestedRepos` (`cmd/skillshare/gitroot.go:159`), and a dry run reports it.
- **Committed and local files.** `.skillfollow` is committed. `.skillfollow.local` is treated like `.skillignore.local`: not auto-ignored, but `doctor` warns if it is tracked or not ignored.
- **Root scope (`git_root: root`).** `ScopeDir("root")` is `BaseDir()`, which contains the skills source, so the source's nested `.gitignore` applies. Two things keep a followed external repo out of the root repo: the guard keeps the link itself out, and the `inside-git-root` rule in §1 rejects targets inside the staging tree. The promise covers skillshare's effective staging tree only, not other repositories a user stages by hand.

**Tracked-repo update on a followed repo.** Once `getTrackedReposImpl` uses the walker, the followed repo appears in `status`, `check`, `update <name>`, `update --all`, and the server update and check handlers. `install.IsGitRepo` uses `os.Stat`, which follows the link. The server's `resolveTrackedRepo` (`internal/server/handler_skills.go:547-564`) already stats `_dev-skills` directly, so the server update route can resolve a followed repo today, while CLI `update dev-skills` reports "not found". The walker makes the two consistent.

A followed repo is the user's working copy. The current update paths are not safe for it:

- `pullWithResolution` (`internal/git/info.go:254-270`) uses `--ff`, and on failure runs conflict resolution, `merge --abort`, and residue cleanup. `resolvePullConflicts` (`internal/git/pull_conflicts.go:127`) can complete a merge commit.
- `updateTrackedRepoQuick` (`cmd/skillshare/update_handlers.go:333-335`) turns pull errors into `false, nil, nil`. `executeBatchUpdate` (`cmd/skillshare/update_batch.go:123-140`) then only counts the repo as skipped.
- The server's `updateTrackedRepo` (`internal/server/handler_update.go:295`) ignores `IsDirty` errors, and after a failure suggests "try force update" (`:324-327`).

**One shared followed-update policy** covers the CLI (single, batch, quick, project), the server (single, update-all, SSE through `handler_update_stream.go:127`), and `install --update` (`install.updateTrackedRepo`, `install_tracked.go:157-182`). Its package placement must respect the existing import direction: `git` already imports `install` (`pull_conflicts.go`). Under this policy:

1. The followed check runs before any mutation, and before dry-run reporting. `--force` returns an explicit refusal for that item on every route. The `git.Restore` calls are never reached (CLI `update_handlers.go:143`, `:317`, `:329`; server `handler_update.go:304`, `:317`).
2. The dirty check fails closed: an `IsDirty` error is a refusal, not permission to continue.
3. The pull is a dedicated fast-forward-only pull (`git pull --ff-only`, or fetch plus `merge --ff-only`), with no conflict resolution, merge abort, or residue cleanup, because there is no merge to undo. If histories diverge, the item is refused with "resolve in `<resolved path>`". skillshare never creates commits in the user's repo.
4. Every refusal and error is reported for its item through batch, project, server, and SSE results. Nothing is silently counted as skipped. Independent items continue.
5. Followed repos never get "try force update" advice.
6. Installed (non-followed) tracked repos keep today's behavior exactly.

**Audit rollback is kept and documented.** When the audit fails after a pull, three paths run `git reset --hard <beforeHash>`, even without `--force`:

- `auditGateAfterPull` (`update_handlers.go:56`, `:96`, `:107`)
- the server's `auditGateTrackedRepo` (`handler_update.go:375`, `:392`)
- `install --update`'s `auditTrackedRepoUpdate` (`internal/install/install_audit.go:277`)

The tree was clean before the pull, so in the normal case this undoes only what skillshare pulled. The docs must say plainly that refusing `--force` does not mean skillshare never hard-resets a followed repo.

**Known limitation: concurrency.** The clean-tree check and the `FollowSet` are snapshots, not locks. If an editor or another git process changes the working copy during fetch, pull, or audit, or the link is re-pointed mid-run, a rollback can discard those changes. The docs say not to edit the repo while `update` runs on it. The tests pin the intended failure messages for scan errors and rollback failures.

**Audit gate scope.** `update` audits each new revision, which requires the repo-root fix in §2. Content that changes because the user pulled outside skillshare is not audited, the same as editing a skill in the source; `audit` and `check` make it visible. So the #207 concern about bypassing the audit gate applies only to changes made outside `update`, and this proposal treats those as the user's own edits.

### 6. Windows

- Detect declared entries with `utils.IsLinkMode(path, info.Mode())`. Never use `ModeSymlink` alone, and never `info.IsDir()`. A junction reports `ModeIrregular`, not a directory (`internal/utils/link.go`).
- The wrapper reads the resolved directory itself and maps every path back to the logical `<source>/<entry>/...` form before calling `fn`. This is where the one-hop rule is enforced. It is also why `discover_walk.go`'s `filepath.Rel(walkRoot, path)` must use the logical path for followed subtrees, not the resolved one.
- Canonical comparisons use `utils.PathsEqual` and `PathHasPrefix`, which are case-insensitive on Windows (`internal/utils/path.go`). The name parser rejects volume and UNC forms on every platform.
- **Windows evidence (resolves the former open question on junction read-back).** A Go probe was built in the devcontainer from `475b1769` plus a probe `main` package that imports `internal/utils`, then cross-compiled for `windows/arm64` with Go 1.25.5. It ran on Windows 11 Home ARM64 with Developer Mode off, as the desktop user with the full token and again with the basic-user token (`runas /trustlevel:0x20000`). The probe created a followed junction `src\_f -> ext` and a target junction `tgt\a -> src\_f\a`. Both tokens gave the same results:
  - `src\_f`: `Lstat` reports `ModeIrregular` (not `ModeSymlink`, not a directory), and `utils.IsLinkMode` is true. `os.Readlink` returns `ext`, and `os.ReadDir` through it lists the content.
  - `tgt\a`: `os.Readlink` and `utils.ResolveLinkTarget` both return the **logical** `src\_f\a`. `PathHasPrefix(src+sep)` and `PathsEqual(src\_f\a)` are true, and `os.ReadDir` lists `SKILL.md`.
  - `filepath.EvalSymlinks` does **not** resolve junctions: on both `src\_f` and `tgt\a` it returns the input path unchanged, with no error. With the full token, which can create directory symlinks, `EvalSymlinks` on a symlink whose target crosses the `_f` junction failed with "The system cannot find the path specified".
  - `filepath.Walk(src\_f)` makes one callback and does not descend. `filepath.Walk(src)` makes two callbacks and never finds `_f\a\SKILL.md`. This confirms star-nebula's report at the Go level.
  - With the basic token, `os.Symlink` fails with "A required privilege is not held by the client", and `mklink /J` succeeds. Junctions are the only link a basic user can create here.

  Consequences for this design:
  - `utils.ResolveSymlink` and `evalOrClean` are no-ops on junctions, so they cannot be used to resolve followed entries, run cycle and overlap checks, or find the staging tree on Windows. The walker reads the entry's target with `utils.ResolveLinkTarget`. Canonical comparisons use a junction-aware canonicalizer that resolves link components one at a time.
  - The relative-link branch of `createLink` (Developer Mode only) canonicalizes with `evalOrClean` today, so it has the same exposure on Windows. §3 switches it to `canon`. It needs a test under Developer Mode, which this probe did not cover.
- **`os.Root` evidence.** A second probe verified that `os.Root` refuses writes through junctions and symlinks that leave the root, on Windows and Linux, with both tokens. §4 has the table.
- Real-Windows coverage goes in an `ai_docs/tests/` runbook, run with `skillshare-windows-utm`. It runs two configurations: a basic-user token (junctions only) and Developer Mode (relative symlinks). It covers:
  - discovery
  - the read-back fallback
  - a missing target
  - a linked source root and a linked target parent
  - sync idempotency
  - status
  - prune after a filter change
  - unfollow and uninstall

### 7. Common base for #253 and #206

- **#206 (live link to an app-bundled skill).** Closed by #334: `check` now detects drift at local install sources, and `update` re-copies through the audit. In #207 the maintainer chose that over `--link`, to keep the audit gate, Windows parity, and backup and git behavior. If `--link` comes back, it could sit on this base:
  - `install --link <dir>` creates the link (a junction on Windows).
  - It adds the name to `.skillfollow.local`, since that file is machine-specific like the target path.
  - It adds the anchored `.gitignore` line.

  This would need followed single skills (rollout step 4). This proposal does not reopen #206.
- **#253 (`include_sources` for `~/.agents/skills`, `~/.cursor/skills`).** A followed entry `_agents -> ~/.agents/skills` would expose those skills as `_agents/<skill>`. If `~/.agents/skills` is an active configured target, §1's overlap rule rejects the entry, because sync would otherwise write into the directory it reads. It is the universal target in `internal/config/targets.yaml`. A path that is not an active target can be followed.

  `.skillfollow` is therefore only a partial answer. #253 could reuse the walker for discovery, but a read-only input gives no authority to prune. #253 would need its own ownership rule and its own answers on name collisions and read-only semantics, and its config surface stays separate.

### 8. CLI and dashboard surface

- **Rollout step 3: no new command.** The file is hand-edited, the way `.skillignore` was before `enable`/`disable` existed.
- **Step 4:** `skillshare follow <name>` and `unfollow <name>`, with `-p`/`-g` and `--local`. They write or remove the declaration and the anchored `.gitignore` line together. `follow --local` also writes the `/.skillfollow.local` ignore. `unfollow --local` removes the name only from `.skillfollow.local`. It reports that the entry remains followed if `.skillfollow` still declares it, and it never claims a full unfollow. A later `follow <name> --to <dir>` could also create the link (a junction on Windows). This mirrors `enable`/`disable` (`cmd/skillshare/enable.go`).
- **`status`.** Add a line next to `printSkillignoreLine` (`cmd/skillshare/status_render.go`), for example `.skillfollow (.local active): 2 entries, 1 skipped, prune paused`. The JSON gets `source.skillfollow` next to `source.skillignore` (`buildSkillignoreJSON`, `cmd/skillshare/status.go:216`).
- **`doctor`.** Add `checkSkillfollow` after `checkSkillignore` (`cmd/skillshare/doctor.go:282`). It reports:
  - each entry's state from §1
  - `not-ignored`, with the exact line
  - an indexed link or a tracked `.skillfollow.local`
  - unmigrated sidecars
  - any prune pause, with recovery steps

  It also prints an info line for each *undeclared* first-level link in the source. Today those links are invisible without notice (star-nebula's point), so this line is useful even without following.
- **`list`.** Followed tracked repos show as tracked. The suffix could add `→ <resolved>`, as #206 suggested. Branch display already falls back to live git when metadata has none (`cmd/skillshare/list.go:320-331`).
- **Dashboard.** `.skillignore` has `GET/PUT /api/skillignore` (`internal/server/server.go:627-633`, `handler_skillignore.go`) and a tab in `ui/src/pages/ConfigPage.tsx`. A `.skillfollow` tab could reuse that pattern in step 4. Editing a declaration file there is a declaration edit, not a root uninstall. Doctor and status data already reach the dashboard through existing endpoints.
- **Docs to update** (English first, then translations through the normal flow):
  - `website/docs/reference/filtering.md`: a new section next to `.skillignore`, with the full setup set from §5
  - `reference/appendix/file-structure.md`
  - `understand/source-and-targets.md`: the symlinked-source tip
  - `reference/commands/sync.md#dotfiles-manager-compatibility`, plus the prune pause
  - `reference/commands/{status,doctor,update,uninstall,collect,install,push,commit,list}.md`, covering the ff-only, `--force`, audit-rollback, and concurrency rules
  - `reference/targets/configuration.md#git-root`
  - `how-to/sharing/cross-machine-sync.md`
  - `troubleshooting/faq.md`: the dotfiles question
  - the built-in skill, `skills/skillshare/references/`

  `reference/commands/backup.md` stays as is. Backup copies only target content and skips links (`internal/backup/backup.go:30`, `:56`, `:308`, `:330`).

## Alternatives Considered

- **Follow every link in the source.** This breaks the deliberate boundary and changes behavior for every existing user who has a stray link. Rejected. Opt-in keeps the default unchanged.
- **Target paths in config (`include_sources`, #253).** Absolute paths differ per machine, so the config cannot be shared. Skills would also have no logical location in the source, which every link and manifest path assumes.
- **Keep the repo inside the source with a link outside it.** This works today, and the issue explains why it fails the user.
- **Wrapper swaps without a ratchet or a behavior fixture.** This is the reference implementation's shape. It is the smallest diff, but it does not meet the future-proofing requirement.
- **A `file:function` allowlist.** A new walk inside an allowed function would pass silently. The call-level ratchet catches it.
- **`manifest.Managed` as the only ownership test.** Names alone cannot tell skillshare's link from a user's link with the same name.
- **Store each managed link's target, or each copy's origin, in the manifest.** This would allow precise cleanup after unfollow and precise replacement while an entry is missing. It is extra schema for edge cases that the warning path and the conservative pause already cover. Rejected for now.
- **A target-name-to-skill mapping in status.** This would make status exact per name, but `CheckStatusMerge` and its callers would need new plumbing. Status stays an aggregate count, and the limitation is documented.
- **Let `--force` override the unavailable-entry pause.** In copy mode that can destroy the only remaining copy. Restoring the path, or removing the declaration, is a one-step recovery.
- **A followed-entry check at each write seam.** This was the earlier design of §4. It is shallow: the check is one predicate, and the real work, remembering to call it before the first write, is spread across every caller. Four review rounds found eleven seams that the list missed, and nothing stops a new write from missing it. The `os.Root` handle makes the boundary a property of the filesystem access itself.
- **Allow mutations of descendants because the entry is declared.** Declaring an entry grants discovery read access. It does not give skillshare ownership of the user's tree.
- **Block commits in every git scope, or decide by path containment.** A skills declaration would then block unrelated repos, and symlinked or alias roots would be misjudged. Git reachability is the correct test.
- **Follow external repos in `NestedRepos`.** It would report what git does not stage, and advise disabling the external `.git`.
- **Commit the link instead of ignoring it.** Absolute targets break on other machines (#207). Relative targets outside the repo are no better.

## Scope

- [ ] Small (1-3 files, < 200 lines)
- [ ] Medium (3-10 files, 200-500 lines)
- [x] Large (10+ files, 500+ lines)

The reference implementation was about 26 insertions and 17 deletions across 11 upstream files, plus a new package. This proposal is substantially larger, because it adds:

- the ratchet and the behavior matrix
- exact link identity
- the repo-root audit fix
- the unavailable-entry pause and the copy-preservation rule
- the external mutation boundary: moving source writes onto an `os.Root` handle across CLI and server
- the shared followed-update policy
- the Git-reachability staging guard
- doctor and status output

Any line estimate is rough, not a commitment.

## Test Plan

- **Unit tests:**
  - The parser: comments, whitespace, duplicates, `.local` merge, every rejected name form on every platform, and escaping in ignore lines.
  - Classification: every state, including precedence when more than one condition holds. That covers `invalid-target`, every safety check before `single-skill` (including `a -> <active-target>/a`), `single-skill` before tracked-repo detection, `inside-git-root`, overlaps in both directions against targets and between entries, and canonicalization through a missing ancestor.
  - The walker: one hop only, logical paths with a symlinked source root, `SkipDir`, a mid-walk unreadable directory marking its entry `missing`, and the junction seam.
  - The ratchet, including a new call inside an allowed function and stale allowances.
  - The §3 table, with flat and standard naming.
- **Integration** (`tests/integration/`, run in the devcontainer):
  - The behavior matrix from §2.3.
  - Default unchanged: with no `.skillfollow`, output is identical.
  - The §4 boundary, through both the CLI and the server, including for missing and rejected entries. The fixture exercises every seam that review found as a regression case: uninstall (CLI and server), collect, install overwrite, `--into` (CLI and both server endpoints), config install, tracked install, update, dashboard content edit, frontmatter edits (server and list TUI), dashboard create with `into`, and the `init` built-in fallback. Each refused write leaves the external tree byte-for-byte unchanged, with no new directory. An undeclared link given as an explicit `--into` path is refused too. A dashboard edit of a skill whose `SKILL.md` is a link is refused with a clear message, and uninstall of that skill still removes it. `install --track --force` onto an existing followed `_repo` is refused, and the link and the external repo stay unchanged. A `not-link` entry keeps ordinary uninstall, install, and update.
  - Unfollow and uninstall of a followed entry remove only the link and the declarations. Partial-write failure is reported as failure.
  - Audit on a followed repo: a pulled commit adding a malicious child skill blocks and rolls back to `beforeHash`, through the CLI, the server, and `install --update`. A zero-file scan of a non-empty root is a scan error.
  - Update: a mixed `update --all` of ordinary and followed repos, covering dirty trees, `--force`, divergence, and an `IsDirty` error. The ordinary repo still updates, and each followed refusal is reported per item in batch, project, server, and SSE output. Rollback-failure and concurrency messages are checked too.
  - Source-repo `pull`, dashboard pull, dashboard checkout, and `init` reset: refused with an indexed declared link, and refused when the incoming revision adds or changes a path through a declared or undeclared link that is only ignored. The link is unchanged after each refusal. Dashboard discard: an ignored followed link survives `git clean -fd`, and the target is unchanged.
  - The staging guard at `commit`, `push`, and `init --remote`: indexed versus unignored links; a source linked out of the git root (not guarded); an alias source pointing into the root (guarded); agents, extras, and custom-root scopes; under both `git_root: skills` and `git_root: root`.
  - A missing or rejected entry with `--force`: the prune pause holds, and copy replacement is refused in standard naming.
- **Windows:** the `skillshare-windows-utm` runbook for §3 cases 11–13, §6, and the §4 boundary through a junction with the basic token.
- `make check` in the devcontainer.

## Phased Rollout

1. **Visibility first, no behavior change:** the `doctor` info line for undeclared first-level links in the source. It can ship on its own.
2. **Walker, write handle, ratchet, and behavior matrix.** Move the existing walks onto `sourcewalk` with following disabled, and move source writes onto the `os.Root` handle. Existing tests must pass unchanged. The one intended behavior change is §4's refusal of explicit paths through links; any existing test that relies on writing through a link is changed in the same commit, with that reason.
3. **Enable `.skillfollow` for groups and tracked repos.** This step includes:
   - parsing and classification
   - exact link identity and the ownership and status rules
   - the unavailable-entry pause and copy preservation
   - the error messages for the mutation boundary, and the git seams outside the handle
   - the repo-root audit fix
   - the shared followed-update policy
   - the staging guard
   - status and doctor output

   Docs ship in the same release, marked experimental.
4. **`follow`/`unfollow`, the dashboard tab, and followed single skills** (with `WalkSkill` and the tracked-root hash refresh). Revisit `install --link` (#206) and `include_sources` (#253) on this base if they are still wanted.

## Decisions

These were open questions in earlier revisions. The maintainer can still reopen any of them in review.

1. **`.skillfollow.local` is not auto-ignored.** Docs show the ignore line, `doctor` warns when the file is not ignored, and step 4's `follow --local` writes the line. Step 3 writes nothing. `.skillignore.local` keeps its current behavior.
2. **Agents and extras are out of scope.** Their pruning (`prunableLink`) has different semantics, so an `.agentfollow` would need its own proposal and a concrete user request.
3. **No `followed` metadata kind.** The one use case so far, branch display, already falls back to live git (`cmd/skillshare/list.go:320-331`). A schema change waits for a concrete need.
4. **Broken external links outside a unavailable-entry pause are a separate follow-up.** The branch at `sync.go:828` removes these without a provenance check. Applying the #314 rule there is a separate issue. While an entry is missing, prune is already paused (§3), which covers the case this proposal creates.
5. **Package placement is settled during implementation.** The starting point is `internal/sourcewalk` with plain inputs. The home of the followed-update policy is chosen in step 3, because `git` already imports `install`.

## Open Questions

1. **Junction source roots (pre-existing, outside this proposal).** The probe shows that `filepath.EvalSymlinks` returns a junction unchanged and that `filepath.Walk` does not descend into one. `discoverSourceSkillsInternal` walks `utils.ResolveSymlink(sourcePath)`, so a skills source root that is itself a junction is probably discovered as empty on Windows. The dotfiles-manager docs promise that a symlinked source works. This was shown at the Go level only. It should be reproduced with `ss.exe` and then filed as a separate bug.
