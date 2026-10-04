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
)

// recordAcceptedFindings persists the findings the user just overrode so
// later updates stop blocking on them.
func recordAcceptedFindings(sourceDir, repoPath string, result *audit.Result, threshold string) {
	n, err := install.RecordAcceptedFindings(sourceDir, repoPath, result, threshold)
	if err != nil {
		ui.Warning("Failed to record accepted findings: %v", err)
		return
	}
	if n > 0 {
		ui.Note(fmt.Sprintf("Recorded %s; future updates won't block on them", plural(n, "accepted finding")))
	}
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
// Returns the audit result (may be nil if skipped or on error) and any error.
func auditGateAfterPull(sourceDir, repoPath, beforeHash string, skipAudit, force bool, threshold string, scanFn auditScanFunc) (*audit.Result, error) {
	if skipAudit {
		return nil, nil
	}
	normalizedThreshold, err := audit.NormalizeThreshold(threshold)
	if err != nil {
		normalizedThreshold = audit.DefaultThreshold()
	}

	result, err := scanFn(repoPath)
	if err != nil {
		// Scan error -> fail-closed across modes.
		if beforeHash == "" {
			return nil, fmt.Errorf("security audit failed: %v — rollback commit unavailable, update aborted and repository state is unknown: %w", err, audit.ErrBlocked)
		}
		if resetErr := git.ResetHard(repoPath, beforeHash); resetErr != nil {
			return nil, fmt.Errorf("security audit failed: %v; WARNING: rollback also failed: %v — malicious content may remain: %w", err, resetErr, audit.ErrBlocked)
		}
		return nil, fmt.Errorf("security audit failed: %v — rolled back (use --skip-audit to bypass): %w", err, audit.ErrBlocked)
	}

	if n := install.ApplyAcceptedFindings(sourceDir, repoPath, result); n > 0 {
		ui.Note(plural(n, "previously accepted finding") + " skipped")
	}
	if !result.HasSeverityAtOrAbove(normalizedThreshold) {
		return result, nil
	}

	// Show findings
	for _, f := range result.Findings {
		if !f.Acknowledged && audit.SeverityRank(f.Severity) <= audit.SeverityRank(normalizedThreshold) {
			ui.Warning("[%s] %s (%s:%d)", f.Severity, f.Message, f.File, f.Line)
		}
	}

	if force {
		ui.Warning("Findings at/above %s; proceeding due to --force", normalizedThreshold)
		recordAcceptedFindings(sourceDir, repoPath, result, normalizedThreshold)
		return result, nil
	}

	if ui.IsTTY() {
		fmt.Printf("\n  Security findings at %s or above detected.\n", normalizedThreshold)
		apply, err := ui.ConfirmAction("Apply anyway?", false)
		if err != nil {
			return result, err
		}
		if apply {
			recordAcceptedFindings(sourceDir, repoPath, result, normalizedThreshold)
			return result, nil
		}
		// User declined → rollback
		if beforeHash == "" {
			return result, fmt.Errorf("security audit failed — findings at/above %s detected, rollback commit unavailable and repository state is unknown: %w", normalizedThreshold, audit.ErrBlocked)
		}
		if err := git.ResetHard(repoPath, beforeHash); err != nil {
			return result, fmt.Errorf("security audit failed — findings at/above %s detected; WARNING: rollback also failed: %v — malicious content may remain: %w", normalizedThreshold, err, audit.ErrBlocked)
		}
		ui.Note(fmt.Sprintf("Rolled back to %s", beforeHash[:12]))
		return result, fmt.Errorf("security audit failed — findings at/above %s detected — rolled back (use --skip-audit to bypass): %w", normalizedThreshold, audit.ErrBlocked)
	}

	// Non-interactive → fail-closed
	if beforeHash == "" {
		return result, fmt.Errorf("security audit failed — findings at/above %s detected, rollback commit unavailable and repository state is unknown: %w", normalizedThreshold, audit.ErrBlocked)
	}
	if err := git.ResetHard(repoPath, beforeHash); err != nil {
		return result, fmt.Errorf("security audit failed — findings at/above %s detected; WARNING: rollback also failed: %v — malicious content may remain: %w", normalizedThreshold, err, audit.ErrBlocked)
	}
	return result, fmt.Errorf("security audit failed — findings at/above %s detected — rolled back (use --skip-audit to bypass): %w", normalizedThreshold, audit.ErrBlocked)
}

func updateTrackedRepo(uc *updateContext, repoName string) (updateResult, error) {
	repoPath := filepath.Join(uc.sourcePath, repoName)
	startUpdate := time.Now()
	policy, policyErr := install.PrepareFollowedUpdate(uc.sourcePath, repoPath, uc.follow, uc.opts.force)
	if policyErr != nil {
		return followedUpdateFailure(repoName, policyErr), policyErr
	}

	// Check for uncommitted changes
	spinner := ui.StartSpinner("Checking " + repoName + "...")

	var isDirty bool
	var dirtyErr error
	if policy == nil {
		isDirty, dirtyErr = git.IsDirty(repoPath)
	}
	if dirtyErr != nil && !uc.opts.force {
		spinner.Stop()
		statusErr := &gitStatusError{err: dirtyErr}
		printUpdateRow(ui.MarkFail, repoName, statusErr.Error(), 0)
		return updateResult{skipped: 1}, statusErr
	}
	if isDirty {
		spinner.Stop()
		files, _ := git.GetDirtyFiles(repoPath)

		if !uc.opts.force {
			printUpdateRow(ui.MarkFail, repoName, "uncommitted changes", 0)
			for _, f := range files {
				ui.Note(f)
			}
			fmt.Println()
			ui.Next("skillshare update "+repoName+" --force", "discard them and update")
			return updateResult{skipped: 1}, fmt.Errorf("uncommitted changes in repository")
		}

		ui.Warning("Discarding local changes (--force)")
		if !uc.opts.dryRun {
			if err := git.Restore(repoPath); err != nil {
				return updateResult{skipped: 1}, fmt.Errorf("failed to discard changes: %w", err)
			}
		}
		spinner = ui.StartSpinner("Fetching " + repoName + "...")
	}

	if uc.opts.dryRun {
		spinner.Stop()
		printUpdateRow(ui.MarkNone, repoName, "would run git pull", 0)
		fmt.Println()
		ui.DryRun()
		return updateResult{skipped: 1}, nil
	}

	spinner.Update("Fetching " + repoName + "...")
	var onProgress func(string)
	if ui.IsTTY() {
		onProgress = func(line string) {
			spinner.Update(line)
		}
	}

	// Use ForcePull if --force to handle force push
	var info *git.UpdateInfo
	var err error
	if policy != nil {
		info, err = git.PullFollowed(policy, onProgress)
	} else if uc.opts.force {
		info, err = git.ForcePullWithProgress(repoPath, git.AuthEnvForRepo(repoPath), onProgress)
	} else {
		info, err = git.PullWithProgress(repoPath, git.AuthEnvForRepo(repoPath), onProgress)
	}
	if err != nil {
		spinner.Stop()
		msg := fmt.Sprintf("git pull failed: %v", err)
		if !uc.opts.force && policy == nil {
			msg += " (try --force)"
		}
		printUpdateRow(ui.MarkFail, repoName, msg, 0)
		if policy != nil {
			return followedUpdateFailure(repoName, err), err
		}
		return updateResult{skipped: 1}, fmt.Errorf("git pull failed: %w", err)
	}

	if info.UpToDate {
		spinner.Stop()
		if err := refreshTrackedRootSkillMetadata(uc, repoName, repoPath); err != nil {
			ui.Warning("Failed to refresh metadata for %s: %v", repoName, err)
		}
		printUpdateRow(ui.MarkOK, repoName, "already up to date", time.Since(startUpdate))
		return updateResult{skipped: 1}, nil
	}

	spinner.Stop()
	printUpdateRow(ui.MarkOK, repoName, fmt.Sprintf("%s, %s changed (+%d −%d)",
		plural(len(info.Commits), "commit"), plural(info.Stats.FilesChanged, "file"),
		info.Stats.Insertions, info.Stats.Deletions), time.Since(startUpdate))

	printCommitNotes(info.Commits)

	if uc.opts.diff {
		renderDiffSummary(repoPath, info.BeforeHash, info.AfterHash)
	}

	// Post-pull audit gate
	scanFn := uc.auditScanFn()
	if _, err := auditGateAfterPull(uc.sourcePath, repoPath, info.BeforeHash, uc.opts.skipAudit, uc.opts.force, uc.opts.threshold, scanFn); err != nil {
		return updateResult{securityFailed: 1}, err
	}

	if err := refreshTrackedRootSkillMetadata(uc, repoName, repoPath); err != nil {
		ui.Warning("Failed to refresh metadata for %s: %v", repoName, err)
	}

	ui.Next("skillshare sync", "link the changes into your targets")
	return updateResult{updated: 1}, nil
}

func updateRegularSkill(uc *updateContext, skillName string) (updateResult, error) {
	skillPath := filepath.Join(uc.sourcePath, skillName)
	if err := install.RefuseFollowedSkillUpdate(skillName, uc.follow); err != nil {
		return updateResult{items: []updateJSONItem{{Name: skillName, Type: "skill", Status: "failed", Error: err.Error()}}}, err
	}

	// Read metadata to get source
	store, storeErr := install.LoadMetadataWithMigration(uc.sourcePath, "")
	if storeErr != nil {
		return updateResult{skipped: 1}, fmt.Errorf("cannot read metadata for '%s': %w", skillName, storeErr)
	}
	meta := store.GetByPath(skillName)
	if meta == nil || meta.Source == "" {
		return updateResult{skipped: 1}, fmt.Errorf("skill '%s' has no source metadata, cannot update", skillName)
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
		beforeHashes, _ = install.ComputeFileHashes(skillPath)
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
		afterHashes, _ := install.ComputeFileHashes(skillPath)
		renderHashDiffSummary(beforeHashes, afterHashes)
	}

	ui.Next("skillshare sync", "link the changes into your targets")

	return updateResult{updated: 1}, nil
}

// updateTrackedRepoQuick updates a single tracked repo in batch mode.
// Output is suppressed; caller handles display via progress bar.
// Returns (updated, auditResult, error).
func updateTrackedRepoQuick(uc *updateContext, repoPath string) (bool, *audit.Result, error) {
	policy, policyErr := install.PrepareFollowedUpdate(uc.sourcePath, repoPath, uc.follow, uc.opts.force)
	if policyErr != nil {
		return false, nil, policyErr
	}

	// Check for uncommitted changes
	var isDirty bool
	var err error
	if policy == nil {
		isDirty, err = git.IsDirty(repoPath)
	}
	if err != nil && !uc.opts.force {
		return false, nil, &gitStatusError{err: err}
	}
	if isDirty {
		if !uc.opts.force {
			return false, nil, nil
		}
		if !uc.opts.dryRun {
			if err := git.Restore(repoPath); err != nil {
				return false, nil, nil
			}
		}
	}

	if uc.opts.dryRun {
		return false, nil, nil
	}

	var info *git.UpdateInfo
	if policy != nil {
		info, err = git.PullFollowed(policy, nil)
	} else if uc.opts.force {
		info, err = git.ForcePullWithProgress(repoPath, git.AuthEnvForRepo(repoPath), nil)
	} else {
		info, err = git.PullWithProgress(repoPath, git.AuthEnvForRepo(repoPath), nil)
	}
	if err != nil {
		if policy != nil {
			return false, nil, err
		}
		return false, nil, nil
	}

	if info.UpToDate {
		_ = refreshTrackedRootSkillMetadata(uc, "", repoPath)
		return false, nil, nil
	}

	// Post-pull audit gate
	auditResult, auditErr := auditGateAfterPull(uc.sourcePath, repoPath, info.BeforeHash, uc.opts.skipAudit, uc.opts.force, uc.opts.threshold, uc.auditScanFn())
	if auditErr != nil {
		return false, auditResult, auditErr
	}

	_ = refreshTrackedRootSkillMetadata(uc, "", repoPath)

	return true, auditResult, nil
}

func refreshTrackedRootSkillMetadata(uc *updateContext, repoName, repoPath string) error {
	if uc.follow != nil {
		rel, err := filepath.Rel(uc.sourcePath, repoPath)
		if err != nil {
			return err
		}
		if _, followed := uc.follow.InFollowed(filepath.ToSlash(rel)); followed {
			return nil
		}
	}
	relPath := repoName
	if relPath == "" {
		rel, err := filepath.Rel(uc.sourcePath, repoPath)
		if err != nil {
			return err
		}
		relPath = rel
	}
	return install.RefreshTrackedRootSkillMetadata(uc.sourcePath, filepath.ToSlash(relPath), repoPath)
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

func followedUpdateFailure(name string, err error) updateResult {
	return updateResult{items: []updateJSONItem{{Name: name, Type: "repo", Status: "failed", Error: err.Error()}}}
}
