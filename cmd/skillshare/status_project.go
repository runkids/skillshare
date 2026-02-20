package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"skillshare/internal/install"
	"skillshare/internal/sync"
	"skillshare/internal/ui"
	"skillshare/internal/utils"
)

func cmdStatusProject(root string) error {
	if !projectConfigExists(root) {
		if err := performProjectInit(root, projectInitOptions{}); err != nil {
			return err
		}
	}

	runtime, err := loadProjectRuntime(root)
	if err != nil {
		return err
	}

	discovered, discoverErr := sync.DiscoverSourceSkills(runtime.sourcePath)
	if discoverErr != nil {
		discovered = nil
	}

	printProjectSourceStatus(runtime.sourcePath)
	printProjectTrackedReposStatus(runtime.sourcePath)
	if err := printProjectTargetsStatus(runtime, discovered); err != nil {
		return err
	}

	return nil
}

func printProjectSourceStatus(sourcePath string) {
	ui.Header("Source (project)")
	info, err := os.Stat(sourcePath)
	if err != nil {
		ui.Error(".skillshare/skills/ (not found)")
		return
	}

	entries, _ := os.ReadDir(sourcePath)
	skillCount := 0
	for _, e := range entries {
		if e.IsDir() && !utils.IsHidden(e.Name()) {
			skillCount++
		}
	}
	ui.Success(".skillshare/skills/ (%d skills, %s)", skillCount, info.ModTime().Format("2006-01-02 15:04"))
}

func printProjectTrackedReposStatus(sourcePath string) {
	trackedRepos, err := install.GetTrackedRepos(sourcePath)
	if err != nil || len(trackedRepos) == 0 {
		return
	}

	ui.Header("Tracked Repositories")
	for _, repoName := range trackedRepos {
		repoPath := filepath.Join(sourcePath, repoName)

		discovered, _ := sync.DiscoverSourceSkills(sourcePath)
		skillCount := 0
		for _, d := range discovered {
			if d.IsInRepo && strings.HasPrefix(d.RelPath, repoName+"/") {
				skillCount++
			}
		}

		statusStr := "up-to-date"
		statusIcon := "✓"
		if isDirty, _ := checkRepoDirty(repoPath); isDirty {
			statusStr = "has uncommitted changes"
			statusIcon = "!"
		}

		ui.Status(repoName, statusIcon, fmt.Sprintf("%d skills, %s", skillCount, statusStr))
	}
}

func printProjectTargetsStatus(runtime *projectRuntime, discovered []sync.DiscoveredSkill) error {
	ui.Header("Targets (project)")
	driftTotal := 0
	for _, entry := range runtime.config.Targets {
		target, ok := runtime.targets[entry.Name]
		if !ok {
			ui.Error("%s: target not found", entry.Name)
			continue
		}

		mode := target.Mode
		if mode == "" {
			mode = "merge"
		}

		statusStr, detail := getTargetStatusDetail(target, runtime.sourcePath, mode)
		ui.Status(entry.Name, statusStr, detail)

		if mode == "merge" || mode == "copy" {
			filtered, err := sync.FilterSkills(discovered, target.Include, target.Exclude)
			if err != nil {
				return fmt.Errorf("target %s has invalid include/exclude config: %w", entry.Name, err)
			}
			filtered = sync.FilterSkillsByTarget(filtered, entry.Name)
			expectedCount := len(filtered)

			var syncedCount int
			if mode == "copy" {
				_, syncedCount, _ = sync.CheckStatusCopy(target.Path)
			} else {
				_, syncedCount, _ = sync.CheckStatusMerge(target.Path, runtime.sourcePath)
			}
			if syncedCount < expectedCount {
				drift := expectedCount - syncedCount
				if drift > driftTotal {
					driftTotal = drift
				}
			}
		} else if len(target.Include) > 0 || len(target.Exclude) > 0 {
			ui.Warning("%s: include/exclude ignored in symlink mode", entry.Name)
		}
	}
	if driftTotal > 0 {
		ui.Warning("%d skill(s) not synced — run 'skillshare sync'", driftTotal)
	}
	return nil
}
