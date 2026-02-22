package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"skillshare/internal/audit"
	"skillshare/internal/config"
	"skillshare/internal/git"
	"skillshare/internal/install"
	"skillshare/internal/oplog"
	"skillshare/internal/ui"
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

	if opts.all {
		err = updateAllTrackedRepos(cfg, opts.dryRun, opts.force, opts.skipAudit, opts.diff)
		logUpdateOp(config.ConfigPath(), []string{"--all"}, start, err)
		return err
	}

	// --- Resolve targets ---
	var targets []resolvedMatch
	seen := map[string]bool{}
	var resolveWarnings []string

	for _, name := range opts.names {
		// Check group directory first (direct path match takes priority
		// over basename, because e.g. "feature-radar" as a directory
		// should expand to all skills, not resolve to the nested
		// "feature-radar/feature-radar" via basename).
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
		// Single target: verbose path
		t := targets[0]
		var updateErr error
		if t.isRepo {
			updateErr = updateTrackedRepo(cfg, t.relPath, opts.dryRun, opts.force, opts.skipAudit, opts.diff)
		} else {
			updateErr = updateRegularSkill(cfg, t.relPath, opts.dryRun, opts.force, opts.skipAudit)
		}
		logUpdateOp(config.ConfigPath(), opts.names, start, updateErr)
		return updateErr
	}

	// Multiple targets: batch path
	total := len(targets)
	ui.HeaderBox("skillshare update",
		fmt.Sprintf("Updating %d skill(s)", total))
	fmt.Println()

	var result updateResult
	for i, t := range targets {
		progress := fmt.Sprintf("[%d/%d]", i+1, total)
		itemPath := filepath.Join(cfg.Source, t.relPath)
		if t.isRepo {
			updated, err := updateTrackedRepoQuick(t.relPath, itemPath, progress, opts.dryRun, opts.force, opts.skipAudit, opts.diff)
			if err != nil {
				result.securityFailed++
				ui.Warning("%s: %v", t.relPath, err)
			} else if updated {
				result.updated++
			} else {
				result.skipped++
			}
		} else {
			updated, err := updateSkillFromMeta(t.relPath, itemPath, progress, opts.dryRun, opts.skipAudit)
			if err != nil && isSecurityError(err) {
				result.securityFailed++
			} else if updated {
				result.updated++
			} else {
				result.skipped++
			}
		}
	}

	if !opts.dryRun {
		fmt.Println()
		lines := []string{
			"",
			fmt.Sprintf("  Total:    %d", total),
			fmt.Sprintf("  Updated:  %d", result.updated),
			fmt.Sprintf("  Skipped:  %d", result.skipped),
		}
		if result.securityFailed > 0 {
			lines = append(lines, fmt.Sprintf("  Blocked:  %d (security)", result.securityFailed))
		}
		lines = append(lines, "")
		ui.Box("Summary", lines...)
	}

	if result.updated > 0 {
		fmt.Println()
		ui.Info("Run 'skillshare sync' to distribute changes")
	}

	// Build oplog names
	var opNames []string
	opNames = append(opNames, opts.names...)
	for _, g := range opts.groups {
		opNames = append(opNames, "--group="+g)
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

// updateTrackedRepoQuick updates a single tracked repo (for --all mode)
func updateTrackedRepoQuick(repo, repoPath, progress string, dryRun, force, skipAudit, showDiff bool) (updated bool, err error) {
	// Check for uncommitted changes
	if isDirty, _ := git.IsDirty(repoPath); isDirty {
		if !force {
			ui.ListItem("warning", repo, "has uncommitted changes (use --force)")
			return false, nil
		}
		if !dryRun {
			if err := git.Restore(repoPath); err != nil {
				ui.ListItem("warning", repo, fmt.Sprintf("failed to discard changes: %v", err))
				return false, nil
			}
		}
	}

	if dryRun {
		ui.ListItem("info", repo, "[dry-run] would git pull")
		return false, nil
	}

	spinner := ui.StartSpinner(fmt.Sprintf("%s Updating %s...", progress, repo))

	var info *git.UpdateInfo
	if force {
		info, err = git.ForcePullWithAuth(repoPath)
	} else {
		info, err = git.PullWithAuth(repoPath)
	}
	if err != nil {
		spinner.Warn(fmt.Sprintf("%s %v", repo, err))
		return false, nil
	}

	if info.UpToDate {
		spinner.Success(fmt.Sprintf("%s Already up to date", repo))
		return false, nil
	}

	spinner.Success(fmt.Sprintf("%s %d commits, %d files", repo, len(info.Commits), info.Stats.FilesChanged))

	if showDiff {
		renderDiffSummary(repoPath, info.BeforeHash, info.AfterHash)
	}

	// Post-pull audit gate
	if err := auditGateAfterPull(repoPath, info.BeforeHash, skipAudit); err != nil {
		return false, err
	}

	return true, nil
}

// updateSkillFromMeta updates a skill using its metadata.
// Returns (true, nil) on success, (false, nil) on non-security skip,
// or (false, err) when install fails (caller should check isSecurityError).
func updateSkillFromMeta(skill, skillPath, progress string, dryRun, skipAudit bool) (bool, error) {
	if dryRun {
		ui.ListItem("info", skill, "[dry-run] would reinstall from source")
		return false, nil
	}

	spinner := ui.StartSpinner(fmt.Sprintf("%s Updating %s...", progress, skill))

	meta, _ := install.ReadMeta(skillPath)
	source, err := install.ParseSource(meta.Source)
	if err != nil {
		spinner.Warn(fmt.Sprintf("%s invalid source: %v", skill, err))
		return false, nil
	}

	opts := install.InstallOptions{Force: true, Update: true, SkipAudit: skipAudit}
	if _, err = install.Install(source, skillPath, opts); err != nil {
		spinner.Warn(fmt.Sprintf("%s %v", skill, err))
		return false, err
	}

	spinner.Success(fmt.Sprintf("%s Reinstalled from source", skill))
	return true, nil
}

func updateAllTrackedRepos(cfg *config.Config, dryRun, force, skipAudit, showDiff bool) error {
	repos, err := install.GetTrackedRepos(cfg.Source)
	if err != nil {
		return fmt.Errorf("failed to get tracked repos: %w", err)
	}

	skills, err := install.GetUpdatableSkills(cfg.Source)
	if err != nil {
		return fmt.Errorf("failed to get updatable skills: %w", err)
	}

	if len(repos) == 0 && len(skills) == 0 {
		ui.Info("No tracked repositories or updatable skills found")
		ui.Info("Use 'skillshare install <repo> --track' to add a tracked repository")
		return nil
	}

	total := len(repos) + len(skills)
	ui.HeaderBox("skillshare update --all",
		fmt.Sprintf("Updating %d tracked repos + %d skills", len(repos), len(skills)))
	fmt.Println()

	var result updateResult

	// Update tracked repos
	for i, repo := range repos {
		repoPath := filepath.Join(cfg.Source, repo)
		progress := fmt.Sprintf("[%d/%d]", i+1, total)
		updated, err := updateTrackedRepoQuick(repo, repoPath, progress, dryRun, force, skipAudit, showDiff)
		if err != nil {
			result.securityFailed++
			ui.Warning("%s: %v", repo, err)
		} else if updated {
			result.updated++
		} else {
			result.skipped++
		}
	}

	// Update regular skills
	for i, skill := range skills {
		skillPath := filepath.Join(cfg.Source, skill)
		progress := fmt.Sprintf("[%d/%d]", len(repos)+i+1, total)
		updated, err := updateSkillFromMeta(skill, skillPath, progress, dryRun, skipAudit)
		if err != nil && isSecurityError(err) {
			result.securityFailed++
		} else if updated {
			result.updated++
		} else {
			result.skipped++
		}
	}

	if !dryRun {
		fmt.Println()
		lines := []string{
			"",
			fmt.Sprintf("  Total:    %d", total),
			fmt.Sprintf("  Updated:  %d", result.updated),
			fmt.Sprintf("  Skipped:  %d", result.skipped),
		}
		if result.securityFailed > 0 {
			lines = append(lines, fmt.Sprintf("  Blocked:  %d (security)", result.securityFailed))
		}
		lines = append(lines, "")
		ui.Box("Summary", lines...)
	}

	if result.updated > 0 {
		fmt.Println()
		ui.Info("Run 'skillshare sync' to distribute changes")
	}

	if result.securityFailed > 0 {
		return fmt.Errorf("%d repo(s) blocked by security audit", result.securityFailed)
	}
	return nil
}

func updateSkillOrRepo(cfg *config.Config, name string, dryRun, force, skipAudit, showDiff bool) error {
	// Try tracked repo first (with _ prefix)
	repoName := name
	if !strings.HasPrefix(repoName, "_") {
		repoName = "_" + name
	}
	repoPath := filepath.Join(cfg.Source, repoName)

	if install.IsGitRepo(repoPath) {
		return updateTrackedRepo(cfg, repoName, dryRun, force, skipAudit, showDiff)
	}

	// Try as regular skill (exact path)
	skillPath := filepath.Join(cfg.Source, name)
	if meta, err := install.ReadMeta(skillPath); err == nil && meta != nil {
		return updateRegularSkill(cfg, name, dryRun, force, skipAudit)
	}

	// Check if it's a nested path that exists as git repo
	if install.IsGitRepo(skillPath) {
		return updateTrackedRepo(cfg, name, dryRun, force, skipAudit, showDiff)
	}

	// Fallback: search by basename in nested skills and repos
	if match, err := resolveByBasename(cfg.Source, name); err == nil {
		if match.isRepo {
			return updateTrackedRepo(cfg, match.relPath, dryRun, force, skipAudit, showDiff)
		}
		return updateRegularSkill(cfg, match.relPath, dryRun, force, skipAudit)
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

func updateTrackedRepo(cfg *config.Config, repoName string, dryRun, force, skipAudit, showDiff bool) error {
	repoPath := filepath.Join(cfg.Source, repoName)

	// Header box
	ui.HeaderBox("skillshare update", fmt.Sprintf("Updating: %s", repoName))
	fmt.Println()

	// Check for uncommitted changes
	spinner := ui.StartSpinner("Checking repository status...")

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

	spinner.Update("Fetching from origin...")

	// Use ForcePull if --force to handle force push
	var info *git.UpdateInfo
	var err error
	if force {
		info, err = git.ForcePullWithAuth(repoPath)
	} else {
		info, err = git.PullWithAuth(repoPath)
	}
	if err != nil {
		spinner.Fail("Failed to update")
		return fmt.Errorf("git pull failed: %w", err)
	}

	if info.UpToDate {
		spinner.Success("Already up to date")
		return nil
	}

	spinner.Stop()
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
	if err := auditGateAfterPull(repoPath, info.BeforeHash, skipAudit); err != nil {
		return err
	}

	ui.SuccessMsg("Updated %s", repoName)
	fmt.Println()
	ui.Info("Run 'skillshare sync' to distribute changes")

	return nil
}

func updateRegularSkill(cfg *config.Config, skillName string, dryRun, force, skipAudit bool) error {
	skillPath := filepath.Join(cfg.Source, skillName)

	// Read metadata to get source
	meta, err := install.ReadMeta(skillPath)
	if err != nil {
		return fmt.Errorf("cannot read metadata for '%s': %w", skillName, err)
	}
	if meta == nil || meta.Source == "" {
		return fmt.Errorf("skill '%s' has no source metadata, cannot update", skillName)
	}

	// Header box
	ui.HeaderBox("skillshare update",
		fmt.Sprintf("Updating: %s\nSource: %s", skillName, meta.Source))
	fmt.Println()

	if dryRun {
		ui.Warning("[dry-run] Would reinstall from: %s", meta.Source)
		return nil
	}

	// Parse source and reinstall
	source, err := install.ParseSource(meta.Source)
	if err != nil {
		return fmt.Errorf("invalid source in metadata: %w", err)
	}

	spinner := ui.StartSpinner("Cloning source repository...")

	opts := install.InstallOptions{
		Force:     true,
		Update:    true,
		SkipAudit: skipAudit,
	}

	result, err := install.Install(source, skillPath, opts)
	if err != nil {
		spinner.Fail("Failed to update")
		return fmt.Errorf("update failed: %w", err)
	}

	spinner.Success(fmt.Sprintf("Updated %s", skillName))

	for _, warning := range result.Warnings {
		ui.Warning("%s", warning)
	}

	fmt.Println()
	ui.Info("Run 'skillshare sync' to distribute changes")

	return nil
}

// auditGateAfterPull scans the repo for security issues after a git pull.
// If HIGH or CRITICAL findings are detected:
//   - TTY mode: prompts the user; on decline, resets to beforeHash.
//   - Non-TTY mode: automatically resets to beforeHash and returns error.
//
// Returns nil if audit passes or is skipped.
func auditGateAfterPull(repoPath, beforeHash string, skipAudit bool) error {
	if skipAudit {
		return nil
	}

	result, err := audit.ScanSkill(repoPath)
	if err != nil {
		// Scan error in non-interactive mode → fail-closed
		if !ui.IsTTY() {
			git.ResetHard(repoPath, beforeHash) //nolint:errcheck
			return fmt.Errorf("security audit failed: %v (use --skip-audit to bypass)", err)
		}
		ui.Warning("security audit error: %v", err)
		return nil
	}

	if !result.HasHigh() {
		return nil
	}

	// Show findings
	for _, f := range result.Findings {
		if audit.SeverityRank(f.Severity) <= audit.SeverityRank(audit.SeverityHigh) {
			ui.Warning("[%s] %s (%s:%d)", f.Severity, f.Message, f.File, f.Line)
		}
	}

	if ui.IsTTY() {
		fmt.Printf("\n  Security findings at HIGH or above detected.\n")
		fmt.Printf("  Apply anyway? [y/N]: ")
		var answer string
		fmt.Scanln(&answer)
		answer = strings.TrimSpace(strings.ToLower(answer))
		if answer == "y" || answer == "yes" {
			return nil
		}
		// User declined → rollback
		if err := git.ResetHard(repoPath, beforeHash); err != nil {
			return fmt.Errorf("rollback failed: %w", err)
		}
		ui.Info("Rolled back to %s", beforeHash[:12])
		return fmt.Errorf("update rejected by user after security audit")
	}

	// Non-interactive → fail-closed
	if err := git.ResetHard(repoPath, beforeHash); err != nil {
		return fmt.Errorf("rollback failed: %w", err)
	}
	return fmt.Errorf("security audit found HIGH/CRITICAL findings — rolled back (use --skip-audit to bypass)")
}

// auditGateAfterPullProject is like auditGateAfterPull but uses project-mode rules.
func auditGateAfterPullProject(repoPath, projectRoot, beforeHash string, skipAudit bool) error {
	if skipAudit {
		return nil
	}

	result, err := audit.ScanSkillForProject(repoPath, projectRoot)
	if err != nil {
		if !ui.IsTTY() {
			git.ResetHard(repoPath, beforeHash) //nolint:errcheck
			return fmt.Errorf("security audit failed: %v (use --skip-audit to bypass)", err)
		}
		ui.Warning("security audit error: %v", err)
		return nil
	}

	if !result.HasHigh() {
		return nil
	}

	for _, f := range result.Findings {
		if audit.SeverityRank(f.Severity) <= audit.SeverityRank(audit.SeverityHigh) {
			ui.Warning("[%s] %s (%s:%d)", f.Severity, f.Message, f.File, f.Line)
		}
	}

	if ui.IsTTY() {
		fmt.Printf("\n  Security findings at HIGH or above detected.\n")
		fmt.Printf("  Apply anyway? [y/N]: ")
		var answer string
		fmt.Scanln(&answer)
		answer = strings.TrimSpace(strings.ToLower(answer))
		if answer == "y" || answer == "yes" {
			return nil
		}
		if err := git.ResetHard(repoPath, beforeHash); err != nil {
			return fmt.Errorf("rollback failed: %w", err)
		}
		ui.Info("Rolled back to %s", beforeHash[:12])
		return fmt.Errorf("update rejected by user after security audit")
	}

	if err := git.ResetHard(repoPath, beforeHash); err != nil {
		return fmt.Errorf("rollback failed: %w", err)
	}
	return fmt.Errorf("security audit found HIGH/CRITICAL findings — rolled back (use --skip-audit to bypass)")
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

// isSecurityError returns true if the error originated from the audit gate.
// Matches errors from auditGateAfterPull ("security audit", "rolled back"),
// install.Install post-update path ("post-update audit"), and rollback failures.
func isSecurityError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "security audit") ||
		strings.Contains(msg, "post-update audit") ||
		strings.Contains(msg, "rolled back") ||
		strings.Contains(msg, "rollback failed")
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
  skillshare update --all --dry-run       # Preview updates
  skillshare update _team --force         # Discard changes and update`)
}
