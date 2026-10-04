# Skillfollow step 3a implementation handoff

## Result and scope

Implemented parsing, ordered classification, junction-aware canonicalization, the operation-scoped `FollowSet`, one-hop logical traversal, and global/project CLI list, status, doctor, and sync discovery integration.

The coordinator approved compatible options-bearing entry points because `config` imports `install`: loading config inside `install.GetTrackedRepos` would introduce an import cycle. The existing public discovery and tracked-repository functions intentionally retain nil-follow behavior. The coordinator approved connecting list/status/doctor/sync first and leaving the other callers listed below for a subsequent slice.

Base: `70cb28767` from `origin/main`, obtained after the coordinator authorized resetting this previously clean worktree from its stale local-main base. No push or pull request was performed. The proposal, wiki/history, and internal/sourcefs were not modified.

## Implementation commits

- `5402316f`: portable parser and parser tests.
- `872ca547`: classification, canonicalization, FollowSet queries, and classification tests.
- `e12d8f7b`: follow-aware Walk, WalkDir, ReadDir and traversal tests.
- `c1581aec`: explicit policy plumbing and CLI caller integration.
- `0b17779c`: bind declarations to the canonical source root when nested consumers reuse a set.
- `7fc17990`: status/doctor output and global/project integration fixture.

## Exact public API

```go
type State string
const (
    Missing State = "missing"
    NotLink State = "not-link"
    InvalidTarget State = "invalid-target"
    Cycle State = "cycle"
    TargetOverlap State = "target-overlap"
    InsideGitRoot State = "inside-git-root"
    EntryOverlap State = "entry-overlap"
    SingleSkill State = "single-skill"
    Followed State = "followed"
    UndeclaredLink State = "undeclared-link"
)
type Entry struct {
    Name string
    State State
    ResolvedTarget string
    Reason string
}
type FollowOptions struct {
    TargetPaths []string
    GitRoot string
}
func Follow(root string, opts FollowOptions) FollowSet
func Canonicalize(path string) (string, error)

func (FollowSet) Entries() []Entry
func (FollowSet) ParsedEntries() []string
func (FollowSet) Warnings() []string
func (FollowSet) DeclarationWarnings() []string
func (FollowSet) Active() bool
func (FollowSet) HasLocal() bool
func (FollowSet) Owns(resolvedPath string) bool
func (FollowSet) InFollowed(logicalRel string) (Entry, bool)
func (FollowSet) Unavailable() []Entry
func (FollowSet) Followed() []Entry
func (FollowSet) Err() error

type Options struct { Follow *FollowSet }
func Walk(root string, opts Options, fn filepath.WalkFunc) error
func WalkDir(root string, opts Options, fn fs.WalkDirFunc) error
func ReadDir(root string, opts Options) ([]os.DirEntry, error)
```

- `Entries` returns declaration order followed by undeclared first-level links in filename order; returned slices are copies.
- `ParsedEntries` contains only valid deduplicated names, in base-file then local-file order.
- `Warnings` contains parse, classification, and traversal diagnostics; `DeclarationWarnings` isolates file-read/parser diagnostics so doctor does not duplicate state warnings.
- `Owns` is containment in the canonical source or currently followed targets, and requires a canonical input. It is not link identity and does not itself authorize pruning.
- `InFollowed` recognizes declared logical prefixes including unavailable entries, excluding `not-link` and `undeclared-link`. The returned entry carries the state.
- `Unavailable` excludes followed, not-link, and undeclared-link entries.
- Walkers mutate the caller's set when a followed subtree cannot be read, changing its entry to missing. `Err` retains those read failures even if callbacks swallow the errors; callback return and SkipDir/SkipAll semantics remain standard-library compatible. Discovery consumers check `Err` and return no partial skills/repos on such failures.
- A set is one operation's snapshot, not a lock; concurrent mutation/traversal is unsupported.
- `Entry` JSON keys are `name`, `state`, `resolved_target` (omitempty), and `reason`.

