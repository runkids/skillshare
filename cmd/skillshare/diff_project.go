package main

import (
	"fmt"

	"skillshare/internal/config"
	"skillshare/internal/sync"
	"skillshare/internal/ui"
)

func cmdDiffProject(root, targetName string) error {
	if !projectConfigExists(root) {
		if err := performProjectInit(root, projectInitOptions{}); err != nil {
			return err
		}
	}

	runtime, err := loadProjectRuntime(root)
	if err != nil {
		return err
	}

	spinner := ui.StartSpinner("Discovering skills")
	discovered, err := sync.DiscoverSourceSkills(runtime.sourcePath)
	if err != nil {
		spinner.Fail("Discovery failed")
		return fmt.Errorf("failed to discover skills: %w", err)
	}
	spinner.Success(fmt.Sprintf("Discovered %d skills", len(discovered)))

	targets := make([]config.ProjectTargetEntry, len(runtime.config.Targets))
	copy(targets, runtime.config.Targets)

	if targetName != "" {
		found := false
		for _, entry := range runtime.config.Targets {
			if entry.Name == targetName {
				targets = []config.ProjectTargetEntry{entry}
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("target '%s' not found", targetName)
		}
	}

	var results []targetDiffResult
	for _, entry := range targets {
		target, ok := runtime.targets[entry.Name]
		if !ok {
			return fmt.Errorf("target '%s' not resolved", entry.Name)
		}

		filtered, err := sync.FilterSkills(discovered, target.Include, target.Exclude)
		if err != nil {
			return fmt.Errorf("target %s has invalid include/exclude config: %w", entry.Name, err)
		}
		mode := target.Mode
		if mode == "" {
			mode = "merge"
		}
		r := collectTargetDiff(entry.Name, target, runtime.sourcePath, mode, filtered)
		results = append(results, r)
	}

	renderGroupedDiffs(results)
	return nil
}
