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
	skillName = strings.TrimRight(strings.TrimSpace(skillName), `/\`)
	if skillName == "" || skillName == "." {
		return nil, fmt.Errorf("invalid skill name: %q", skillName)
	}

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

// performProjectUninstallQuiet moves a project skill to trash without printing output.
// Used by batch mode; returns the type label for StepDone display.
// Note: .gitignore cleanup is handled in batch by the caller.
func performProjectUninstallQuiet(target *uninstallTarget, trashDir string) (typeLabel string, err error) {
	groupSkillCount := 0
	if !target.isTrackedRepo {
		groupSkillCount = len(countGroupSkills(target.path))
	}

	if _, err := trash.MoveToTrash(target.path, target.name, trashDir); err != nil {
		return "", fmt.Errorf("failed to move to trash: %w", err)
	}

	if target.isTrackedRepo {
		return "tracked repo", nil
	}
	if groupSkillCount > 0 {
		return fmt.Sprintf("group, %d skill%s", groupSkillCount, pluralS(groupSkillCount)), nil
	}
	return "skill", nil
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

	if opts.all {
		sp := ui.StartSpinner("Discovering skills...")
		discovered, _, err := discoveryCache.DiscoverLite(sourceDir)
		if err != nil {
			sp.Fail("Discovery failed")
			return fmt.Errorf("failed to discover skills: %w", err)
		}
		sp.Success(fmt.Sprintf("Found %d skills", len(discovered)))
		if len(discovered) == 0 {
			return fmt.Errorf("no skills found in project source")
		}
		topDirs := map[string]bool{}
		for _, d := range discovered {
			topDirs[topLevelDir(d.RelPath)] = true
		}
		for dir := range topDirs {
			skillPath := filepath.Join(sourceDir, dir)
			targets = append(targets, &uninstallTarget{
				name:          dir,
				path:          skillPath,
				isTrackedRepo: install.IsGitRepo(skillPath),
			})
			seen[skillPath] = true
		}
	}

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

	// Shell glob detection
	if !opts.all && looksLikeShellGlob(opts.skillNames, resolveWarnings) {
		ui.Warning("It looks like '*' was expanded by your shell into file names.")
		ui.Info("To uninstall all skills, use: skillshare uninstall --all")
		return fmt.Errorf("shell glob expansion detected")
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
	summary := summarizeUninstallTargets(targets)
	if single {
		displayUninstallInfo(targets[0])
	} else {
		ui.Header(fmt.Sprintf("Uninstalling %d %s", len(targets), summary.noun()))
		if len(targets) > 20 {
			// Compressed: only list non-skill items (groups, tracked repos) individually
			ui.Info("Includes: %s", summary.details())
			for _, t := range targets {
				if t.isTrackedRepo {
					fmt.Printf("  - %s (tracked repository)\n", t.name)
				} else if c := summary.groupSkillCount[t.path]; c > 0 {
					fmt.Printf("  - %s (group, %d skill%s)\n", t.name, c, pluralS(c))
				}
			}
			if summary.skills > 0 {
				fmt.Printf("  ... and %d skill%s\n", summary.skills, pluralS(summary.skills))
			}
		} else {
			if summary.isMixed() {
				ui.Info("Includes: %s", summary.details())
			}
			for _, t := range targets {
				label := t.name
				if t.isTrackedRepo {
					label += " (tracked repository)"
				} else if c := summary.groupSkillCount[t.path]; c > 0 {
					label += fmt.Sprintf(" (group, %d skill%s)", c, pluralS(c))
				} else {
					label += " (skill)"
				}
				fmt.Printf("  - %s\n", label)
			}
		}
		fmt.Println()
	}

	// --- Phase 4: PRE-FLIGHT ---
	if !opts.dryRun {
		var preflight []*uninstallTarget
		for _, t := range targets {
			if err := checkTrackedRepoStatus(t, opts.force); err != nil {
				if single {
					return err
				}
				ui.Warning("Skipping %s: %v", t.name, err)
				continue
			}
			preflight = append(preflight, t)
		}
		skippedCount := len(targets) - len(preflight)
		targets = preflight
		summary = summarizeUninstallTargets(targets)

		if skippedCount > 0 {
			ui.Info("%d tracked repo%s skipped, %d remaining", skippedCount, pluralS(skippedCount), len(targets))
			fmt.Println()
		}

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
			confirmed, err := confirmProjectUninstall(targets[0])
			if err != nil {
				return err
			}
			if !confirmed {
				ui.Info("Cancelled")
				return nil
			}
		} else {
			confirmSummary := summarizeUninstallTargets(targets)
			fmt.Printf("Uninstall %d %s from the project? [y/N]: ", len(targets), confirmSummary.noun())
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
	batch := len(targets) > 1
	type batchResult struct {
		target    *uninstallTarget
		typeLabel string
		errMsg    string
	}

	var succeeded []*uninstallTarget
	var failed []string

	if batch {
		sp := ui.StartSpinner(fmt.Sprintf("Uninstalling %d %s", len(targets), summary.noun()))
		var results []batchResult

		for _, t := range targets {
			typeLabel, err := performProjectUninstallQuiet(t, trashDir)
			if err != nil {
				results = append(results, batchResult{target: t, errMsg: err.Error()})
				failed = append(failed, fmt.Sprintf("%s: %v", t.name, err))
			} else {
				results = append(results, batchResult{target: t, typeLabel: typeLabel})
				succeeded = append(succeeded, t)
			}
		}

		// Batch-remove .gitignore entries (one read/write pass for all succeeded targets).
		if len(succeeded) > 0 {
			entries := make([]string, len(succeeded))
			for i, t := range succeeded {
				entries[i] = filepath.Join("skills", t.name)
			}
			install.RemoveFromGitIgnoreBatch(filepath.Join(root, ".skillshare"), entries) //nolint:errcheck
		}

		// Spinner end state
		if len(failed) > 0 && len(succeeded) == 0 {
			sp.Fail(fmt.Sprintf("Failed to uninstall %d %s", len(failed), summary.noun()))
		} else if len(failed) > 0 {
			sp.Warn(fmt.Sprintf("Uninstalled %d, failed %d", len(succeeded), len(failed)))
		} else {
			sp.Success(fmt.Sprintf("Uninstalled %d %s", len(succeeded), summary.noun()))
		}

		// Failures always shown individually
		var successes []batchResult
		var failures []batchResult
		for _, r := range results {
			if r.errMsg != "" {
				failures = append(failures, r)
			} else {
				successes = append(successes, r)
			}
		}

		if len(failures) > 0 {
			ui.SectionLabel("Failed")
			for _, r := range failures {
				ui.StepFail(r.target.name, r.errMsg)
			}
		}

		// Successes: condensed when many
		if len(successes) > 0 {
			ui.SectionLabel("Removed")
			switch {
			case len(successes) > 50:
				ui.StepDone(fmt.Sprintf("%d uninstalled", len(successes)), "")
			case len(successes) > 10:
				const maxShown = 10
				names := make([]string, 0, maxShown)
				for i := 0; i < maxShown && i < len(successes); i++ {
					names = append(names, successes[i].target.name)
				}
				detail := strings.Join(names, ", ")
				if len(successes) > maxShown {
					detail = fmt.Sprintf("%s ... +%d more", detail, len(successes)-maxShown)
				}
				ui.StepDone(fmt.Sprintf("%d uninstalled", len(successes)), detail)
			default:
				for _, r := range successes {
					ui.StepDone(r.target.name, r.typeLabel)
				}
			}
		}

		// Batch summary
		ui.SectionLabel("Next Steps")
		ui.Info("Moved to trash (7 days).")
		ui.Info("Run 'skillshare sync' to clean up symlinks")

		// Opportunistic cleanup of expired trash items
		if n, _ := trash.Cleanup(trashDir, 0); n > 0 {
			ui.Info("Cleaned up %d expired trash item%s", n, pluralS(n))
		}
	} else {
		for _, t := range targets {
			meta, _ := install.ReadMeta(t.path)
			groupSkillCount := 0
			if !t.isTrackedRepo {
				groupSkillCount = len(countGroupSkills(t.path))
			}

			trashPath, err := trash.MoveToTrash(t.path, t.name, trashDir)
			if err != nil {
				failed = append(failed, fmt.Sprintf("%s: %v", t.name, err))
				ui.Warning("Failed to uninstall %s: %v", t.name, err)
				continue
			}

			if t.isTrackedRepo {
				ui.Success("Uninstalled tracked repository: %s", t.name)
			} else if groupSkillCount > 0 {
				ui.Success("Uninstalled group: %s", t.name)
			} else {
				ui.Success("Uninstalled skill: %s", t.name)
			}
			ui.Info("Moved to trash (7 days): %s", trashPath)
			if meta != nil && meta.Source != "" {
				ui.Info("Reinstall: skillshare install %s --project", meta.Source)
			}
			succeeded = append(succeeded, t)
		}

		// Batch-remove .gitignore entries after all targets processed.
		if len(succeeded) > 0 {
			entries := make([]string, len(succeeded))
			for i, t := range succeeded {
				entries[i] = filepath.Join("skills", t.name)
			}
			if _, err := install.RemoveFromGitIgnoreBatch(filepath.Join(root, ".skillshare"), entries); err != nil {
				ui.Warning("Could not update .skillshare/.gitignore: %v", err)
			}
		}
	}

	// --- Phase 7: FINALIZE ---
	if len(succeeded) > 0 {
		regDir := filepath.Join(root, ".skillshare")
		reg, regErr := config.LoadRegistry(regDir)
		if regErr != nil {
			ui.Warning("Failed to load registry: %v", regErr)
		} else if len(reg.Skills) > 0 {
			removedNames := map[string]bool{}
			for _, t := range succeeded {
				removedNames[t.name] = true
			}
			updated := make([]config.SkillEntry, 0, len(reg.Skills))
			for _, s := range reg.Skills {
				fullName := s.FullName()
				if removedNames[fullName] {
					continue
				}
				// When a group directory is uninstalled, also remove its member skills
				memberOfRemoved := false
				for name := range removedNames {
					if strings.HasPrefix(fullName, name+"/") {
						memberOfRemoved = true
						break
					}
				}
				if memberOfRemoved {
					continue
				}
				updated = append(updated, s)
			}
			if len(updated) != len(reg.Skills) {
				reg.Skills = updated
				if saveErr := reg.Save(regDir); saveErr != nil {
					ui.Warning("Failed to update registry after uninstall: %v", saveErr)
				}
			}
		}
	}

	if len(succeeded) > 0 {
		discoveryCache.Invalidate(sourceDir)
	}

	if !batch {
		ui.SectionLabel("Next Steps")
		ui.Info("Run 'skillshare sync' to clean up symlinks")

		// Opportunistic cleanup of expired trash items
		if n, _ := trash.Cleanup(trashDir, 0); n > 0 {
			ui.Info("Cleaned up %d expired trash item%s", n, pluralS(n))
		}
	}

	if len(failed) > 0 && len(succeeded) == 0 {
		return fmt.Errorf("all uninstalls failed")
	}
	return nil
}

func confirmProjectUninstall(target *uninstallTarget) (bool, error) {
	prompt := "Are you sure you want to uninstall this skill from the project? [y/N]: "
	if target.isTrackedRepo {
		prompt = "Are you sure you want to uninstall this tracked repository from the project? [y/N]: "
	} else if len(countGroupSkills(target.path)) > 0 {
		prompt = "Are you sure you want to uninstall this group from the project? [y/N]: "
	}
	fmt.Print(prompt)
	reader := bufio.NewReader(os.Stdin)
	input, err := reader.ReadString('\n')
	if err != nil {
		return false, err
	}
	input = strings.TrimSpace(strings.ToLower(input))
	return input == "y" || input == "yes", nil
}
