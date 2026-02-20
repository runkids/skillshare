package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"skillshare/internal/git"
	"skillshare/internal/install"
	"skillshare/internal/ui"
	"skillshare/internal/utils"
)

func cmdUpdateProject(args []string, root string) error {
	opts, showHelp, parseErr := parseUpdateArgs(args)
	if showHelp {
		printUpdateHelp()
		return parseErr
	}
	if parseErr != nil {
		return parseErr
	}

	// Project mode default: no args and no groups → --all
	if len(opts.names) == 0 && len(opts.groups) == 0 && !opts.all {
		opts.all = true
	}

	if !projectConfigExists(root) {
		if err := performProjectInit(root, projectInitOptions{}); err != nil {
			return err
		}
	}

	sourcePath := filepath.Join(root, ".skillshare", "skills")

	if opts.all {
		return updateAllProjectSkills(sourcePath, opts.dryRun, opts.force)
	}

	return cmdUpdateProjectBatch(sourcePath, opts)
}

func cmdUpdateProjectBatch(sourcePath string, opts *updateOptions) error {
	// --- Resolve targets ---
	type projectTarget struct {
		name   string
		path   string
		isRepo bool
	}

	var targets []projectTarget
	seen := map[string]bool{}
	var resolveWarnings []string

	for _, name := range opts.names {
		// Check group directory first (before repo/skill lookup,
		// so "feature-radar" expands to all skills rather than
		// matching a single nested "feature-radar/feature-radar").
		if isGroupDir(name, sourcePath) {
			groupMatches, groupErr := resolveGroupUpdatable(name, sourcePath)
			if groupErr != nil {
				resolveWarnings = append(resolveWarnings, fmt.Sprintf("%s: %v", name, groupErr))
				continue
			}
			if len(groupMatches) == 0 {
				resolveWarnings = append(resolveWarnings, fmt.Sprintf("%s: no updatable skills in group", name))
				continue
			}
			ui.Info("'%s' is a group — expanding to %d updatable skill(s)", name, len(groupMatches))
			for _, m := range groupMatches {
				p := filepath.Join(sourcePath, m.relPath)
				if !seen[p] {
					seen[p] = true
					targets = append(targets, projectTarget{name: m.relPath, path: p, isRepo: m.isRepo})
				}
			}
			continue
		}

		// Normalize _ prefix for tracked repos
		repoName := name
		if !strings.HasPrefix(repoName, "_") {
			prefixed := filepath.Join(sourcePath, "_"+name)
			if install.IsGitRepo(prefixed) {
				repoName = "_" + name
			}
		}
		repoPath := filepath.Join(sourcePath, repoName)

		if install.IsGitRepo(repoPath) {
			if !seen[repoPath] {
				seen[repoPath] = true
				targets = append(targets, projectTarget{name: repoName, path: repoPath, isRepo: true})
			}
			continue
		}

		// Regular skill with metadata
		skillPath := filepath.Join(sourcePath, name)
		if info, err := os.Stat(skillPath); err == nil && info.IsDir() {
			meta, metaErr := install.ReadMeta(skillPath)
			if metaErr == nil && meta != nil && meta.Source != "" {
				if !seen[skillPath] {
					seen[skillPath] = true
					targets = append(targets, projectTarget{name: name, path: skillPath, isRepo: false})
				}
				continue
			}
			resolveWarnings = append(resolveWarnings, fmt.Sprintf("%s is a local skill, nothing to update", name))
			continue
		}

		resolveWarnings = append(resolveWarnings, fmt.Sprintf("skill '%s' not found", name))
	}

	for _, group := range opts.groups {
		groupMatches, err := resolveGroupUpdatable(group, sourcePath)
		if err != nil {
			resolveWarnings = append(resolveWarnings, fmt.Sprintf("--group %s: %v", group, err))
			continue
		}
		if len(groupMatches) == 0 {
			resolveWarnings = append(resolveWarnings, fmt.Sprintf("--group %s: no updatable skills in group", group))
			continue
		}
		for _, m := range groupMatches {
			p := filepath.Join(sourcePath, m.relPath)
			if !seen[p] {
				seen[p] = true
				targets = append(targets, projectTarget{name: m.relPath, path: p, isRepo: m.isRepo})
			}
		}
	}

	for _, w := range resolveWarnings {
		ui.Warning("%s", w)
	}

	if len(targets) == 0 {
		if len(resolveWarnings) > 0 {
			return fmt.Errorf("no valid skills to update")
		}
		return fmt.Errorf("no skills found")
	}

	// --- Execute ---
	if len(targets) == 1 {
		t := targets[0]
		if t.isRepo {
			return updateProjectTrackedRepo(t.name, t.path, opts.dryRun, opts.force)
		}
		return updateSingleProjectSkill(sourcePath, t.name, opts.dryRun, opts.force)
	}

	// Batch mode
	if opts.dryRun {
		ui.Warning("Dry run mode - no changes will be made")
	}

	updated := 0
	for _, t := range targets {
		if t.isRepo {
			if err := updateProjectTrackedRepo(t.name, t.path, opts.dryRun, opts.force); err != nil {
				ui.Warning("%s: %v", t.name, err)
			} else {
				updated++
			}
		} else {
			if err := updateSingleProjectSkill(sourcePath, t.name, opts.dryRun, opts.force); err != nil {
				ui.Warning("%s: %v", t.name, err)
			} else {
				updated++
			}
		}
	}

	if updated > 0 && !opts.dryRun {
		fmt.Println()
		ui.Info("Run 'skillshare sync' to distribute changes")
	}

	return nil
}

