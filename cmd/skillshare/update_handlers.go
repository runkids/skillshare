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
	"skillshare/internal/git"
	"skillshare/internal/install"
	"skillshare/internal/ui"
)

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
func auditGateAfterPull(repoPath, beforeHash string, skipAudit bool, threshold string, scanFn auditScanFunc) (*audit.Result, error) {
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

	if !result.HasSeverityAtOrAbove(normalizedThreshold) {
		return result, nil
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
			return result, nil
		}
		// User declined → rollback
		if beforeHash == "" {
			return result, fmt.Errorf("security audit failed — findings at/above %s detected, rollback commit unavailable and repository state is unknown: %w", normalizedThreshold, audit.ErrBlocked)
		}
		if err := git.ResetHard(repoPath, beforeHash); err != nil {
			return result, fmt.Errorf("security audit failed — findings at/above %s detected; WARNING: rollback also failed: %v — malicious content may remain: %w", normalizedThreshold, err, audit.ErrBlocked)
		}
		ui.Info("Rolled back to %s", beforeHash[:12])
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

func updateTrackedRepo(uc *updateContext, repoName string) error {
	repoPath := filepath.Join(uc.sourcePath, repoName)
	start := time.Now()

	ui.StepContinue("Repo", repoName+" (tracked)")

	startUpdate := time.Now()
	// Check for uncommitted changes
	spinner := ui.StartSpinner("Checking status...")

	isDirty, _ := git.IsDirty(repoPath)
	if isDirty {
		spinner.Stop()
		files, _ := git.GetDirtyFiles(repoPath)

		if !uc.opts.force {
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
		if !uc.opts.dryRun {
			if err := git.Restore(repoPath); err != nil {
				return fmt.Errorf("failed to discard changes: %w", err)
			}
		}
		spinner = ui.StartSpinner("Fetching from origin...")
	}

	if uc.opts.dryRun {
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
	if uc.opts.force {
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

	if uc.opts.diff {
		renderDiffSummary(repoPath, info.BeforeHash, info.AfterHash)
	}
	fmt.Println()

	// Post-pull audit gate
	scanFn := uc.auditScanFn()
	if _, err := auditGateAfterPull(repoPath, info.BeforeHash, uc.opts.skipAudit, uc.opts.threshold, scanFn); err != nil {
		return err
	}

	ui.SuccessMsg("Updated %s", repoName)
	ui.StepResult("success", "Updated successfully", time.Since(start))
	fmt.Println()
	ui.SectionLabel("Next Steps")
	ui.Info("Run 'skillshare sync' to distribute changes")

	return nil
}

func updateRegularSkill(uc *updateContext, skillName string) error {
	skillPath := filepath.Join(uc.sourcePath, skillName)

	// Read metadata to get source
	meta, err := install.ReadMeta(skillPath)
	if err != nil {
		return fmt.Errorf("cannot read metadata for '%s': %w", skillName, err)
	}
	if meta == nil || meta.Source == "" {
		return fmt.Errorf("skill '%s' has no source metadata, cannot update", skillName)
	}

	ui.StepContinue("Skill", skillName)
	ui.StepContinue("Source", meta.Source)

	if uc.opts.dryRun {
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
	if uc.opts.diff {
		beforeHashes, _ = install.ComputeFileHashes(skillPath)
	}

	spinner := ui.StartSpinner("Updating...")

	installOpts := uc.makeInstallOpts()
	if ui.IsTTY() {
		installOpts.OnProgress = func(line string) {
			spinner.Update("   " + line)
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
					ui.StepResult("warning", "Pruned — stale (deleted upstream)", 0)
					return nil
				}
			}
			ui.StepResult("warning", "Stale (deleted upstream)", 0)
			fmt.Println()
			displayStaleWarning([]string{skillName})
			return nil
		}

		ui.StepResult("error", fmt.Sprintf("Failed: %v", err), 0)
		return fmt.Errorf("update failed: %w", err)
	}

	spinner.Stop()
	ui.StepResult("success", "Updated successfully", time.Since(startUpdate))
	fmt.Println()

	renderInstallWarningsWithResult("", result.Warnings, uc.opts.auditVerbose, result)

	if uc.opts.diff {
		afterHashes, _ := install.ComputeFileHashes(skillPath)
		renderHashDiffSummary(beforeHashes, afterHashes)
	}

	ui.SectionLabel("Next Steps")
	ui.Info("Run 'skillshare sync' to distribute changes")

	return nil
}

// updateTrackedRepoQuick updates a single tracked repo in batch mode.
// Output is suppressed; caller handles display via progress bar.
// Returns (updated, auditResult, error).
func updateTrackedRepoQuick(uc *updateContext, repoPath string) (bool, *audit.Result, error) {
	// Check for uncommitted changes
	if isDirty, _ := git.IsDirty(repoPath); isDirty {
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
	var err error
	if uc.opts.force {
		info, err = git.ForcePullWithProgress(repoPath, git.AuthEnvForRepo(repoPath), nil)
	} else {
		info, err = git.PullWithProgress(repoPath, git.AuthEnvForRepo(repoPath), nil)
	}
	if err != nil {
		return false, nil, nil
	}

	if info.UpToDate {
		return false, nil, nil
	}

	// Post-pull audit gate
	auditResult, auditErr := auditGateAfterPull(repoPath, info.BeforeHash, uc.opts.skipAudit, uc.opts.threshold, uc.auditScanFn())
	if auditErr != nil {
		return false, auditResult, auditErr
	}

	return true, auditResult, nil
}

// updateSkillFromMeta updates a skill using its metadata in batch mode.
// Output is suppressed; caller handles display via progress bar.
// If cachedMeta is non-nil it is used directly; otherwise metadata is read from disk.
// Returns (updated, installResult, error).
func updateSkillFromMeta(uc *updateContext, skillPath string, cachedMeta *install.SkillMeta) (bool, *install.InstallResult, error) {
	if uc.opts.dryRun {
		return false, nil, nil
	}

	if _, err := os.Stat(skillPath); err != nil {
		return false, nil, nil
	}

	meta := cachedMeta
	if meta == nil {
		var readErr error
		meta, readErr = install.ReadMeta(skillPath)
		if readErr != nil || meta == nil || meta.Source == "" {
			return false, nil, nil
		}
	}

	source, err := install.ParseSource(meta.Source)
	if err != nil {
		return false, nil, nil
	}

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

func truncateString(s string, maxLen int) string { return truncateStr(s, maxLen) }
