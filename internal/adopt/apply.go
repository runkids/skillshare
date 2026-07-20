package adopt

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"skillshare/internal/config"
	"skillshare/internal/install"
	"skillshare/internal/sync"
	"skillshare/internal/trash"
	"skillshare/internal/utils"
)

// LockWarning describes a skill that was adopted but is still referenced in the
// external tool's lockfile. The lockfile is read-only — we only warn.
type LockWarning struct {
	Name       string `json:"name"`
	SourceTool string `json:"source_tool"`
}

// Request carries the inputs for the destructive adopt flow. It is the shared
// contract between the CLI handler and the server handler.
type Request struct {
	// AgentsPath is the universal/agents target skills dir (e.g. ~/.agents/skills).
	AgentsPath string
	// SourcePath is skillshare's source of truth skills dir.
	SourcePath string
	// TrashBase is the trash dir (global or project) for soft-deleting originals.
	TrashBase string
	// Targets maps target name -> resolved config, for re-sync after migration.
	Targets map[string]config.TargetConfig
	// DefaultMode is the config-level sync mode (cfg.Mode) used when a target
	// does not set its own mode. Empty falls back to "merge". Re-sync honors
	// each target's effective mode so copy/symlink targets are not forced to
	// merge-mode symlinks.
	DefaultMode string
	// ProjectRoot enables project-mode sync behavior, including portable
	// relative links for targets under the project root.
	ProjectRoot string
	// FileIgnorePatterns controls file-level exclusions for copy-mode re-sync.
	// Callers pass the same effective patterns used by the normal sync command.
	FileIgnorePatterns []string
	// ProjectSkills are declarative project install entries. They reserve their
	// source-relative paths even when the machine-local metadata is missing,
	// because a later bare project install will replay them.
	ProjectSkills []config.SkillEntry
	// DryRun previews without mutating anything.
	DryRun bool
	// Force overwrites conflicting skills already present in source.
	Force bool
	// Selected lists candidate names to adopt. Empty means "all detected".
	Selected []string
}

// Result is the outcome of an adopt run.
type Result struct {
	Adopted      []string          // skill names migrated into source
	Skipped      []string          // skills skipped (conflict, no Force)
	Failed       map[string]string // skill or follow-up operation -> error message
	Trashed      int               // originals moved to trash
	PrunedLinks  int               // orphan symlinks removed across targets
	LockWarnings []LockWarning     // adopted skills still present in the lockfile
	DryRun       bool
}

func newResult(dryRun bool) *Result {
	return &Result{Failed: make(map[string]string), DryRun: dryRun}
}

