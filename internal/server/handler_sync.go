package server

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"

	ssync "skillshare/internal/sync"
	"skillshare/internal/utils"
)

type syncTargetResult struct {
	Target  string   `json:"target"`
	Linked  []string `json:"linked"`
	Updated []string `json:"updated"`
	Skipped []string `json:"skipped"`
	Pruned  []string `json:"pruned"`
}

func (s *Server) handleSync(w http.ResponseWriter, r *http.Request) {
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

		if mode == "merge" {
			mergeResult, err := ssync.SyncTargetMerge(name, target, s.cfg.Source, body.DryRun, body.Force)
			if err != nil {
				writeError(w, http.StatusInternalServerError, "sync failed for "+name+": "+err.Error())
				return
			}
			res.Linked = mergeResult.Linked
			res.Updated = mergeResult.Updated
			res.Skipped = mergeResult.Skipped

			// Prune orphans
			pruneResult, err := ssync.PruneOrphanLinks(target.Path, s.cfg.Source, body.DryRun)
			if err == nil {
				res.Pruned = pruneResult.Removed
			}
		} else {
			err := ssync.SyncTarget(name, target, s.cfg.Source, body.DryRun)
			if err != nil {
				writeError(w, http.StatusInternalServerError, "sync failed for "+name+": "+err.Error())
				return
			}
			res.Linked = []string{"(symlink mode)"}
		}

		results = append(results, res)
	}

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

		mode := target.Mode
		if mode == "" {
			mode = globalMode
		}

		dt := diffTarget{Target: name, Items: make([]diffItem, 0)}

		if mode != "merge" {
			status := ssync.CheckStatus(target.Path, s.cfg.Source)
			if status != ssync.StatusLinked {
				dt.Items = append(dt.Items, diffItem{Skill: "(entire directory)", Action: "link", Reason: "missing"})
			}
			diffs = append(diffs, dt)
			continue
		}

		// Merge mode: check each skill
		for _, skill := range discovered {
			targetSkillPath := filepath.Join(target.Path, skill.FlatName)
			_, err := os.Lstat(targetSkillPath)
			if err != nil {
				if os.IsNotExist(err) {
					dt.Items = append(dt.Items, diffItem{Skill: skill.FlatName, Action: "link", Reason: "missing"})
				}
				continue
			}

			if utils.IsSymlinkOrJunction(targetSkillPath) {
				absLink, err := utils.ResolveLinkTarget(targetSkillPath)
				if err != nil {
					dt.Items = append(dt.Items, diffItem{Skill: skill.FlatName, Action: "update", Reason: "link target unreadable"})
					continue
				}
				absSource, _ := filepath.Abs(skill.SourcePath)
				if !utils.PathsEqual(absLink, absSource) {
					dt.Items = append(dt.Items, diffItem{Skill: skill.FlatName, Action: "update", Reason: "symlink points elsewhere"})
				}
			} else {
				dt.Items = append(dt.Items, diffItem{Skill: skill.FlatName, Action: "skip", Reason: "local copy (sync --force to replace)"})
			}
		}

		// Check for orphans
		entries, _ := os.ReadDir(target.Path)
		validNames := make(map[string]bool)
		for _, skill := range discovered {
			validNames[skill.FlatName] = true
		}
		for _, entry := range entries {
			eName := entry.Name()
			if utils.IsHidden(eName) {
				continue
			}
			if !validNames[eName] {
				_, err := os.Lstat(filepath.Join(target.Path, eName))
				if err != nil {
					continue
				}
				entryPath := filepath.Join(target.Path, eName)
				if utils.IsSymlinkOrJunction(entryPath) {
					absLink, err := utils.ResolveLinkTarget(entryPath)
					if err != nil {
						continue
					}
					absSource, _ := filepath.Abs(s.cfg.Source)
					if utils.PathHasPrefix(absLink, absSource+string(filepath.Separator)) {
						dt.Items = append(dt.Items, diffItem{Skill: eName, Action: "prune", Reason: "orphan symlink"})
					}
				} else {
					dt.Items = append(dt.Items, diffItem{Skill: eName, Action: "local", Reason: "local only"})
				}
			}
		}

		diffs = append(diffs, dt)
	}

	writeJSON(w, map[string]any{"diffs": diffs})
}
