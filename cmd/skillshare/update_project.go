package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

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

	runtime, err := loadProjectRuntime(root)
	if err != nil {
		return err
	}

	sourcePath := runtime.sourcePath
	if opts.threshold == "" {
		opts.threshold = runtime.config.Audit.BlockThreshold
	}

	ui.Header("Project Update")
	ui.Info("Directory  %s", root)
	fmt.Println()

	if opts.all {
		uc := &updateContext{sourcePath: sourcePath, projectRoot: root, opts: opts}
		return updateAllProjectSkills(uc)
	}

	return cmdUpdateProjectBatch(sourcePath, opts, root)
}

func cmdUpdateProjectBatch(sourcePath string, opts *updateOptions, projectRoot string) error {
	// --- Resolve targets ---
	var targets []updateTarget
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
				if !seen[m.path] {
					seen[m.path] = true
					targets = append(targets, m)
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
				targets = append(targets, updateTarget{name: repoName, path: repoPath, isRepo: true})
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
					targets = append(targets, updateTarget{name: name, path: skillPath, isRepo: false})
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
			if !seen[m.path] {
				seen[m.path] = true
				targets = append(targets, m)
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
			return updateTrackedRepo(sourcePath, t.name, opts.dryRun, opts.force, opts.skipAudit, opts.diff, opts.threshold, projectRoot)
		}
		return updateRegularSkill(sourcePath, t.name, opts.dryRun, opts.force, opts.skipAudit, opts.diff, opts.threshold, opts.auditVerbose, projectRoot)
	}

	// Batch mode
	total := len(targets)
	ui.Header(fmt.Sprintf("Updating %d skill(s)", total))
	fmt.Println()

	if opts.dryRun {
		ui.Warning("[dry-run] No changes will be made")
	}

	uc := &updateContext{sourcePath: sourcePath, projectRoot: projectRoot, opts: opts}
	_, batchErr := executeBatchUpdate(uc, targets)
	return batchErr
}

func updateAllProjectSkills(uc *updateContext) error {
	var targets []updateTarget

	err := filepath.Walk(uc.sourcePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if path == uc.sourcePath {
			return nil
		}
		if info.IsDir() && utils.IsHidden(info.Name()) {
			return filepath.SkipDir
		}
		if info.IsDir() && info.Name() == ".git" {
			return filepath.SkipDir
		}

		// Tracked repo (_-prefixed)
		if info.IsDir() && strings.HasPrefix(info.Name(), "_") {
			if install.IsGitRepo(path) {
				rel, _ := filepath.Rel(uc.sourcePath, path)
				targets = append(targets, updateTarget{name: rel, path: path, isRepo: true})
				return filepath.SkipDir
			}
		}

		// Regular skill with metadata
		if !info.IsDir() && info.Name() == "SKILL.md" {
			skillDir := filepath.Dir(path)
			meta, metaErr := install.ReadMeta(skillDir)
			if metaErr == nil && meta != nil && meta.Source != "" {
				rel, _ := filepath.Rel(uc.sourcePath, skillDir)
				if rel != "." {
					targets = append(targets, updateTarget{name: rel, path: skillDir, isRepo: false})
				}
			}
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to scan skills: %w", err)
	}

	total := len(targets)
	if total == 0 {
		ui.Header("Updating 0 skill(s)")
		fmt.Println()
		ui.SuccessMsg("Updated 0, skipped 0 of 0 skill(s)")
		return nil
	}

	// Single item: use verbose single-target path
	if total == 1 {
		t := targets[0]
		if t.isRepo {
			return updateTrackedRepo(uc.sourcePath, t.name, uc.opts.dryRun, uc.opts.force, uc.opts.skipAudit, uc.opts.diff, uc.opts.threshold, uc.projectRoot)
		}
		return updateRegularSkill(uc.sourcePath, t.name, uc.opts.dryRun, uc.opts.force, uc.opts.skipAudit, uc.opts.diff, uc.opts.threshold, uc.opts.auditVerbose, uc.projectRoot)
	}

	ui.Header(fmt.Sprintf("Updating %d skill(s)", total))
	fmt.Println()

	if uc.opts.dryRun {
		ui.Warning("[dry-run] No changes will be made")
	}

	_, batchErr := executeBatchUpdate(uc, targets)
	return batchErr
}
