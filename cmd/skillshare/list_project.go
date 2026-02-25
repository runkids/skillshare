package main

import (
	"fmt"
	"os"
	"path/filepath"

	"skillshare/internal/config"
	"skillshare/internal/sync"
	"skillshare/internal/ui"

	"golang.org/x/term"
)

func cmdListProject(root string, opts listOptions) error {
	if !projectConfigExists(root) {
		if err := performProjectInit(root, projectInitOptions{}); err != nil {
			return err
		}
	}

	sourcePath := filepath.Join(root, ".skillshare", "skills")
	isTTY := term.IsTerminal(int(os.Stdout.Fd()))

	// TTY + not JSON + not --no-tui → launch TUI with async loading (no blank screen)
	if !opts.JSON && !opts.NoTUI && isTTY {
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
			discovered, _, err := sync.DiscoverSourceSkillsLite(sourcePath)
			if err != nil {
				return listLoadResult{err: fmt.Errorf("cannot discover project skills: %w", err)}
			}
			skills := buildSkillEntries(discovered)
			total := len(skills)
			skills = filterSkillEntries(skills, opts.Pattern, opts.TypeFilter)
			sortSkillEntries(skills, sortBy)
			return listLoadResult{skills: toSkillItems(skills), totalCount: total}
		}
		action, skillName, err := runListTUI(loadFn, "project", sourcePath, targets)
		if err != nil {
			return err
		}
		switch action {
		case "audit":
			return cmdAudit([]string{"-p", skillName})
		case "update":
			return cmdUpdateProject([]string{skillName}, root)
		case "uninstall":
			return cmdUninstallProject([]string{skillName}, root)
		}
		return nil
	}

	// Non-TUI path (JSON or plain text): synchronous loading with spinner
	var sp *ui.Spinner
	if !opts.JSON && isTTY {
		sp = ui.StartSpinner("Loading skills...")
	}

	// Use lite discovery (skips frontmatter I/O, collects tracked repos in one walk)
	discovered, trackedRepos, err := sync.DiscoverSourceSkillsLite(sourcePath)
	if err != nil {
		if sp != nil {
			sp.Fail("Discovery failed")
		}
		return fmt.Errorf("cannot discover project skills: %w", err)
	}

	if sp != nil {
		sp.Update(fmt.Sprintf("Reading metadata for %d skills...", len(discovered)))
	}
	skills := buildSkillEntries(discovered)
	if sp != nil {
		sp.Success(fmt.Sprintf("Loaded %d skills", len(skills)))
	}
	totalCount := len(skills)
	hasFilter := opts.Pattern != "" || opts.TypeFilter != ""

	// Apply filter and sort
	skills = filterSkillEntries(skills, opts.Pattern, opts.TypeFilter)
	sortBy := opts.SortBy
	if sortBy == "" {
		sortBy = "name" // project mode default
	}
	sortSkillEntries(skills, sortBy)

	// JSON output
	if opts.JSON {
		return displaySkillsJSON(skills)
	}

	// Handle empty results before starting pager
	if len(skills) == 0 && len(trackedRepos) == 0 && !hasFilter {
		ui.Info("No skills installed")
		ui.Info("Use 'skillshare install -p <source>' to install a skill")
		return nil
	}

	if hasFilter && len(skills) == 0 {
		if opts.Pattern != "" && opts.TypeFilter != "" {
			ui.Info("No skills matching %q (type: %s)", opts.Pattern, opts.TypeFilter)
		} else if opts.Pattern != "" {
			ui.Info("No skills matching %q", opts.Pattern)
		} else {
			ui.Info("No skills matching type %q", opts.TypeFilter)
		}
		return nil
	}

	// Plain text output (--no-tui or non-TTY)
	if len(skills) > 0 {
		ui.Header("Installed skills (project)")
		if opts.Verbose {
			displaySkillsVerbose(skills)
		} else {
			displaySkillsCompact(skills)
		}
	}

	// Hide tracked repos section when filter/pattern is active
	if len(trackedRepos) > 0 && !hasFilter {
		displayTrackedRepos(trackedRepos, discovered, sourcePath)
	}

	// Show match stats when filter is active
	if hasFilter && len(skills) > 0 {
		fmt.Println()
		if opts.Pattern != "" {
			ui.Info("%d of %d skill(s) matching %q", len(skills), totalCount, opts.Pattern)
		} else {
			ui.Info("%d of %d skill(s)", len(skills), totalCount)
		}
	} else {
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
	}

	return nil
}
