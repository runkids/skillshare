package main

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"skillshare/internal/audit"
	"skillshare/internal/config"
	"skillshare/internal/install"
	"skillshare/internal/ui"
	"skillshare/internal/validate"
	appversion "skillshare/internal/version"
)

func handleTrackedRepoInstall(source *install.Source, cfg *config.Config, opts install.InstallOptions) (installLogSummary, error) {
	logSummary := installLogSummary{
		Source:         source.Raw,
		DryRun:         opts.DryRun,
		Tracked:        true,
		Into:           opts.Into,
		SkipAudit:      opts.SkipAudit,
		AuditVerbose:   opts.AuditVerbose,
		AuditThreshold: opts.AuditThreshold,
	}

	// Show logo with version
	ui.Logo(appversion.Version)

	// Step 1: Show source
	ui.StepStart("Source", source.Raw)
	if opts.Name != "" {
		ui.StepContinue("Name", "_"+opts.Name)
	}
	if opts.Into != "" {
		ui.StepContinue("Into", opts.Into)
	}

	// Step 2: Clone with tree spinner
	progressMsg := "Cloning repository..."
	if source.HasSubdir() {
		progressMsg = "Sparse-checkout cloning subdirectory..."
	}
	treeSpinner := ui.StartTreeSpinner(progressMsg, false)
	if ui.IsTTY() {
		opts.OnProgress = func(line string) {
			treeSpinner.Update(line)
		}
	}

	result, err := install.InstallTrackedRepo(source, cfg.Source, opts)
	if err != nil {
		if errors.Is(err, install.ErrSkipSameRepo) {
			treeSpinner.Warn(firstWarningLine(err.Error()))
			return logSummary, nil
		}
		if errors.Is(err, audit.ErrBlocked) {
			treeSpinner.Fail("Blocked by security audit")
			return logSummary, renderBlockedAuditError(err)
		}
		treeSpinner.Fail("Failed to clone")
		return logSummary, err
	}

	treeSpinner.Success("Cloned")

	// Step 3: Show result
	if opts.DryRun {
		ui.StepEnd("Action", result.Action)
		fmt.Println()
		ui.Warning("[dry-run] Would install tracked repo")
	} else {
		ui.StepContinue("Found", fmt.Sprintf("%d skill(s)", result.SkillCount))
		renderTrackedRepoMeta(result.RepoName, result.Skills, result.RepoPath)
	}

	// Display warnings and risk info
	res := &install.InstallResult{
		AuditRiskScore: result.AuditRiskScore,
		AuditRiskLabel: result.AuditRiskLabel,
		AuditSkipped:   result.AuditSkipped,
		Warnings:       result.Warnings,
	}
	renderInstallWarningsWithResult("", result.Warnings, opts.AuditVerbose, res)

	if !opts.DryRun {
		logSummary.SkillCount = result.SkillCount
		logSummary.InstalledSkills = append(logSummary.InstalledSkills, result.Skills...)
	}

	// Show next steps
	if !opts.DryRun {
		ui.SectionLabel("Next Steps")
		ui.Info("Run 'skillshare sync' to distribute skills to all targets")
		ui.Info("Run 'skillshare update %s' to update this repo later", result.RepoName)
	}

	return logSummary, nil
}

