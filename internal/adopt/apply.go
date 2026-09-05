package adopt

import (
	"skillshare/internal/config"
	"skillshare/internal/sync"
	"skillshare/internal/trash"
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
	// SyncMode is the agents target's sync mode (reserved for parity with detect).
	SyncMode string
	// TrashBase is the trash dir (global or project) for soft-deleting originals.
	TrashBase string
	// AllTargets maps target name -> skills dir, for orphan-link pruning.
	AllTargets map[string]string
	// Targets maps target name -> resolved config, for re-sync after migration.
	Targets map[string]config.TargetConfig
	// DefaultMode is the config-level sync mode (cfg.Mode) used when a target
	// does not set its own mode. Empty falls back to "merge". Re-sync honors
	// each target's effective mode so copy/symlink targets are not forced to
	// merge-mode symlinks.
	DefaultMode string
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
	Failed       map[string]string // skill name -> error message
	Trashed      int               // originals moved to trash
	PrunedLinks  int               // orphan symlinks removed across targets
	LockWarnings []LockWarning     // adopted skills still present in the lockfile
	DryRun       bool
}

func newResult(dryRun bool) *Result {
	return &Result{Failed: make(map[string]string), DryRun: dryRun}
}

// Apply performs the destructive adopt flow: copy canonical files into source,
// trash the originals, prune orphan symlinks, re-sync to all targets, and warn
// about lingering lockfile entries.
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

	selected := filterSelected(candidates, req.Selected)
	if len(selected) == 0 {
		return res, nil
	}

	// 1. Migrate canonical files into source via PullSkills.
	locals := make([]sync.LocalSkillInfo, len(selected))
	for i, c := range selected {
		locals[i] = sync.LocalSkillInfo{Name: c.Name, Path: c.Path}
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

	// 2. Trash the originals in the agents dir (only after a successful copy).
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

	// 3. Prune orphan symlinks across all targets (source now owns the skills).
	for name, targetPath := range req.AllTargets {
		naming := targetNaming(req.Targets, name)
		prune, perr := sync.PruneOrphanLinks(targetPath, req.SourcePath, nil, nil, name, naming, false, false)
		if perr != nil || prune == nil {
			continue
		}
		res.PrunedLinks += len(prune.Removed)
	}

	// 4. Re-sync from source to all targets, honoring each target's mode.
	// Best-effort; individual target failures must not abort the flow.
	for name, target := range req.Targets {
		switch effectiveMode(target, req.DefaultMode) {
		case "copy":
			_, _ = sync.SyncTargetCopy(name, target, req.SourcePath, false, req.Force)
		case "symlink":
			_ = sync.SyncTarget(name, target, req.SourcePath, false, "")
		default: // merge
			_, _ = sync.SyncTargetMerge(name, target, req.SourcePath, false, false, "")
		}
	}

	// 5. Warn about lingering lockfile entries (never write the lockfile).
	res.LockWarnings = lockWarningsFor(req.AgentsPath, res.Adopted)

	return res, nil
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

// targetNaming resolves the naming scheme for prune, falling back to the empty
// string (default flat naming) when the target config is unknown.
func targetNaming(targets map[string]config.TargetConfig, name string) string {
	if t, ok := targets[name]; ok {
		return t.SkillsConfig().TargetNaming
	}
	return ""
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
