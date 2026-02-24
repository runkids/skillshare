package main

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"skillshare/internal/audit"
	"skillshare/internal/config"
	"skillshare/internal/git"
	"skillshare/internal/install"
	"skillshare/internal/oplog"
	"skillshare/internal/ui"
	"skillshare/internal/utils"
)

// updateOptions holds parsed arguments for update command
type updateOptions struct {
	names     []string // positional args (0+)
	groups    []string // --group/-G values (repeatable)
	all       bool
	dryRun    bool
	force     bool
	skipAudit bool
	diff      bool
	threshold string
}

// parseUpdateArgs parses command line arguments for the update command.
// Returns (opts, showHelp, error).
func parseUpdateArgs(args []string) (*updateOptions, bool, error) {
	opts := &updateOptions{}

	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--all" || arg == "-a":
			opts.all = true
		case arg == "--dry-run" || arg == "-n":
			opts.dryRun = true
		case arg == "--force" || arg == "-f":
			opts.force = true
		case arg == "--skip-audit":
			opts.skipAudit = true
		case arg == "--audit-threshold" || arg == "--threshold" || arg == "-T":
			i++
			if i >= len(args) {
				return nil, false, fmt.Errorf("%s requires a value", arg)
			}
			threshold, err := normalizeInstallAuditThreshold(args[i])
			if err != nil {
				return nil, false, err
			}
			opts.threshold = threshold
		case arg == "--diff":
			opts.diff = true
		case arg == "--group" || arg == "-G":
			i++
			if i >= len(args) {
				return nil, false, fmt.Errorf("--group requires a value")
			}
			opts.groups = append(opts.groups, args[i])
		case arg == "--help" || arg == "-h":
			return nil, true, nil
		case strings.HasPrefix(arg, "-"):
			return nil, false, fmt.Errorf("unknown option: %s", arg)
		default:
			opts.names = append(opts.names, arg)
		}
	}

	if opts.all && (len(opts.names) > 0 || len(opts.groups) > 0) {
		return nil, false, fmt.Errorf("--all cannot be used with skill names or --group")
	}

	if len(opts.names) == 0 && len(opts.groups) == 0 && !opts.all {
		return nil, true, fmt.Errorf("specify a skill or repo name, or use --all")
	}

	return opts, false, nil
}

// resolveGroupUpdatable finds all updatable items (tracked repos or skills with
// metadata) under a group directory. Local skills without metadata are skipped.
func resolveGroupUpdatable(group, sourceDir string) ([]resolvedMatch, error) {
	group = strings.TrimSuffix(group, "/")
	groupPath := filepath.Join(sourceDir, group)

	info, err := os.Stat(groupPath)
	if err != nil || !info.IsDir() {
		return nil, fmt.Errorf("group '%s' not found in source", group)
	}

	var matches []resolvedMatch
	if walkErr := filepath.Walk(groupPath, func(path string, fi os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if path == groupPath || !fi.IsDir() {
			return nil
		}
		if fi.Name() == ".git" {
			return filepath.SkipDir
		}

		rel, relErr := filepath.Rel(sourceDir, path)
		if relErr != nil || rel == "." {
			return nil
		}

		// Tracked repo (has .git)
		if install.IsGitRepo(path) {
			matches = append(matches, resolvedMatch{relPath: rel, isRepo: true})
			return filepath.SkipDir
		}

		// Skill with metadata (has .skillshare-meta.json)
		if meta, metaErr := install.ReadMeta(path); metaErr == nil && meta != nil && meta.Source != "" {
			matches = append(matches, resolvedMatch{relPath: rel, isRepo: false})
			return filepath.SkipDir
		}

		return nil
	}); walkErr != nil {
		return nil, fmt.Errorf("failed to walk group '%s': %w", group, walkErr)
	}

	return matches, nil
}