func handleGitInstall(source *install.Source, cfg *config.Config, opts install.InstallOptions) (installLogSummary, error) {
	logSummary := installLogSummary{
		Source:         source.Raw,
		DryRun:         opts.DryRun,
		Into:           opts.Into,
		SkipAudit:      opts.SkipAudit,
		AuditVerbose:   opts.AuditVerbose,
		AuditThreshold: opts.AuditThreshold,
	}

	// Show logo with version
	ui.Logo(appversion.Version)

	// Step 1: Show source
	ui.StepStart("Source", source.Raw)
	if source.HasSubdir() {
		ui.StepContinue("Subdir", source.Subdir)
	}
	if opts.Into != "" {
		ui.StepContinue("Into", opts.Into)
	}

	// Step 2: Clone with tree spinner animation
	progressMsg := "Cloning repository..."
	if source.HasSubdir() && source.GitHubOwner() != "" && source.GitHubRepo() != "" {
		progressMsg = "Downloading via GitHub API..."
	}
	treeSpinner := ui.StartTreeSpinner(progressMsg, false)
	if ui.IsTTY() {
		opts.OnProgress = func(line string) {
			treeSpinner.Update(line)
		}
	}

	var discovery *install.DiscoveryResult
	var err error
	if source.HasSubdir() {
		discovery, err = install.DiscoverFromGitSubdirWithProgress(source, opts.OnProgress)
	} else {
		discovery, err = install.DiscoverFromGitWithProgress(source, opts.OnProgress)
	}
	if err != nil {
		treeSpinner.Fail("Failed to clone")
		return logSummary, err
	}
	defer install.CleanupDiscovery(discovery)

	treeSpinner.Success("Cloned")

	// Show subdir-specific warnings (e.g. GitHub API fallback notices)
	if source.HasSubdir() {
		for _, w := range discovery.Warnings {
			ui.Warning("%s", w)
		}
	}

	// Cross-path duplicate detection: block if same repo is already installed
	// at a different location (e.g. user forgot they used --into before).
	if !opts.Force && source.CloneURL != "" {
		if err := install.CheckCrossPathDuplicate(cfg.Source, source.CloneURL, opts.Into); err != nil {
			return logSummary, err
		}
	}

	// Subdir mode handles single skill before empty check (shows "1 skill: name")
	if source.HasSubdir() && len(discovery.Skills) == 1 {
		skill := discovery.Skills[0]
		if opts.Name != "" {
			if err := validate.SkillName(opts.Name); err != nil {
				return logSummary, fmt.Errorf("invalid skill name '%s': %w", opts.Name, err)
			}
			skill.Name = opts.Name
		}
		ui.StepContinue("Found", fmt.Sprintf("1 skill: %s", skill.Name))

		displayPath := skill.Name
		if opts.Into != "" {
			displayPath = filepath.Join(opts.Into, skill.Name)
		}

		renderSkillMeta(skill, displayPath)

		destPath := destWithInto(cfg.Source, opts, skill.Name)
		if err := ensureIntoDirExists(cfg.Source, opts); err != nil {
			return logSummary, fmt.Errorf("failed to create --into directory: %w", err)
		}

		fmt.Println()
		installSpinner := ui.StartSpinner("Installing...")

		result, err := install.InstallFromDiscovery(discovery, skill, destPath, opts)
		if err != nil {
			installSpinner.Stop()
			if errors.Is(err, install.ErrSkipSameRepo) {
				ui.Warning("%s: %s", skill.Name, firstWarningLine(err.Error()))
				return logSummary, nil
			}
			if errors.Is(err, audit.ErrBlocked) {
				return logSummary, renderBlockedAuditError(err)
			}
			ui.ErrorMsg("Failed to install: %v", err)
			return logSummary, err
		}

		installSpinner.Stop()
		if opts.DryRun {
			ui.Warning("[dry-run] Would install: %s", skill.Name)
		} else {
			ui.SuccessMsg("Installed: %s", skill.Name)
		}
		renderInstallWarningsWithResult("", result.Warnings, opts.AuditVerbose, result)

		if !opts.DryRun {
			ui.SectionLabel("Next Steps")
			ui.Info("Run 'skillshare sync' to distribute to all targets")
			logSummary.InstalledSkills = append(logSummary.InstalledSkills, skill.Name)
			logSummary.SkillCount = len(logSummary.InstalledSkills)
		}
		return logSummary, nil
	}

	// Step 3: Show found skills
	if len(discovery.Skills) == 0 {
		ui.StepEnd("Found", "No skills (no SKILL.md files)")
		return logSummary, nil
	}

	ui.StepEnd("Found", fmt.Sprintf("%d skill(s)", len(discovery.Skills)))

	// Apply --exclude early so excluded skills never appear in prompts
	if len(opts.Exclude) > 0 {
		discovery.Skills = applyExclude(discovery.Skills, opts.Exclude)
		if len(discovery.Skills) == 0 {
			ui.Info("All skills were excluded")
			return logSummary, nil
		}
	}

	if opts.Name != "" && len(discovery.Skills) != 1 {
		return logSummary, fmt.Errorf("--name can only be used when exactly one skill is discovered")
	}

	// Single skill (non-subdir): show detailed box and install directly
	if len(discovery.Skills) == 1 {
		skill := discovery.Skills[0]
		if opts.Name != "" {
			if err := validate.SkillName(opts.Name); err != nil {
				return logSummary, fmt.Errorf("invalid skill name '%s': %w", opts.Name, err)
			}
			skill.Name = opts.Name
		}

		// Determine local installation info for display (relative to skills dir)
		displayPath := skill.Name
		if opts.Into != "" {
			displayPath = filepath.Join(opts.Into, skill.Name)
		}

		renderSkillMeta(skill, displayPath)

		destPath := destWithInto(cfg.Source, opts, skill.Name)
		if err := ensureIntoDirExists(cfg.Source, opts); err != nil {
			return logSummary, fmt.Errorf("failed to create --into directory: %w", err)
		}
		fmt.Println()

		installSpinner := ui.StartSpinner("Installing...")
		result, err := install.InstallFromDiscovery(discovery, skill, destPath, opts)
		if err != nil {
			installSpinner.Stop()
			if errors.Is(err, install.ErrSkipSameRepo) {
				ui.Warning("%s: %s", skill.Name, firstWarningLine(err.Error()))
				return logSummary, nil
			}
			if errors.Is(err, audit.ErrBlocked) {
				return logSummary, renderBlockedAuditError(err)
			}
			ui.ErrorMsg("Failed to install: %v", err)
			return logSummary, err
		}

		installSpinner.Stop()
		if opts.DryRun {
			ui.Warning("[dry-run] Would install: %s", skill.Name)
		} else {
			ui.SuccessMsg("Installed: %s", skill.Name)
		}
		renderInstallWarningsWithResult("", result.Warnings, opts.AuditVerbose, result)

		if !opts.DryRun {
			ui.SectionLabel("Next Steps")
			ui.Info("Run 'skillshare sync' to distribute to all targets")
			logSummary.InstalledSkills = append(logSummary.InstalledSkills, skill.Name)
			logSummary.SkillCount = len(logSummary.InstalledSkills)
		}

		return logSummary, nil
	}

	// Non-interactive path: --skill or --all/--yes
	if opts.HasSkillFilter() || opts.ShouldInstallAll() {
		selected, err := selectSkills(discovery.Skills, opts)
		if err != nil {
			return logSummary, err
		}

		if opts.DryRun {
			fmt.Println()
			printSkillListCompact(selected)
			fmt.Println()
			ui.Warning("[dry-run] Would install %d skill(s)", len(selected))
			return logSummary, nil
		}

		fmt.Println()
		batchSummary := installSelectedSkills(selected, discovery, cfg, opts)
		logSummary.InstalledSkills = append(logSummary.InstalledSkills, batchSummary.InstalledSkills...)
		logSummary.FailedSkills = append(logSummary.FailedSkills, batchSummary.FailedSkills...)
		logSummary.SkillCount = len(logSummary.InstalledSkills)
		return logSummary, nil
	}

	if opts.DryRun {
		// Show skill list in dry-run mode
		fmt.Println()
		printSkillListCompact(discovery.Skills)
		fmt.Println()
		ui.Warning("[dry-run] Would prompt for selection")
		return logSummary, nil
	}

	// Non-TTY with large repo: require explicit flags
	if !ui.IsTTY() && len(discovery.Skills) >= largeRepoThreshold {
		ui.Info("Found %d skills. Non-interactive mode requires --all, --yes, or --skill <names>", len(discovery.Skills))
		return logSummary, fmt.Errorf("interactive selection not available in non-TTY mode")
	}

	fmt.Println()

	selected, err := promptSkillSelection(discovery.Skills)
	if err != nil {
		return logSummary, err
	}

	if len(selected) == 0 {
		ui.Info("No skills selected")
		return logSummary, nil
	}

	fmt.Println()
	batchSummary := installSelectedSkills(selected, discovery, cfg, opts)
	logSummary.InstalledSkills = append(logSummary.InstalledSkills, batchSummary.InstalledSkills...)
	logSummary.FailedSkills = append(logSummary.FailedSkills, batchSummary.FailedSkills...)
	logSummary.SkillCount = len(logSummary.InstalledSkills)

	return logSummary, nil
}