// Apply performs the destructive adopt flow: copy canonical files into source,
// remove the previewed external links, trash the originals, re-sync to all
// targets, and warn about lingering lockfile entries.
//
// Safety semantics (must not regress):
//   - copy happens before trash (originals only trashed after a successful copy)
//   - DryRun performs zero mutation
//   - conflicting skills are skipped unless Force is set
//   - the lockfile is never written, only read for warnings
//
// candidates are the already-detected candidates (with provenance annotated).
// req.Selected filters them by name; an empty Selected adopts all candidates.
func Apply(candidates []Candidate, req Request) (*Result, error) {
	res := newResult(req.DryRun)
	if err := validateSeparateRoots(req.AgentsPath, req.SourcePath); err != nil {
		return res, err
	}

	selected := filterSelected(candidates, req.Selected)
	if len(selected) == 0 {
		return res, nil
	}

	// 1. Migrate canonical files into source via PullSkills. Install metadata is
	// an independent ownership claim: silently retaining it would let a later
	// update overwrite the adopted copy. Require an explicit uninstall first.
	ownership, metadataErr := inspectSourceOwnership(req.SourcePath, req.ProjectSkills)
	if metadataErr != nil {
		return res, fmt.Errorf("inspect source install metadata: %w", metadataErr)
	}
	locals := make([]sync.LocalSkillInfo, 0, len(selected))
	for _, c := range selected {
		claim, claimErr := ownership.claim(c.Name)
		if claimErr != nil {
			return res, fmt.Errorf("inspect source ownership for %s: %w", c.Name, claimErr)
		}
		if claim != "" {
			res.Failed[c.Name] = fmt.Sprintf("source skill is managed by %s; uninstall it before adopting", claim)
			continue
		}
		locals = append(locals, sync.LocalSkillInfo{Name: c.Name, Path: c.Path})
	}
	pull, err := sync.PullSkills(locals, req.SourcePath, sync.PullOptions{DryRun: req.DryRun, Force: req.Force})
	if err != nil {
		return res, err
	}
	res.Adopted = pull.Pulled
	res.Skipped = pull.Skipped
	for k, v := range pull.Failed {
		res.Failed[k] = v.Error()
	}

	// Set of names successfully copied (only these get trashed/pruned).
	adopted := make(map[string]bool, len(res.Adopted))
	for _, n := range res.Adopted {
		adopted[n] = true
	}

	if req.DryRun {
		// Still surface lockfile warnings in dry-run.
		res.LockWarnings = lockWarningsFor(req.AgentsPath, res.Adopted)
		return res, nil
	}

	// 2. Remove only the external links recorded for successfully copied skills.
	// A full target prune is intentionally unsafe here: it can remove unrelated
	// broken links and managed entries that were not part of this adoption.
	for _, c := range selected {
		if !adopted[c.Name] {
			continue
		}
		for _, linkPath := range uniqueEffectiveEntryPaths(c.ExternalLinks) {
			if err := removeRecordedLink(linkPath, c.Path); err != nil {
				res.Failed["cleanup "+linkPath] = err.Error()
				continue
			}
			res.PrunedLinks++
		}
	}

	// 3. Trash the originals in the agents dir (only after a successful copy).
	for _, c := range selected {
		if !adopted[c.Name] {
			continue
		}
		if _, terr := trash.MoveToTrash(c.Path, c.Name, req.TrashBase); terr != nil {
			res.Failed[c.Name] = terr.Error()
			continue
		}
		res.Trashed++
	}

	// 4. Re-sync from source to all targets, honoring each target's mode.
	// Best-effort; individual target failures must not abort the flow.
	for _, namedTarget := range uniqueTargets(req.Targets) {
		name := namedTarget.name
		target := namedTarget.target
		var syncErr error
		switch effectiveMode(target, req.DefaultMode) {
		case "copy":
			var copyResult *sync.CopyResult
			copyResult, syncErr = sync.SyncTargetCopyWithOptions(name, target, req.SourcePath, false, false, sync.CopyOptions{
				IgnorePatterns: req.FileIgnorePatterns,
			})
			if syncErr == nil {
				manifest, manifestErr := sync.ReadManifest(target.SkillsConfig().Path)
				if manifestErr != nil {
					syncErr = fmt.Errorf("read target manifest after sync: %w", manifestErr)
				} else {
					syncErr = recordAdoptSyncResult(res, adopted, name, target, req.SourcePath,
						append(copyResult.Copied, copyResult.Updated...), copyResult.Skipped, manifest.Managed)
				}
			}
		case "symlink":
			syncErr = sync.SyncTarget(name, target, req.SourcePath, false, req.ProjectRoot)
		default: // merge
			var mergeResult *sync.MergeResult
			mergeResult, syncErr = sync.SyncTargetMerge(name, target, req.SourcePath, false, false, req.ProjectRoot)
			if syncErr == nil {
				syncErr = recordAdoptSyncResult(res, adopted, name, target, req.SourcePath,
					append(mergeResult.Linked, mergeResult.Updated...), mergeResult.Skipped, nil)
			}
		}
		if syncErr != nil {
			res.Failed["sync "+name] = syncErr.Error()
		}
	}

	// 5. Warn about lingering lockfile entries (never write the lockfile).
	res.LockWarnings = lockWarningsFor(req.AgentsPath, res.Adopted)

	return res, nil
}

type sourceOwnership struct {
	sourcePath     string
	metadata       *install.MetadataStore
	legacyEntries  []config.SkillEntry
	projectEntries []config.SkillEntry
}

func inspectSourceOwnership(sourcePath string, projectEntries []config.SkillEntry) (*sourceOwnership, error) {
	metadata, err := install.ReadMetadata(sourcePath)
	if err != nil {
		return nil, err
	}

	legacyEntries := make([]config.SkillEntry, 0)
	for _, dir := range []string{sourcePath, filepath.Dir(sourcePath)} {
		registry, loadErr := config.LoadRegistry(dir)
		if loadErr != nil {
			return nil, loadErr
		}
		legacyEntries = append(legacyEntries, registry.Skills...)
	}

	return &sourceOwnership{
		sourcePath:     sourcePath,
		metadata:       metadata,
		legacyEntries:  legacyEntries,
		projectEntries: projectEntries,
	}, nil
}

