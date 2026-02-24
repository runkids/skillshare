package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"skillshare/internal/audit"
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
		return updateAllProjectSkills(sourcePath, opts.dryRun, opts.force, opts.skipAudit, opts.diff, opts.threshold, opts.auditVerbose, root)
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
			return updateProjectTrackedRepo(t.name, t.path, opts.dryRun, opts.force, opts.skipAudit, opts.diff, opts.threshold, projectRoot)
		}
		return updateSingleProjectSkill(sourcePath, t.name, opts.dryRun, opts.force, opts.skipAudit, opts.diff, opts.threshold, opts.auditVerbose, projectRoot)
	}

	// Batch mode with progress bar
	total := len(targets)
	ui.Header(fmt.Sprintf("Updating %d skill(s)", total))
	fmt.Println()

	if opts.dryRun {
		ui.Warning("[dry-run] No changes will be made")
	}

	progressBar := ui.StartProgress("Updating skills", total)

	updated := 0
	skipped := 0
	securityFailed := 0
	var auditEntries []batchAuditEntry
	var blockedEntries []batchBlockedEntry

	// Group skills by RepoURL to optimize updates
	repoGroups := make(map[string][]updateTarget)
	var standaloneSkills []updateTarget
	var trackedRepos []updateTarget

	for _, t := range targets {
		if t.isRepo {
			trackedRepos = append(trackedRepos, t)
			continue
		}

		meta, err := install.ReadMeta(t.path)
		if err == nil && meta != nil && meta.RepoURL != "" {
			repoGroups[meta.RepoURL] = append(repoGroups[meta.RepoURL], t)
		} else {
			standaloneSkills = append(standaloneSkills, t)
		}
	}

	// 1. Process tracked repositories (git pull)
	for _, t := range trackedRepos {
		progressBar.UpdateTitle(fmt.Sprintf("Updating %s", t.name))
		repoUpdated, auditResult, err := updateProjectTrackedRepoQuick(t.name, t.path, opts.dryRun, opts.force, opts.skipAudit, opts.threshold, projectRoot)
		if err != nil {
			if isSecurityError(err) {
				securityFailed++
				blockedEntries = append(blockedEntries, batchBlockedEntry{name: t.name, errMsg: err.Error()})
			} else {
				skipped++
			}
		} else if repoUpdated {
			updated++
		} else {
			skipped++
		}
		if auditResult != nil {
			auditEntries = append(auditEntries, batchAuditEntryFromAuditResult(t.name, auditResult, opts.skipAudit))
		}
		progressBar.Increment()
	}

	// 2. Process grouped skills (one clone per repo)
	for repoURL, groupTargets := range repoGroups {
		if opts.dryRun {
			for _, t := range groupTargets {
				progressBar.UpdateTitle(fmt.Sprintf("Updating %s", t.name))
				progressBar.Increment()
				skipped++
			}
			continue
		}

		progressBar.UpdateTitle(fmt.Sprintf("Updating %d skills from %s", len(groupTargets), repoURL))

		// Map repo-internal subdir → local absolute path
		skillTargetMap := make(map[string]string)
		pathToTarget := make(map[string]updateTarget)
		for _, t := range groupTargets {
			meta, _ := install.ReadMeta(t.path)
			if meta != nil {
				skillTargetMap[meta.Subdir] = t.path
				pathToTarget[meta.Subdir] = t
			}
		}

		batchOpts := install.InstallOptions{
			Force:            true,
			Update:           true,
			SkipAudit:        opts.skipAudit,
			AuditThreshold:   opts.threshold,
			AuditProjectRoot: projectRoot,
		}
		if ui.IsTTY() {
			batchOpts.OnProgress = func(line string) {
				progressBar.UpdateTitle(line)
			}
		}

		batchResult, err := install.UpdateSkillsFromRepo(repoURL, skillTargetMap, batchOpts)
		if err != nil {
			for _, t := range groupTargets {
				progressBar.UpdateTitle(fmt.Sprintf("Failed %s: %v", t.name, err))
				skipped++
				progressBar.Increment()
			}
			continue
		}

		for subdir := range skillTargetMap {
			t := pathToTarget[subdir]
			progressBar.UpdateTitle(fmt.Sprintf("Updating %s", t.name))

			if ui.IsTTY() {
				time.Sleep(50 * time.Millisecond)
			}

			if err := batchResult.Errors[subdir]; err != nil {
				if isSecurityError(err) {
					securityFailed++
					blockedEntries = append(blockedEntries, batchBlockedEntry{name: t.name, errMsg: err.Error()})
				} else {
					skipped++
				}
			} else if res := batchResult.Results[subdir]; res != nil {
				updated++
				auditEntries = append(auditEntries, batchAuditEntryFromInstallResult(t.name, res))
			} else {
				skipped++
			}
			progressBar.Increment()
		}
	}

	// 3. Process standalone skills
	for _, t := range standaloneSkills {
		progressBar.UpdateTitle(fmt.Sprintf("Updating %s", t.name))
		ok, installRes, err := updateProjectSkillFromMeta(sourcePath, t.name, opts.dryRun, opts.skipAudit, opts.threshold)
		if err != nil {
			if isSecurityError(err) {
				securityFailed++
				blockedEntries = append(blockedEntries, batchBlockedEntry{name: t.name, errMsg: err.Error()})
			} else {
				skipped++
			}
		} else if ok {
			updated++
		} else {
			skipped++
		}
		if installRes != nil {
			auditEntries = append(auditEntries, batchAuditEntryFromInstallResult(t.name, installRes))
		}
		progressBar.Increment()
	}

	progressBar.Stop()

	if !opts.dryRun {
		displayUpdateBlockedSection(blockedEntries)
		displayUpdateAuditResults(auditEntries, opts.auditVerbose)
		fmt.Println()
		ui.SuccessMsg("Updated %d, skipped %d of %d skill(s)", updated, skipped, total)
		if securityFailed > 0 {
			ui.Warning("Blocked: %d (security)", securityFailed)
		}
	}

	if updated > 0 && !opts.dryRun {
		ui.SectionLabel("Next Steps")
		ui.Info("Run 'skillshare sync' to distribute changes")
	}

	if securityFailed > 0 {
		return fmt.Errorf("%d repo(s) blocked by security audit", securityFailed)
	}
	return nil
}

