package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"skillshare/internal/config"
	"skillshare/internal/install"
	"skillshare/internal/ui"
)

func cmdUninstallProject(args []string, root string) error {
	opts, showHelp, err := parseUninstallArgs(args)
	if showHelp {
		printUninstallHelp()
		return err
	}
	if err != nil {
		return err
	}

	if !projectConfigExists(root) {
		if err := performProjectInit(root, projectInitOptions{}); err != nil {
			return err
		}
	}

	skillName := opts.skillName

	// Normalize _ prefix for tracked repos
	if !strings.HasPrefix(skillName, "_") {
		prefixed := filepath.Join(root, ".skillshare", "skills", "_"+skillName)
		if install.IsGitRepo(prefixed) {
			skillName = "_" + skillName
		}
	}

	skillPath := filepath.Join(root, ".skillshare", "skills", skillName)
	if info, err := os.Stat(skillPath); err != nil || !info.IsDir() {
		return fmt.Errorf("skill '%s' not found in .skillshare/skills", skillName)
	}

	isTracked := install.IsGitRepo(skillPath)

	if opts.dryRun {
		ui.Warning("[dry-run] would remove %s", skillPath)
		ui.Warning("[dry-run] would update .skillshare/config.yaml and .skillshare/.gitignore")
		return nil
	}

	// Check for uncommitted changes in tracked repos
	if isTracked && !opts.force {
		if isDirty, _ := isRepoDirty(skillPath); isDirty {
			ui.Error("Repository has uncommitted changes!")
			ui.Info("Use --force to uninstall anyway, or commit/stash your changes first")
			return fmt.Errorf("uncommitted changes detected, use --force to override")
		}
	}

	if !opts.force {
		confirmed, err := confirmProjectUninstall()
		if err != nil {
			return err
		}
		if !confirmed {
			ui.Info("Cancelled")
			return nil
		}
	}

	if err := os.RemoveAll(skillPath); err != nil {
		return fmt.Errorf("failed to remove: %w", err)
	}

	cfg, err := config.LoadProject(root)
	if err != nil {
		return err
	}

	updatedSkills := make([]config.ProjectSkill, 0, len(cfg.Skills))
	for _, skill := range cfg.Skills {
		if skill.Name != skillName {
			updatedSkills = append(updatedSkills, skill)
		}
	}
	cfg.Skills = updatedSkills
	if err := cfg.Save(root); err != nil {
		return err
	}

	if _, err := install.RemoveFromGitIgnore(filepath.Join(root, ".skillshare"), filepath.Join("skills", skillName)); err != nil {
		ui.Warning("Could not update .skillshare/.gitignore: %v", err)
	}

	if isTracked {
		ui.Success("Uninstalled tracked repository: %s", skillName)
	} else {
		ui.Success("Uninstalled: %s", skillName)
	}
	ui.Info("Run 'skillshare sync' to clean up symlinks")
	return nil
}

func confirmProjectUninstall() (bool, error) {
	fmt.Print("Are you sure you want to uninstall this skill from the project? [y/N]: ")
	reader := bufio.NewReader(os.Stdin)
	input, err := reader.ReadString('\n')
	if err != nil {
		return false, err
	}
	input = strings.TrimSpace(strings.ToLower(input))
	return input == "y" || input == "yes", nil
}
