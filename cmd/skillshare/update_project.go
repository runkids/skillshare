package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

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

	if opts.all {
		return updateAllProjectSkills(sourcePath, opts.dryRun, opts.force, opts.skipAudit, opts.diff, opts.threshold, root)
	}

	return cmdUpdateProjectBatch(sourcePath, opts, root)
}

func cmdUpdateProjectBatch(sourcePath string, opts *updateOptions, projectRoot string) error {
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
			return updateProjectTrackedRepo(t.name, t.path, opts.dryRun, opts.force, opts.skipAudit, opts.diff, opts.threshold, projectRoot)
		}
		return updateSingleProjectSkill(sourcePath, t.name, opts.dryRun, opts.force, opts.skipAudit, opts.diff, opts.threshold, projectRoot)
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
	for _, t := range targets {
		progressBar.UpdateTitle(fmt.Sprintf("Updating %s", t.name))
		if t.isRepo {
			if err := updateProjectTrackedRepoQuick(t.name, t.path, opts.dryRun, opts.force, opts.skipAudit, opts.threshold, projectRoot); err != nil {
				if isSecurityError(err) {
					securityFailed++
				} else {
					skipped++
				}
			} else {
				updated++
			}
		} else {
			ok, riskLabel, err := updateProjectSkillFromMeta(sourcePath, t.name, opts.dryRun, opts.skipAudit, opts.threshold)
			if err != nil {
				if isSecurityError(err) {
					securityFailed++
				} else {
					skipped++
				}
			} else if ok {
				updated++
				if riskLabel != "" {
					auditEntries = append(auditEntries, batchAuditEntry{name: t.name, risk: riskLabel})
				}
			} else {
				skipped++
			}
		}
		progressBar.Increment()
	}
	progressBar.Stop()

	if !opts.dryRun {
		renderBatchAuditSummary(auditEntries)
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

func updateSingleProjectSkill(sourcePath, name string, dryRun, force, skipAudit, showDiff bool, threshold, projectRoot string) error {
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

	// Snapshot before update for --diff
	var beforeHashes map[string]string
	if showDiff {
		beforeHashes, _ = install.ComputeFileHashes(skillPath)
	}

	spinner := ui.StartSpinner(fmt.Sprintf("Updating %s...", name))
	opts := install.InstallOptions{
		Force:          true,
		Update:         true,
		SkipAudit:      skipAudit,
		AuditThreshold: threshold,
	}
	if ui.IsTTY() {
		opts.OnProgress = func(line string) {
			spinner.Update(line)
		}
	}
	result, err := install.Install(source, skillPath, opts)
	if err != nil {
		spinner.Fail(fmt.Sprintf("%s failed: %v", name, err))
		return err
	}
	spinner.Success(fmt.Sprintf("Updated %s", name))
	renderInstallWarningsWithResult("", result.Warnings, false, result)

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

	spinner := ui.StartSpinner(fmt.Sprintf("Updating %s...", repoName))
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
		spinner.Fail(fmt.Sprintf("%s failed: %v", repoName, err))
		return nil
	}

	if info.UpToDate {
		spinner.Success(fmt.Sprintf("%s already up to date", repoName))
		return nil
	}

	spinner.Success(fmt.Sprintf("%s %d commits, %d files", repoName, len(info.Commits), info.Stats.FilesChanged))

	if showDiff {
		renderDiffSummary(repoPath, info.BeforeHash, info.AfterHash)
	}

	scanFn := func(path string) (*audit.Result, error) {
		return audit.ScanSkillForProject(path, projectRoot)
	}
	if err := auditGateAfterPull(repoPath, info.BeforeHash, skipAudit, threshold, scanFn); err != nil {
		return err
	}

	ui.SectionLabel("Next Steps")
	ui.Info("Run 'skillshare sync' to distribute changes")
	return nil
}

// updateProjectTrackedRepoQuick updates a tracked repo in project batch mode (quiet).
func updateProjectTrackedRepoQuick(repoName, repoPath string, dryRun, force, skipAudit bool, threshold, projectRoot string) error {
	if isDirty, _ := git.IsDirty(repoPath); isDirty {
		if !force {
			return fmt.Errorf("uncommitted changes in %s", repoName)
		}
		if !dryRun {
			if err := git.Restore(repoPath); err != nil {
				return fmt.Errorf("failed to discard changes: %w", err)
			}
		}
	}

	if dryRun {
		return nil
	}

	var info *git.UpdateInfo
	var err error
	if force {
		info, err = git.ForcePullWithProgress(repoPath, git.AuthEnvForRepo(repoPath), nil)
	} else {
		info, err = git.PullWithProgress(repoPath, git.AuthEnvForRepo(repoPath), nil)
	}
	if err != nil || info.UpToDate {
		return nil
	}

	scanFn := func(path string) (*audit.Result, error) {
		return audit.ScanSkillForProject(path, projectRoot)
	}
	return auditGateAfterPull(repoPath, info.BeforeHash, skipAudit, threshold, scanFn)
}

