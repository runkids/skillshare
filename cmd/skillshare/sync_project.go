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
		collisions := sync.CheckNameCollisions(discoveredSkills)
		if len(collisions) > 0 {
			ui.Header("Name conflicts detected")
			for _, collision := range collisions {
				ui.Warning("Skill name '%s' is defined in multiple places:", collision.Name)
				for _, path := range collision.Paths {
					ui.Info("  - %s", path)
				}
			}
			ui.Info("CLI tools may not distinguish between them.")
			ui.Info("Suggestion: Rename one in SKILL.md (e.g., 'repo:skillname')")
			fmt.Println()
		}
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

		if mode == "symlink" {
			if err := syncSymlinkMode(name, target, runtime.sourcePath, dryRun, force); err != nil {
				ui.Error("%s: %v", name, err)
				failedTargets++
			}
		} else {
			if err := syncMergeMode(name, target, runtime.sourcePath, dryRun, force); err != nil {
				ui.Error("%s: %v", name, err)
				failedTargets++
			}
		}
	}

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
