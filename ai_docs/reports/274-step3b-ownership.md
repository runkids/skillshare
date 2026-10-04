# Skillfollow step 3b implementation handoff

## Result and scope

Implemented proposal §3 (link identity, link ownership, the status count, and the unavailable-entry safety rules) and the reconcile paragraph of §4 on top of the 3a `FollowSet`. `internal/sourcewalk` semantics are unchanged and no `FollowSet` query was added. Git operations, the staging guard, the followed-update policy, repo-root audits, `handlePatchSkillSource`, and server handlers were not touched. No push or pull request was performed.

## Commits

- `936f4db8` identity: skill links keep the followed entry in their text; `sameSkillLink` replaces the lexical compare in merge sync.
- `0ff85840` ownership: prune owns managed links into followed targets; after-unfollow warning.
- `ab1724af` pause: prune pauses while an entry is unavailable; unprovable standard-name copies are kept.
- `38e79324` status: followed links count as linked; CLI status and doctor pass the set.
- `2f24cb83` reconcile: followed names are live without metadata writes; metadata under unavailable prefixes is kept.
- `f5b9674d` output: CLI sync passes the set; `prune paused` in sync, status, and doctor.
- `da763482` case 11: simulated junction discovered only when declared.

## Added API

```go
// internal/sync
type MergeOptions struct{ Follow *sourcewalk.FollowSet }
func SyncTargetMergeWithSkillsOptions(name string, target config.TargetConfig, allSkills []DiscoveredSkill, sourcePath string, dryRun, force bool, projectRoot string, opts MergeOptions) (*MergeResult, error)
type StatusOptions struct{ Follow *sourcewalk.FollowSet }
func CheckStatusMergeWithOptions(targetPath, sourcePath string, opts StatusOptions) (TargetStatus, int, int)
func PruneOrphanCopiesWithOptions(opts PruneOptions) (*PruneResult, error)
// New fields: PruneOptions.Follow, CopyOptions.Follow, SkillRunOptions.Follow,
// PruneResult.Paused, CopyResult.Kept, SkillTargetResult.PrunePaused and .Kept.

// internal/config
type ReconcileOptions struct{ Follow *sourcewalk.FollowSet }
func ReconcileGlobalSkillsWithOptions(cfg *Config, store *install.MetadataStore, opts ReconcileOptions) error
func ReconcileProjectSkillsWithOptions(projectRoot string, projectCfg *ProjectConfig, store *install.MetadataStore, sourcePath string, opts ReconcileOptions) error
```

The existing functions are nil-follow wrappers. Unexported: `createSkillLink`, `reformatSkillLink`, `relativeLinkText` (skills canonicalize only the source root and the link parent; agents and extras keep `evalOrClean`), `sameSkillLink`, and `followScope` (`ownsLink`, `inFollowedTarget`, `paused`).

## Behavior

- Identity: relative skill links run from `Canonicalize(linkDir)` to `Canonicalize(sourceRoot)` plus the logical tail. `CreateSymlink` and symlink-mode reformat use the skills path with an empty tail. `sameSkillLink` reads relative text against the link's canonical parent and accepts the logical path, the canonical root plus tail, or the fully resolved path.
- Ownership: with a follow set, a live orphan link is owned when it resolves under the logical source, the canonical source root, or a followed entry's resolved target; removal still needs the manifest or `--force`. A live managed link outside those roots is kept with "managed link resolves outside the source after unfollow; remove it or re-run with --force". Without a set, prune is unchanged.
- Status: in-source links count exactly as before (with a linked source root, an in-source relative link still counts as local, as today). A link counts as linked through a followed root only when it is in the manifest.
- Pause: merge and copy prune return early with `Paused` on every target, including with `--force`. Copy sync in standard naming keeps an existing managed copy that would be overwritten or replaced (`Kept`); flat naming proceeds. A merge link replaced during the pause gets a warning that does not claim the old origin. New links and copies are still created.
- Reconcile: names under a followed entry with metadata, or git repos, are marked live and skipped; no `store.Set`, `RefreshTrackedRootSkillHashes`, or `onFound`. `pruneStaleEntries` keeps names under an unavailable entry's prefix.
- Output: sync prints `<target>: prune paused; unavailable .skillfollow entry: <name> (<state>)` and the kept copies; JSON details gain `prune_paused` and `kept` (omitempty). Status text and `source.skillfollow.prune_paused`, and the doctor check `skillfollow_prune`, say "prune paused: <name> is <state>; restore or fix <path>, or remove <name> from .skillfollow[.local], to resume cleanup".

## Tests

- `internal/sync/skillfollow_test.go`: cases 1 to 4 with second-sync idempotency (global absolute, project relative, linked source root plus linked target parent, fully resolved managed link), sibling rejection, link text; ownership cases 4, 6 (with and without a linked source root), 7, 10, plus unmanaged links into a followed target; pause cases 8 (merge/copy × flat/standard × force), 8b, 9 (standard copy kept, flat copy proceeds, merge relinked with warning), resume; status cases 1 to 5 and the nil-policy count.
- `internal/config/reconcile_follow_test.go`: followed names live without writes or tracked entries; metadata kept only under the unavailable prefix; nil policy unchanged.
- `internal/sourcewalk/junction_walk_test.go`: case 11.
- `tests/integration/skillfollow_sync_test.go`: global and project fixtures with a followed group and a missing entry; sync links the group, prints `prune paused`, keeps a broken link into the missing entry; status and doctor JSON name the entry; after restoring it, sync prunes again.
- No existing test changed. Ratchet rows were renamed for moved calls only.

## Verification

`docker exec skillshare-274-step3b bash -c 'make check'` passed in a throwaway devcontainer for this worktree (docs-check, formatting, vet, both ratchets, unit and integration tests). Tail:

```text
--- PASS: TestXDG_StatusWorksWithXDGPath (0.04s)
PASS
ok  	skillshare/tests/integration	69.552s

✓ All tests passed!
```

Not run: the Windows build and runtime (cases 12 and 13).

## Remaining

- Cases 12 and 13 need the Windows runbook (`skillshare-windows-utm`).
- Reconcile callers still use the nil-follow wrappers: `cmd/skillshare/install.go`, `install_context.go`, `project_skills.go`, `search.go`, `search_batch.go`, and `internal/server/handler_install.go`. Wiring them changes which metadata survives an install, so it was left for a decision.
- `CheckStatusMerge` callers outside status and doctor (`target.go`, `target_project.go`, `target_list_tui.go`, `internal/server/handler_targets.go`) and the server sync path still use nil-follow wrappers.
- The discovering wrappers `SyncTargetMerge`, `PruneOrphanLinks`, and `PruneOrphanCopies` have only test callers and remain nil-follow; `SyncTargetCopyWithOptions` follows `CopyOptions.Follow`.
