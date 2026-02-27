package server

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"time"

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
	allSkills, err := s.cache.Discover(s.cfg.Source)
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

	s.cache.Invalidate(s.cfg.Source)

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
	filterTarget := r.URL.Query().Get("target")

	globalMode := s.cfg.Mode
	if globalMode == "" {
		globalMode = "merge"
	}

	discovered, err := s.cache.Discover(s.cfg.Source)
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
		filtered := discovered

		if mode == "symlink" {
			status := ssync.CheckStatus(target.Path, s.cfg.Source)
			if status != ssync.StatusLinked {
				dt.Items = append(dt.Items, diffItem{Skill: "(entire directory)", Action: "link", Reason: "source only"})
			}
			diffs = append(diffs, dt)
			continue
		}
		filtered, err = ssync.FilterSkills(discovered, target.Include, target.Exclude)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid include/exclude for target "+name+": "+err.Error())
			return
		}

		if mode == "copy" {
			// Copy mode: check via manifest + checksum comparison
			manifest, _ := ssync.ReadManifest(target.Path)
			for _, skill := range filtered {
				oldChecksum, isManaged := manifest.Managed[skill.FlatName]
				targetSkillPath := filepath.Join(target.Path, skill.FlatName)
				if !isManaged {
					if info, err := os.Stat(targetSkillPath); err == nil {
						if info.IsDir() {
							dt.Items = append(dt.Items, diffItem{Skill: skill.FlatName, Action: "skip", Reason: "local copy (sync --force to replace)"})
						} else {
							dt.Items = append(dt.Items, diffItem{Skill: skill.FlatName, Action: "update", Reason: "target entry is not a directory"})
						}
					} else if os.IsNotExist(err) {
						dt.Items = append(dt.Items, diffItem{Skill: skill.FlatName, Action: "link", Reason: "source only"})
					} else {
						dt.Items = append(dt.Items, diffItem{Skill: skill.FlatName, Action: "update", Reason: "cannot access target entry"})
					}
				} else {
					// Verify target directory still exists
					targetInfo, statErr := os.Stat(targetSkillPath)
					if os.IsNotExist(statErr) {
						dt.Items = append(dt.Items, diffItem{Skill: skill.FlatName, Action: "link", Reason: "missing (deleted from target)"})
					} else if statErr != nil {
						dt.Items = append(dt.Items, diffItem{Skill: skill.FlatName, Action: "update", Reason: "cannot access target entry"})
					} else if !targetInfo.IsDir() {
						dt.Items = append(dt.Items, diffItem{Skill: skill.FlatName, Action: "update", Reason: "target entry is not a directory"})
					} else {
						// mtime fast-path: skip checksum if source mtime unchanged
						oldMtime := manifest.Mtimes[skill.FlatName]
						currentMtime, mtimeErr := ssync.DirMaxMtime(skill.SourcePath)
						if mtimeErr == nil && oldMtime > 0 && currentMtime == oldMtime {
							continue // unchanged
						}
						// Compare checksums to detect content drift
						srcChecksum, err := ssync.DirChecksum(skill.SourcePath)
						if err != nil {
							dt.Items = append(dt.Items, diffItem{Skill: skill.FlatName, Action: "update", Reason: "cannot compute checksum"})
						} else if srcChecksum != oldChecksum {
							dt.Items = append(dt.Items, diffItem{Skill: skill.FlatName, Action: "update", Reason: "content changed"})
						}
					}
				}
			}
			// Check for orphan managed copies
			validNames := make(map[string]bool)
			for _, skill := range filtered {
				validNames[skill.FlatName] = true
			}
			for managedName := range manifest.Managed {
				if !validNames[managedName] {
					dt.Items = append(dt.Items, diffItem{Skill: managedName, Action: "prune", Reason: "orphan copy"})
				}
			}
			diffs = append(diffs, dt)
			continue
		}

		// Merge mode: check each skill
		for _, skill := range filtered {
			targetSkillPath := filepath.Join(target.Path, skill.FlatName)
			_, err := os.Lstat(targetSkillPath)
			if err != nil {
				if os.IsNotExist(err) {
					dt.Items = append(dt.Items, diffItem{Skill: skill.FlatName, Action: "link", Reason: "source only"})
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
		for _, skill := range filtered {
			validNames[skill.FlatName] = true
		}
		manifest, _ := ssync.ReadManifest(target.Path)
		for _, entry := range entries {
			eName := entry.Name()
			if utils.IsHidden(eName) {
				continue
			}
			managed, err := ssync.ShouldSyncFlatName(eName, target.Include, target.Exclude)
			if err != nil {
				writeError(w, http.StatusBadRequest, "invalid include/exclude for target "+name+": "+err.Error())
				return
			}
			entryPath := filepath.Join(target.Path, eName)
			if !managed {
				if utils.IsSymlinkOrJunction(entryPath) {
					absLink, err := utils.ResolveLinkTarget(entryPath)
					if err == nil {
						absSource, _ := filepath.Abs(s.cfg.Source)
						if utils.PathHasPrefix(absLink, absSource+string(filepath.Separator)) {
							dt.Items = append(dt.Items, diffItem{Skill: eName, Action: "prune", Reason: "excluded by filter"})
						}
					}
				} else if _, inManifest := manifest.Managed[eName]; inManifest {
					dt.Items = append(dt.Items, diffItem{Skill: eName, Action: "prune", Reason: "excluded managed directory"})
				}
				continue
			}
			if !validNames[eName] {
				info, err := os.Lstat(entryPath)
				if err != nil {
					continue
				}
				if utils.IsSymlinkOrJunction(entryPath) {
					absLink, err := utils.ResolveLinkTarget(entryPath)
					if err != nil {
						continue
					}
					absSource, _ := filepath.Abs(s.cfg.Source)
					if utils.PathHasPrefix(absLink, absSource+string(filepath.Separator)) {
						dt.Items = append(dt.Items, diffItem{Skill: eName, Action: "prune", Reason: "orphan symlink"})
					}
				} else if info.IsDir() {
					if _, inManifest := manifest.Managed[eName]; inManifest {
						dt.Items = append(dt.Items, diffItem{Skill: eName, Action: "prune", Reason: "orphan managed directory (manifest)"})
					} else {
						dt.Items = append(dt.Items, diffItem{Skill: eName, Action: "local", Reason: "local only"})
					}
				}
			}
		}

		diffs = append(diffs, dt)
	}

	writeJSON(w, map[string]any{"diffs": diffs})
}