func updateSingleProjectSkill(sourcePath, name string, dryRun, force, skipAudit, showDiff bool, threshold string, auditVerbose bool, projectRoot string) error {
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
		return updateProjectTrackedRepo(repoName, repoPath, dryRun, force, skipAudit, showDiff, threshold, projectRoot)
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

	ui.StepStart("Skill", name)
	ui.StepContinue("Source", meta.Source)

	// Snapshot before update for --diff
	var beforeHashes map[string]string
	if showDiff {
		beforeHashes, _ = install.ComputeFileHashes(skillPath)
	}

	startUpdate := time.Now()
	spinner := ui.StartSpinner("Updating...")
	opts := install.InstallOptions{
		Force:          true,
		Update:         true,
		SkipAudit:      skipAudit,
		AuditThreshold: threshold,
	}
	if ui.IsTTY() {
		opts.OnProgress = func(line string) {
			spinner.Update("   " + line) // Indent to align with tree
		}
	}
	result, err := install.Install(source, skillPath, opts)
	if err != nil {
		spinner.Stop()
		ui.StepResult("error", fmt.Sprintf("Failed: %v", err), 0)
		return err
	}
	spinner.Stop()
	ui.StepResult("success", "Updated successfully", time.Since(startUpdate))
	fmt.Println()

	renderInstallWarningsWithResult("", result.Warnings, auditVerbose, result)

	if showDiff {
		afterHashes, _ := install.ComputeFileHashes(skillPath)
		renderHashDiffSummary(beforeHashes, afterHashes)
	}

	ui.SectionLabel("Next Steps")
	ui.Info("Run 'skillshare sync' to distribute changes")
	return nil
}