// largeBatchProgressThreshold is the skill count above which a progress bar
// is used instead of a step spinner during batch install.
const largeBatchProgressThreshold = 20

// installSelectedSkills installs multiple skills with progress display
func installSelectedSkills(selected []install.SkillInfo, discovery *install.DiscoveryResult, cfg *config.Config, opts install.InstallOptions) installBatchSummary {
	results := make([]skillInstallResult, 0, len(selected))

	// Choose progress indicator: progress bar for large batches, spinner otherwise
	var installSpinner *ui.Spinner
	var progressBar *ui.ProgressBar
	usePB := len(selected) > largeBatchProgressThreshold
	if usePB {
		progressBar = ui.StartProgress("Installing skills", len(selected))
	} else {
		installSpinner = ui.StartSpinnerWithSteps("Installing...", len(selected))
	}

	// Ensure Into directory exists for batch installs
	if opts.Into != "" {
		if err := ensureIntoDirExists(cfg.Source, opts); err != nil {
			if installSpinner != nil {
				installSpinner.Fail("Failed to create --into directory")
			}
			if progressBar != nil {
				progressBar.Stop()
				ui.ErrorMsg("Failed to create --into directory")
			}
			return installBatchSummary{}
		}
	}

	// Detect orchestrator: if root skill (path=".") is selected, children nest under it
	var parentName string
	var rootIdx = -1
	for i, skill := range selected {
		if skill.Path == "." {
			parentName = skill.Name
			rootIdx = i
			break
		}
	}

	// Reorder: install root skill first so children can nest under it
	orderedSkills := selected
	if rootIdx > 0 {
		orderedSkills = make([]install.SkillInfo, 0, len(selected))
		orderedSkills = append(orderedSkills, selected[rootIdx])
		orderedSkills = append(orderedSkills, selected[:rootIdx]...)
		orderedSkills = append(orderedSkills, selected[rootIdx+1:]...)
	}

	// Track if root was installed (children are already included in root)
	rootInstalled := false

	for i, skill := range orderedSkills {
		if installSpinner != nil {
			installSpinner.NextStep(fmt.Sprintf("Installing %s...", skill.Name))
			if i == 0 {
				installSpinner.Update(fmt.Sprintf("Installing %s...", skill.Name))
			}
		}
		if progressBar != nil {
			progressBar.UpdateTitle(fmt.Sprintf("Installing %s", skill.Name))
		}

		// Determine destination path and effective --into for force hints.
		var destPath string
		skillOpts := opts
		if skill.Path == "." {
			// Root skill - install directly
			destPath = destWithInto(cfg.Source, opts, skill.Name)
		} else if parentName != "" {
			// Child skill with parent selected - nest under parent group
			effectiveInto := parentName
			if opts.Into != "" {
				effectiveInto = filepath.Join(opts.Into, parentName)
			}
			skillOpts.Into = effectiveInto
			destPath = destWithInto(cfg.Source, opts, filepath.Join(parentName, skill.Name))
		} else {
			// Standalone child skill - install to root
			destPath = destWithInto(cfg.Source, opts, skill.Name)
		}

		// If root was installed, children are already included - skip reinstall
		if rootInstalled && skill.Path != "." {
			results = append(results, skillInstallResult{skill: skill, success: true, message: fmt.Sprintf("included in %s", parentName)})
			if progressBar != nil {
				progressBar.Increment()
			}
			continue
		}

		installResult, err := install.InstallFromDiscovery(discovery, skill, destPath, skillOpts)
		if err != nil {
			r := skillInstallResult{skill: skill, success: false, message: err.Error(), err: err}
			if errors.Is(err, install.ErrSkipSameRepo) {
				r.skipped = true
			}
			results = append(results, r)
			if progressBar != nil {
				progressBar.Increment()
			}
			continue
		}

		if skill.Path == "." {
			rootInstalled = true
		}
		message := "installed"
		if len(installResult.Warnings) > 0 {
			message = fmt.Sprintf("installed (%d warning(s))", len(installResult.Warnings))
		}
		results = append(results, skillInstallResult{
			skill:          skill,
			success:        true,
			message:        message,
			warnings:       installResult.Warnings,
			auditRiskLabel: installResult.AuditRiskLabel,
			auditRiskScore: installResult.AuditRiskScore,
			auditSkipped:   installResult.AuditSkipped,
			result:         installResult,
		})
		if progressBar != nil {
			progressBar.Increment()
		}
	}

	if progressBar != nil {
		progressBar.Stop()
	}

	// Extract repo label for skipped display (e.g. "runkids/feature-radar")
	repoLabel := repoLabelFromSource(discovery.Source)
	displayInstallResults(results, installSpinner, opts.AuditVerbose, repoLabel)

	summary := installBatchSummary{
		InstalledSkills: make([]string, 0, len(results)),
		FailedSkills:    make([]string, 0, len(results)),
	}
	for _, r := range results {
		switch {
		case r.success:
			summary.InstalledSkills = append(summary.InstalledSkills, r.skill.Name)
		case r.skipped:
			// Same-repo skips are not failures; exclude from FailedSkills.
		default:
			summary.FailedSkills = append(summary.FailedSkills, r.skill.Name)
		}
	}
	return summary
}

