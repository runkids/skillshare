package main

import (
	"fmt"
	"os"
	"time"

	"skillshare/internal/config"
	"skillshare/internal/sync"
	"skillshare/internal/trash"
	"skillshare/internal/ui"
)

func cmdSyncProject(root string, dryRun, force, jsonOutput bool) (syncLogStats, []syncTargetResult, error) {
	start := time.Now()
	stats := syncLogStats{
		DryRun:       dryRun,
		Force:        force,
		ProjectScope: true,
	}

	if !projectConfigExists(root) {
		if err := performProjectInit(root, projectInitOptions{}); err != nil {
			return stats, nil, err
		}
	}

	runtime, err := loadProjectRuntime(root)
	if err != nil {
		return stats, nil, err
	}
	stats.Targets = len(runtime.config.Targets)

	if _, err := os.Stat(runtime.sourcePath); os.IsNotExist(err) {
		return stats, nil, fmt.Errorf("source directory does not exist: %s", runtime.sourcePath)
	}

	// Phase 1: Discovery
	var discoveredSkills []sync.DiscoveredSkill
	if jsonOutput {
		fmt.Fprintf(os.Stderr, "Discovering skills...\n")
		var discoverErr error
		discoveredSkills, discoverErr = sync.DiscoverSourceSkills(runtime.sourcePath)
		if discoverErr != nil {
			return stats, nil, discoverErr
		}
		fmt.Fprintf(os.Stderr, "Discovered %d skills\n", len(discoveredSkills))
	} else {
		spinner := ui.StartSpinner("Discovering skills")
		var discoverErr error
		discoveredSkills, discoverErr = sync.DiscoverSourceSkills(runtime.sourcePath)
		if discoverErr != nil {
			spinner.Fail("Discovery failed")
			return stats, nil, discoverErr
		}
		spinner.Success(fmt.Sprintf("Discovered %d skills", len(discoveredSkills)))
		reportCollisions(discoveredSkills, runtime.targets)
	}

	// Phase 2: Per-target sync
	if !jsonOutput {
		ui.Header("Syncing skills (project)")
		if dryRun {
			ui.Warning("Dry run mode - no changes will be made")
		}
	}

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
		mode := target.Mode
		if mode == "" {
			mode = "merge"
		}
		entries = append(entries, syncTargetEntry{name: name, target: target, mode: mode})
	}

	var results []syncTargetResult
	var failedTargets int
	if jsonOutput {
		results, failedTargets = runParallelSyncQuiet(entries, runtime.sourcePath, discoveredSkills, dryRun, force)
	} else {
		results, failedTargets = runParallelSync(entries, runtime.sourcePath, discoveredSkills, dryRun, force)
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
	}

	if failedTargets > 0 {
		return stats, results, fmt.Errorf("some targets failed to sync")
	}

	// Opportunistic cleanup of expired trash items
	if !dryRun {
		if n, _ := trash.Cleanup(trash.ProjectTrashDir(root), 0); n > 0 {
			if !jsonOutput {
				ui.Info("Cleaned up %d expired trash item(s)", n)
			}
		}
	}

	return stats, results, nil
}

func projectTargetDisplayPath(entry config.ProjectTargetEntry) string {
	if entry.Path != "" {
		return entry.Path
	}
	if known, ok := config.LookupProjectTarget(entry.Name); ok {
		return known.Path
	}
	return ""
}