func updateSingleProjectSkill(sourcePath, name string, dryRun, force bool) error {
	// Normalize _ prefix for tracked repos
	repoName := name
	if !strings.HasPrefix(repoName, "_") {
		prefixed := filepath.Join(sourcePath, "_"+name)
		if install.IsGitRepo(prefixed) {
			repoName = "_" + name
		}
	}
	repoPath := filepath.Join(sourcePath, repoName)

	// Try as tracked repo first
	if install.IsGitRepo(repoPath) {
		return updateProjectTrackedRepo(repoName, repoPath, dryRun, force)
	}

	// Regular skill with metadata
	skillPath := filepath.Join(sourcePath, name)
	if _, err := os.Stat(skillPath); err != nil {
		return fmt.Errorf("skill '%s' not found", name)
	}

	meta, err := install.ReadMeta(skillPath)
	if err != nil || meta == nil {
		return fmt.Errorf("%s is a local skill, nothing to update", name)
	}

	source, err := install.ParseSource(meta.Source)
	if err != nil {
		return fmt.Errorf("invalid source for %s: %w", name, err)
	}

	if dryRun {
		ui.Info("[dry-run] would update %s", name)
		return nil
	}

	spinner := ui.StartSpinner(fmt.Sprintf("Updating %s...", name))
	opts := install.InstallOptions{Force: true, Update: true}
	if _, err := install.Install(source, skillPath, opts); err != nil {
		spinner.Fail(fmt.Sprintf("%s failed: %v", name, err))
		return nil
	}
	spinner.Success(fmt.Sprintf("Updated %s", name))
	fmt.Println()
	ui.Info("Run 'skillshare sync' to distribute changes")
	return nil
}

func updateProjectTrackedRepo(repoName, repoPath string, dryRun, force bool) error {
	// Check for uncommitted changes
	if isDirty, _ := git.IsDirty(repoPath); isDirty {
		if !force {
			ui.Warning("%s has uncommitted changes (use --force to discard)", repoName)
			return fmt.Errorf("uncommitted changes in %s", repoName)
		}
		if !dryRun {
			if err := git.Restore(repoPath); err != nil {
				return fmt.Errorf("failed to discard changes: %w", err)
			}
		}
	}

	if dryRun {
		ui.Info("[dry-run] would git pull %s", repoName)
		return nil
	}

	spinner := ui.StartSpinner(fmt.Sprintf("Updating %s...", repoName))

	var info *git.UpdateInfo
	var err error
	if force {
		info, err = git.ForcePull(repoPath)
	} else {
		info, err = git.Pull(repoPath)
	}
	if err != nil {
		spinner.Fail(fmt.Sprintf("%s failed: %v", repoName, err))
		return nil
	}

	if info.UpToDate {
		spinner.Success(fmt.Sprintf("%s already up to date", repoName))
	} else {
		spinner.Success(fmt.Sprintf("%s %d commits, %d files", repoName, len(info.Commits), info.Stats.FilesChanged))
	}
	fmt.Println()
	ui.Info("Run 'skillshare sync' to distribute changes")
	return nil
}

func updateAllProjectSkills(sourcePath string, dryRun, force bool) error {
	entries, err := os.ReadDir(sourcePath)
	if err != nil {
		return fmt.Errorf("failed to read project skills: %w", err)
	}

	if dryRun {
		ui.Warning("Dry run mode - no changes will be made")
	}

	updated := 0
	for _, entry := range entries {
		if !entry.IsDir() || utils.IsHidden(entry.Name()) {
			continue
		}

		skillName := entry.Name()
		skillPath := filepath.Join(sourcePath, skillName)

		// Tracked repo: git pull
		if install.IsGitRepo(skillPath) {
			if err := updateProjectTrackedRepo(skillName, skillPath, dryRun, force); err != nil {
				ui.Warning("%s: %v", skillName, err)
			} else {
				updated++
			}
			continue
		}

		// Regular skill with metadata: reinstall
		meta, err := install.ReadMeta(skillPath)
		if err != nil || meta == nil {
			continue
		}

		source, err := install.ParseSource(meta.Source)
		if err != nil {
			ui.Warning("%s invalid source: %v", skillName, err)
			continue
		}

		if dryRun {
			ui.Info("[dry-run] would update %s", skillName)
			continue
		}

		spinner := ui.StartSpinner(fmt.Sprintf("Updating %s...", skillName))
		if _, err := install.Install(source, skillPath, install.InstallOptions{Force: true, Update: true}); err != nil {
			spinner.Fail(fmt.Sprintf("%s failed: %v", skillName, err))
			continue
		}
		spinner.Success(fmt.Sprintf("Updated %s", skillName))
		updated++
	}

	if updated > 0 && !dryRun {
		fmt.Println()
		ui.Info("Run 'skillshare sync' to distribute changes")
	}

	return nil
}