// updateProjectTrackedRepo updates a single tracked repo in project mode (verbose).
func updateProjectTrackedRepo(repoName, repoPath string, dryRun, force, skipAudit, showDiff bool, threshold, projectRoot string) error {
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

	ui.StepStart("Repo", repoName)

	startUpdate := time.Now()
	spinner := ui.StartSpinner("Checking status...")
	var onProgress func(string)
	if ui.IsTTY() {
		onProgress = func(line string) { spinner.Update(line) }
	}

	var info *git.UpdateInfo
	var err error
	if force {
		info, err = git.ForcePullWithProgress(repoPath, git.AuthEnvForRepo(repoPath), onProgress)
	} else {
		info, err = git.PullWithProgress(repoPath, git.AuthEnvForRepo(repoPath), onProgress)
	}
	if err != nil {
		spinner.Stop()
		ui.StepResult("error", fmt.Sprintf("Failed: %v", err), 0)
		return nil
	}

	if info.UpToDate {
		spinner.Stop()
		ui.StepResult("success", "Already up to date", time.Since(startUpdate))
		return nil
	}

	spinner.Stop()
	ui.StepResult("success", fmt.Sprintf("%d commits, %d files updated", len(info.Commits), info.Stats.FilesChanged), time.Since(startUpdate))
	fmt.Println()

	if showDiff {
		renderDiffSummary(repoPath, info.BeforeHash, info.AfterHash)
	}

	scanFn := func(path string) (*audit.Result, error) {
		return audit.ScanSkillForProject(path, projectRoot)
	}
	if _, err := auditGateAfterPull(repoPath, info.BeforeHash, skipAudit, threshold, scanFn); err != nil {
		return err
	}

	ui.SectionLabel("Next Steps")
	ui.Info("Run 'skillshare sync' to distribute changes")
	return nil
}

// updateProjectTrackedRepoQuick updates a tracked repo in project batch mode (quiet).
// Returns (updated, auditResult, error).
func updateProjectTrackedRepoQuick(repoName, repoPath string, dryRun, force, skipAudit bool, threshold, projectRoot string) (bool, *audit.Result, error) {
	if isDirty, _ := git.IsDirty(repoPath); isDirty {
		if !force {
			return false, nil, fmt.Errorf("uncommitted changes in %s", repoName)
		}
		if !dryRun {
			if err := git.Restore(repoPath); err != nil {
				return false, nil, fmt.Errorf("failed to discard changes: %w", err)
			}
		}
	}

	if dryRun {
		return false, nil, nil
	}

	var info *git.UpdateInfo
	var err error
	if force {
		info, err = git.ForcePullWithProgress(repoPath, git.AuthEnvForRepo(repoPath), nil)
	} else {
		info, err = git.PullWithProgress(repoPath, git.AuthEnvForRepo(repoPath), nil)
	}
	if err != nil || info.UpToDate {
		return false, nil, nil
	}

	scanFn := func(path string) (*audit.Result, error) {
		return audit.ScanSkillForProject(path, projectRoot)
	}
	auditResult, auditErr := auditGateAfterPull(repoPath, info.BeforeHash, skipAudit, threshold, scanFn)
	if auditErr != nil {
		return false, auditResult, auditErr
	}
	return true, auditResult, nil
}