func (o *sourceOwnership) claim(name string) (string, error) {
	if o.metadata.GetByPath(name) != nil {
		return "install metadata", nil
	}
	if entryClaimsSourcePath(o.legacyEntries, name) {
		return "legacy install metadata", nil
	}
	legacySidecar, err := install.ReadMeta(filepath.Join(o.sourcePath, filepath.FromSlash(name)))
	if err != nil {
		return "", err
	}
	if legacySidecar != nil {
		return "legacy install metadata", nil
	}
	if entryClaimsSourcePath(o.projectEntries, name) {
		return "project install replay", nil
	}
	return "", nil
}

func entryClaimsSourcePath(entries []config.SkillEntry, name string) bool {
	want := filepath.ToSlash(filepath.Clean(filepath.FromSlash(name)))
	for _, entry := range entries {
		if entry.EffectiveKind() != "skill" {
			continue
		}
		got := filepath.ToSlash(filepath.Clean(filepath.FromSlash(entry.FullName())))
		if got == want {
			return true
		}
	}
	return false
}

type namedTarget struct {
	name   string
	target config.TargetConfig
}

func uniqueTargets(targets map[string]config.TargetConfig) []namedTarget {
	names := make([]string, 0, len(targets))
	for name := range targets {
		names = append(names, name)
	}
	sort.Strings(names)

	unique := make([]namedTarget, 0, len(names))
	seen := make([]string, 0, len(names))
	for _, name := range names {
		target := targets[name]
		path := target.SkillsConfig().Path
		effective := effectiveTargetPath(path)
		duplicate := false
		for _, existing := range seen {
			if path != "" && utils.PathsEqual(existing, effective) {
				duplicate = true
				break
			}
		}
		if duplicate {
			continue
		}
		seen = append(seen, effective)
		unique = append(unique, namedTarget{name: name, target: target})
	}
	return unique
}

type adoptTargetPlan struct {
	expected       map[string]bool
	sourceByActive map[string]string
	unresolved     map[string]string
}

func recordAdoptSyncResult(
	res *Result,
	adopted map[string]bool,
	targetName string,
	target config.TargetConfig,
	sourcePath string,
	synced, skipped []string,
	managed map[string]string,
) error {
	plan, err := planAdoptTargetSync(adopted, targetName, target, sourcePath)
	if err != nil {
		return fmt.Errorf("resolve adopted target entries: %w", err)
	}

	succeeded := make(map[string]bool, len(plan.expected))
	failed := make(map[string]bool, len(plan.expected))
	for _, activeName := range synced {
		if sourceName, ok := plan.sourceByActive[activeName]; ok {
			succeeded[sourceName] = true
		}
	}
	for _, activeName := range skipped {
		sourceName, ok := plan.sourceByActive[activeName]
		if !ok {
			continue
		}
		if managed != nil {
			if _, ok := managed[activeName]; ok {
				succeeded[sourceName] = true
				continue
			}
		}
		res.Failed["sync "+targetName+"/"+sourceName] =
			fmt.Sprintf("local target entry %q was preserved; adopted skill was not distributed", activeName)
		failed[sourceName] = true
	}
	for sourceName, reason := range plan.unresolved {
		res.Failed["sync "+targetName+"/"+sourceName] = reason
		failed[sourceName] = true
	}
	for sourceName := range plan.expected {
		if !succeeded[sourceName] && !failed[sourceName] {
			res.Failed["sync "+targetName+"/"+sourceName] = "target sync did not report the adopted skill as distributed"
		}
	}
	return nil
}

