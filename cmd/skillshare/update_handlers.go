package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"skillshare/internal/audit"
	"skillshare/internal/git"
	"skillshare/internal/install"
	"skillshare/internal/ui"
	"skillshare/internal/update"
)

// runTrackedRepoUpdate updates one tracked repo and prints what the audit gate
// found. On a terminal it asks whether to keep blocking findings. onReport,
// when set, runs once with the result so far, before any audit output or
// prompt, so the caller can print its own lines above them.
func runTrackedRepoUpdate(uc *updateContext, repoPath string, onProgress func(string), onReport func(*update.TrackedRepoResult)) (*update.TrackedRepoResult, error) {
	printed := false
	printAudit := func(res *update.TrackedRepoResult) {
		if printed {
			return
		}
		printed = true
		if onReport != nil {
			onReport(res)
		}
		if res.Audit == nil {
			return
		}
		if res.AcceptedSkipped > 0 {
			ui.Note(plural(res.AcceptedSkipped, "previously accepted finding") + " skipped")
		}
		for _, f := range res.Audit.Findings {
			if !f.Acknowledged && audit.SeverityRank(f.Severity) <= audit.SeverityRank(res.Threshold) {
				ui.Warning("[%s] %s (%s:%d)", f.Severity, f.Message, f.File, f.Line)
			}
		}
	}

	opts := update.TrackedRepoOptions{
		SourceDir:   uc.sourcePath,
		Force:       uc.opts.force,
		DryRun:      uc.opts.dryRun,
		SkipAudit:   uc.opts.skipAudit,
		Threshold:   uc.opts.threshold,
		ProjectRoot: uc.projectRoot,
		Follow:      uc.follow,
		OnProgress:  onProgress,
	}
	declined := false
	if ui.IsTTY() {
		opts.Confirm = func(res *update.TrackedRepoResult) (bool, error) {
			printAudit(res)
			fmt.Printf("\n  Security findings at %s or above detected.\n", res.Threshold)
			apply, err := ui.ConfirmAction("Apply anyway?", false)
			declined = err == nil && !apply
			return apply, err
		}
	}

	res, err := update.TrackedRepo(repoPath, opts)
	printAudit(res)

	if res.Overridden {
		if uc.opts.force {
			ui.Warning("Findings at/above %s; proceeding due to --force", res.Threshold)
		}
		if res.RecordErr != nil {
			ui.Warning("Failed to record accepted findings: %v", res.RecordErr)
		} else if res.Recorded > 0 {
			ui.Note(fmt.Sprintf("Recorded %s; future updates won't block on them", plural(res.Recorded, "accepted finding")))
		}
	}
	var blocked *install.AuditGateError
	if declined && errors.As(err, &blocked) && blocked.Rollback == install.RolledBack {
		ui.Note(fmt.Sprintf("Rolled back to %s", blocked.BeforeHash[:12]))
	}
	var opErr *update.Error
	if errors.As(err, &opErr) && opErr.Stage == update.StageStatus {
		return res, &gitStatusError{err: opErr.Err}
	}
	return res, err
}

func updateTrackedRepo(uc *updateContext, repoName string) (updateResult, error) {
	repoPath := filepath.Join(uc.sourcePath, repoName)
	startUpdate := time.Now()

	spinner := ui.StartSpinner("Fetching " + repoName + "...")
	var onProgress func(string)
	if ui.IsTTY() {
		onProgress = func(line string) {
			spinner.Update(line)
		}
	}

	res, err := runTrackedRepoUpdate(uc, repoPath, onProgress, func(res *update.TrackedRepoResult) {
		spinner.Stop()
		if res.Discarded {
			ui.Warning("Discarding local changes (--force)")
		}
		info := res.Info
		if info == nil || info.UpToDate {
			return
		}
		printUpdateRow(ui.MarkOK, repoName, fmt.Sprintf("%s, %s changed (+%d −%d)",
			plural(len(info.Commits), "commit"), plural(info.Stats.FilesChanged, "file"),
			info.Stats.Insertions, info.Stats.Deletions), time.Since(startUpdate))

		printCommitNotes(info.Commits)

		if uc.opts.diff {
			renderDiffSummary(repoPath, info.BeforeHash, info.AfterHash)
		}
	})

	var statusErr *gitStatusError
	var opErr *update.Error
	switch {
	case errors.As(err, &statusErr):
		printUpdateRow(ui.MarkFail, repoName, statusErr.Error(), 0)
		return updateResult{skipped: 1}, statusErr
	case errors.As(err, &opErr) && opErr.Stage == update.StageDiscard:
		return updateResult{skipped: 1}, err
	case errors.As(err, &opErr):
		msg := opErr.Error()
		if !uc.opts.force {
			msg += " (try --force)"
		}
		printUpdateRow(ui.MarkFail, repoName, msg, 0)
		return updateResult{skipped: 1}, err
	case err != nil:
		return updateResult{securityFailed: 1}, err
	}

	switch res.Status {
	case update.StatusDirty:
		files, _ := git.GetDirtyFiles(repoPath)
		printUpdateRow(ui.MarkFail, repoName, "uncommitted changes", 0)
		for _, f := range files {
			ui.Note(f)
		}
		fmt.Println()
		ui.Next("skillshare update "+repoName+" --force", "discard them and update")
		return updateResult{skipped: 1}, fmt.Errorf("uncommitted changes in repository")
	case update.StatusDryRun:
		printUpdateRow(ui.MarkNone, repoName, "would run git pull", 0)
		fmt.Println()
		ui.DryRun()
		return updateResult{skipped: 1}, nil
	}

	if res.MetadataErr != nil {
		ui.Warning("Failed to refresh metadata for %s: %v", repoName, res.MetadataErr)
	}
	for _, w := range res.Warnings {
		ui.Warning("%s", w)
	}
	if res.Status == update.StatusUpToDate {
		printUpdateRow(ui.MarkOK, repoName, "already up to date", time.Since(startUpdate))
		return updateResult{skipped: 1}, nil
	}

	ui.Next("skillshare sync", "link the changes into your targets")
	return updateResult{updated: 1}, nil
}

