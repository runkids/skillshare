package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"skillshare/internal/config"
	"skillshare/internal/sync"
	"skillshare/internal/ui"
)

// collectLocalSkills collects local skills from targets (non-symlinked)
func collectLocalSkills(targets map[string]config.TargetConfig, source string) []sync.LocalSkillInfo {
	var allLocalSkills []sync.LocalSkillInfo
	for name, target := range targets {
		skills, err := sync.FindLocalSkills(target.Path, source)
		if err != nil {
			ui.Warning("%s: %v", name, err)
			continue
		}
		for i := range skills {
			skills[i].TargetName = name
		}
		allLocalSkills = append(allLocalSkills, skills...)
	}
	return allLocalSkills
}

// displayLocalSkills shows the local skills found
func displayLocalSkills(skills []sync.LocalSkillInfo) {
	ui.Header(ui.WithModeLabel("Local skills found"))
	for _, skill := range skills {
		ui.ListItem("info", skill.Name, fmt.Sprintf("[%s] %s", skill.TargetName, skill.Path))
	}
}

func cmdCollect(args []string) error {
	mode, rest, err := parseModeArgs(args)
	if err != nil {
		return err
	}

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("cannot determine working directory: %w", err)
	}

	if mode == modeAuto {
		if projectConfigExists(cwd) {
			mode = modeProject
		} else {
			mode = modeGlobal
		}
	}

	applyModeLabel(mode)

	if mode == modeProject {
		return cmdCollectProject(rest, cwd)
	}

	dryRun := false
	force := false
	collectAll := false
	var targetName string

	for _, arg := range rest {
		switch arg {
		case "--dry-run", "-n":
			dryRun = true
		case "--force", "-f":
			force = true
		case "--all", "-a":
			collectAll = true
		default:
			if targetName == "" && !strings.HasPrefix(arg, "-") {
				targetName = arg
			}
		}
	}

	cfg, err := config.Load()
	if err != nil {
		return err
	}

	// Select targets to collect from
	targets, err := selectCollectTargets(cfg, targetName, collectAll)
	if err != nil {
		return err
	}
	if targets == nil {
		return nil // User needs to specify target
	}

	// Collect all local skills
	allLocalSkills := collectLocalSkills(targets, cfg.Source)

	if len(allLocalSkills) == 0 {
		ui.Info("No local skills to collect")
		return nil
	}

	// Display found skills
	displayLocalSkills(allLocalSkills)

	if dryRun {
		ui.Info("Dry run - no changes made")
		return nil
	}

	// Confirm unless --force
	if !force {
		if !confirmCollect() {
			ui.Info("Cancelled")
			return nil
		}
	}

	// Execute collect
	return executeCollect(allLocalSkills, cfg.Source, dryRun, force)
}

func selectCollectTargets(cfg *config.Config, targetName string, collectAll bool) (map[string]config.TargetConfig, error) {
	if targetName != "" {
		if t, exists := cfg.Targets[targetName]; exists {
			return map[string]config.TargetConfig{targetName: t}, nil
		}
		return nil, fmt.Errorf("target '%s' not found", targetName)
	}

	if collectAll || len(cfg.Targets) == 1 {
		return cfg.Targets, nil
	}

	// If no target specified and multiple targets exist, ask or require --all
	ui.Warning("Multiple targets found. Specify a target name or use --all")
	fmt.Println("  Available targets:")
	for name := range cfg.Targets {
		fmt.Printf("    - %s\n", name)
	}
	return nil, nil
}

func confirmCollect() bool {
	fmt.Println()
	fmt.Print("Collect these skills to source? [y/N]: ")
	var input string
	fmt.Scanln(&input)
	input = strings.ToLower(strings.TrimSpace(input))
	return input == "y" || input == "yes"
}

func executeCollect(skills []sync.LocalSkillInfo, source string, dryRun, force bool) error {
	ui.Header(ui.WithModeLabel("Collecting skills"))
	result, err := sync.PullSkills(skills, source, sync.PullOptions{
		DryRun: dryRun,
		Force:  force,
	})
	if err != nil {
		return err
	}

	// Display results
	for _, name := range result.Pulled {
		ui.Success("%s: copied to source", name)
	}
	for _, name := range result.Skipped {
		ui.Warning("%s: skipped (already exists in source, use --force to overwrite)", name)
	}
	for name, err := range result.Failed {
		ui.Error("%s: %v", name, err)
	}

	if len(result.Pulled) > 0 {
		showCollectNextSteps(source)
	}

	return nil
}

func showCollectNextSteps(source string) {
	fmt.Println()
	if ui.ModeLabel == "project" {
		ui.Info("Run 'skillshare sync -p' to distribute to all targets")
		return
	}
	ui.Info("Run 'skillshare sync' to distribute to all targets")

	// Check if source has git
	gitDir := filepath.Join(source, ".git")
	if _, err := os.Stat(gitDir); err == nil {
		ui.Info("Commit changes: cd %s && git add . && git commit", source)
	}
}