// isGroupDir checks if a name corresponds to a group directory (a container
// for other skills). Returns false for tracked repos, skills with metadata,
// and directories that are themselves a skill (have SKILL.md).
func isGroupDir(name, sourceDir string) bool {
	path := filepath.Join(sourceDir, name)
	info, err := os.Stat(path)
	if err != nil || !info.IsDir() {
		return false
	}
	// Not a tracked repo
	if install.IsGitRepo(path) {
		return false
	}
	// Not a skill with metadata
	if meta, metaErr := install.ReadMeta(path); metaErr == nil && meta != nil && meta.Source != "" {
		return false
	}
	// Not a skill directory (has SKILL.md)
	if _, statErr := os.Stat(filepath.Join(path, "SKILL.md")); statErr == nil {
		return false
	}
	return true
}

func cmdUpdate(args []string) error {
	start := time.Now()

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
		err := cmdUpdateProject(rest, cwd)
		logUpdateOp(config.ProjectConfigPath(cwd), rest, start, err)
		return err
	}

	opts, showHelp, parseErr := parseUpdateArgs(rest)
	if showHelp {
		printUpdateHelp()
		return parseErr
	}
	if parseErr != nil {
		return parseErr
	}

	cfg, err := config.Load()
	if err != nil {
		return err
	}
	if opts.threshold == "" {
		opts.threshold = cfg.Audit.BlockThreshold
	}

	ui.Header("Global Update")
	ui.Info("Storage    %s", cfg.Source)
	fmt.Println()

	// --- Resolve targets ---
	var targets []resolvedMatch
	seen := map[string]bool{}
	var resolveWarnings []string

	if opts.all {
		// Recursive discovery for --all
		err := filepath.Walk(cfg.Source, func(path string, info os.FileInfo, err error) error {
			if err != nil || path == cfg.Source {
				return nil
			}
			if info.IsDir() && utils.IsHidden(info.Name()) {
				return filepath.SkipDir
			}
			if info.IsDir() && info.Name() == ".git" {
				return filepath.SkipDir
			}

			// Tracked repo
			if info.IsDir() && strings.HasPrefix(info.Name(), "_") {
				if install.IsGitRepo(path) {
					rel, _ := filepath.Rel(cfg.Source, path)
					if !seen[rel] {
						seen[rel] = true
						targets = append(targets, resolvedMatch{relPath: rel, isRepo: true})
					}
					return filepath.SkipDir
				}
			}

			// Regular skill
			if !info.IsDir() && info.Name() == "SKILL.md" {
				skillDir := filepath.Dir(path)
				meta, metaErr := install.ReadMeta(skillDir)
				if metaErr == nil && meta != nil && meta.Source != "" {
					rel, _ := filepath.Rel(cfg.Source, skillDir)
					if rel != "." && !seen[rel] {
						seen[rel] = true
						targets = append(targets, resolvedMatch{relPath: rel, isRepo: false})
					}
				}
			}
			return nil
		})
		if err != nil {
			return fmt.Errorf("failed to scan skills: %w", err)
		}
	} else {
		// Resolve by specific names/groups
		for _, name := range opts.names {
			if isGroupDir(name, cfg.Source) {
				groupMatches, groupErr := resolveGroupUpdatable(name, cfg.Source)
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
					if !seen[m.relPath] {
						seen[m.relPath] = true
						targets = append(targets, m)
					}
				}
				continue
			}

			match, err := resolveByBasename(cfg.Source, name)
			if err != nil {
				resolveWarnings = append(resolveWarnings, fmt.Sprintf("%s: %v", name, err))
				continue
			}
			if !seen[match.relPath] {
				seen[match.relPath] = true
				targets = append(targets, match)
			}
		}

		for _, group := range opts.groups {
			groupMatches, err := resolveGroupUpdatable(group, cfg.Source)
			if err != nil {
				resolveWarnings = append(resolveWarnings, fmt.Sprintf("--group %s: %v", group, err))
				continue
			}
			if len(groupMatches) == 0 {
				resolveWarnings = append(resolveWarnings, fmt.Sprintf("--group %s: no updatable skills in group", group))
				continue
			}
			for _, m := range groupMatches {
				if !seen[m.relPath] {
					seen[m.relPath] = true
					targets = append(targets, m)
				}
			}
		}
	}

	for _, w := range resolveWarnings {
		ui.Warning("%s", w)
	}

	if len(targets) == 0 {
		if opts.all {
			ui.Header("Updating 0 skill(s)")
			fmt.Println()
			ui.SuccessMsg("Updated 0, skipped 0 of 0 skill(s)")
			return nil
		}
		if len(resolveWarnings) > 0 {
			return fmt.Errorf("no valid skills to update")
		}
		return fmt.Errorf("no skills found")
	}

	// --- Execute ---
	if len(targets) == 1 {
		// Single target: verbose path
		t := targets[0]
		var updateErr error
		if t.isRepo {
			updateErr = updateTrackedRepo(cfg, t.relPath, opts.dryRun, opts.force, opts.skipAudit, opts.diff, opts.threshold)
		} else {
			updateErr = updateRegularSkill(cfg, t.relPath, opts.dryRun, opts.force, opts.skipAudit, opts.diff, opts.threshold)
		}

		var opNames []string
		if opts.all {
			opNames = []string{"--all"}
		} else {
			opNames = opts.names
		}
		logUpdateOp(config.ConfigPath(), opNames, start, updateErr)
		return updateErr
	}

	// Multiple targets: batch path with progress bar
	total := len(targets)
	ui.Header(fmt.Sprintf("Updating %d skill(s)", total))
	fmt.Println()

	if opts.dryRun {
		ui.Warning("[dry-run] No changes will be made")
	}

	progressBar := ui.StartProgress("Updating skills", total)

	var result updateResult
	var auditEntries []batchAuditEntry
	for _, t := range targets {
		progressBar.UpdateTitle(fmt.Sprintf("Updating %s", t.relPath))
		itemPath := filepath.Join(cfg.Source, t.relPath)
		if t.isRepo {
			updated, err := updateTrackedRepoQuick(t.relPath, itemPath, opts.dryRun, opts.force, opts.skipAudit, opts.threshold)
			if err != nil {
				result.securityFailed++
			} else if updated {
				result.updated++
			} else {
				result.skipped++
			}
		} else {
			updated, riskLabel, err := updateSkillFromMeta(t.relPath, itemPath, opts.dryRun, opts.skipAudit, opts.threshold)
			if err != nil && isSecurityError(err) {
				result.securityFailed++
			} else if updated {
				result.updated++
				if riskLabel != "" {
					auditEntries = append(auditEntries, batchAuditEntry{name: t.relPath, risk: riskLabel})
				}
			} else {
				result.skipped++
			}
		}
		progressBar.Increment()
	}
	progressBar.Stop()

	if !opts.dryRun {
		renderBatchAuditSummary(auditEntries)
		fmt.Println()
		ui.SuccessMsg("Updated %d, skipped %d of %d skill(s)", result.updated, result.skipped, total)
		if result.securityFailed > 0 {
			ui.Warning("Blocked: %d (security)", result.securityFailed)
		}
	}

	if result.updated > 0 && !opts.dryRun {
		ui.SectionLabel("Next Steps")
		ui.Info("Run 'skillshare sync' to distribute changes")
	}

	// Build oplog names
	var opNames []string
	if opts.all {
		opNames = []string{"--all"}
	} else {
		opNames = append(opNames, opts.names...)
		for _, g := range opts.groups {
			opNames = append(opNames, "--group="+g)
		}
	}

	var batchErr error
	if result.securityFailed > 0 {
		batchErr = fmt.Errorf("%d repo(s) blocked by security audit", result.securityFailed)
	}
	logUpdateOp(config.ConfigPath(), opNames, start, batchErr)

	return batchErr
}

