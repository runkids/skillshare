package main

import (
	"fmt"
	"path/filepath"
	"sort"

	"skillshare/internal/install"
	"skillshare/internal/sync"
	"skillshare/internal/ui"
)

func cmdListProject(root string) error {
	if !projectConfigExists(root) {
		if err := performProjectInit(root, projectInitOptions{}); err != nil {
			return err
		}
	}

	sourcePath := filepath.Join(root, ".skillshare", "skills")

	// Use recursive discovery (same as global list and web UI)
	discovered, err := sync.DiscoverSourceSkills(sourcePath)
	if err != nil {
		return fmt.Errorf("cannot discover project skills: %w", err)
	}

	trackedRepos, _ := install.GetTrackedRepos(sourcePath)
	skills := buildSkillEntries(discovered)

	sort.Slice(skills, func(i, j int) bool {
		return skills[i].Name < skills[j].Name
	})

	if len(skills) == 0 && len(trackedRepos) == 0 {
		ui.Info("No skills installed")
		ui.Info("Use 'skillshare install -p <source>' to install a skill")
		return nil
	}

	if len(skills) > 0 {
		ui.Header("Installed skills (project)")
		displaySkillsCompact(skills)
	}

	if len(trackedRepos) > 0 {
		displayTrackedRepos(trackedRepos, discovered, sourcePath)
	}

	fmt.Println()
	trackedCount := 0
	remoteCount := 0
	for _, skill := range skills {
		if skill.RepoName != "" {
			trackedCount++
		} else if skill.Source != "" {
			remoteCount++
		}
	}
	localCount := len(skills) - trackedCount - remoteCount
	ui.Info("%d skill(s): %d tracked, %d remote, %d local", len(skills), trackedCount, remoteCount, localCount)
	return nil
}