The compatible entry points are:

```go
// internal/sync
type DiscoveryOptions struct {
    Follow *sourcewalk.FollowSet
    CollectIgnored bool
    CollectContext bool
    IncludeIgnored bool
}
func DiscoverSourceSkillsWithOptions(sourcePath string, opts DiscoveryOptions) ([]DiscoveredSkill, *skillignore.IgnoreStats, error)

// internal/install
func GetTrackedReposWithOptions(sourceDir string, opts sourcewalk.Options) ([]string, error)
```

Both take a caller-built set so discovery and nested consumers share the same state. No config/sync imports were added to sourcewalk, and no global policy callback was introduced.

## Changed callers and output

- `internal/sync/discover_walk.go`: follow pointer in the internal discovery options; logical walk root when following; reject partial results via `FollowSet.Err`.
- `internal/install/install_queries.go`: options-bearing tracked-repo entry point; legacy implementation delegates with nil policy.
- `cmd/skillshare/skillfollow.go`: convert the command's active enabled skills targets and effective git root into one set; absent declarations yield nil.
- Global/project `list`: regular and TUI discovery paths plus tracked-repo lookup.
- Global/project `status`: discovery, tracked-repo lookup, text line, and optional `source.skillfollow` JSON.
- Global/project `doctor`: discovery and per-entry/parser diagnostic output, with only undeclared links in the existing informational check.
- Global/project `sync`: the initial skills discovery pass, preserving existing mutation/ownership/prune behavior.

The global path uses `Config.EffectiveGitRoot`; project commands use their project root as the staging boundary and their resolved project target map. SourcePath, RelPath, and FlatName retain logical source paths.

`source.skillfollow` contains `active`, `local_active`, `entry_count`, `followed_count`, `skipped_count`, declared `entries`, and optional `warnings`. The field is omitted entirely with no declarations. Doctor includes the state and reason in each `skillfollow` check message, using pass for followed/not-link and warning for unavailable states. With an active declaration file, the undeclared-link information mentions `.skillfollow`; without one, its existing text is retained to meet byte-for-byte compatibility. No prune-paused or not-ignored output was added.

## Verification

- Parser tests cover portable rejected names, blank/comment trimming, local union, duplicates, and absent files.
- Classifier tests cover every state, dangling/non-link files, source self/ancestor/descendant cycles, active target ancestor/descendant overlap, git-root precedence, single-skill before tracked-repo detection, overlap chains, and query-copy behavior.
- Canonicalizer tests cover a missing suffix through an existing link, link loops, and an operation-local junction simulation seam (no global test hook).
- Walker tests cover logical paths with a linked source root, first-level directory reporting and names, one-hop behavior, Walk/WalkDir SkipDir/SkipAll/error parity, deterministic mid-walk read failure, and reuse of a set at a child root without another declaration hop.
- Discovery and tracked-repo API tests prove explicit following and nil-wrapper compatibility.
- Surface unit tests cover counts, states, parse diagnostics, and omitted output when inactive.
- `TestSkillfollowDiscoverySurfaces` runs global and project fixtures with a followed tracked repo (two skills), followed group, missing entry, invalid-target entry, undeclared link, and local declaration duplicate. It checks list/status/doctor JSON and byte snapshots of source/external files before and after read-only queries.
- Existing `TestUninstallGroup_ExternalSymlinkRejected` and `TestUpdateGroup_ExternalSymlinkRejected` passed unchanged.
- `GOOS=windows GOARCH=arm64 go test -c ./internal/sourcewalk -o /tmp/sourcewalk-windows-arm64.test.exe` passed inside the throwaway Linux devcontainer. Real Windows runtime validation was not performed.
- A binary built from baseline `70cb28767` and the new binary produced byte-for-byte equal stdout, stderr, and exit codes for no-declaration list/status/doctor JSON across all four global/project and regular/symlinked-source fixtures (including an undeclared link).