func logUpdateOp(cfgPath string, args []string, start time.Time, cmdErr error) {
	e := oplog.NewEntry("update", statusFromErr(cmdErr), time.Since(start))
	if len(args) == 1 {
		e.Args = map[string]any{"name": args[0]}
	} else if len(args) > 1 {
		e.Args = map[string]any{"names": args}
	}
	if cmdErr != nil {
		e.Message = cmdErr.Error()
	}
	oplog.Write(cfgPath, oplog.OpsFile, e) //nolint:errcheck
}

// updateResult tracks the result of an update operation
type updateResult struct {
	updated        int
	skipped        int
	securityFailed int
}

// batchAuditEntry holds per-item audit info for post-batch summary.
type batchAuditEntry struct {
	name string
	risk string // e.g. "CLEAN", "MEDIUM (42/100)"
}

func renderBatchAuditSummary(entries []batchAuditEntry) {
	if len(entries) == 0 {
		return
	}

	ui.SectionLabel("Audit Findings")

	clean := 0
	var notable []batchAuditEntry
	for _, e := range entries {
		if e.risk == "CLEAN" {
			clean++
		} else {
			notable = append(notable, e)
		}
	}

	for _, e := range notable {
		ui.Warning("risk: %s — %s", e.risk, e.name)
	}
	if clean > 0 {
		ui.Info("Audit: %d skill(s) CLEAN", clean)
	}
}