func updateRegularSkill(uc *updateContext, skillName string) (updateResult, error) {
	skillPath := filepath.Join(uc.sourcePath, skillName)

	// Read metadata to get source
	store, storeErr := install.LoadMetadataWithMigration(uc.sourcePath, "")
	if storeErr != nil {
		return updateResult{skipped: 1}, fmt.Errorf("cannot read metadata for '%s': %w", skillName, storeErr)
	}
	meta := store.GetByPath(skillName)
	if meta == nil || meta.Source == "" {
		return updateResult{skipped: 1}, fmt.Errorf("skill '%s' has no source metadata, cannot update", skillName)
	}

	if !uc.opts.force && meta.LocallyModified(skillPath, uc.follow) {
		if uc.opts.dryRun {
			printUpdateRow(ui.MarkWarn, skillName, "would skip — "+modifiedSkillReason, 0)
			fmt.Println()
			ui.DryRun()
			return updateResult{skipped: 1}, nil
		}
		// Only a skill named on purpose fails; --all and --group skip it as the batch path does.
		if uc.opts.all || len(uc.opts.groups) > 0 {
			printUpdateRow(ui.MarkWarn, skillName, modifiedSkillReason, 0)
			ui.Next("skillshare check "+skillName+" --diff", "see the changes",
				"skillshare update "+skillName+" --force", "discard them and reinstall")
			return updateResult{skipped: 1}, nil
		}
		printUpdateRow(ui.MarkFail, skillName, modifiedSkillReason, 0)
		ui.Next("skillshare check "+skillName+" --diff", "see the changes",
			"skillshare update "+skillName+" --force", "discard them and reinstall")
		return updateResult{skipped: 1}, fmt.Errorf("local changes in skill")
	}

	from := "from " + sourceLabel(meta.Source)
	if uc.opts.dryRun {
		printUpdateRow(ui.MarkNone, skillName, "would reinstall "+from, 0)
		fmt.Println()
		ui.DryRun()
		return updateResult{skipped: 1}, nil
	}

	startUpdate := time.Now()
	// Parse source and reinstall
	source, err := install.ParseSourceWithOptions(meta.Source, uc.parseOpts)
	if err != nil {
		return updateResult{skipped: 1}, fmt.Errorf("invalid source in metadata: %w", err)
	}
	// Preserve branch from original install
	source.ApplyRecordedBranch(meta.Branch)

	// Snapshot before update for --diff
	var beforeHashes map[string]string
	if uc.opts.diff {
		beforeHashes, _ = install.ComputeFileHashes(skillPath, uc.follow)
	}

	spinner := ui.StartSpinner("Updating " + skillName + "...")

	installOpts := uc.makeInstallOpts()
	if ui.IsTTY() {
		installOpts.OnProgress = func(line string) {
			spinner.Update(line)
		}
	}

	result, err := install.Install(source, skillPath, installOpts)
	if err != nil {
		spinner.Stop()

		// Stale skill: subdir deleted from upstream
		if isStaleError(err) {
			if uc.opts.prune {
				if pruneErr := pruneSkill(skillPath, skillName, uc); pruneErr == nil {
					pruneRegistry([]string{skillName}, uc)
					printUpdateRow(ui.MarkWarn, skillName, "pruned — deleted upstream", 0)
					return updateResult{pruned: 1}, nil
				}
			}
			printUpdateRow(ui.MarkWarn, skillName, "stale — deleted upstream", 0)
			displayStaleWarning([]string{skillName})
			return updateResult{skipped: 1}, nil
		}

		if isSecurityError(err) {
			return updateResult{securityFailed: 1}, err
		}

		printUpdateRow(ui.MarkFail, skillName, err.Error(), 0)
		return updateResult{skipped: 1}, fmt.Errorf("update failed: %w", err)
	}

	spinner.Stop()
	printUpdateRow(ui.MarkOK, skillName, from, time.Since(startUpdate))
	renderInstallWarningsWithResult("", result.Warnings, uc.opts.auditVerbose, result)

	if uc.opts.diff {
		afterHashes, _ := install.ComputeFileHashes(skillPath, uc.follow)
		renderHashDiffSummary(beforeHashes, afterHashes)
	}

	ui.Next("skillshare sync", "link the changes into your targets")

	return updateResult{updated: 1}, nil
}