- A direct `sync -g --dry-run --json` fixture planned one followed skill, retained identical source/external file bytes, and left the target absent.
- Final `make check` passed inside the throwaway devcontainer, including docs-check, formatting, vet, both ratchets, unit tests, and integration tests.
- One intervening full-suite run timed out at the existing `TestInit_AlreadyInitialized_RemoteFlag_AddsRemote` after 600 seconds. The isolated test subsequently passed against both the new binary (0.829 seconds) and baseline (0.838 seconds); a full retry passed without code, test, or environment changes. The precise timeout cause was not established.

Final command:

```sh
docker exec skillshare-274-step3a bash -c 'make check'
```

Final tail (ANSI color removed):

```text
=== RUN   TestXDG_StatusWorksWithXDGPath
--- PASS: TestXDG_StatusWorksWithXDGPath (0.03s)
PASS
ok  skillshare/tests/integration 58.024s

✓ All tests passed!
```

The throwaway `skillshare-274-step3a` container was removed after verification; the worktree and commits remain for coordinator review.

## Remaining work

Per the approved scope adjustment, the following callers still intentionally use the nil-follow public wrappers. This list is an inventory, not a request to switch mutation consumers without the later ownership/git policy work. Raw sourcewalk call sites that were already migrated in step 2 remain nil-follow unless explicitly connected above.