// updateTrackedRepoQuick updates a single tracked repo in batch mode.
// Output is suppressed; caller handles display via progress bar.
func updateTrackedRepoQuick(repo, repoPath string, dryRun, force, skipAudit bool, threshold string) (updated bool, err error) {
	// Check for uncommitted changes
	if isDirty, _ := git.IsDirty(repoPath); isDirty {
		if !force {
			return false, nil
		}
		if !dryRun {
			if err := git.Restore(repoPath); err != nil {
				return false, nil
			}
		}
	}

	if dryRun {
		return false, nil
	}

	var info *git.UpdateInfo
	if force {
		info, err = git.ForcePullWithProgress(repoPath, git.AuthEnvForRepo(repoPath), nil)
	} else {
		info, err = git.PullWithProgress(repoPath, git.AuthEnvForRepo(repoPath), nil)
	}
	if err != nil {
		return false, nil
	}

	if info.UpToDate {
		return false, nil
	}

	// Post-pull audit gate
	if err := auditGateAfterPull(repoPath, info.BeforeHash, skipAudit, threshold, audit.ScanSkill); err != nil {
		return false, err
	}

	return true, nil
}

// updateSkillFromMeta updates a skill using its metadata in batch mode.
// Output is suppressed; caller handles display via progress bar.
// Returns (updated, riskLabel, error). riskLabel is e.g. "CLEAN" or "MEDIUM (42/100)".
func updateSkillFromMeta(skill, skillPath string, dryRun, skipAudit bool, threshold string) (bool, string, error) {
	if dryRun {
		return false, "", nil
	}

	meta, _ := install.ReadMeta(skillPath)
	source, err := install.ParseSource(meta.Source)
	if err != nil {
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

	// Build risk label for batch summary
	riskLabel := formatRiskLabel(result)
	return true, riskLabel, nil
}

// formatRiskLabel builds a display string from install result audit info.
func formatRiskLabel(result *install.InstallResult) string {
	if result == nil || result.AuditSkipped || result.AuditRiskLabel == "" {
		return ""
	}
	label := strings.ToUpper(result.AuditRiskLabel)
	if result.AuditRiskScore > 0 {
		return fmt.Sprintf("%s (%d/100)", label, result.AuditRiskScore)
	}
	return label
}

// updateSkillOrRepo updates a skill or repo by name, handling _ prefix and
// basename resolution.
func updateSkillOrRepo(cfg *config.Config, name string, dryRun, force, skipAudit, showDiff bool, threshold string) error {
	// Try tracked repo first (with _ prefix)
	repoName := name
	if !strings.HasPrefix(repoName, "_") {
		repoName = "_" + name
	}
	repoPath := filepath.Join(cfg.Source, repoName)

	if install.IsGitRepo(repoPath) {
		return updateTrackedRepo(cfg, repoName, dryRun, force, skipAudit, showDiff, threshold)
	}

	// Try as regular skill (exact path)
	skillPath := filepath.Join(cfg.Source, name)
	if meta, err := install.ReadMeta(skillPath); err == nil && meta != nil {
		return updateRegularSkill(cfg, name, dryRun, force, skipAudit, showDiff, threshold)
	}

	// Check if it's a nested path that exists as git repo
	if install.IsGitRepo(skillPath) {
		return updateTrackedRepo(cfg, name, dryRun, force, skipAudit, showDiff, threshold)
	}

	// Fallback: search by basename in nested skills and repos
	if match, err := resolveByBasename(cfg.Source, name); err == nil {
		if match.isRepo {
			return updateTrackedRepo(cfg, match.relPath, dryRun, force, skipAudit, showDiff, threshold)
		}
		return updateRegularSkill(cfg, match.relPath, dryRun, force, skipAudit, showDiff, threshold)
	} else {
		return err
	}
}

type resolvedMatch struct {
	relPath string
	isRepo  bool
}

// resolveByBasename searches nested skills and tracked repos by their
// directory basename. Returns an error when zero or multiple matches found.
func resolveByBasename(sourceDir, name string) (resolvedMatch, error) {
	var matches []resolvedMatch

	// Search tracked repos
	repos, _ := install.GetTrackedRepos(sourceDir)
	for _, r := range repos {
		if filepath.Base(r) == "_"+name || filepath.Base(r) == name {
			matches = append(matches, resolvedMatch{relPath: r, isRepo: true})
		}
	}

	// Search updatable skills
	skills, _ := install.GetUpdatableSkills(sourceDir)
	for _, s := range skills {
		if filepath.Base(s) == name {
			matches = append(matches, resolvedMatch{relPath: s, isRepo: false})
		}
	}

	if len(matches) == 0 {
		return resolvedMatch{}, fmt.Errorf("'%s' not found as tracked repo or skill with metadata", name)
	}
	if len(matches) == 1 {
		return matches[0], nil
	}

	// Ambiguous: list all matches
	lines := []string{fmt.Sprintf("'%s' matches multiple items:", name)}
	for _, m := range matches {
		lines = append(lines, fmt.Sprintf("  - %s", m.relPath))
	}
	lines = append(lines, "Please specify the full path")
	return resolvedMatch{}, fmt.Errorf("%s", strings.Join(lines, "\n"))
}

func updateTrackedRepo(cfg *config.Config, repoName string, dryRun, force, skipAudit, showDiff bool, threshold string) error {
	repoPath := filepath.Join(cfg.Source, repoName)
	start := time.Now()

	ui.StepStart("Repo", repoName)

	startUpdate := time.Now()
	// Check for uncommitted changes
	spinner := ui.StartSpinner("Checking status...")

	isDirty, _ := git.IsDirty(repoPath)
	if isDirty {
		spinner.Stop()
		files, _ := git.GetDirtyFiles(repoPath)

		if !force {
			lines := []string{
				"",
				"Repository has uncommitted changes:",
				"",
			}
			lines = append(lines, files...)
			lines = append(lines, "", "Use --force to discard changes and update", "")

			ui.WarningBox("Warning", lines...)
			fmt.Println()
			ui.ErrorMsg("Update aborted")
			return fmt.Errorf("uncommitted changes in repository")
		}

		ui.Warning("Discarding local changes (--force)")
		if !dryRun {
			if err := git.Restore(repoPath); err != nil {
				return fmt.Errorf("failed to discard changes: %w", err)
			}
		}
		spinner = ui.StartSpinner("Fetching from origin...")
	}

	if dryRun {
		spinner.Stop()
		ui.Warning("[dry-run] Would run: git pull")
		return nil
	}

	spinner.Update("   Fetching from origin...")
	var onProgress func(string)
	if ui.IsTTY() {
		onProgress = func(line string) {
			spinner.Update("   " + line)
		}
	}

	// Use ForcePull if --force to handle force push
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
		return fmt.Errorf("git pull failed: %w", err)
	}

	if info.UpToDate {
		spinner.Stop()
		ui.StepResult("success", "Already up to date", time.Since(startUpdate))
		return nil
	}

	spinner.Stop()
	ui.StepResult("success", fmt.Sprintf("%d commits, %d files updated", len(info.Commits), info.Stats.FilesChanged), time.Since(startUpdate))
	fmt.Println()

	// Show changes box
	lines := []string{
		"",
		fmt.Sprintf("  Commits:  %d new", len(info.Commits)),
		fmt.Sprintf("  Files:    %d changed (+%d / -%d)",
			info.Stats.FilesChanged, info.Stats.Insertions, info.Stats.Deletions),
		"",
	}

	// Show up to 5 commits
	maxCommits := 5
	for i, c := range info.Commits {
		if i >= maxCommits {
			lines = append(lines, fmt.Sprintf("  ... and %d more", len(info.Commits)-maxCommits))
			break
		}
		lines = append(lines, fmt.Sprintf("  %s  %s", c.Hash, truncateString(c.Message, 40)))
	}
	lines = append(lines, "")

	ui.Box("Changes", lines...)

	if showDiff {
		renderDiffSummary(repoPath, info.BeforeHash, info.AfterHash)
	}
	fmt.Println()

	// Post-pull audit gate
	if err := auditGateAfterPull(repoPath, info.BeforeHash, skipAudit, threshold, audit.ScanSkill); err != nil {
		return err
	}

	ui.SuccessMsg("Updated %s", repoName)
	ui.StepResult("success", "Updated successfully", time.Since(start))
	fmt.Println()
	ui.SectionLabel("Next Steps")
	ui.Info("Run 'skillshare sync' to distribute changes")

	return nil
}