func planAdoptTargetSync(adopted map[string]bool, targetName string, target config.TargetConfig, sourcePath string) (*adoptTargetPlan, error) {
	plan := &adoptTargetPlan{
		expected:       make(map[string]bool),
		sourceByActive: make(map[string]string),
		unresolved:     make(map[string]string),
	}
	allSkills, err := sync.DiscoverSourceSkills(sourcePath)
	if err != nil {
		return nil, err
	}
	sc := target.SkillsConfig()
	eligible, err := sync.FilterSkills(allSkills, sc.Include, sc.Exclude)
	if err != nil {
		return nil, err
	}
	eligible = sync.FilterSkillsByTarget(eligible, targetName)
	for _, skill := range eligible {
		if adopted[skill.RelPath] {
			plan.expected[skill.RelPath] = true
		}
	}

	resolution, err := sync.ResolveTargetSkillsForTarget(targetName, sc, allSkills)
	if err != nil {
		return nil, err
	}
	resolved := make(map[string]bool, len(plan.expected))
	for _, skill := range resolution.Skills {
		sourceName := skill.Skill.RelPath
		if !adopted[sourceName] {
			continue
		}
		resolved[sourceName] = true
		plan.sourceByActive[skill.TargetName] = sourceName
		// Standard naming can retain a legacy flat entry as the active path.
		plan.sourceByActive[skill.Skill.FlatName] = sourceName
	}

	colliding := make(map[string]bool)
	for _, collision := range resolution.Collisions {
		for _, path := range collision.Paths {
			colliding[path] = true
		}
	}
	for sourceName := range plan.expected {
		if resolved[sourceName] {
			continue
		}
		if colliding[sourceName] {
			plan.unresolved[sourceName] = "target naming collision excluded adopted skill from distribution"
		} else {
			plan.unresolved[sourceName] = "target naming validation excluded adopted skill from distribution"
		}
	}
	return plan, nil
}

// removeRecordedLink removes a previewed external link only if it is still a
// link to the candidate being adopted. The target exists at this point, so both
// sides can be canonicalized before the destructive remove.
func removeRecordedLink(linkPath, candidatePath string) error {
	if _, err := os.Lstat(linkPath); err != nil {
		return fmt.Errorf("recorded link is no longer available: %w", err)
	}
	if !utils.IsSymlinkOrJunction(linkPath) {
		return fmt.Errorf("recorded link changed and is no longer a symlink")
	}
	resolvedLink, err := filepath.EvalSymlinks(linkPath)
	if err != nil {
		return fmt.Errorf("resolve recorded link: %w", err)
	}
	resolvedCandidate, err := filepath.EvalSymlinks(candidatePath)
	if err != nil {
		return fmt.Errorf("resolve adopted skill: %w", err)
	}
	if !utils.PathsEqual(resolvedLink, resolvedCandidate) {
		return fmt.Errorf("recorded link now points somewhere else")
	}
	if err := os.Remove(linkPath); err != nil {
		return fmt.Errorf("remove recorded link: %w", err)
	}
	return nil
}

func uniqueEffectiveEntryPaths(paths []string) []string {
	unique := make([]string, 0, len(paths))
	seen := make([]string, 0, len(paths))
	for _, path := range paths {
		effective := filepath.Join(effectiveTargetPath(filepath.Dir(path)), filepath.Base(path))
		duplicate := false
		for _, existing := range seen {
			if utils.PathsEqual(existing, effective) {
				duplicate = true
				break
			}
		}
		if duplicate {
			continue
		}
		seen = append(seen, effective)
		unique = append(unique, path)
	}
	return unique
}

// filterSelected returns candidates whose name appears in names. An empty names
// slice returns all candidates unchanged.
func filterSelected(candidates []Candidate, names []string) []Candidate {
	if len(names) == 0 {
		return candidates
	}
	want := make(map[string]bool, len(names))
	for _, n := range names {
		want[n] = true
	}
	picked := make([]Candidate, 0, len(names))
	for _, c := range candidates {
		if want[c.Name] {
			picked = append(picked, c)
		}
	}
	return picked
}

// effectiveMode resolves the sync mode for a target during re-sync: the
// target's own mode, then the config-level default, then "merge". Mirrors the
// dispatch in cmd/skillshare/sync.go so adopt re-sync matches `skillshare sync`.
func effectiveMode(target config.TargetConfig, defaultMode string) string {
	mode := target.SkillsConfig().Mode
	if mode == "" {
		mode = defaultMode
	}
	if mode == "" {
		mode = "merge"
	}
	return mode
}

// lockWarningsFor returns lock warnings for any adopted skill still present in
// the agents-dir lockfile. The lockfile is read-only.
func lockWarningsFor(agentsPath string, adopted []string) []LockWarning {
	entries, _ := ReadLock(agentsPath)
	if len(entries) == 0 {
		return nil
	}
	var warnings []LockWarning
	for _, name := range adopted {
		if _, ok := entries[name]; ok {
			warnings = append(warnings, LockWarning{Name: name, SourceTool: Provenance(entries, name)})
		}
	}
	return warnings
}
