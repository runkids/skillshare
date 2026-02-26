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

func cmdSyncProject(root string, dryRun, force bool) (syncLogStats, error) {
	start := time.Now()
	stats := syncLogStats{
		DryRun:       dryRun,
		Force:        force,
		ProjectScope: true,
	}

	if !projectConfigExists(root) {
		if err := performProjectInit(root, projectInitOptions{}); err != nil {
			return stats, err
		}
	}

	runtime, err := loadProjectRuntime(root)
	if err != nil {
		return stats, err
	}
	stats.Targets = len(runtime.config.Targets)

	if _, err := os.Stat(runtime.sourcePath); os.IsNotExist(err) {
		return stats, fmt.Errorf("source directory does not exist: %s", runtime.sourcePath)
	}

	// Phase 1: Discovery — spinner
	spinner := ui.StartSpinner("Discovering skills")
	discoveredSkills, discoverErr := sync.DiscoverSourceSkills(runtime.sourcePath)
	if discoverErr != nil {
		spinner.Fail("Discovery failed")
		return stats, discoverErr
	}
	spinner.Success(fmt.Sprintf("Discovered %d skills", len(discoveredSkills)))
	reportCollisions(discoveredSkills, runtime.targets)

	// Phase 2: Per-target sync
	ui.Header("Syncing skills (project)")
	if dryRun {
		ui.Warning("Dry run mode - no changes will be made")
	}

	failedTargets := 0
	var totals syncModeStats
	for _, entry := range runtime.config.Targets {
		name := entry.Name
		target, ok := runtime.targets[name]
		if !ok {
			ui.Error("%s: target not found", name)
			failedTargets++
			continue
		}

		mode := target.Mode
		if mode == "" {
			mode = "merge"
		}

		var s syncModeStats
		var syncErr error
		switch mode {
		case "symlink":
			syncErr = syncSymlinkMode(name, target, runtime.sourcePath, dryRun, force)
		case "copy":
			s, syncErr = syncCopyModeWithSkills(name, target, runtime.sourcePath, discoveredSkills, dryRun, force)
		default:
			s, syncErr = syncMergeModeWithSkills(name, target, runtime.sourcePath, discoveredSkills, dryRun, force)
		}
		if syncErr != nil {
			ui.Error("%s: %v", name, syncErr)
			failedTargets++
		}
		totals.linked += s.linked
		totals.local += s.local
		totals.updated += s.updated
		totals.pruned += s.pruned
	}
	stats.Failed = failedTargets

	// Phase 3: Summary
	ui.SyncSummary(ui.SyncStats{
		Targets:  len(runtime.config.Targets),
		Linked:   totals.linked,
		Local:    totals.local,
		Updated:  totals.updated,
		Pruned:   totals.pruned,
		Duration: time.Since(start),
	})

	if failedTargets > 0 {
		return stats, fmt.Errorf("some targets failed to sync")
	}

	// Opportunistic cleanup of expired trash items
	if !dryRun {
		if n, _ := trash.Cleanup(trash.ProjectTrashDir(root), 0); n > 0 {
			ui.Info("Cleaned up %d expired trash item(s)", n)
		}
	}

	return stats, nil
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