// repoLabelFromSource extracts a short "owner/repo" label from a Source's
// CloneURL for display purposes. Falls back to Source.Raw if unparsable.
func repoLabelFromSource(source *install.Source) string {
	if source == nil {
		return ""
	}
	if owner, repo := source.GitHubOwner(), source.GitHubRepo(); owner != "" && repo != "" {
		return owner + "/" + repo
	}
	// Generic: use normalised clone URL and take last two segments
	norm := source.CloneURL
	if norm == "" {
		norm = source.Raw
	}
	norm = strings.TrimSuffix(strings.TrimSuffix(strings.TrimSpace(norm), "/"), ".git")
	if i := strings.LastIndex(norm, "/"); i >= 0 {
		if j := strings.LastIndex(norm[:i], "/"); j >= 0 {
			return norm[j+1:]
		}
		return norm[i+1:]
	}
	return norm
}

// displayInstallResults shows the final install results.
// repoLabel is used in the Skipped section header (e.g. "owner/repo").
func displayInstallResults(results []skillInstallResult, spinner *ui.Spinner, auditVerbose bool, repoLabel string) {
	var successes, skipped, failures []skillInstallResult
	totalWarnings := 0
	for _, r := range results {
		switch {
		case r.success:
			successes = append(successes, r)
		case r.skipped:
			skipped = append(skipped, r)
		default:
			failures = append(failures, r)
		}
		totalWarnings += len(r.warnings)
	}

	installed := len(successes)
	failed := len(failures)
	skippedCount := len(skipped)

	// Summary line — skipped does not count as failure.
	summaryMsg := buildInstallSummary(installed, failed, skippedCount)
	if spinner != nil {
		switch {
		case failed > 0 && installed == 0:
			spinner.Fail(summaryMsg)
		case failed > 0:
			spinner.Warn(summaryMsg)
		case skippedCount > 0:
			spinner.Success(summaryMsg)
		default:
			spinner.Success(summaryMsg)
		}
	} else {
		// Progress bar mode — print summary line directly
		fmt.Println()
		if failed > 0 && installed == 0 {
			ui.ErrorMsg("%s", summaryMsg)
		} else {
			ui.SuccessMsg("%s", summaryMsg)
		}
	}

	// Show failures first with details
	if failed > 0 {
		var blockedFailures, otherFailures []skillInstallResult
		for _, r := range failures {
			if r.err != nil && errors.Is(r.err, audit.ErrBlocked) {
				blockedFailures = append(blockedFailures, r)
				continue
			}
			otherFailures = append(otherFailures, r)
		}

		ui.SectionLabel("Blocked / Failed")
		if len(blockedFailures) > 0 && !auditVerbose {
			threshold := summarizeBlockedThreshold(blockedFailures)
			ui.Warning("%d skill(s) blocked by security audit (%s threshold)", len(blockedFailures), formatBlockedThresholdLabel(threshold))
			ui.Info("Use --force to continue blocked installs, or --skip-audit to bypass scanning for this run")
		}

		const blockedVerboseLimit = 20
		if auditVerbose && len(blockedFailures) > blockedVerboseLimit {
			// Large batch: summary line + first N verbose + rest compact
			threshold := summarizeBlockedThreshold(blockedFailures)
			ui.Warning("%d skill(s) blocked by security audit (%s threshold)", len(blockedFailures), formatBlockedThresholdLabel(threshold))
			ui.Info("Use --force to continue blocked installs, or --skip-audit to bypass scanning for this run")
			for i, r := range blockedFailures {
				digest := parseAuditBlockedFailure(r.message)
				if i < blockedVerboseLimit {
					ui.StepFail(blockedSkillLabel(r.skill.Name, digest.threshold), r.message)
				} else {
					ui.StepFail(blockedSkillLabel(r.skill.Name, digest.threshold), compactInstallFailureMessage(r))
				}
			}
			remaining := len(blockedFailures) - blockedVerboseLimit
			if remaining > 0 {
				ui.Info("%d more blocked skill(s) shown in compact form above", remaining)
			}
		} else {
			for _, r := range blockedFailures {
				digest := parseAuditBlockedFailure(r.message)
				msg := r.message
				if !auditVerbose {
					msg = compactInstallFailureMessage(r)
				}
				ui.StepFail(blockedSkillLabel(r.skill.Name, digest.threshold), msg)
			}
		}
		for _, r := range otherFailures {
			msg := r.message
			if !auditVerbose {
				msg = compactInstallFailureMessage(r)
			}
			ui.StepFail(r.skill.Name, msg)
		}
	}

	// Show skipped (same-repo) — grouped by directory with repo label
	if skippedCount > 0 {
		label := "Skipped (same repo)"
		if repoLabel != "" {
			label = fmt.Sprintf("Skipped (%s)", repoLabel)
		}
		ui.SectionLabel(label)
		renderSkippedByGroup(skipped)
		ui.Info("Use 'skillshare update' to refresh, or --force to overwrite")
	}

	// Show successes — condensed when many
	if installed > 0 {
		ui.SectionLabel("Installed")
		switch {
		case installed > 50:
			ui.StepDone(fmt.Sprintf("%d skills installed", installed), "")
		case installed > 10:
			maxShown := 10
			names := make([]string, 0, maxShown)
			for i, r := range successes {
				if i >= maxShown {
					break
				}
				names = append(names, r.skill.Name)
			}
			detail := strings.Join(names, ", ")
			if installed > maxShown {
				detail = fmt.Sprintf("%s ... +%d more", detail, installed-maxShown)
			}
			ui.StepDone(fmt.Sprintf("%d skills installed", installed), detail)
		default:
			for _, r := range successes {
				if installed == 1 {
					// Single skill: show full audit info
					fmt.Println()
					renderInstallWarningsWithResult("", r.warnings, auditVerbose, r.result)
				} else {
					ui.StepDone(r.skill.Name, r.message)
				}
			}
		}
	}

	if totalWarnings > 0 {
		ui.SectionLabel("Audit Warnings")
		if auditVerbose {
			skillsWithWarnings := countSkillsWithWarnings(results)
			if skillsWithWarnings <= 20 {
				// Small batch: show full verbose detail per skill
				ui.Warning("%d warning(s) detected during install", totalWarnings)
				for _, r := range results {
					renderInstallWarnings(r.skill.Name, r.warnings, true)
				}
			} else {
				// Large batch: compact summary + only HIGH/CRITICAL findings from top skills
				renderBatchInstallWarningsCompact(results, totalWarnings,
					"%d audit finding line(s) across all skills; HIGH/CRITICAL detail expanded below")
				fmt.Println()
				ui.Warning("HIGH/CRITICAL detail (top skills):")
				shown := 0
				for _, r := range sortResultsByHighCritical(results) {
					if shown >= 20 || !hasHighCriticalWarnings(r) {
						break
					}
					renderInstallWarningsHighCriticalOnly(r.skill.Name, r.warnings)
					shown++
				}
				remaining := skillsWithWarnings - shown
				if remaining > 0 {
					ui.Info("%d more skill(s) with findings; use 'skillshare check <name>' for details", remaining)
				}
			}
		} else if len(results) > 100 {
			renderUltraCompactAuditSummary(results, totalWarnings)
		} else {
			renderBatchInstallWarningsCompact(results, totalWarnings)
		}
	}

	if installed > 0 {
		ui.SectionLabel("Next Steps")
		ui.Info("Run 'skillshare sync' to distribute to all targets")
	}
}

