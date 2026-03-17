package server

import (
	"net/http"
	"os"
	"path/filepath"

	ssync "skillshare/internal/sync"
	"skillshare/internal/utils"
)

// handleDiffStream serves an SSE endpoint that streams diff computation progress.
// Events:
//   - "discovering" → {"phase":"..."}                immediately on connect
//   - "start"       → {"total": N}                   after discovery (N = target count)
//   - "result"      → diffTarget                     per-target diff result
//   - "done"        → {"diffs":[...]}                final payload (same shape as GET /api/diff)
func (s *Server) handleDiffStream(w http.ResponseWriter, r *http.Request) {
	safeSend, ok := initSSE(w)
	if !ok {
		return
	}

	ctx := r.Context()

	// Snapshot config under RLock, then release before slow I/O.
	s.mu.RLock()
	source := s.cfg.Source
	globalMode := s.cfg.Mode
	targets := s.cloneTargets()
	s.mu.RUnlock()

	if globalMode == "" {
		globalMode = "merge"
	}

	safeSend("discovering", map[string]string{"phase": "scanning source directory"})

	discovered, ignoreStats, err := ssync.DiscoverSourceSkillsWithStats(source)
	if err != nil {
		safeSend("error", map[string]string{"error": err.Error()})
		return
	}

	safeSend("start", map[string]int{"total": len(targets)})

	var diffs []diffTarget
	checked := 0

	for name, target := range targets {
		select {
		case <-ctx.Done():
			return
		default:
		}

		dt := s.computeTargetDiff(name, target, discovered, globalMode, source)
		diffs = append(diffs, dt)
		checked++

		safeSend("result", map[string]any{
			"diff":    dt,
			"checked": checked,
		})
	}

	donePayload := map[string]any{"diffs": diffs}
	for k, v := range ignorePayload(ignoreStats) {
		donePayload[k] = v
	}
	safeSend("done", donePayload)
}

// computeTargetDiff computes the diff for a single target.
// Extracted from handleDiff to share logic with the stream handler.
func (s *Server) computeTargetDiff(name string, target struct {
	Path    string   `yaml:"path"`
	Mode    string   `yaml:"mode,omitempty"`
	Include []string `yaml:"include,omitempty"`
	Exclude []string `yaml:"exclude,omitempty"`
}, discovered []ssync.DiscoveredSkill, globalMode, source string) diffTarget {
	mode := target.Mode
	if mode == "" {
		mode = globalMode
	}

	dt := diffTarget{Target: name, Items: make([]diffItem, 0)}

	if mode == "symlink" {
		status := ssync.CheckStatus(target.Path, source)
		if status != ssync.StatusLinked {
			dt.Items = append(dt.Items, diffItem{Skill: "(entire directory)", Action: "link", Reason: "source only"})
		}
		return dt
	}

	filtered, err := ssync.FilterSkills(discovered, target.Include, target.Exclude)
	if err != nil {
		return dt
	}

	if mode == "copy" {
		manifest, _ := ssync.ReadManifest(target.Path)
		for _, skill := range filtered {
			oldChecksum, isManaged := manifest.Managed[skill.FlatName]
			targetSkillPath := filepath.Join(target.Path, skill.FlatName)
			if !isManaged {
				if info, statErr := os.Stat(targetSkillPath); statErr == nil {
					if info.IsDir() {
						dt.Items = append(dt.Items, diffItem{Skill: skill.FlatName, Action: "skip", Reason: "local copy (sync --force to replace)"})
					} else {
						dt.Items = append(dt.Items, diffItem{Skill: skill.FlatName, Action: "update", Reason: "target entry is not a directory"})
					}
				} else if os.IsNotExist(statErr) {
					dt.Items = append(dt.Items, diffItem{Skill: skill.FlatName, Action: "link", Reason: "source only"})
				} else {
					dt.Items = append(dt.Items, diffItem{Skill: skill.FlatName, Action: "update", Reason: "cannot access target entry"})
				}
			} else {
				targetInfo, statErr := os.Stat(targetSkillPath)
				if os.IsNotExist(statErr) {
					dt.Items = append(dt.Items, diffItem{Skill: skill.FlatName, Action: "link", Reason: "missing (deleted from target)"})
				} else if statErr != nil {
					dt.Items = append(dt.Items, diffItem{Skill: skill.FlatName, Action: "update", Reason: "cannot access target entry"})
				} else if !targetInfo.IsDir() {
					dt.Items = append(dt.Items, diffItem{Skill: skill.FlatName, Action: "update", Reason: "target entry is not a directory"})
				} else {
					oldMtime := manifest.Mtimes[skill.FlatName]
					currentMtime, mtimeErr := ssync.DirMaxMtime(skill.SourcePath)
					if mtimeErr == nil && oldMtime > 0 && currentMtime == oldMtime {
						return dt // unchanged, but continue loop below
					}
					srcChecksum, checksumErr := ssync.DirChecksum(skill.SourcePath)
					if checksumErr != nil {
						dt.Items = append(dt.Items, diffItem{Skill: skill.FlatName, Action: "update", Reason: "cannot compute checksum"})
					} else if srcChecksum != oldChecksum {
						dt.Items = append(dt.Items, diffItem{Skill: skill.FlatName, Action: "update", Reason: "content changed"})
					}
				}
			}
		}
		validNames := make(map[string]bool)
		for _, skill := range filtered {
			validNames[skill.FlatName] = true
		}
		for managedName := range manifest.Managed {
			if !validNames[managedName] {
				dt.Items = append(dt.Items, diffItem{Skill: managedName, Action: "prune", Reason: "orphan copy"})
			}
		}
		return dt
	}

	// Merge mode
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
			absLink, linkErr := utils.ResolveLinkTarget(targetSkillPath)
			if linkErr != nil {
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

	// Orphan check
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
		managed, filterErr := ssync.ShouldSyncFlatName(eName, target.Include, target.Exclude)
		if filterErr != nil {
			continue
		}
		entryPath := filepath.Join(target.Path, eName)
		if !managed {
			if utils.IsSymlinkOrJunction(entryPath) {
				absLink, linkErr := utils.ResolveLinkTarget(entryPath)
				if linkErr == nil {
					absSource, _ := filepath.Abs(source)
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
			info, statErr := os.Lstat(entryPath)
			if statErr != nil {
				continue
			}
			if utils.IsSymlinkOrJunction(entryPath) {
				absLink, linkErr := utils.ResolveLinkTarget(entryPath)
				if linkErr != nil {
					continue
				}
				absSource, _ := filepath.Abs(source)
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

	return dt
}