// updateTrackedRepoQuick updates a single tracked repo in batch mode.
// Output is suppressed; caller handles display via progress bar.
// Returns (updated, auditResult, error).
func updateTrackedRepoQuick(uc *updateContext, repoPath string) (bool, *audit.Result, error) {
	res, err := runTrackedRepoUpdate(uc, repoPath, nil, nil)
	var opErr *update.Error
	if errors.As(err, &opErr) {
		// A failed discard or pull counts as skipped in a batch.
		return false, nil, nil
	}
	if err != nil {
		return false, res.Audit, err
	}
	return res.Status == update.StatusUpdated, res.Audit, nil
}

// updateSkillFromMeta updates a skill using its metadata in batch mode.
// Output is suppressed; caller handles display via progress bar.
// If cachedMeta is non-nil it is used directly; otherwise metadata is loaded from the store.
// Returns (updated, installResult, error).
func updateSkillFromMeta(uc *updateContext, skillPath string, cachedMeta *install.MetadataEntry) (bool, *install.InstallResult, error) {
	if uc.opts.dryRun {
		return false, nil, nil
	}

	if _, err := os.Stat(skillPath); err != nil {
		return false, nil, nil
	}

	meta := cachedMeta
	if meta == nil {
		store, _ := install.LoadMetadataWithMigration(uc.sourcePath, "")
		// GetByPath handles both full-path keys and legacy basename+group keys.
		if rel, relErr := filepath.Rel(uc.sourcePath, skillPath); relErr == nil {
			meta = store.GetByPath(filepath.ToSlash(rel))
		}
		if meta == nil || meta.Source == "" {
			return false, nil, nil
		}
	}

	source, err := install.ParseSourceWithOptions(meta.Source, uc.parseOpts)
	if err != nil {
		return false, nil, nil
	}
	source.ApplyRecordedBranch(meta.Branch)

	result, err := install.Install(source, skillPath, uc.makeInstallOpts())
	if err != nil {
		return false, nil, err
	}

	return true, result, nil
}

// renderDiffSummary prints a file-level change summary for the given repo.
func renderDiffSummary(repoPath, beforeHash, afterHash string) {
	changes, err := git.GetChangedFiles(repoPath, beforeHash, afterHash)
	if err != nil || len(changes) == 0 {
		return
	}

	ui.Section("Files changed")
	const maxFiles = 20
	for i, c := range changes {
		if i >= maxFiles {
			ui.Note(fmt.Sprintf("… and %d more", len(changes)-maxFiles))
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
		fmt.Println(detail)
	}
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
		ui.Note("No file changes")
		return
	}

	// Sort for deterministic output
	sort.Slice(changes, func(i, j int) bool {
		return changes[i].path < changes[j].path
	})

	ui.Section("Files changed")
	const maxFiles = 20
	for i, c := range changes {
		if i >= maxFiles {
			ui.Note(fmt.Sprintf("… and %d more", len(changes)-maxFiles))
			break
		}
		fmt.Printf("  %s %s\n", c.marker, c.path)
	}
}

// printCommitNotes lists up to five pulled commits in dim text.
func printCommitNotes(commits []git.CommitInfo) {
	const maxCommits = 5
	for i, c := range commits {
		if i >= maxCommits {
			ui.Note(fmt.Sprintf("… and %d more", len(commits)-maxCommits))
			break
		}
		ui.Note(c.Hash + "  " + truncateString(c.Message, 60))
	}
}

// printUpdateRow reports one skill or tracked repo: its name, a dim detail
// and, when known, how long it took.
func printUpdateRow(mark, name, detail string, took time.Duration) {
	value := name
	if detail != "" {
		value += " " + ui.DimText("· "+detail)
	}
	value += ui.Took(took)
	ui.Row(mark, "Update", value, ui.RowWidth("Update", "Audit"))
}

// isSecurityError returns true if the error originated from the audit gate.
// All security-related errors wrap audit.ErrBlocked as a sentinel.
func isSecurityError(err error) bool {
	return errors.Is(err, audit.ErrBlocked)
}

func truncateString(s string, maxLen int) string { return truncateStr(s, maxLen) }
