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

func cmdUpdateProject(args []string, root string) (*updateResult, error) {
	opts, showHelp, parseErr := parseUpdateArgs(args)
	if showHelp {
		printUpdateHelp()
		return nil, parseErr
	}
	if parseErr != nil {
		return nil, parseErr
	}

	// Project mode default: no args and no groups → --all
	if len(opts.names) == 0 && len(opts.groups) == 0 && !opts.all {
		opts.all = true
	}

	if !projectConfigExists(root) {
		if err := performProjectInit(root, projectInitOptions{}); err != nil {
			return nil, err
		}
	}

	runtime, err := loadProjectRuntime(root)
	if err != nil {
		return nil, err
	}

	sourcePath := utils.ResolveSymlink(runtime.sourcePath)
	if opts.threshold == "" {
		opts.threshold = runtime.config.Audit.BlockThreshold
	}

	ui.Header(ui.WithModeLabel("Updating"))
	ui.StepStart("Source", sourcePath)

	if opts.all {
		uc := &updateContext{sourcePath: sourcePath, projectRoot: root, opts: opts, parseOpts: parseOptsFromProjectConfig(runtime.config)}
		return updateAllProjectSkills(uc)
	}

	return cmdUpdateProjectBatch(sourcePath, opts, root, parseOptsFromProjectConfig(runtime.config))
}

func cmdUpdateProjectBatch(sourcePath string, opts *updateOptions, projectRoot string, pOpts install.ParseOptions) (*updateResult, error) {
	// --- Resolve targets ---
	var targets []updateTarget
	seen := map[string]bool{}
	var resolveWarnings []string

	metaStore, _ := install.LoadMetadataWithMigration(sourcePath, "")

	for _, name := range opts.names {
		// Check group directory first (before repo/skill lookup,
		// so "feature-radar" expands to all skills rather than
		// matching a single nested "feature-radar/feature-radar").
		if isGroupDir(name, sourcePath, metaStore) {
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
			entry := metaStore.Get(name)
			if entry != nil && entry.Source != "" {
				if !seen[skillPath] {
					seen[skillPath] = true
					targets = append(targets, updateTarget{name: name, path: skillPath, isRepo: false, meta: entry})
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

	var repoCount, skillCount int
	for _, t := range targets {
		if t.isRepo {
			repoCount++
		} else {
			skillCount++
		}
	}
	ui.StepEnd("Items", fmt.Sprintf("%d tracked repo(s), %d skill(s)", repoCount, skillCount))

	for _, w := range resolveWarnings {
		ui.Warning("%s", w)
	}

	if len(targets) == 0 {
		if len(resolveWarnings) > 0 {
			return nil, fmt.Errorf("no valid skills to update")
		}
		return nil, fmt.Errorf("no skills found")
	}

	// --- Execute ---
	uc := &updateContext{sourcePath: sourcePath, projectRoot: projectRoot, opts: opts, parseOpts: pOpts}

	if len(targets) == 1 {
		t := targets[0]
		var r updateResult
		var updateErr error
		if t.isRepo {
			r, updateErr = updateTrackedRepo(uc, t.name)
		} else {
			r, updateErr = updateRegularSkill(uc, t.name)
		}
		return &r, updateErr
	}

	// Batch mode
	if opts.dryRun {
		ui.Warning("[dry-run] No changes will be made")
	}

	batchResult, batchErr := executeBatchUpdate(uc, targets)
	return &batchResult, batchErr
}

func updateAllProjectSkills(uc *updateContext) (*updateResult, error) {
	var targets []updateTarget

	scanSpinner := ui.StartSpinner("Scanning skills...")
	walkRoot := uc.sourcePath
	metaStore, _ := install.LoadMetadataWithMigration(uc.sourcePath, "")
	err := filepath.Walk(walkRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if path == walkRoot {
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
				rel, _ := filepath.Rel(walkRoot, path)
				targets = append(targets, updateTarget{name: rel, path: path, isRepo: true})
				return filepath.SkipDir
			}
		}

		// Regular skill with metadata
		if !info.IsDir() && info.Name() == "SKILL.md" {
			skillDir := filepath.Dir(path)
			rel, _ := filepath.Rel(walkRoot, skillDir)
			if rel != "." {
				if entry := metaStore.Get(rel); entry != nil && entry.Source != "" {
					targets = append(targets, updateTarget{name: rel, path: skillDir, isRepo: false, meta: entry})
				}
			}
		}
		return nil
	})
	scanSpinner.Stop()
	if err != nil {
		return nil, fmt.Errorf("failed to scan skills: %w", err)
	}

	// Tracked repos declared in metadata but absent on disk (issue #212):
	// surface them instead of silently ignoring (mirrors global --all).
	existing := make(map[string]bool, len(targets))
	for _, t := range targets {
		existing[t.name] = true
	}
	missingRepos, _ := install.GetMissingTrackedRepos(uc.sourcePath)
	for _, repo := range missingRepos {
		if !existing[repo.Name] {
			existing[repo.Name] = true
			targets = append(targets, updateTarget{name: repo.Name, path: filepath.Join(uc.sourcePath, filepath.FromSlash(repo.Name)), isRepo: true})
		}
	}

	var repoCount, skillCount int
	for _, t := range targets {
		if t.isRepo {
			repoCount++
		} else {
			skillCount++
		}
	}
	ui.StepEnd("Items", fmt.Sprintf("%d tracked repo(s), %d skill(s)", repoCount, skillCount))

	total := len(targets)
	if total == 0 {
		ui.UpdateSummary(ui.UpdateStats{})
		return &updateResult{}, nil
	}

	// Single item: use verbose single-target path
	if total == 1 {
		t := targets[0]
		var r updateResult
		var updateErr error
		if t.isRepo {
			if mr, missing := missingTrackedRepoResult(t); missing {
				r = mr
				displayMissingTrackedReposWarning(r.missingTrackedRepos)
			} else {
				r, updateErr = updateTrackedRepo(uc, t.name)
			}
		} else {
			r, updateErr = updateRegularSkill(uc, t.name)
		}
		return &r, updateErr
	}

	if uc.opts.dryRun {
		ui.Warning("[dry-run] No changes will be made")
	}

	batchResult, batchErr := executeBatchUpdate(uc, targets)
	return &batchResult, batchErr
}