func updateRegularSkill(cfg *config.Config, skillName string, dryRun, force, skipAudit, showDiff bool, threshold string) error {
	skillPath := filepath.Join(cfg.Source, skillName)

	// Read metadata to get source
	meta, err := install.ReadMeta(skillPath)
	if err != nil {
		return fmt.Errorf("cannot read metadata for '%s': %w", skillName, err)
	}
	if meta == nil || meta.Source == "" {
		return fmt.Errorf("skill '%s' has no source metadata, cannot update", skillName)
	}

	ui.StepStart("Skill", skillName)
	ui.StepContinue("Source", meta.Source)

	if dryRun {
		ui.Warning("[dry-run] Would reinstall from: %s", meta.Source)
		return nil
	}

	startUpdate := time.Now()
	// Parse source and reinstall
	source, err := install.ParseSource(meta.Source)
	if err != nil {
		return fmt.Errorf("invalid source in metadata: %w", err)
	}

	// Snapshot before update for --diff
	var beforeHashes map[string]string
	if showDiff {
		beforeHashes, _ = install.ComputeFileHashes(skillPath)
	}

	spinner := ui.StartSpinner("Updating...")

	opts := install.InstallOptions{
		Force:          true,
		Update:         true,
		SkipAudit:      skipAudit,
		AuditThreshold: threshold,
	}
	if ui.IsTTY() {
		opts.OnProgress = func(line string) {
			spinner.Update("   " + line)
		}
	}

	result, err := install.Install(source, skillPath, opts)
	if err != nil {
		spinner.Stop()
		ui.StepResult("error", fmt.Sprintf("Failed: %v", err), 0)
		return fmt.Errorf("update failed: %w", err)
	}

	spinner.Stop()
	ui.StepResult("success", "Updated successfully", time.Since(startUpdate))
	fmt.Println()

	renderInstallWarningsWithResult("", result.Warnings, false, result)

	if showDiff {
		afterHashes, _ := install.ComputeFileHashes(skillPath)
		renderHashDiffSummary(beforeHashes, afterHashes)
	}

	ui.SectionLabel("Next Steps")
	ui.Info("Run 'skillshare sync' to distribute changes")

	return nil
}