// updateProjectSkillFromMeta updates a project skill using metadata in batch mode (quiet).
// Returns (updated, riskLabel, error).
func updateProjectSkillFromMeta(sourcePath, name string, dryRun, skipAudit bool, threshold string) (bool, string, error) {
	skillPath := filepath.Join(sourcePath, name)
	if _, err := os.Stat(skillPath); err != nil {
		return false, "", nil
	}

	meta, err := install.ReadMeta(skillPath)
	if err != nil || meta == nil || meta.Source == "" {
		return false, "", nil
	}

	source, err := install.ParseSource(meta.Source)
	if err != nil {
		return false, "", nil
	}

	if dryRun {
		return false, "", nil
	}

	opts := install.InstallOptions{
		Force:          true,
		Update:         true,
		SkipAudit:      skipAudit,
		AuditThreshold: threshold,
	}
	result, err := install.Install(source, skillPath, opts)
	if err != nil {
		return false, "", err
	}

	return true, formatRiskLabel(result), nil
}

func updateAllProjectSkills(sourcePath string, dryRun, force, skipAudit, showDiff bool, threshold, projectRoot string) error {
	entries, err := os.ReadDir(sourcePath)
	if err != nil {
		return fmt.Errorf("failed to read project skills: %w", err)
	}

	// Count total updatable items
	var total int
	for _, entry := range entries {
		if !entry.IsDir() || utils.IsHidden(entry.Name()) {
			continue
		}
		p := filepath.Join(sourcePath, entry.Name())
		if install.IsGitRepo(p) {
			total++
			continue
		}
		if meta, metaErr := install.ReadMeta(p); metaErr == nil && meta != nil && meta.Source != "" {
			total++
		}
	}

	// Single item: use verbose single-target path
	if total == 1 {
		for _, entry := range entries {
			if !entry.IsDir() || utils.IsHidden(entry.Name()) {
				continue
			}
			name := entry.Name()
			p := filepath.Join(sourcePath, name)
			if install.IsGitRepo(p) {
				return updateProjectTrackedRepo(name, p, dryRun, force, skipAudit, showDiff, threshold, projectRoot)
			}
			if meta, metaErr := install.ReadMeta(p); metaErr == nil && meta != nil && meta.Source != "" {
				return updateSingleProjectSkill(sourcePath, name, dryRun, force, skipAudit, showDiff, threshold, projectRoot)
			}
		}
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
	for _, entry := range entries {
		if !entry.IsDir() || utils.IsHidden(entry.Name()) {
			continue
		}

		skillName := entry.Name()
		skillPath := filepath.Join(sourcePath, skillName)

		// Tracked repo: git pull
		if install.IsGitRepo(skillPath) {
			progressBar.UpdateTitle(fmt.Sprintf("Updating %s", skillName))
			if err := updateProjectTrackedRepoQuick(skillName, skillPath, dryRun, force, skipAudit, threshold, projectRoot); err != nil {
				if isSecurityError(err) {
					securityFailed++
				} else {
					skipped++
				}
			} else {
				updated++
			}
			progressBar.Increment()
			continue
		}

		// Skip non-updatable entries (no metadata)
		if meta, metaErr := install.ReadMeta(skillPath); metaErr != nil || meta == nil || meta.Source == "" {
			continue
		}

		// Regular skill with metadata: use batch helper
		progressBar.UpdateTitle(fmt.Sprintf("Updating %s", skillName))
		ok, riskLabel, err := updateProjectSkillFromMeta(sourcePath, skillName, dryRun, skipAudit, threshold)
		if err != nil {
			if isSecurityError(err) {
				securityFailed++
			} else {
				skipped++
			}
		} else if ok {
			updated++
			if riskLabel != "" {
				auditEntries = append(auditEntries, batchAuditEntry{name: skillName, risk: riskLabel})
			}
		}
		progressBar.Increment()
	}
	progressBar.Stop()

	if !dryRun {
		renderBatchAuditSummary(auditEntries)
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
