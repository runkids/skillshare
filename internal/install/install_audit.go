package install

import (
	"fmt"
	"strings"

	"skillshare/internal/audit"
)

func auditInstalledSkill(destPath string, result *InstallResult, opts InstallOptions) error {
	if opts.SkipAudit {
		result.AuditSkipped = true
		result.AuditThreshold = opts.AuditThreshold
		result.Warnings = append(result.Warnings, "audit skipped (--skip-audit)")
		return nil
	}

	threshold, err := audit.NormalizeThreshold(opts.AuditThreshold)
	if err != nil {
		threshold = audit.DefaultThreshold()
	}
	result.AuditThreshold = threshold

	var scanResult *audit.Result
	if opts.AuditProjectRoot != "" {
		scanResult, err = audit.ScanSkillForProject(destPath, opts.AuditProjectRoot)
	} else {
		scanResult, err = audit.ScanSkill(destPath)
	}
	if err != nil {
		// Non-fatal: warn but don't block
		result.Warnings = append(result.Warnings, fmt.Sprintf("audit scan error: %v", err))
		return nil
	}
	result.AuditRiskScore = scanResult.RiskScore
	result.AuditRiskLabel = scanResult.RiskLabel
	if result.AuditRiskLabel == "" && len(scanResult.Findings) == 0 {
		result.AuditRiskLabel = "CLEAN"
	}
	scanResult.Threshold = threshold
	scanResult.IsBlocked = scanResult.HasSeverityAtOrAbove(threshold)

	if len(scanResult.Findings) == 0 {
		return nil
	}

	// Build warning messages for all findings (include snippet for context)
	for _, f := range scanResult.Findings {
		msg := fmt.Sprintf("audit %s: %s (%s:%d)", f.Severity, f.Message, f.File, f.Line)
		if f.Snippet != "" {
			msg += fmt.Sprintf("\n       %q", f.Snippet)
		}
		result.Warnings = append(result.Warnings, msg)
	}

	// Findings at or above threshold block installation unless --force.
	if scanResult.IsBlocked && !opts.Force {
		details := blockedFindingDetails(scanResult.Findings, threshold)
		if removeErr := removeAll(destPath); removeErr != nil {
			return fmt.Errorf(
				"security audit failed — findings at/above %s detected:\n%s\n\nAutomatic cleanup failed for %s: %v\nManual removal is required: %w",
				threshold,
				strings.Join(details, "\n"),
				destPath,
				removeErr,
				audit.ErrBlocked,
			)
		}
		return fmt.Errorf(
			"security audit failed — findings at/above %s detected:\n%s\n\nUse --force to override or --skip-audit to bypass scanning: %w",
			threshold,
			strings.Join(details, "\n"),
			audit.ErrBlocked,
		)
	}

	if scanResult.IsBlocked && opts.Force {
		result.Warnings = append(result.Warnings,
			fmt.Sprintf("audit findings at/above block threshold (%s); proceeding due to --force", threshold))
	} else if !scanResult.IsBlocked {
		result.Warnings = append(result.Warnings,
			fmt.Sprintf("audit findings detected, but none at/above block threshold (%s)", threshold))
	}

	return nil
}

