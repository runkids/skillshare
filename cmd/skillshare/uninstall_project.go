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

// resolveProjectUninstallTarget resolves a skill name to an uninstallTarget
// within a project's .skillshare/skills directory.
func resolveProjectUninstallTarget(skillName, sourceDir string) (*uninstallTarget, error) {
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
			return nil, fmt.Errorf("skill '%s' not found in .skillshare/skills", skillName)
		}
		skillName = resolved
		skillPath = filepath.Join(sourceDir, resolved)
	}

	return &uninstallTarget{
		name:          skillName,
		path:          skillPath,
		isTrackedRepo: install.IsGitRepo(skillPath),
	}, nil
}

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

	sourceDir := filepath.Join(root, ".skillshare", "skills")
	trashDir := trash.ProjectTrashDir(root)

	// --- Phase 1: RESOLVE ---
	var targets []*uninstallTarget
	seen := map[string]bool{}
	var resolveWarnings []string

	for _, name := range opts.skillNames {
		t, err := resolveProjectUninstallTarget(name, sourceDir)
		if err != nil {
			resolveWarnings = append(resolveWarnings, fmt.Sprintf("%s: %v", name, err))
			continue
		}
		if !seen[t.path] {
			seen[t.path] = true
			targets = append(targets, t)
		}
	}

	for _, group := range opts.groups {
		groupTargets, err := resolveGroupSkills(group, sourceDir)
		if err != nil {
			resolveWarnings = append(resolveWarnings, fmt.Sprintf("--group %s: %v", group, err))
			continue
		}
		for _, t := range groupTargets {
			if !seen[t.path] {
				seen[t.path] = true
				targets = append(targets, t)
			}
		}
	}

	for _, w := range resolveWarnings {
		ui.Warning("%s", w)
	}

	// --- Phase 2: VALIDATE ---
	if len(targets) == 0 {
		if len(resolveWarnings) > 0 {
			return fmt.Errorf("no valid skills to uninstall")
		}
		return fmt.Errorf("no skills found")
	}

	// --- Phase 3: DISPLAY ---
	single := len(targets) == 1
	if single {
		displayUninstallInfo(targets[0])
	} else {
		ui.Header(fmt.Sprintf("Uninstalling %d skill(s)", len(targets)))
		for _, t := range targets {
			label := t.name
			if t.isTrackedRepo {
				label += " (tracked)"
			}
			fmt.Printf("  - %s\n", label)
		}
		fmt.Println()
	}

	// --- Phase 4: PRE-FLIGHT ---
	if !opts.dryRun {
		var preflight []*uninstallTarget
		for _, t := range targets {
			if t.isTrackedRepo && !opts.force {
				if isDirty, _ := isRepoDirty(t.path); isDirty {
					if single {
						ui.Error("Repository has uncommitted changes!")
						ui.Info("Use --force to uninstall anyway, or commit/stash your changes first")
						return fmt.Errorf("uncommitted changes detected, use --force to override")
					}
					ui.Warning("Skipping %s: uncommitted changes (use --force to override)", t.name)
					continue
				}
			}
			preflight = append(preflight, t)
		}
		targets = preflight

		if len(targets) == 0 {
			return fmt.Errorf("no skills to uninstall after pre-flight checks")
		}
	}

	// --- Phase 5: DRY-RUN or CONFIRM ---
	if opts.dryRun {
		for _, t := range targets {
			ui.Warning("[dry-run] would move to trash: %s", t.path)
			ui.Warning("[dry-run] would update .skillshare/.gitignore")
			if meta, err := install.ReadMeta(t.path); err == nil && meta != nil && meta.Source != "" {
				ui.Info("[dry-run] Reinstall: skillshare install %s --project", meta.Source)
			}
		}
		return nil
	}

	if !opts.force {
		if single {
			confirmed, err := confirmProjectUninstall()
			if err != nil {
				return err
			}
			if !confirmed {
				ui.Info("Cancelled")
				return nil
			}
		} else {
			fmt.Printf("Uninstall %d skill(s) from the project? [y/N]: ", len(targets))
			reader := bufio.NewReader(os.Stdin)
			input, err := reader.ReadString('\n')
			if err != nil {
				return err
			}
			input = strings.TrimSpace(strings.ToLower(input))
			if input != "y" && input != "yes" {
				ui.Info("Cancelled")
				return nil
			}
		}
	}

	// --- Phase 6: EXECUTE ---
	var succeeded []*uninstallTarget
	var failed []string
	for _, t := range targets {
		meta, _ := install.ReadMeta(t.path)

		// Clean up .gitignore (all project skills have gitignore entries)
		if _, err := install.RemoveFromGitIgnore(filepath.Join(root, ".skillshare"), filepath.Join("skills", t.name)); err != nil {
			ui.Warning("Could not update .skillshare/.gitignore: %v", err)
		}

		trashPath, err := trash.MoveToTrash(t.path, t.name, trashDir)
		if err != nil {
			failed = append(failed, fmt.Sprintf("%s: %v", t.name, err))
			ui.Warning("Failed to uninstall %s: %v", t.name, err)
			continue
		}

		if t.isTrackedRepo {
			ui.Success("Uninstalled tracked repository: %s", t.name)
		} else {
			ui.Success("Uninstalled: %s", t.name)
		}
		ui.Info("Moved to trash (7 days): %s", trashPath)
		if meta != nil && meta.Source != "" {
			ui.Info("Reinstall: skillshare install %s --project", meta.Source)
		}
		succeeded = append(succeeded, t)
	}

	// --- Phase 7: FINALIZE ---
	if len(succeeded) > 0 {
		cfg, err := config.LoadProject(root)
		if err != nil {
			ui.Warning("Failed to load project config: %v", err)
		} else {
			removedNames := map[string]bool{}
			for _, t := range succeeded {
				removedNames[t.name] = true
			}
			updated := make([]config.SkillEntry, 0, len(cfg.Skills))
			for _, s := range cfg.Skills {
				if !removedNames[s.FullName()] {
					updated = append(updated, s)
				}
			}
			cfg.Skills = updated
			if err := cfg.Save(root); err != nil {
				ui.Warning("Failed to update project config: %v", err)
			}
		}
	}

	fmt.Println()
	ui.Info("Run 'skillshare sync' to clean up symlinks")

	// Opportunistic cleanup of expired trash items
	if n, _ := trash.Cleanup(trashDir, 0); n > 0 {
		ui.Info("Cleaned up %d expired trash item(s)", n)
	}

	if len(failed) > 0 && len(succeeded) == 0 {
		return fmt.Errorf("all uninstalls failed")
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
