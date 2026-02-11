package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"skillshare/internal/config"
	"skillshare/internal/install"
	"skillshare/internal/trash"
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
	sourceDir := filepath.Join(root, ".skillshare", "skills")

	// Normalize _ prefix for tracked repos
	if !strings.HasPrefix(skillName, "_") {
		prefixed := filepath.Join(sourceDir, "_"+skillName)
		if install.IsGitRepo(prefixed) {
			skillName = "_" + skillName
		}
	}

	skillPath := filepath.Join(sourceDir, skillName)
	if info, err := os.Stat(skillPath); err != nil || !info.IsDir() {
		// Fallback: search by basename in nested directories
		resolved, resolveErr := resolveNestedSkillDir(sourceDir, skillName)
		if resolveErr != nil {
			return fmt.Errorf("skill '%s' not found in .skillshare/skills", skillName)
		}
		skillName = resolved
		skillPath = filepath.Join(sourceDir, resolved)
	}

	isTracked := install.IsGitRepo(skillPath)

	if opts.dryRun {
		ui.Warning("[dry-run] would move to trash: %s", skillPath)
		ui.Warning("[dry-run] would update .skillshare/config.yaml and .skillshare/.gitignore")
		if meta, err := install.ReadMeta(skillPath); err == nil && meta != nil && meta.Source != "" {
			ui.Info("[dry-run] Reinstall: skillshare install %s --project", meta.Source)
		}
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

	// Read metadata before moving (for reinstall hint)
	meta, _ := install.ReadMeta(skillPath)

	trashPath, err := trash.MoveToTrash(skillPath, skillName, trash.ProjectTrashDir(root))
	if err != nil {
		return fmt.Errorf("failed to move to trash: %w", err)
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
	ui.Info("Moved to trash (7 days): %s", trashPath)
	if meta != nil && meta.Source != "" {
		ui.Info("Reinstall: skillshare install %s --project", meta.Source)
	}
	ui.Info("Run 'skillshare sync' to clean up symlinks")

	// Opportunistic cleanup of expired trash items
	if n, _ := trash.Cleanup(trash.ProjectTrashDir(root), 0); n > 0 {
		ui.Info("Cleaned up %d expired trash item(s)", n)
	}

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