// auditTrackedRepo scans an entire tracked repo directory for security threats.
// It blocks installation when findings are at or above configured threshold
// unless force is enabled. On block, the repo directory is removed.
func auditTrackedRepo(repoPath string, result *TrackedRepoResult, opts InstallOptions) error {
	if opts.SkipAudit {
		result.AuditSkipped = true
		result.AuditThreshold = opts.AuditThreshold
		result.Warnings = append(result.Warnings, "audit skipped (--skip-audit)")
		return nil
	}

	threshold, err := audit.NormalizeThreshold(opts.AuditThreshold)
	if err != nil {
		threshold = audit.DefaultThreshold()
	}
	result.AuditThreshold = threshold

	var scanResult *audit.Result
	if opts.AuditProjectRoot != "" {
		scanResult, err = audit.ScanSkillForProject(repoPath, opts.AuditProjectRoot)
	} else {
		scanResult, err = audit.ScanSkill(repoPath)
	}
	if err != nil {
		result.Warnings = append(result.Warnings, fmt.Sprintf("audit scan error: %v", err))
		return nil
	}
	result.AuditRiskScore = scanResult.RiskScore
	result.AuditRiskLabel = scanResult.RiskLabel
	if result.AuditRiskLabel == "" && len(scanResult.Findings) == 0 {
		result.AuditRiskLabel = "CLEAN"
	}
	scanResult.Threshold = threshold
	scanResult.IsBlocked = scanResult.HasSeverityAtOrAbove(threshold)

	if len(scanResult.Findings) == 0 {
		return nil
	}

	for _, f := range scanResult.Findings {
		msg := fmt.Sprintf("audit %s: %s (%s:%d)", f.Severity, f.Message, f.File, f.Line)
		if f.Snippet != "" {
			msg += fmt.Sprintf("\n       %q", f.Snippet)
		}
		result.Warnings = append(result.Warnings, msg)
	}

	if scanResult.IsBlocked && !opts.Force {
		details := blockedFindingDetails(scanResult.Findings, threshold)
		if removeErr := removeAll(repoPath); removeErr != nil {
			return fmt.Errorf(
				"security audit failed — findings at/above %s detected in tracked repository:\n%s\n\nAutomatic cleanup failed for %s: %v\nManual removal is required: %w",
				threshold,
				strings.Join(details, "\n"),
				repoPath,
				removeErr,
				audit.ErrBlocked,
			)
		}
		return fmt.Errorf(
			"security audit failed — findings at/above %s detected in tracked repository:\n%s\n\nUse --force to override or --skip-audit to bypass scanning: %w",
			threshold,
			strings.Join(details, "\n"),
			audit.ErrBlocked,
		)
	}

	if scanResult.IsBlocked && opts.Force {
		result.Warnings = append(result.Warnings,
			fmt.Sprintf("audit findings at/above block threshold (%s); proceeding due to --force", threshold))
	} else if !scanResult.IsBlocked {
		result.Warnings = append(result.Warnings,
			fmt.Sprintf("audit findings detected, but none at/above block threshold (%s)", threshold))
	}

	return nil
}

// auditGateFailClosed scans a repo after git pull and rolls back on scan
// error or findings at/above threshold. Used by handleUpdate for non-tracked
// skill updates where fail-closed is the only behaviour.
func auditGateFailClosed(repoPath, beforeHash, threshold, projectRoot string) (*audit.Result, error) {
	if beforeHash == "" {
		return nil, fmt.Errorf(
			"post-update audit failed — rollback commit unavailable, update aborted and repository state is unknown: %w",
			audit.ErrBlocked,
		)
	}

	normalizedThreshold, err := audit.NormalizeThreshold(threshold)
	if err != nil {
		normalizedThreshold = audit.DefaultThreshold()
	}

	var scanResult *audit.Result
	var scanErr error
	if projectRoot != "" {
		scanResult, scanErr = audit.ScanSkillForProject(repoPath, projectRoot)
	} else {
		scanResult, scanErr = audit.ScanSkill(repoPath)
	}
	if scanErr != nil {
		if resetErr := gitResetHard(repoPath, beforeHash); resetErr != nil {
			return nil, fmt.Errorf("post-update audit failed: %v; WARNING: rollback also failed: %v — malicious content may remain: %w", scanErr, resetErr, audit.ErrBlocked)
		}
		return nil, fmt.Errorf("post-update audit failed: %v — rolled back (use --skip-audit to bypass): %w", scanErr, audit.ErrBlocked)
	}
	if scanResult.HasSeverityAtOrAbove(normalizedThreshold) {
		details := blockedFindingDetails(scanResult.Findings, normalizedThreshold)
		if resetErr := gitResetHard(repoPath, beforeHash); resetErr != nil {
			return nil, fmt.Errorf("post-update audit found findings at/above %s; WARNING: rollback also failed: %v — malicious content may remain: %w", normalizedThreshold, resetErr, audit.ErrBlocked)
		}
		return nil, fmt.Errorf(
			"post-update audit failed — findings at/above %s detected (rolled back to %s):\n%s\n\nUse --skip-audit to bypass: %w",
			normalizedThreshold,
			shortHash(beforeHash),
			strings.Join(details, "\n"),
			audit.ErrBlocked,
		)
	}
	return scanResult, nil
}

