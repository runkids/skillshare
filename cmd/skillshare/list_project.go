package main

import (
	"fmt"
	"path/filepath"
	"time"

	"skillshare/internal/config"
	"skillshare/internal/sync"
	"skillshare/internal/trash"
	"skillshare/internal/ui"
)

func cmdListProject(root string, opts listOptions, kind resourceKindFilter) error {
	if !projectConfigExists(root) {
		if err := performProjectInit(root, projectInitOptions{}); err != nil {
			return err
		}
	}

	skillsSource := filepath.Join(root, ".skillshare", "skills")
	agentsSource := filepath.Join(root, ".skillshare", "agents")

	resourceLabel := "skills"
	if kind == kindAgents {
		resourceLabel = "agents"
	} else if kind == kindAll {
		resourceLabel = "resources"
	}

	// TTY + not JSON + TUI enabled → launch TUI with async loading (no blank screen)
	if !opts.JSON && shouldLaunchTUI(opts.NoTUI, nil) {
		// Load project targets for TUI detail panel (synced-to info)
		var targets map[string]config.TargetConfig
		if rt, rtErr := loadProjectRuntime(root); rtErr == nil {
			targets = rt.targets
		}
		sortBy := opts.SortBy
		if sortBy == "" {
			sortBy = "name"
		}
		loadFn := func() listLoadResult {
			// Always load both skills and agents — tab UI filters the view.
			var allEntries []skillEntry
			discovered, err := sync.DiscoverSourceSkillsAll(skillsSource)
			if err != nil {
				return listLoadResult{err: fmt.Errorf("cannot discover project skills: %w", err)}
			}
			allEntries = append(allEntries, buildSkillEntries(discovered)...)
			allEntries = append(allEntries, discoverAndBuildAgentEntries(agentsSource)...)
			total := len(allEntries)
			allEntries = filterSkillEntries(allEntries, opts.Pattern, opts.TypeFilter)
			sortSkillEntries(allEntries, sortBy)
			return listLoadResult{skills: toSkillItems(allEntries), totalCount: total}
		}
		action, skillName, skillKind, err := runListTUI(loadFn, "project", skillsSource, agentsSource, targets, kind)
		if err != nil {
			return err
		}
		switch action {
		case "empty":
			ui.Info("No %s installed", resourceLabel)
			if kind.IncludesSkills() {
				ui.Info("Use 'skillshare install -p <source>' to install a skill")
			}
			return nil
		case "audit":
			if skillKind == "agent" {
				return cmdAudit([]string{"agents", "-p", skillName})
			}
			return cmdAudit([]string{"-p", skillName})
		case "update":
			if skillKind == "agent" {
				return cmdUpdateAgentsProject([]string{skillName}, root, time.Now())
			}
			_, updateErr := cmdUpdateProject([]string{skillName}, root)
			return updateErr
		case "uninstall":
			if skillKind == "agent" {
				agentsDir := filepath.Join(root, ".skillshare", "agents")
				uOpts := &uninstallOptions{skillNames: []string{skillName}, force: true}
				return cmdUninstallAgents(agentsDir, uOpts, config.ProjectConfigPath(root), trash.ProjectAgentTrashDir(root), time.Now())
			}
			return cmdUninstallProject([]string{"--force", skillName}, root)
		}
		return nil
	}

	// Non-TUI path (JSON or plain text): synchronous loading with spinner
	var sp *ui.Spinner
	if !opts.JSON && ui.IsTTY() {
		sp = ui.StartSpinner(fmt.Sprintf("Loading %s...", resourceLabel))
	}

	var allEntries []skillEntry
	var trackedRepos []string
	var discoveredSkills []sync.DiscoveredSkill

	if kind.IncludesSkills() {
		var discErr error
		discoveredSkills, discErr = sync.DiscoverSourceSkillsAll(skillsSource)
		if discErr != nil {
			if sp != nil {
				sp.Fail("Discovery failed")
			}
			return fmt.Errorf("cannot discover project skills: %w", discErr)
		}
		trackedRepos = extractTrackedRepos(discoveredSkills)
		if sp != nil {
			sp.Update(fmt.Sprintf("Reading metadata for %d skills...", len(discoveredSkills)))
		}
		allEntries = append(allEntries, buildSkillEntries(discoveredSkills)...)
	}

	if kind.IncludesAgents() {
		allEntries = append(allEntries, discoverAndBuildAgentEntries(agentsSource)...)
	}

	if sp != nil {
		sp.Success(fmt.Sprintf("Loaded %d %s", len(allEntries), resourceLabel))
	}
	totalCount := len(allEntries)
	hasFilter := opts.Pattern != "" || opts.TypeFilter != ""

	// Apply filter and sort
	allEntries = filterSkillEntries(allEntries, opts.Pattern, opts.TypeFilter)
	sortBy := opts.SortBy
	if sortBy == "" {
		sortBy = "name" // project mode default
	}
	sortSkillEntries(allEntries, sortBy)

	// JSON output
	if opts.JSON {
		return displaySkillsJSON(allEntries)
	}

	// Handle empty results
	if len(allEntries) == 0 && len(trackedRepos) == 0 && !hasFilter {
		ui.Info("No %s installed", resourceLabel)
		if kind.IncludesSkills() {
			ui.Info("Use 'skillshare install -p <source>' to install a skill")
		}
		return nil
	}

	if hasFilter && len(allEntries) == 0 {
		if opts.Pattern != "" && opts.TypeFilter != "" {
			ui.Info("No %s matching %q (type: %s)", resourceLabel, opts.Pattern, opts.TypeFilter)
		} else if opts.Pattern != "" {
			ui.Info("No %s matching %q", resourceLabel, opts.Pattern)
		} else {
			ui.Info("No %s matching type %q", resourceLabel, opts.TypeFilter)
		}
		return nil
	}

	// Plain text output (--no-tui or non-TTY)
	if len(allEntries) > 0 {
		headerLabel := "Installed skills (project)"
		if kind == kindAgents {
			headerLabel = "Installed agents (project)"
		} else if kind == kindAll {
			headerLabel = "Installed skills & agents (project)"
		}
		ui.Header(headerLabel)
		if opts.Verbose {
			displaySkillsVerbose(allEntries)
		} else {
			displaySkillsCompact(allEntries)
		}
	}

	// Hide tracked repos section when filter/pattern is active
	if len(trackedRepos) > 0 && !hasFilter {
		displayTrackedRepos(trackedRepos, discoveredSkills, skillsSource)
	}

	// Show match stats when filter is active
	if hasFilter && len(allEntries) > 0 {
		fmt.Println()
		if opts.Pattern != "" {
			ui.Info("%d of %d %s matching %q", len(allEntries), totalCount, resourceLabel, opts.Pattern)
		} else {
			ui.Info("%d of %d %s", len(allEntries), totalCount, resourceLabel)
		}
	} else {
		fmt.Println()
		trackedCount := 0
		remoteCount := 0
		for _, entry := range allEntries {
			if entry.RepoName != "" {
				trackedCount++
			} else if entry.Source != "" {
				remoteCount++
			}
		}
		localCount := len(allEntries) - trackedCount - remoteCount
		ui.Info("%d %s: %d tracked, %d remote, %d local", len(allEntries), resourceLabel, trackedCount, remoteCount, localCount)
	}

	return nil
}
