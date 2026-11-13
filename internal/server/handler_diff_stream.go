package server

import (
	"maps"
	"net/http"
	"os"
	"path/filepath"
	"slices"

	"skillshare/internal/config"
	ssync "skillshare/internal/sync"
	"skillshare/internal/utils"
)

type profileDiffContext struct {
	aware bool
	root  string
	moved []ssync.MovedTarget
}

// Caller holds the config read lock; project state is owned by projectCfg.
func (s *Server) snapshotProfileDiff(targets map[string]config.TargetConfig) profileDiffContext {
	state := s.cfg.ProfileState()
	if s.IsProjectMode() {
		state = s.projectCfg.ProfileState()
	}
	return profileDiffContext{aware: state.Active != "" || state.Pending, root: s.projectRoot, moved: ssync.MovedProjectTargets(s.projectCfg, targets)}
}

// handleDiffStream serves an SSE endpoint that streams diff computation progress.
// Events:
//   - "discovering" → {"phase":"..."}                immediately on connect
//   - "start"       → {"total": N}                   after discovery (N = target count)
//   - "result"      → diffTarget                     per-target diff result
//   - "done"        → {"diffs":[...]}                final payload (same shape as GET /api/diff)
func (s *Server) handleDiffStream(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Snapshot config under RLock, then release before slow I/O.
	s.mu.RLock()
	if s.refuseUnavailableProfile(w) {
		s.mu.RUnlock()
		return
	}
	source := s.skillsSource()
	agentsSource := s.agentsSource()
	globalMode := s.cfg.Mode
	ignorePatterns := ssync.EffectiveFileIgnorePatterns(s.cfg.Ignore)
	targets := s.cloneTargets()
	preview := s.snapshotProfileDiff(targets)
	walk := s.skillsWalk()
	if s.IsProjectMode() {
		ignorePatterns = ssync.EffectiveFileIgnorePatterns(s.projectCfg.Ignore)
	}
	s.mu.RUnlock()

	safeSend, ok := initSSE(w)
	if !ok {
		return
	}

	if globalMode == "" {
		globalMode = "merge"
	}

	safeSend("discovering", map[string]string{"phase": "scanning source directory"})

	discovered, ignoreStats, err := ssync.DiscoverSourceSkillsWithStats(source, walk)
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

		dt := s.computeTargetDiff(name, target, discovered, globalMode, source, ignorePatterns, preview)
		diffs = append(diffs, dt)
		checked++

		safeSend("result", map[string]any{
			"diff":    dt,
			"checked": checked,
		})
	}

	diffs = s.appendAgentDiffs(diffs, targets, agentsSource, "")

	donePayload := map[string]any{"diffs": diffs}
	maps.Copy(donePayload, ignorePayload(ignoreStats))
	maps.Copy(donePayload, agentIgnorePayload(agentsSource, nil))
	safeSend("done", donePayload)
}

