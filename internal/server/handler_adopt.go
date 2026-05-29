package server

import (
	"encoding/json"
	"net/http"
	"time"

	"skillshare/internal/adopt"
	"skillshare/internal/config"
	ssync "skillshare/internal/sync"
	"skillshare/internal/trash"
)

// adoptAgentsTargetNames are the canonical name + alias of the universal target
// (~/.agents/skills) that external CLI tools write into. Mirrors the CLI.
var adoptAgentsTargetNames = []string{"universal", "agents"}

// findAgentsTarget locates the universal/agents target in a target map.
func findAgentsTarget(targets map[string]config.TargetConfig) (config.TargetConfig, bool) {
	for _, name := range adoptAgentsTargetNames {
		if t, ok := targets[name]; ok {
			return t, true
		}
	}
	return config.TargetConfig{}, false
}

// adoptCandidate is the JSON shape of a detected adoptable skill.
type adoptCandidate struct {
	Name          string   `json:"name"`
	Path          string   `json:"path"`
	SourceTool    string   `json:"sourceTool"`
	Conflict      bool     `json:"conflict"`
	ExternalLinks []string `json:"externalLinks"`
}

// handleAdoptPreview detects adoptable skills in the agents/universal target for
// the current mode and returns them (with lockfile provenance) without mutating.
// GET /api/adopt/preview -> { candidates: [...], lockPresent: bool }
func (s *Server) handleAdoptPreview(w http.ResponseWriter, r *http.Request) {
	// Snapshot config under RLock, then release before I/O.
	s.mu.RLock()
	source := s.skillsSource()
	globalMode := s.cfg.Mode
	targets := s.cloneTargets()
	s.mu.RUnlock()

	agentsTarget, ok := findAgentsTarget(targets)
	if !ok {
		writeError(w, http.StatusBadRequest, "universal/agents target not configured; nothing to adopt")
		return
	}
	sc := agentsTarget.SkillsConfig()

	allTargets := make(map[string]string, len(targets))
	for name, t := range targets {
		allTargets[name] = t.SkillsConfig().Path
	}

	syncMode := ssync.EffectiveMode(sc.Mode)
	if sc.Mode == "" && globalMode != "" {
		syncMode = globalMode
	}

	candidates, err := adopt.DetectAdoptable(sc.Path, source, syncMode, allTargets)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "adopt preview failed: "+err.Error())
		return
	}

	// Annotate provenance from the lockfile (read-only).
	lockEntries, _ := adopt.ReadLock(sc.Path)
	for i := range candidates {
		candidates[i].SourceTool = adopt.Provenance(lockEntries, candidates[i].Name)
	}

	out := make([]adoptCandidate, 0, len(candidates))
	for _, c := range candidates {
		links := c.ExternalLinks
		if links == nil {
			links = []string{}
		}
		out = append(out, adoptCandidate{
			Name:          c.Name,
			Path:          c.Path,
			SourceTool:    c.SourceTool,
			Conflict:      c.Conflict,
			ExternalLinks: links,
		})
	}

	writeJSON(w, map[string]any{
		"candidates":  out,
		"lockPresent": len(lockEntries) > 0,
	})
}

// handleAdoptApply migrates selected adoptable skills into source, trashes the
// originals, prunes orphan symlinks, re-syncs to all targets, and warns about
// lingering lockfile entries.
// POST /api/adopt/apply  { names: []string (empty = all), force: bool, dryRun: bool }
func (s *Server) handleAdoptApply(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()

	var body struct {
		Names  []string `json:"names"`
		Force  bool     `json:"force"`
		DryRun bool     `json:"dryRun"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	source := s.skillsSource()
	globalMode := s.cfg.Mode
	targets := s.cloneTargets()

	agentsTarget, ok := findAgentsTarget(targets)
	if !ok {
		writeError(w, http.StatusBadRequest, "universal/agents target not configured; nothing to adopt")
		return
	}
	sc := agentsTarget.SkillsConfig()

	allTargets := make(map[string]string, len(targets))
	for name, t := range targets {
		allTargets[name] = t.SkillsConfig().Path
	}

	syncMode := ssync.EffectiveMode(sc.Mode)
	if sc.Mode == "" && globalMode != "" {
		syncMode = globalMode
	}

	trashBase := trash.TrashDir()
	if s.IsProjectMode() {
		trashBase = trash.ProjectTrashDir(s.projectRoot)
	}

	// Detect first, then annotate provenance, then apply (mirror of the CLI).
	candidates, err := adopt.DetectAdoptable(sc.Path, source, syncMode, allTargets)
	if err != nil {
		s.writeOpsLog("adopt", "error", start, map[string]any{"scope": "ui"}, err.Error())
		writeError(w, http.StatusInternalServerError, "adopt detect failed: "+err.Error())
		return
	}
	lockEntries, _ := adopt.ReadLock(sc.Path)
	for i := range candidates {
		candidates[i].SourceTool = adopt.Provenance(lockEntries, candidates[i].Name)
	}

	res, err := adopt.Apply(candidates, adopt.Request{
		AgentsPath: sc.Path,
		SourcePath: source,
		SyncMode:   syncMode,
		TrashBase:  trashBase,
		AllTargets: allTargets,
		Targets:    targets,
		DryRun:     body.DryRun,
		Force:      body.Force,
		Selected:   body.Names,
	})
	if err != nil {
		s.writeOpsLog("adopt", "error", start, map[string]any{"scope": "ui"}, err.Error())
		writeError(w, http.StatusInternalServerError, "adopt failed: "+err.Error())
		return
	}

	status := "ok"
	msg := ""
	if len(res.Failed) > 0 {
		status = "partial"
		msg = "some skills failed to adopt"
	}
	s.writeOpsLog("adopt", status, start, map[string]any{
		"adopted": len(res.Adopted),
		"trashed": res.Trashed,
		"pruned":  res.PrunedLinks,
		"dry_run": res.DryRun,
		"force":   body.Force,
		"scope":   "ui",
	}, msg)

	writeJSON(w, adoptResultJSON(res))
}

// adoptResultJSON converts an adopt.Result into a stable JSON payload,
// normalising nil slices/maps to their empty forms.
func adoptResultJSON(res *adopt.Result) map[string]any {
	adopted := res.Adopted
	if adopted == nil {
		adopted = []string{}
	}
	skipped := res.Skipped
	if skipped == nil {
		skipped = []string{}
	}
	failed := res.Failed
	if failed == nil {
		failed = map[string]string{}
	}
	warnings := res.LockWarnings
	if warnings == nil {
		warnings = []adopt.LockWarning{}
	}
	return map[string]any{
		"adopted":      adopted,
		"skipped":      skipped,
		"failed":       failed,
		"trashed":      res.Trashed,
		"prunedLinks":  res.PrunedLinks,
		"lockWarnings": warnings,
		"dryRun":       res.DryRun,
	}
}