// auditScanFunc abstracts the audit scan call so the same gate logic
// can be used for both global mode (audit.ScanSkill) and project mode
// (audit.ScanSkillForProject with a captured projectRoot).
type auditScanFunc func(repoPath string) (*audit.Result, error)

// auditGateAfterPull scans the repo for security issues after a git pull.
// If findings are detected at or above threshold:
//   - TTY mode: prompts the user; on decline, resets to beforeHash.
//   - Non-TTY mode: automatically resets to beforeHash and returns error.
//
// Returns nil if audit passes or is skipped.
func auditGateAfterPull(repoPath, beforeHash string, skipAudit bool, threshold string, scanFn auditScanFunc) error {
	if skipAudit {
		return nil
	}
	normalizedThreshold, err := audit.NormalizeThreshold(threshold)
	if err != nil {
		normalizedThreshold = audit.DefaultThreshold()
	}

	result, err := scanFn(repoPath)
	if err != nil {
		// Scan error -> fail-closed across modes.
		if beforeHash == "" {
			return fmt.Errorf("security audit failed: %v — rollback commit unavailable, update aborted and repository state is unknown: %w", err, audit.ErrBlocked)
		}
		if resetErr := git.ResetHard(repoPath, beforeHash); resetErr != nil {
			return fmt.Errorf("security audit failed: %v; WARNING: rollback also failed: %v — malicious content may remain: %w", err, resetErr, audit.ErrBlocked)
		}
		return fmt.Errorf("security audit failed: %v — rolled back (use --skip-audit to bypass): %w", err, audit.ErrBlocked)
	}

	if !result.HasSeverityAtOrAbove(normalizedThreshold) {
		return nil
	}

	// Show findings
	for _, f := range result.Findings {
		if audit.SeverityRank(f.Severity) <= audit.SeverityRank(normalizedThreshold) {
			ui.Warning("[%s] %s (%s:%d)", f.Severity, f.Message, f.File, f.Line)
		}
	}

	if ui.IsTTY() {
		fmt.Printf("\n  Security findings at %s or above detected.\n", normalizedThreshold)
		fmt.Printf("  Apply anyway? [y/N]: ")
		reader := bufio.NewReader(os.Stdin)
		answer, _ := reader.ReadString('\n')
		answer = strings.TrimSpace(strings.ToLower(answer))
		if answer == "y" || answer == "yes" {
			return nil
		}
		// User declined → rollback
		if beforeHash == "" {
			return fmt.Errorf("security audit failed — findings at/above %s detected, rollback commit unavailable and repository state is unknown: %w", normalizedThreshold, audit.ErrBlocked)
		}
		if err := git.ResetHard(repoPath, beforeHash); err != nil {
			return fmt.Errorf("security audit failed — findings at/above %s detected; WARNING: rollback also failed: %v — malicious content may remain: %w", normalizedThreshold, err, audit.ErrBlocked)
		}
		ui.Info("Rolled back to %s", beforeHash[:12])
		return fmt.Errorf("security audit failed — findings at/above %s detected — rolled back (use --skip-audit to bypass): %w", normalizedThreshold, audit.ErrBlocked)
	}

	// Non-interactive → fail-closed
	if beforeHash == "" {
		return fmt.Errorf("security audit failed — findings at/above %s detected, rollback commit unavailable and repository state is unknown: %w", normalizedThreshold, audit.ErrBlocked)
	}
	if err := git.ResetHard(repoPath, beforeHash); err != nil {
		return fmt.Errorf("security audit failed — findings at/above %s detected; WARNING: rollback also failed: %v — malicious content may remain: %w", normalizedThreshold, err, audit.ErrBlocked)
	}
	return fmt.Errorf("security audit failed — findings at/above %s detected — rolled back (use --skip-audit to bypass): %w", normalizedThreshold, audit.ErrBlocked)
}