// renderSkippedByGroup prints skipped skills grouped by their parent directory.
// Flat repos (all skills at root) print a single line; nested repos show one
// line per directory group.
func renderSkippedByGroup(skipped []skillInstallResult) {
	// Build ordered groups keyed by parent directory.
	type group struct {
		dir   string
		names []string
	}
	groupIndex := map[string]int{}
	var groups []group

	for _, r := range skipped {
		dir := filepath.Dir(r.skill.Path) // e.g. "." for root, "frontend" for "frontend/skill-a"
		if dir == "." {
			dir = ""
		}
		if idx, ok := groupIndex[dir]; ok {
			groups[idx].names = append(groups[idx].names, r.skill.Name)
		} else {
			groupIndex[dir] = len(groups)
			groups = append(groups, group{dir: dir, names: []string{r.skill.Name}})
		}
	}

	if len(groups) == 1 {
		// Single group (flat repo or all under one dir) — compact one-liner.
		prefix := ""
		if groups[0].dir != "" {
			prefix = groups[0].dir + "/ "
		}
		ui.StepSkip(prefix+strings.Join(groups[0].names, ", "), "")
		return
	}

	// Multiple groups — one line per directory.
	for _, g := range groups {
		label := strings.Join(g.names, ", ")
		if g.dir != "" {
			label = g.dir + "/ " + label
		}
		ui.StepSkip(label, "")
	}
}

