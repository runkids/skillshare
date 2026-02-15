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
	RelPath  string
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
			Name:    d.FlatName,
			RelPath: d.RelPath,
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
		displayProjectSkillsCompact(skills)
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

// displayProjectSkillsCompact displays project skills in compact mode, grouped by directory.
// Reuses extractGroupDir() and displayName logic from list.go via projectSkillEntry → skillEntry adapter.
func displayProjectSkillsCompact(skills []projectSkillEntry) {
	// Check if any grouping is needed
	hasGrouping := false
	for _, s := range skills {
		if extractGroupDir(s.RelPath) != "" {
			hasGrouping = true
			break
		}
	}

	if !hasGrouping {
		// Flat display
		maxNameLen := 0
		for _, s := range skills {
			if len(s.Name) > maxNameLen {
				maxNameLen = len(s.Name)
			}
		}
		for _, s := range skills {
			suffix := getProjectSkillSuffix(s)
			format := fmt.Sprintf("  %s→%s %%-%ds  %s%%s%s\n", ui.Cyan, ui.Reset, maxNameLen, ui.Gray, ui.Reset)
			fmt.Printf(format, s.Name, suffix)
		}
		return
	}

	// Group by parent directory
	groups := make(map[string][]projectSkillEntry)
	for _, s := range skills {
		dir := extractGroupDir(s.RelPath)
		groups[dir] = append(groups[dir], s)
	}

	var dirs []string
	for k := range groups {
		if k != "" {
			dirs = append(dirs, k)
		}
	}
	sort.Strings(dirs)
	if _, ok := groups[""]; ok {
		dirs = append(dirs, "")
	}

	for i, dir := range dirs {
		if dir != "" {
			if i > 0 {
				fmt.Println()
			}
			fmt.Printf("  %s%s/%s\n", ui.Gray, dir, ui.Reset)
		} else if i > 0 {
			fmt.Println()
		}

		maxNameLen := 0
		for _, s := range groups[dir] {
			name := projectDisplayName(s, dir)
			if len(name) > maxNameLen {
				maxNameLen = len(name)
			}
		}

		for _, s := range groups[dir] {
			name := projectDisplayName(s, dir)
			suffix := getProjectSkillSuffix(s)
			if dir != "" {
				format := fmt.Sprintf("    %s→%s %%-%ds  %s%%s%s\n", ui.Cyan, ui.Reset, maxNameLen, ui.Gray, ui.Reset)
				fmt.Printf(format, name, suffix)
			} else {
				format := fmt.Sprintf("  %s→%s %%-%ds  %s%%s%s\n", ui.Cyan, ui.Reset, maxNameLen, ui.Gray, ui.Reset)
				fmt.Printf(format, name, suffix)
			}
		}
	}
}

// projectDisplayName returns the base skill name for display within a group.
func projectDisplayName(s projectSkillEntry, groupDir string) string {
	if groupDir == "" {
		return s.Name
	}
	base := s.RelPath
	if i := strings.LastIndex(base, "/"); i >= 0 {
		base = base[i+1:]
	}
	return base
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