```text
cmd/skillshare/analyze.go:202:			discovered, err := ssync.DiscoverSourceSkillsForAnalyze(cfg.EffectiveSkillsSource())
cmd/skillshare/analyze.go:223:	discovered, err := ssync.DiscoverSourceSkillsForAnalyze(sourcePath)
cmd/skillshare/analyze_project.go:25:			discovered, err := ssync.DiscoverSourceSkillsForAnalyze(runtime.sourcePath)
cmd/skillshare/audit.go:400:	discovered, _, err := sync.DiscoverSourceSkillsLite(sourcePath)
cmd/skillshare/check.go:259:	repos, err := install.GetTrackedRepos(sourceDir)
cmd/skillshare/check.go:880:	discovered, err := ssync.DiscoverSourceSkills(sourceDir)
cmd/skillshare/diff.go:368:	discovered, discoverErr := sync.DiscoverSourceSkills(cfg.EffectiveSkillsSource())
cmd/skillshare/diff_project.go:30:	discovered, err := sync.DiscoverSourceSkills(runtime.sourcePath)
cmd/skillshare/init.go:799:	discovered, _ := ssync.DiscoverSourceSkills(sourcePath)
cmd/skillshare/init_apply.go:167:	if discovered, err := ssync.DiscoverSourceSkills(p.source()); err == nil {
cmd/skillshare/init_apply.go:268:	discovered, err := ssync.DiscoverSourceSkills(cfg.EffectiveSkillsSource())
cmd/skillshare/init_remote.go:84:	if discovered, err := ssync.DiscoverSourceSkills(skillsDir); err == nil {
cmd/skillshare/uninstall_handlers.go:202:		discovered, _, err := sync.DiscoverSourceSkillsLite(mode.sourceDir)
cmd/skillshare/update_resolve.go:27:	repos, _ := install.GetTrackedRepos(sourceDir)
cmd/skillshare/update_resolve.go:64:	repos, _ := install.GetTrackedRepos(sourceDir)
internal/hub/draft_candidates.go:16:	discovered, err := ssync.DiscoverSourceSkills(sourcePath)
internal/hub/index.go:62:	discovered, err := ssync.DiscoverSourceSkills(sourcePath)
internal/install/install_queries.go:49:	existingRepos, err := GetTrackedRepos(sourceDir)
internal/install/install_queries.go:98:	existingRepos, err := GetTrackedRepos(sourceDir)
internal/server/handler_analyze.go:46:	discovered, err := ssync.DiscoverSourceSkillsForAnalyze(source)
internal/server/handler_audit.go:107:	discovered, err := sync.DiscoverSourceSkills(source)
internal/server/handler_check.go:58:	repos, _ := install.GetTrackedRepos(sourceDir)
internal/server/handler_check_stream.go:41:	repos, _ := install.GetTrackedRepos(sourceDir)
internal/server/handler_diff_stream.go:43:	discovered, ignoreStats, err := ssync.DiscoverSourceSkillsWithStats(source)
internal/server/handler_overview.go:41:	skills, err := sync.DiscoverSourceSkills(source)
internal/server/handler_overview.go:98:	repoNames, err := install.GetTrackedRepos(sourceDir)
internal/server/handler_skill_content.go:250:		discovered, err := sync.DiscoverSourceSkillsAll(source)
internal/server/handler_skill_content.go:311:		discovered, err := sync.DiscoverSourceSkillsAll(source)
internal/server/handler_skillignore.go:44:	_, stats, discoverErr := sync.DiscoverSourceSkillsWithStats(source)
internal/server/handler_skills.go:163:		discovered, err := sync.DiscoverSourceSkillsAll(source)
internal/server/handler_skills.go:319:	discovered, err := sync.DiscoverSourceSkills(source)
internal/server/handler_skills.go:513:	discovered, err := sync.DiscoverSourceSkillsAll(s.cfg.EffectiveSkillsSource())
internal/server/handler_skills.go:578:	repos, err := install.GetTrackedRepos(s.cfg.EffectiveSkillsSource())
internal/server/handler_skills.go:69:		discovered, err := sync.DiscoverSourceSkillsAll(source)
internal/server/handler_skills_batch.go:199:	discovered, err := ssync.DiscoverSourceSkillsAll(source)
internal/server/handler_skills_batch.go:70:	discovered, err := ssync.DiscoverSourceSkillsAll(source)
internal/server/handler_sync.go:216:		allSkills, ignoreStats, err = ssync.DiscoverSourceSkillsWithStatsAndContext(s.cfg.EffectiveSkillsSource())
internal/server/handler_sync.go:597:	discovered, ignoreStats, err := ssync.DiscoverSourceSkillsWithStats(source)
internal/server/handler_sync_matrix.go:183:	skills, err := ssync.DiscoverSourceSkills(source)
internal/server/handler_sync_matrix.go:67:	skills, err := ssync.DiscoverSourceSkills(source)
internal/server/handler_targets.go:97:	discovered, discoveredErr := ssync.DiscoverSourceSkills(source)
internal/server/handler_toggle.go:115:	discovered, err := ssync.DiscoverSourceSkillsAll(source)
internal/server/handler_toggle_batch.go:96:		discovered, err := ssync.DiscoverSourceSkillsAll(source)
internal/server/handler_uninstall.go:152:	discovered, err := sync.DiscoverSourceSkillsAll(s.cfg.EffectiveSkillsSource())
internal/server/handler_update.go:465:	repos, err := install.GetTrackedRepos(s.cfg.EffectiveSkillsSource())
internal/server/handler_update_stream.go:78:		repos, err := install.GetTrackedRepos(source)
internal/sync/copy.go:244:	allSourceSkills, err := DiscoverSourceSkills(sourcePath)
internal/sync/copy.go:41:	skills, err := DiscoverSourceSkills(sourcePath)
internal/sync/sync.go:532:	skills, err := DiscoverSourceSkills(sourcePath)
internal/sync/sync.go:717:	allSourceSkills, err := DiscoverSourceSkills(sourcePath)
```

Link identity, ownership, status aggregate changes, unavailable-entry prune/copy protection, reconciliation policy, mutation boundaries, git staging/reachability, repo-root audits, followed-update policy, and followed single skills remain deferred as requested. Server endpoint plumbing is not part of this completed CLI-first connection; its current discovery remains unchanged. Public release documentation and real Windows runs belong to the remaining rollout work.