// buildInstallSummary formats the one-line summary for batch install results.
func buildInstallSummary(installed, failed, skipped int) string {
	parts := make([]string, 0, 3)
	if installed > 0 {
		parts = append(parts, fmt.Sprintf("Installed %d", installed))
	}
	if failed > 0 {
		parts = append(parts, fmt.Sprintf("failed %d", failed))
	}
	if skipped > 0 {
		parts = append(parts, fmt.Sprintf("%d skipped", skipped))
	}
	if len(parts) == 0 {
		return "No skills installed"
	}
	return strings.Join(parts, ", ")
}

func handleDirectInstall(source *install.Source, cfg *config.Config, opts install.InstallOptions) (installLogSummary, error) {
	logSummary := installLogSummary{
		Source:         source.Raw,
		DryRun:         opts.DryRun,
		Into:           opts.Into,
		SkipAudit:      opts.SkipAudit,
		AuditVerbose:   opts.AuditVerbose,
		AuditThreshold: opts.AuditThreshold,
	}

	// Warn about inapplicable flags
	if len(opts.Exclude) > 0 {
		ui.Warning("--exclude is only supported for multi-skill repos; ignored for direct install")
	}

	// Determine skill name
	skillName := source.Name
	if opts.Name != "" {
		skillName = opts.Name
	}

	// Validate skill name
	if err := validate.SkillName(skillName); err != nil {
		return logSummary, fmt.Errorf("invalid skill name '%s': %w", skillName, err)
	}

	// Set the name in source for display
	source.Name = skillName

	// Determine destination path
	destPath := destWithInto(cfg.Source, opts, skillName)

	// Ensure Into directory exists
	if err := ensureIntoDirExists(cfg.Source, opts); err != nil {
		return logSummary, fmt.Errorf("failed to create --into directory: %w", err)
	}

	// Cross-path duplicate detection (same as handleGitInstall)
	if !opts.Force && source.CloneURL != "" {
		if err := install.CheckCrossPathDuplicate(cfg.Source, source.CloneURL, opts.Into); err != nil {
			return logSummary, err
		}
	}

	// Show logo with version
	ui.Logo(appversion.Version)

	// Step 1: Show source info
	ui.StepStart("Source", source.Raw)
	ui.StepContinue("Name", skillName)
	if opts.Into != "" {
		ui.StepContinue("Into", opts.Into)
	}
	if source.HasSubdir() {
		ui.StepContinue("Subdir", source.Subdir)
	}

	// Step 2: Clone/copy with tree spinner
	var actionMsg string
	if source.IsGit() {
		actionMsg = "Cloning repository..."
	} else {
		actionMsg = "Copying files..."
	}
	treeSpinner := ui.StartTreeSpinner(actionMsg, true)
	if source.IsGit() && ui.IsTTY() {
		opts.OnProgress = func(line string) {
			treeSpinner.Update(line)
		}
	}

	// Execute installation
	result, err := install.Install(source, destPath, opts)
	if err != nil {
		if errors.Is(err, install.ErrSkipSameRepo) {
			treeSpinner.Warn(firstWarningLine(err.Error()))
			return logSummary, nil
		}
		treeSpinner.Fail("Failed to install")
		if errors.Is(err, audit.ErrBlocked) {
			return logSummary, renderBlockedAuditError(err)
		}
		return logSummary, err
	}

	// Display result
	if opts.DryRun {
		treeSpinner.Success("Ready")
		fmt.Println()
		ui.Warning("[dry-run] %s", result.Action)
	} else {
		treeSpinner.Success(fmt.Sprintf("Installed: %s", skillName))
	}

	// Display warnings
	renderInstallWarnings("", result.Warnings, opts.AuditVerbose)

	// Show next steps
	if !opts.DryRun {
		ui.SectionLabel("Next Steps")
		ui.Info("Run 'skillshare sync' to distribute to all targets")
		logSummary.InstalledSkills = append(logSummary.InstalledSkills, skillName)
		logSummary.SkillCount = len(logSummary.InstalledSkills)
	}

	return logSummary, nil
}

