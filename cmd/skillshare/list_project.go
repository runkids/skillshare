package main

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"skillshare/internal/install"
	"skillshare/internal/sync"
	"skillshare/internal/ui"
)

type projectSkillEntry struct {
	Name     string
	Source   string
	Remote   bool
	RepoName string
}

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

	var skills []projectSkillEntry
	for _, d := range discovered {
		entry := projectSkillEntry{
			Name: d.FlatName,
		}

		if d.IsInRepo {
			parts := strings.SplitN(d.RelPath, "/", 2)
			if len(parts) > 0 {
				entry.RepoName = parts[0]
			}
		}

		meta, _ := install.ReadMeta(d.SourcePath)
		if meta != nil && meta.Source != "" {
			entry.Source = meta.Source
			entry.Remote = true
		}

		skills = append(skills, entry)
	}

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

		maxNameLen := 0
		for _, skill := range skills {
			if len(skill.Name) > maxNameLen {
				maxNameLen = len(skill.Name)
			}
		}

		for _, skill := range skills {
			suffix := getProjectSkillSuffix(skill)
			format := fmt.Sprintf("  %s→%s %%-%ds  %s%%s%s\n", ui.Cyan, ui.Reset, maxNameLen, ui.Gray, ui.Reset)
			fmt.Printf(format, skill.Name, suffix)
		}
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
		} else if skill.Remote {
			remoteCount++
		}
	}
	localCount := len(skills) - trackedCount - remoteCount
	ui.Info("%d skill(s): %d tracked, %d remote, %d local", len(skills), trackedCount, remoteCount, localCount)
	return nil
}

func getProjectSkillSuffix(s projectSkillEntry) string {
	if s.RepoName != "" {
		return fmt.Sprintf("tracked: %s", s.RepoName)
	}
	if s.Remote {
		return abbreviateSource(s.Source)
	}
	return "local"
}