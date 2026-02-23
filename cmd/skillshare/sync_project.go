package main

import (
	"fmt"
	"os"

	"skillshare/internal/config"
	"skillshare/internal/sync"
	"skillshare/internal/trash"
	"skillshare/internal/ui"
)

func cmdSyncProject(root string, dryRun, force bool) (syncLogStats, error) {
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

	discoveredSkills, discoverErr := sync.DiscoverSourceSkills(runtime.sourcePath)
	if discoverErr == nil {
		reportCollisions(discoveredSkills, runtime.targets)
	}

	ui.Header("Syncing skills (project)")
	if dryRun {
		ui.Warning("Dry run mode - no changes will be made")
	}

	failedTargets := 0
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

		var syncErr error
		switch mode {
		case "symlink":
			syncErr = syncSymlinkMode(name, target, runtime.sourcePath, dryRun, force)
		case "copy":
			syncErr = syncCopyMode(name, target, runtime.sourcePath, dryRun, force)
		default:
			syncErr = syncMergeMode(name, target, runtime.sourcePath, dryRun, force)
		}
		if syncErr != nil {
			ui.Error("%s: %v", name, syncErr)
			failedTargets++
		}
	}
	printPerTargetModeHint(runtime.targets, "merge", true)

	stats.Failed = failedTargets
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