func installFromGlobalConfig(cfg *config.Config, opts install.InstallOptions) (installLogSummary, error) {
	summary := installLogSummary{
		Mode:         "global",
		Source:       "global-config",
		DryRun:       opts.DryRun,
		AuditVerbose: opts.AuditVerbose,
	}

	reg, regErr := config.LoadRegistry(filepath.Dir(config.ConfigPath()))
	if regErr != nil {
		return summary, fmt.Errorf("failed to load registry: %w", regErr)
	}
	ctx := &globalInstallContext{cfg: cfg, reg: reg}

	if len(ctx.ConfigSkills()) == 0 {
		ui.Info("No remote skills defined in registry")
		ui.Info("Install a skill first: skillshare install <source>")
		return summary, nil
	}

	ui.Logo(appversion.Version)
	total := len(ctx.ConfigSkills())
	installStart := time.Now()

	// Use quiet mode + TreeSpinner: suppress per-skill output, show summary
	opts.Quiet = true
	ui.StepStart("Installing", fmt.Sprintf("%d skill(s) from config", total))
	treeSpinner := ui.StartTreeSpinner("Resolving skills...", false)
	if ui.IsTTY() {
		opts.OnProgress = func(line string) {
			if text := parseGitProgressLine(line); text != "" {
				treeSpinner.Update(text)
			}
		}
	}

	result, err := install.InstallFromConfig(ctx, opts)

	summary.InstalledSkills = result.InstalledSkills
	summary.FailedSkills = result.FailedSkills
	summary.SkillCount = len(result.InstalledSkills)

	if err != nil {
		treeSpinner.Fail("Install failed")
		elapsed := time.Since(installStart)
		parts := []string{fmt.Sprintf("Installed %d skill(s)", len(result.InstalledSkills))}
		if len(result.FailedSkills) > 0 {
			parts = append(parts, fmt.Sprintf("%d failed", len(result.FailedSkills)))
		}
		ui.StepResult("error", strings.Join(parts, ", "), elapsed)
		return summary, err
	}

	if opts.DryRun {
		treeSpinner.Success("Ready")
		return summary, nil
	}

	treeSpinner.Success("Done")

	elapsed := time.Since(installStart)
	parts := []string{fmt.Sprintf("Installed %d skill(s)", result.Installed)}
	if result.Skipped > 0 {
		parts = append(parts, fmt.Sprintf("%d skipped", result.Skipped))
	}
	if len(result.FailedSkills) > 0 {
		parts = append(parts, fmt.Sprintf("%d failed", len(result.FailedSkills)))
	}
	status := "success"
	if len(result.FailedSkills) > 0 {
		status = "error"
	}
	ui.StepResult(status, strings.Join(parts, ", "), elapsed)

	// Show failed details
	if len(result.FailedSkills) > 0 {
		fmt.Println()
		for _, name := range result.FailedSkills {
			ui.StepFail(name, "install failed")
		}
	}

	// Sync hint
	if result.Installed > 0 {
		fmt.Println()
		ui.Info("Run 'skillshare sync' to distribute to all targets")
	}

	return summary, nil
}

// renderSkillMeta prints skill description, license, and location as inline tree steps.
func renderSkillMeta(skill install.SkillInfo, displayPath string) {
	if skill.Description != "" {
		ui.StepContinue("Desc", truncateDesc(skill.Description, 100))
	}
	if skill.License != "" {
		ui.StepContinue("License", skill.License)
	}
	ui.StepEnd("Location", "skills/"+displayPath)
}

// renderTrackedRepoMeta prints tracked repo metadata as inline tree steps.
func renderTrackedRepoMeta(repoName string, skills []string, repoPath string) {
	ui.StepContinue("Tracked", repoName)
	if len(skills) > 0 && len(skills) <= 10 {
		ui.StepContinue("Skills", strings.Join(skills, ", "))
	}
	ui.StepEnd("Location", repoPath)
}

// truncateDesc truncates a description string to max runes, appending " ..." if truncated.
func truncateDesc(s string, max int) string {
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max]) + " ..."
}
