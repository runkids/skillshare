package main

import (
	"fmt"
	"time"

	"skillshare/internal/config"
	"skillshare/internal/sync"
	"skillshare/internal/trash"
	"skillshare/internal/ui"
)

func cmdListProject(root string, opts listOptions, kind resourceKindFilter) error {
	if err := ensureProjectConfig(root); err != nil {
		return err
	}

	projCfg, err := config.LoadProject(root)
	if err != nil {
		return err
	}
	skillsSource := projCfg.EffectiveSkillsSource(root)
	agentsSource := projCfg.EffectiveAgentsSource(root)
	targets, _ := config.ResolveValidProjectTargets(root, projCfg)
	follow := skillFollowSet(skillsSource, targets, root)

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
			discovered, _, err := sync.DiscoverSourceSkillsWithOptions(skillsSource, sync.DiscoveryOptions{Follow: follow, IncludeIgnored: true})
			if err != nil {
				return listLoadResult{err: fmt.Errorf("cannot discover project skills: %w", err)}
			}
			allEntries = append(allEntries, buildSkillEntries(discovered)...)
			allEntries = append(allEntries, discoverAndBuildAgentEntries(agentsSource)...)
			total := len(allEntries)
			// Status is NOT filtered here — the TUI needs both enabled and
			// disabled entries loaded so the `s` key can restore them.
			allEntries = filterSkillEntries(allEntries, opts.Pattern, opts.TypeFilter, statusFilterAll)
			sortSkillEntries(allEntries, sortBy)
			return listLoadResult{skills: toSkillItems(allEntries), totalCount: total}
		}
		action, skillName, skillKind, err := runListTUI(loadFn, "project", skillsSource, agentsSource, targets, kind, opts.Status)
		if err != nil {
			return err
		}
		switch action {
		case "empty":
			printListEmpty(resourceLabel, kind, true)
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
				uOpts := &uninstallOptions{skillNames: []string{skillName}, force: true}
				return cmdUninstallAgents(agentsSource, uOpts, config.ProjectConfigPath(root), trash.ProjectAgentTrashDir(root), time.Now())
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
		discoveredSkills, _, discErr = sync.DiscoverSourceSkillsWithOptions(skillsSource, sync.DiscoveryOptions{Follow: follow, IncludeIgnored: true})
		if discErr != nil {
			if sp != nil {
				sp.Fail("Discovery failed")
			}
			return fmt.Errorf("cannot discover project skills: %w", discErr)
		}
		trackedRepos = extractTrackedReposWithFollow(skillsSource, follow)
		if sp != nil {
			sp.Update(fmt.Sprintf("Reading metadata for %d skills...", len(discoveredSkills)))
		}
		allEntries = append(allEntries, buildSkillEntries(discoveredSkills)...)
	}

	if kind.IncludesAgents() {
		allEntries = append(allEntries, discoverAndBuildAgentEntries(agentsSource)...)
	}

	if sp != nil {
		sp.Stop()
	}
	totalCount := len(allEntries)
	// Apply filter and sort
	allEntries = filterSkillEntries(allEntries, opts.Pattern, opts.TypeFilter, opts.Status)
	sortBy := opts.SortBy
	if sortBy == "" {
		sortBy = "name" // project mode default
	}
	sortSkillEntries(allEntries, sortBy)

	// JSON output
	if opts.JSON {
		return displaySkillsJSON(allEntries)
	}

	printSkillList(skillList{
		entries: allEntries, total: totalCount, trackedRepos: trackedRepos,
		discovered: discoveredSkills, skillsSource: skillsSource, follow: follow,
		label: resourceLabel, kind: kind, opts: opts, project: true,
	})
	return nil
}