// computeTargetDiff computes the diff for a single target.
// Extracted from handleDiff to share logic with the stream handler.
func (s *Server) computeTargetDiff(name string, target config.TargetConfig, discovered []ssync.DiscoveredSkill, globalMode, source string, ignorePatterns []string, preview ...profileDiffContext) (dt diffTarget) {
	sc := target.SkillsConfig()
	mode := sc.Mode
	if mode == "" {
		mode = globalMode
	}

	dt = diffTarget{Target: name, Items: make([]diffItem, 0)}
	if !sc.IsEnabled() {
		return dt // skills off: nothing is synced, so nothing differs
	}

	if mode == "symlink" {
		status := ssync.CheckStatus(sc.Path, source)
		if status != ssync.StatusLinked {
			dt.Items = append(dt.Items, diffItem{Skill: "(entire directory)", Action: "link", Reason: "source only", Kind: kindSkill})
		}
		return dt
	}

	resolution, err := ssync.ResolveTargetSkillsForTarget(name, sc, discovered)
	if err != nil {
		dt.Items = append(dt.Items, diffItem{Skill: "(target naming)", Action: "skip", Reason: err.Error(), Kind: kindSkill})
		return dt
	}
	// Surface collision/validation stats so the UI can show why skills were skipped
	dt.CollisionCount = len(resolution.Collisions)
	dt.SkippedCount = resolution.SkippedCount
	dt.NamingSkippedCount = resolution.NamingSkippedCount
	dt.Warnings = resolution.Warnings
	for _, warning := range resolution.SelectionWarnings {
		dt.Items = append(dt.Items, diffItem{Skill: "(skill selection)", Action: "skip", Reason: warning, Kind: kindSkill})
	}
	validNames := resolution.ValidTargetNames()
	manifest, _ := ssync.ReadManifest(sc.Path)
	legacyNames := resolution.LegacyNames(mode, sc.Path, manifest)
	renamedFrom := ssync.RenamedFrom(legacyNames)
	if len(preview) > 0 && preview[0].aware && target.ProjectRoot() == "" {
		defer func() {
			var orphans []string
			for i, item := range dt.Items {
				if mode == "merge" && item.Reason == "local copy (sync --force to replace)" {
					entry, _ := ssync.PreviewEntry(item.Skill, sc.Path, source, discovered, manifest, ignorePatterns)
					if entry.Class == ssync.EntryCleanCopy {
						dt.Items[i].Action, dt.Items[i].Reason = "update", ssync.CopyToLinkReason
					}
				}
				if item.Action == "prune" {
					orphans = append(orphans, item.Skill)
				}
			}
			kept := ssync.PreviewPreservedEntries(name, sc.Path, source, mode, resolution, discovered, manifest, ignorePatterns, orphans...)
			if preview[0].root != "" {
				moved := slices.Clone(preview[0].moved)
				for i := range moved {
					moved[i].Disabled = moved[i].Disabled || moved[i].Name != name
				}
				kept = append(kept, ssync.PreviewMovedProjectEntries(preview[0].root, source, moved, discovered, ignorePatterns)...)
			}
			currentDir, _ := filepath.Abs(sc.Path)
			for _, change := range kept {
				action, projected := "kept", change.Entry
				if change.Step == "refresh" {
					action = "skip" // selected entries retain existing Force behavior
					if old, ok := legacyNames[change.Entry]; ok {
						projected = old.TargetName
					}
				}
				if change.Dir == currentDir {
					dt.Items = slices.DeleteFunc(dt.Items, func(item diffItem) bool { return item.Skill == change.Entry || item.Skill == projected })
				}
				dt.Items = append(dt.Items, diffItem{Skill: change.Entry, Action: action, Reason: change.Reason, Kind: kindSkill})
			}
		}()
	}

	if mode == "copy" {
		for _, resolved := range resolution.Skills {
			skill := resolved.Skill
			oldChecksum, isManaged := manifest.Managed[resolved.TargetName]
			targetSkillPath := filepath.Join(sc.Path, resolved.TargetName)
			if !isManaged {
				if info, statErr := os.Stat(targetSkillPath); statErr == nil {
					if old, ok := renamedFrom[resolved.TargetName]; ok {
						dt.Items = append(dt.Items, diffItem{Skill: resolved.TargetName, Action: "kept", Reason: ssync.KeptLegacyReason(old), Kind: kindSkill})
					} else if info.IsDir() {
						dt.Items = append(dt.Items, diffItem{Skill: resolved.TargetName, Action: "skip", Reason: "local copy (sync --force to replace)", Kind: kindSkill})
					} else {
						dt.Items = append(dt.Items, diffItem{Skill: resolved.TargetName, Action: "update", Reason: "target entry is not a directory", Kind: kindSkill})
					}
				} else if old, ok := renamedFrom[resolved.TargetName]; ok && os.IsNotExist(statErr) {
					dt.Items = append(dt.Items, diffItem{Skill: resolved.TargetName, Action: "update", Reason: ssync.RenameReason(old), Kind: kindSkill})
				} else if os.IsNotExist(statErr) {
					dt.Items = append(dt.Items, diffItem{Skill: resolved.TargetName, Action: "link", Reason: "source only", Kind: kindSkill})
				} else {
					dt.Items = append(dt.Items, diffItem{Skill: resolved.TargetName, Action: "update", Reason: "cannot access target entry", Kind: kindSkill})
				}
			} else {
				targetInfo, statErr := os.Stat(targetSkillPath)
				if os.IsNotExist(statErr) {
					dt.Items = append(dt.Items, diffItem{Skill: resolved.TargetName, Action: "link", Reason: "missing (deleted from target)", Kind: kindSkill})
				} else if statErr != nil {
					dt.Items = append(dt.Items, diffItem{Skill: resolved.TargetName, Action: "update", Reason: "cannot access target entry", Kind: kindSkill})
				} else if !targetInfo.IsDir() {
					dt.Items = append(dt.Items, diffItem{Skill: resolved.TargetName, Action: "update", Reason: "target entry is not a directory", Kind: kindSkill})
				} else if recorded := manifest.Naming[resolved.TargetName]; recorded != "" && recorded != resolution.Naming {
					// A copy made under another naming carries another name:, so sync re-copies it.
					dt.Items = append(dt.Items, diffItem{Skill: resolved.TargetName, Action: "update", Reason: ssync.NamingChangedReason, Kind: kindSkill})
				} else {
					oldMtime := manifest.Mtimes[resolved.TargetName]
					currentMtime, mtimeErr := ssync.DirMaxMtimeWithIgnore(skill.SourcePath, ignorePatterns)
					if !(len(preview) > 0 && preview[0].aware) && mtimeErr == nil && oldMtime > 0 && currentMtime == oldMtime {
						continue
					}
					srcChecksum, checksumErr := ssync.DirChecksumWithIgnore(skill.SourcePath, ignorePatterns)
					if checksumErr != nil {
						dt.Items = append(dt.Items, diffItem{Skill: resolved.TargetName, Action: "update", Reason: "cannot compute checksum", Kind: kindSkill})
					} else if srcChecksum != oldChecksum {
						dt.Items = append(dt.Items, diffItem{Skill: resolved.TargetName, Action: "update", Reason: "content changed", Kind: kindSkill})
					}
				}
			}
		}
		for managedName := range manifest.Managed {
			if _, keepLegacy := legacyNames[managedName]; keepLegacy {
				continue
			}
			if !validNames[managedName] {
				dt.Items = append(dt.Items, diffItem{Skill: managedName, Action: "prune", Reason: "orphan copy", Kind: kindSkill})
			}
		}
		return dt
	}

	// Merge mode
	for _, resolved := range resolution.Skills {
		skill := resolved.Skill
		targetSkillPath := filepath.Join(sc.Path, resolved.TargetName)
		_, err := os.Lstat(targetSkillPath)
		if err != nil {
			if old, ok := renamedFrom[resolved.TargetName]; ok && os.IsNotExist(err) {
				dt.Items = append(dt.Items, diffItem{Skill: resolved.TargetName, Action: "update", Reason: ssync.RenameReason(old), Kind: kindSkill})
			} else if os.IsNotExist(err) {
				dt.Items = append(dt.Items, diffItem{Skill: resolved.TargetName, Action: "link", Reason: "source only", Kind: kindSkill})
			}
			continue
		}

		if utils.IsSymlinkOrJunction(targetSkillPath) {
			absLink, linkErr := utils.ResolveLinkTarget(targetSkillPath)
			if linkErr != nil {
				dt.Items = append(dt.Items, diffItem{Skill: resolved.TargetName, Action: "update", Reason: "link target unreadable", Kind: kindSkill})
				continue
			}
			absSource, _ := filepath.Abs(skill.SourcePath)
			if !utils.PathsEqual(absLink, absSource) {
				dt.Items = append(dt.Items, diffItem{Skill: resolved.TargetName, Action: "update", Reason: "symlink points elsewhere", Kind: kindSkill})
			}
		} else if old, ok := renamedFrom[resolved.TargetName]; ok {
			dt.Items = append(dt.Items, diffItem{Skill: resolved.TargetName, Action: "kept", Reason: ssync.KeptLegacyReason(old), Kind: kindSkill})
		} else if manifest.OwnsCopy(resolved.TargetName, targetSkillPath) {
			dt.Items = append(dt.Items, diffItem{Skill: resolved.TargetName, Action: "update", Reason: ssync.CopyToLinkReason, Kind: kindSkill})
		} else {
			dt.Items = append(dt.Items, diffItem{Skill: resolved.TargetName, Action: "skip", Reason: "local copy (sync --force to replace)", Kind: kindSkill})
		}
	}

	// Orphan check
	entries, _ := os.ReadDir(sc.Path)
	for _, entry := range entries {
		eName := entry.Name()
		if manifest.SkipsHidden(eName) {
			continue
		}
		if _, keepLegacy := legacyNames[eName]; keepLegacy {
			continue
		}
		entryPath := filepath.Join(sc.Path, eName)
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
					dt.Items = append(dt.Items, diffItem{Skill: eName, Action: "prune", Reason: "orphan symlink", Kind: kindSkill})
				}
			} else if info.IsDir() {
				if _, inManifest := manifest.Managed[eName]; inManifest {
					dt.Items = append(dt.Items, diffItem{Skill: eName, Action: "prune", Reason: "orphan managed directory (manifest)", Kind: kindSkill})
				} else {
					if resolution.Naming == "flat" && (utils.HasNestedSeparator(eName) || utils.IsTrackedRepoDir(eName)) {
						dt.Items = append(dt.Items, diffItem{Skill: eName, Action: "prune", Reason: "orphan managed directory", Kind: kindSkill})
					} else {
						dt.Items = append(dt.Items, diffItem{Skill: eName, Action: "local", Reason: "local only", Kind: kindSkill})
					}
				}
			}
		}
	}

	return dt
}
