package main

import (
	"fmt"
	"os"
	"time"

	"skillshare/internal/config"
	"skillshare/internal/skillignore"
	"skillshare/internal/sync"
	"skillshare/internal/trash"
	"skillshare/internal/ui"
)

func cmdSyncProject(root string, resources resourceSelection, dryRun, force, jsonOutput bool) (syncLogStats, []syncTargetResult, *skillignore.IgnoreStats, error) {
	start := time.Now()
	stats := syncLogStats{
		DryRun:       dryRun,
		Force:        force,
		ProjectScope: true,
		Resources:    resources.names(),
	}

	if !projectConfigExists(root) {
		if err := performProjectInit(root, projectInitOptions{}); err != nil {
			return stats, nil, nil, err
		}
	}

	runtime, err := loadProjectRuntime(root)
	if err != nil {
		return stats, nil, nil, err
	}
	stats.Targets = len(runtime.config.Targets)
	var entries []syncTargetEntry
	notFoundCount := 0
	for _, entry := range runtime.config.Targets {
		name := entry.Name
		target, ok := runtime.targets[name]
		if !ok {
			if !jsonOutput {
				ui.Error("%s: target not found", name)
			}
			notFoundCount++
			continue
		}
		mode := target.SkillsConfig().Mode
		if mode == "" {
			mode = "merge"
		}
		entries = append(entries, syncTargetEntry{name: name, target: target, mode: mode})
	}

	var discoveredSkills []sync.DiscoveredSkill
	var ignoreStats *skillignore.IgnoreStats
	var skillSyncErr error
	if resources.skills {
		if _, err := os.Stat(runtime.sourcePath); os.IsNotExist(err) {
			sourceErr := fmt.Errorf("source directory does not exist: %s", runtime.sourcePath)
			if !resources.includesManaged() {
				return stats, nil, nil, sourceErr
			}
			skillSyncErr = sourceErr
		} else {
			var spinner *ui.Spinner
			if !jsonOutput {
				spinner = ui.StartSpinner("Discovering skills")
			}
			discoveredSkills, ignoreStats, err = sync.DiscoverSourceSkillsWithStats(runtime.sourcePath)
			if err != nil {
				if spinner != nil {
					spinner.Fail("Discovery failed")
				}
				if !resources.includesManaged() {
					return stats, nil, nil, err
				}
				skillSyncErr = err
			} else if spinner != nil {
				spinner.Success(fmt.Sprintf("Discovered %d skills", len(discoveredSkills)))
				reportCollisions(discoveredSkills, runtime.targets)
			}
		}
	}

	if !jsonOutput {
		switch {
		case resources.skills && resources.includesManaged():
			ui.Header("Syncing skills and resources (project)")
		case resources.skills:
			ui.Header("Syncing skills (project)")
		default:
			ui.Header("Syncing resources (project)")
		}
		if dryRun {
			ui.Warning("Dry run mode - no changes will be made")
		}
	}

	var results []syncTargetResult
	var failedTargets int
	if resources.skills {
		if skillSyncErr != nil {
			results, failedTargets = syncResultsForSkillError(entries, skillSyncErr)
		} else if jsonOutput || resources.includesManaged() {
			results, failedTargets = runParallelSyncQuiet(entries, runtime.sourcePath, discoveredSkills, dryRun, force, root)
		} else {
			results, failedTargets = runParallelSync(entries, runtime.sourcePath, discoveredSkills, dryRun, force, root)
		}
	}
	if resources.includesManaged() {
		var managedFailed int
		results, managedFailed = syncManagedResourcesForEntries(entries, results, resources, root, dryRun)
		failedTargets += managedFailed
		if !jsonOutput {
			renderSyncResults(results)
		}
	}
	failedTargets += notFoundCount

	var totals syncModeStats
	for _, r := range results {
		totals.linked += r.stats.linked
		totals.local += r.stats.local
		totals.updated += r.stats.updated
		totals.pruned += r.stats.pruned
	}
	stats.Failed = failedTargets

	if !jsonOutput {
		// Phase 3: Summary
		ui.SyncSummary(ui.SyncStats{
			Targets:  len(runtime.config.Targets),
			Linked:   totals.linked,
			Local:    totals.local,
			Updated:  totals.updated,
			Pruned:   totals.pruned,
			Duration: time.Since(start),
		})

		// Show ignored skills from .skillignore
		printIgnoredSkills(ignoreStats)
	}

	if !dryRun {
		if n, _ := trash.Cleanup(trash.ProjectTrashDir(root), 0); n > 0 {
			if !jsonOutput {
				ui.Info("Cleaned up %d expired trash item(s)", n)
			}
		}

		// Registry entries are managed by install/uninstall, not sync.
		// Sync only manages symlinks — it must not prune registry entries
		// for installed skills whose files may be missing from disk.
	}

	if failedTargets > 0 {
		return stats, results, ignoreStats, fmt.Errorf("some targets failed to sync")
	}

	return stats, results, ignoreStats, nil
}

func projectTargetDisplayPath(entry config.ProjectTargetEntry) string {
	if p := entry.SkillsConfig().Path; p != "" {
		return p
	}
	if known, ok := config.LookupProjectTarget(entry.Name); ok {
		return known.Path
	}
	return ""
}