// auditTrackedRepoUpdate scans an updated tracked repo for security threats.
// Unlike auditTrackedRepo (used for fresh installs), on block it rolls back
// via git reset --hard to preserve the repo and its history.
func auditTrackedRepoUpdate(repoPath, beforeHash string, result *TrackedRepoResult, opts InstallOptions) error {
	if opts.SkipAudit {
		result.AuditSkipped = true
		result.AuditThreshold = opts.AuditThreshold
		result.Warnings = append(result.Warnings, "audit skipped (--skip-audit)")
		return nil
	}

	threshold, err := audit.NormalizeThreshold(opts.AuditThreshold)
	if err != nil {
		threshold = audit.DefaultThreshold()
	}
	result.AuditThreshold = threshold

	var scanResult *audit.Result
	if opts.AuditProjectRoot != "" {
		scanResult, err = audit.ScanSkillForProject(repoPath, opts.AuditProjectRoot)
	} else {
		scanResult, err = audit.ScanSkill(repoPath)
	}
	if err != nil {
		if beforeHash == "" {
			return fmt.Errorf(
				"security audit failed: %v — rollback commit unavailable, update aborted and repository state is unknown: %w",
				err,
				audit.ErrBlocked,
			)
		}
		if resetErr := gitResetHard(repoPath, beforeHash); resetErr != nil {
			return fmt.Errorf("security audit failed: %v; WARNING: rollback also failed: %v — malicious content may remain: %w",
				err, resetErr, audit.ErrBlocked)
		}
		return fmt.Errorf("security audit failed: %v — rolled back (use --skip-audit to bypass): %w",
			err, audit.ErrBlocked)
	}
	result.AuditRiskScore = scanResult.RiskScore
	result.AuditRiskLabel = scanResult.RiskLabel
	if result.AuditRiskLabel == "" && len(scanResult.Findings) == 0 {
		result.AuditRiskLabel = "CLEAN"
	}
	scanResult.Threshold = threshold
	scanResult.IsBlocked = scanResult.HasSeverityAtOrAbove(threshold)

	if len(scanResult.Findings) == 0 {
		return nil
	}

	for _, f := range scanResult.Findings {
		msg := fmt.Sprintf("audit %s: %s (%s:%d)", f.Severity, f.Message, f.File, f.Line)
		if f.Snippet != "" {
			msg += fmt.Sprintf("\n       %q", f.Snippet)
		}
		result.Warnings = append(result.Warnings, msg)
	}

	if scanResult.IsBlocked && !opts.Force {
		if beforeHash == "" {
			return fmt.Errorf(
				"security audit found findings at/above %s in tracked repository — rollback commit unavailable, update aborted and repository state is unknown: %w",
				threshold,
				audit.ErrBlocked,
			)
		}

		// Rollback via git reset to preserve the repo
		if resetErr := gitResetHard(repoPath, beforeHash); resetErr != nil {
			return fmt.Errorf("security audit found findings at/above %s in tracked repository — WARNING: rollback also failed: %v — malicious content may remain: %w",
				threshold, resetErr, audit.ErrBlocked)
		}
		details := blockedFindingDetails(scanResult.Findings, threshold)
		return fmt.Errorf(
			"security audit failed — findings at/above %s detected in tracked repository (rolled back to %s):\n%s\n\nUse --force to override or --skip-audit to bypass scanning: %w",
			threshold,
			shortHash(beforeHash),
			strings.Join(details, "\n"),
			audit.ErrBlocked,
		)
	}

	if scanResult.IsBlocked && opts.Force {
		result.Warnings = append(result.Warnings,
			fmt.Sprintf("audit findings at/above block threshold (%s); proceeding due to --force", threshold))
	} else if !scanResult.IsBlocked {
		result.Warnings = append(result.Warnings,
			fmt.Sprintf("audit findings detected, but none at/above block threshold (%s)", threshold))
	}

	return nil
}

func blockedFindingDetails(findings []audit.Finding, threshold string) []string {
	var details []string
	for _, f := range findings {
		if audit.SeverityRank(f.Severity) <= audit.SeverityRank(threshold) {
			detail := fmt.Sprintf("  %s: %s (%s:%d)", f.Severity, f.Message, f.File, f.Line)
			if f.Snippet != "" {
				detail += fmt.Sprintf("\n    %q", f.Snippet)
			}
			details = append(details, detail)
		}
	}
	return details
}

// isGitInstalled checks if git command is available