// updateProjectSkillFromMeta updates a project skill using metadata in batch mode (quiet).
// Returns (updated, installResult, error).
func updateProjectSkillFromMeta(sourcePath, name string, dryRun, skipAudit bool, threshold string) (bool, *install.InstallResult, error) {
	skillPath := filepath.Join(sourcePath, name)
	if _, err := os.Stat(skillPath); err != nil {
		return false, nil, nil
	}

	meta, err := install.ReadMeta(skillPath)
	if err != nil || meta == nil || meta.Source == "" {
		return false, nil, nil
	}

	source, err := install.ParseSource(meta.Source)
	if err != nil {
		return false, nil, nil
	}

	if dryRun {
		return false, nil, nil
	}

	opts := install.InstallOptions{
		Force:          true,
		Update:         true,
		SkipAudit:      skipAudit,
		AuditThreshold: threshold,
	}
	result, err := install.Install(source, skillPath, opts)
	if err != nil {
		return false, nil, err
	}

	return true, result, nil
}

func updateAllProjectSkills(sourcePath string, dryRun, force, skipAudit, showDiff bool, threshold string, auditVerbose bool, projectRoot string) error {
	type target struct {
		name   string
		path   string
		isRepo bool
	}
	var targets []target

	err := filepath.Walk(sourcePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if path == sourcePath {
			return nil
		}
		// Skip hidden directories (like .skillshare)
		if info.IsDir() && utils.IsHidden(info.Name()) {
			return filepath.SkipDir
		}
		// Skip .git
		if info.IsDir() && info.Name() == ".git" {
			return filepath.SkipDir
		}

		// Tracked repo (_-prefixed)
		if info.IsDir() && strings.HasPrefix(info.Name(), "_") {
			if install.IsGitRepo(path) {
				rel, _ := filepath.Rel(sourcePath, path)
				targets = append(targets, target{name: rel, path: path, isRepo: true})
				return filepath.SkipDir // Don't look inside tracked repos
			}
		}

		// Regular skill with metadata
		if !info.IsDir() && info.Name() == "SKILL.md" {
			skillDir := filepath.Dir(path)
			meta, metaErr := install.ReadMeta(skillDir)
			if metaErr == nil && meta != nil && meta.Source != "" {
				rel, _ := filepath.Rel(sourcePath, skillDir)
				if rel != "." {
					targets = append(targets, target{name: rel, path: skillDir, isRepo: false})
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
			return updateProjectTrackedRepo(t.name, t.path, dryRun, force, skipAudit, showDiff, threshold, projectRoot)
		}
		return updateSingleProjectSkill(sourcePath, t.name, dryRun, force, skipAudit, showDiff, threshold, auditVerbose, projectRoot)
	}

	ui.Header(fmt.Sprintf("Updating %d skill(s)", total))
	fmt.Println()

	if dryRun {
		ui.Warning("[dry-run] No changes will be made")
	}

	progressBar := ui.StartProgress("Updating skills", total)

	updated := 0
	skipped := 0
	securityFailed := 0
	var auditEntries []batchAuditEntry
	var blockedEntries []batchBlockedEntry

	// Group skills by RepoURL to optimize updates
	repoGroups := make(map[string][]target)
	var standaloneSkills []target
	var trackedRepos []target

	for _, t := range targets {
		if t.isRepo {
			trackedRepos = append(trackedRepos, t)
			continue
		}

		meta, err := install.ReadMeta(t.path)
		if err == nil && meta != nil && meta.RepoURL != "" {
			repoGroups[meta.RepoURL] = append(repoGroups[meta.RepoURL], t)
		} else {
			standaloneSkills = append(standaloneSkills, t)
		}
	}

	// 1. Process tracked repositories (git pull)
	for _, t := range trackedRepos {
		progressBar.UpdateTitle(fmt.Sprintf("Updating %s", t.name))
		repoUpdated, auditResult, err := updateProjectTrackedRepoQuick(t.name, t.path, dryRun, force, skipAudit, threshold, projectRoot)
		if err != nil {
			if isSecurityError(err) {
				securityFailed++
				blockedEntries = append(blockedEntries, batchBlockedEntry{name: t.name, errMsg: err.Error()})
			} else {
				skipped++
			}
		} else if repoUpdated {
			updated++
		} else {
			skipped++
		}
		if auditResult != nil {
			auditEntries = append(auditEntries, batchAuditEntryFromAuditResult(t.name, auditResult, skipAudit))
		}
		progressBar.Increment()
	}

	// 2. Process grouped skills (one clone per repo)
	for repoURL, groupTargets := range repoGroups {
		if dryRun {
			for _, t := range groupTargets {
				progressBar.UpdateTitle(fmt.Sprintf("Updating %s", t.name))
				progressBar.Increment()
				skipped++
			}
			continue
		}

		progressBar.UpdateTitle(fmt.Sprintf("Updating %d skills from %s", len(groupTargets), repoURL))

		// Map repo-internal subdir → local absolute path
		skillTargetMap := make(map[string]string)
		pathToTarget := make(map[string]target)
		for _, t := range groupTargets {
			meta, _ := install.ReadMeta(t.path)
			if meta != nil {
				skillTargetMap[meta.Subdir] = t.path
				pathToTarget[meta.Subdir] = t
			}
		}

		batchOpts := install.InstallOptions{
			Force:          true,
			Update:         true,
			SkipAudit:      skipAudit,
			AuditThreshold: threshold,
		}
		if ui.IsTTY() {
			batchOpts.OnProgress = func(line string) {
				progressBar.UpdateTitle(line)
			}
		}

		batchResult, err := install.UpdateSkillsFromRepo(repoURL, skillTargetMap, batchOpts)
		if err != nil {
			for _, t := range groupTargets {
				progressBar.UpdateTitle(fmt.Sprintf("Failed %s: %v", t.name, err))
				skipped++
				progressBar.Increment()
			}
			continue
		}

		for subdir := range skillTargetMap {
			t := pathToTarget[subdir]
			progressBar.UpdateTitle(fmt.Sprintf("Updating %s", t.name))

			if ui.IsTTY() {
				time.Sleep(50 * time.Millisecond)
			}

			if err := batchResult.Errors[subdir]; err != nil {
				if isSecurityError(err) {
					securityFailed++
					blockedEntries = append(blockedEntries, batchBlockedEntry{name: t.name, errMsg: err.Error()})
				} else {
					skipped++
				}
			} else if res := batchResult.Results[subdir]; res != nil {
				updated++
				auditEntries = append(auditEntries, batchAuditEntryFromInstallResult(t.name, res))
			} else {
				skipped++
			}
			progressBar.Increment()
		}
	}

	// 3. Process standalone skills
	for _, t := range standaloneSkills {
		progressBar.UpdateTitle(fmt.Sprintf("Updating %s", t.name))
		ok, installRes, err := updateProjectSkillFromMeta(sourcePath, t.name, dryRun, skipAudit, threshold)
		if err != nil {
			if isSecurityError(err) {
				securityFailed++
				blockedEntries = append(blockedEntries, batchBlockedEntry{name: t.name, errMsg: err.Error()})
			} else {
				skipped++
			}
		} else if ok {
			updated++
		} else {
			skipped++
		}
		if installRes != nil {
			auditEntries = append(auditEntries, batchAuditEntryFromInstallResult(t.name, installRes))
		}
		progressBar.Increment()
	}

	progressBar.Stop()

	if !dryRun {
		displayUpdateBlockedSection(blockedEntries)
		displayUpdateAuditResults(auditEntries, auditVerbose)
		fmt.Println()
		ui.SuccessMsg("Updated %d, skipped %d of %d skill(s)", updated, skipped, total)
		if securityFailed > 0 {
			ui.Warning("Blocked: %d (security)", securityFailed)
		}
	}

	if updated > 0 && !dryRun {
		ui.SectionLabel("Next Steps")
		ui.Info("Run 'skillshare sync' to distribute changes")
	}

	if securityFailed > 0 {
		return fmt.Errorf("%d repo(s) blocked by security audit", securityFailed)
	}
	return nil
}