// renderDiffSummary prints a file-level change summary for the given repo.
func renderDiffSummary(repoPath, beforeHash, afterHash string) {
	changes, err := git.GetChangedFiles(repoPath, beforeHash, afterHash)
	if err != nil || len(changes) == 0 {
		return
	}

	maxFiles := 20
	lines := []string{""}
	for i, c := range changes {
		if i >= maxFiles {
			lines = append(lines, fmt.Sprintf("  ... and %d more file(s)", len(changes)-maxFiles))
			break
		}
		var marker string
		switch c.Status {
		case "A":
			marker = "+"
		case "D":
			marker = "-"
		default:
			marker = "~"
		}
		detail := fmt.Sprintf("  %s %s", marker, c.Path)
		if c.LinesAdded > 0 || c.LinesDeleted > 0 {
			detail += fmt.Sprintf(" (+%d -%d)", c.LinesAdded, c.LinesDeleted)
		}
		if c.OldPath != "" {
			detail += fmt.Sprintf(" (from %s)", c.OldPath)
		}
		lines = append(lines, detail)
	}
	lines = append(lines, "")

	ui.Box("Files Changed", lines...)
}

// renderHashDiffSummary prints a file-level change summary by comparing
// file hashes before and after an update. Works for non-git skill updates.
func renderHashDiffSummary(beforeHashes, afterHashes map[string]string) {
	type fileChange struct {
		path   string
		marker string // "+", "-", "~"
	}

	var changes []fileChange

	// Added or modified
	for path, afterHash := range afterHashes {
		beforeHash, existed := beforeHashes[path]
		if !existed {
			changes = append(changes, fileChange{path: path, marker: "+"})
		} else if beforeHash != afterHash {
			changes = append(changes, fileChange{path: path, marker: "~"})
		}
	}

	// Removed
	for path := range beforeHashes {
		if _, exists := afterHashes[path]; !exists {
			changes = append(changes, fileChange{path: path, marker: "-"})
		}
	}

	if len(changes) == 0 {
		ui.Info("No file changes detected")
		return
	}

	// Sort for deterministic output
	sort.Slice(changes, func(i, j int) bool {
		return changes[i].path < changes[j].path
	})

	maxFiles := 20
	lines := []string{""}
	for i, c := range changes {
		if i >= maxFiles {
			lines = append(lines, fmt.Sprintf("  ... and %d more file(s)", len(changes)-maxFiles))
			break
		}
		lines = append(lines, fmt.Sprintf("  %s %s", c.marker, c.path))
	}
	lines = append(lines, "")

	ui.Box("Files Changed", lines...)
}

