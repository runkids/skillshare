package server

import (
	"encoding/json"
	"net/http"
	"time"

	ssync "skillshare/internal/sync"
)

type syncTargetResult struct {
	Target  string   `json:"target"`
	Linked  []string `json:"linked"`
	Updated []string `json:"updated"`
	Skipped []string `json:"skipped"`
	Pruned  []string `json:"pruned"`
}

func (s *Server) handleSync(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()

	var body struct {
		DryRun bool `json:"dryRun"`
		Force  bool `json:"force"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		// Default to non-dry-run, non-force
	}

	globalMode := s.cfg.Mode
	if globalMode == "" {
		globalMode = "merge"
	}

	// Discover skills once for all targets
	allSkills, err := ssync.DiscoverSourceSkills(s.cfg.Source)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to discover skills: "+err.Error())
		return
	}

	results := make([]syncTargetResult, 0)

	for name, target := range s.cfg.Targets {
		mode := target.Mode
		if mode == "" {
			mode = globalMode
		}

		res := syncTargetResult{
			Target:  name,
			Linked:  make([]string, 0),
			Updated: make([]string, 0),
			Skipped: make([]string, 0),
			Pruned:  make([]string, 0),
		}

		syncErrArgs := map[string]any{
			"targets_total":  len(s.cfg.Targets),
			"targets_failed": 1,
			"target":         name,
			"dry_run":        body.DryRun,
			"force":          body.Force,
			"scope":          "ui",
		}

		switch mode {
		case "merge":
			mergeResult, err := ssync.SyncTargetMergeWithSkills(name, target, allSkills, s.cfg.Source, body.DryRun, body.Force)
			if err != nil {
				s.writeOpsLog("sync", "error", start, syncErrArgs, err.Error())
				writeError(w, http.StatusInternalServerError, "sync failed for "+name+": "+err.Error())
				return
			}
			res.Linked = mergeResult.Linked
			res.Updated = mergeResult.Updated
			res.Skipped = mergeResult.Skipped

			pruneResult, err := ssync.PruneOrphanLinksWithSkills(ssync.PruneOptions{
				TargetPath: target.Path, SourcePath: s.cfg.Source, Skills: allSkills,
				Include: target.Include, Exclude: target.Exclude, TargetName: name,
				DryRun: body.DryRun, Force: body.Force,
			})
			if err == nil {
				res.Pruned = pruneResult.Removed
			}

		case "copy":
			copyResult, err := ssync.SyncTargetCopyWithSkills(name, target, allSkills, s.cfg.Source, body.DryRun, body.Force, nil)
			if err != nil {
				s.writeOpsLog("sync", "error", start, syncErrArgs, err.Error())
				writeError(w, http.StatusInternalServerError, "sync failed for "+name+": "+err.Error())
				return
			}
			res.Linked = copyResult.Copied
			res.Updated = copyResult.Updated
			res.Skipped = copyResult.Skipped

			pruneResult, err := ssync.PruneOrphanCopiesWithSkills(target.Path, allSkills, target.Include, target.Exclude, name, body.DryRun)
			if err == nil {
				res.Pruned = pruneResult.Removed
			}

		default:
			err := ssync.SyncTarget(name, target, s.cfg.Source, body.DryRun)
			if err != nil {
				s.writeOpsLog("sync", "error", start, syncErrArgs, err.Error())
				writeError(w, http.StatusInternalServerError, "sync failed for "+name+": "+err.Error())
				return
			}
			res.Linked = []string{"(symlink mode)"}
		}

		results = append(results, res)
	}

	// Log the sync operation
	s.writeOpsLog("sync", "ok", start, map[string]any{
		"targets_total":  len(results),
		"targets_failed": 0,
		"dry_run":        body.DryRun,
		"force":          body.Force,
		"scope":          "ui",
	}, "")

	writeJSON(w, map[string]any{"results": results})
}

type diffItem struct {
	Skill  string `json:"skill"`
	Action string `json:"action"` // "link", "update", "skip", "prune", "local"
	Reason string `json:"reason"` // human-readable description
}

type diffTarget struct {
	Target string     `json:"target"`
	Items  []diffItem `json:"items"`
}

func (s *Server) handleDiff(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	filterTarget := r.URL.Query().Get("target")

	globalMode := s.cfg.Mode
	if globalMode == "" {
		globalMode = "merge"
	}

	discovered, err := ssync.DiscoverSourceSkills(s.cfg.Source)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	diffs := make([]diffTarget, 0)
	for name, target := range s.cfg.Targets {
		if filterTarget != "" && filterTarget != name {
			continue
		}
		diffs = append(diffs, s.computeTargetDiff(name, target, discovered, globalMode))
	}

	writeJSON(w, map[string]any{"diffs": diffs})
}
