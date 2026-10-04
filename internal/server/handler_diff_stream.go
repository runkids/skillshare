package server

import (
	"maps"
	"net/http"
	"os"
	"path/filepath"

	"skillshare/internal/config"
	"skillshare/internal/sourcewalk"
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
	source := s.cfg.EffectiveSkillsSource()
	follow := s.skillFollowSet()
	agentsSource := s.agentsSource()
	globalMode := s.cfg.Mode
	ignorePatterns := ssync.EffectiveFileIgnorePatterns(s.cfg.Ignore)
	targets := s.cloneTargets()
	s.mu.RUnlock()

	if globalMode == "" {
		globalMode = "merge"
	}

	safeSend("discovering", map[string]string{"phase": "scanning source directory"})

	discovered, ignoreStats, err := ssync.DiscoverSourceSkillsWithOptions(source, ssync.DiscoveryOptions{Follow: follow, CollectIgnored: true})
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

		dt := s.computeTargetDiff(name, target, discovered, globalMode, source, ignorePatterns, follow)
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
// follow is the operation's set, so the diff makes sync's prune and link
// identity decisions; a nil set keeps the legacy comparisons.
func (s *Server) computeTargetDiff(name string, target config.TargetConfig, discovered []ssync.DiscoveredSkill, globalMode, source string, ignorePatterns []string, follow *sourcewalk.FollowSet) diffTarget {
	sc := target.SkillsConfig()
	mode := sc.Mode
	if mode == "" {
		mode = globalMode
	}

	dt := diffTarget{Target: name, Items: make([]diffItem, 0)}
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

	// While an entry is unavailable sync prunes nothing, so neither does the preview.
	dt.PrunePaused = ssync.PrunePaused(source, follow)
	paused := len(dt.PrunePaused) > 0

	filtered, err := ssync.FilterSkills(discovered, sc.Include, sc.Exclude)
	if err != nil {
		return dt
	}
	filtered = ssync.FilterSkillsByTarget(filtered, name)
	resolution, err := ssync.ResolveTargetSkillsForTarget(name, config.ResourceTargetConfig{
		Path:         sc.Path,
		TargetNaming: sc.TargetNaming,
	}, filtered)
	if err != nil {
		dt.Items = append(dt.Items, diffItem{Skill: "(target naming)", Action: "skip", Reason: err.Error(), Kind: kindSkill})
		return dt
	}
	// Surface collision/validation stats so the UI can show why skills were skipped
	dt.CollisionCount = len(resolution.Collisions)
	dt.SkippedCount = len(filtered) - len(resolution.Skills)
	validNames := resolution.ValidTargetNames()
	legacyNames := resolution.LegacyFlatNames()

	if mode == "copy" {
		keepManaged := ssync.KeepsManagedCopies(source, sc.TargetNaming, follow)
		kept := diffItem{Action: "skip", Reason: "managed copy kept while prune is paused; origin cannot be proven", Kind: kindSkill}
		manifest, _ := ssync.ReadManifest(sc.Path)
		for _, resolved := range resolution.Skills {
			skill := resolved.Skill
			oldChecksum, isManaged := manifest.Managed[resolved.TargetName]
			targetSkillPath := filepath.Join(sc.Path, resolved.TargetName)
			if !isManaged {
				if info, statErr := os.Stat(targetSkillPath); statErr == nil {
					if info.IsDir() {
						dt.Items = append(dt.Items, diffItem{Skill: resolved.TargetName, Action: "skip", Reason: "local copy (sync --force to replace)", Kind: kindSkill})
					} else {
						dt.Items = append(dt.Items, diffItem{Skill: resolved.TargetName, Action: "update", Reason: "target entry is not a directory", Kind: kindSkill})
					}
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
					if keepManaged {
						kept.Skill = resolved.TargetName
						dt.Items = append(dt.Items, kept)
						continue
					}
					dt.Items = append(dt.Items, diffItem{Skill: resolved.TargetName, Action: "update", Reason: "target entry is not a directory", Kind: kindSkill})
				} else {
					oldMtime := manifest.Mtimes[resolved.TargetName]
					currentMtime, mtimeErr := ssync.DirMaxMtimeWithIgnore(skill.SourcePath, ignorePatterns)
					if mtimeErr == nil && oldMtime > 0 && currentMtime == oldMtime {
						continue
					}
					srcChecksum, checksumErr := ssync.DirChecksumWithIgnore(skill.SourcePath, ignorePatterns)
					if checksumErr != nil {
						dt.Items = append(dt.Items, diffItem{Skill: resolved.TargetName, Action: "update", Reason: "cannot compute checksum", Kind: kindSkill})
					} else if srcChecksum != oldChecksum && keepManaged {
						kept.Skill = resolved.TargetName
						dt.Items = append(dt.Items, kept)
					} else if srcChecksum != oldChecksum {
						dt.Items = append(dt.Items, diffItem{Skill: resolved.TargetName, Action: "update", Reason: "content changed", Kind: kindSkill})
					}
				}
			}
		}
		for managedName := range manifest.Managed {
			if paused {
				break
			}
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
			if os.IsNotExist(err) {
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
			same := utils.PathsEqual(absLink, absSource)
			if follow != nil {
				same = ssync.SameSkillLink(targetSkillPath, skill, source, follow)
			}
			if !same {
				dt.Items = append(dt.Items, diffItem{Skill: resolved.TargetName, Action: "update", Reason: "symlink points elsewhere", Kind: kindSkill})
			}
		} else {
			dt.Items = append(dt.Items, diffItem{Skill: resolved.TargetName, Action: "skip", Reason: "local copy (sync --force to replace)", Kind: kindSkill})
		}
	}

	// Orphan check
	entries, _ := os.ReadDir(sc.Path)
	manifest, _ := ssync.ReadManifest(sc.Path)
	for _, entry := range entries {
		eName := entry.Name()
		if manifest.SkipsHidden(eName) {
			continue
		}
		if _, keepLegacy := legacyNames[eName]; keepLegacy {
			continue
		}
		// prune reports an orphan unless prune is paused.
		prune := func(reason string) {
			if !paused {
				dt.Items = append(dt.Items, diffItem{Skill: eName, Action: "prune", Reason: reason, Kind: kindSkill})
			}
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
					prune("orphan symlink")
				}
			} else if info.IsDir() {
				if _, inManifest := manifest.Managed[eName]; inManifest {
					prune("orphan managed directory (manifest)")
				} else {
					if resolution.Naming == "flat" && (utils.HasNestedSeparator(eName) || utils.IsTrackedRepoDir(eName)) {
						prune("orphan managed directory")
					} else {
						dt.Items = append(dt.Items, diffItem{Skill: eName, Action: "local", Reason: "local only", Kind: kindSkill})
					}
				}
			}
		}
	}

	return dt
}