// isSecurityError returns true if the error originated from the audit gate.
// All security-related errors wrap audit.ErrBlocked as a sentinel.
func isSecurityError(err error) bool {
	return errors.Is(err, audit.ErrBlocked)
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

func printUpdateHelp() {
	fmt.Println(`Usage: skillshare update <name>... [options]
       skillshare update --group <group> [options]
       skillshare update --all [options]

Update one or more skills or tracked repositories.

For tracked repos (_repo-name): runs git pull
For regular skills: reinstalls from stored source metadata

If a positional name matches a group directory (not a repo or skill), it is
automatically expanded to all updatable skills in that group.

Safety: Tracked repos with uncommitted changes are skipped by default.
Use --force to discard local changes and update.

Arguments:
  name...             Skill name(s) or tracked repo name(s)

Options:
  --all, -a           Update all tracked repos + skills with metadata
  --group, -G <name>  Update all updatable skills in a group (repeatable)
  --force, -f         Discard local changes and force update
  --dry-run, -n       Preview without making changes
  --skip-audit        Skip post-update security audit
  --audit-threshold, --threshold, -T <t>
                      Override update audit block threshold (critical|high|medium|low|info;
                      shorthand: c|h|m|l|i, plus crit, med)
  --diff              Show file-level change summary after update
  --project, -p       Use project-level config in current directory
  --global, -g        Use global config (~/.config/skillshare)
  --help, -h          Show this help

Examples:
  skillshare update my-skill              # Update single skill from source
  skillshare update a b c                 # Update multiple skills at once
  skillshare update --group frontend      # Update all skills in frontend/
  skillshare update x -G backend          # Mix names and groups
  skillshare update _team-skills          # Update tracked repo (git pull)
  skillshare update team-skills           # _ prefix is optional for repos
  skillshare update --all                 # Update all tracked repos + skills
  skillshare update --all -T high         # Use HIGH threshold for this run
  skillshare update --all --dry-run       # Preview updates
  skillshare update _team --force         # Discard changes and update`)
}
