package install

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"skillshare/internal/audit"
	"skillshare/internal/sourcewalk"
)

// Audit rollback resets the whole checkout, so followed user repositories
// must be clean before pulling even when their edits do not conflict upstream.
func checkFollowedCheckoutClean(repoPath string, opts InstallOptions) error {
	checkout, followed := opts.SourceFollow.Resolve(repoPath)
	if !followed {
		return nil
	}
	status, err := gitOutput(checkout, "status", "--porcelain")
	if err != nil {
		return fmt.Errorf("cannot check Git status of followed checkout %s; resolve the status error and commit or stash changes before updating: %w", checkout, err)
	}
	if status != "" {
		return fmt.Errorf("uncommitted changes in followed checkout %s: commit or stash them before updating", checkout)
	}
	return nil
}

// auditInstalledResource runs the audit gate shared by skill and agent installs.
// scan performs the actual scan; cleanupOnBlock removes the installed artifact
// when findings at/above threshold fire and --force is not set.
func auditInstalledResource(
	destPath string,
	result *InstallResult,
	opts InstallOptions,
	scan func() (*audit.Result, error),
	cleanupOnBlock func() error,
) error {
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

	scanResult, err := scan()
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
	acceptRoot, acceptPath := opts.auditAcceptTarget(destPath)
	ApplyAcceptedFindings(acceptRoot, acceptPath, scanResult)
	scanResult.IsBlocked = scanResult.HasSeverityAtOrAbove(threshold)

	if len(scanResult.Findings) == 0 {
		return nil
	}

	for _, f := range scanResult.Findings {
		msg := fmt.Sprintf("audit %s: %s (%s:%d)", f.Severity, f.Message, f.File, f.Line)
		if f.Acknowledged {
			msg += " (accepted earlier)"
		}
		if f.Snippet != "" {
			msg += fmt.Sprintf("\n       %q", f.Snippet)
		}
		result.Warnings = append(result.Warnings, msg)
	}

	if scanResult.IsBlocked && !opts.AuditOverride {
		details := blockedFindingDetails(scanResult.Findings, threshold)
		if cleanupErr := cleanupOnBlock(); cleanupErr != nil {
			return fmt.Errorf(
				"security audit failed — findings at/above %s detected:\n%s\n\nAutomatic cleanup failed for %s: %v\nManual removal is required: %w",
				threshold,
				strings.Join(details, "\n"),
				destPath,
				cleanupErr,
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

	if scanResult.IsBlocked && opts.AuditOverride {
		result.Warnings = append(result.Warnings,
			fmt.Sprintf("audit findings at/above block threshold (%s); proceeding due to --force", threshold))
		result.Warnings = append(result.Warnings, recordAcceptedWarning(acceptRoot, acceptPath, scanResult, threshold)...)
	} else if !scanResult.IsBlocked {
		result.Warnings = append(result.Warnings,
			fmt.Sprintf("audit findings detected, but none at/above block threshold (%s)", threshold))
	}

	return nil
}

func auditInstalledSkill(destPath string, result *InstallResult, opts InstallOptions) error {
	scan := func() (*audit.Result, error) {
		if opts.AuditProjectRoot != "" {
			return audit.ScanSkillForProject(destPath, opts.AuditProjectRoot, opts.SourceFollow)
		}
		return audit.ScanSkillWithFollow(destPath, opts.SourceFollow)
	}
	cleanup := func() error { return removeAll(destPath) }
	return auditInstalledResource(destPath, result, opts, scan, cleanup)
}

func auditInstalledAgent(destFile string, result *InstallResult, opts InstallOptions) error {
	scan := func() (*audit.Result, error) {
		if opts.AuditProjectRoot != "" {
			return audit.ScanFileForProject(destFile, opts.AuditProjectRoot)
		}
		return audit.ScanFile(destFile)
	}
	// Agents are single .md files. After removing the file, walk up any
	// newly-empty parent directories (e.g. into-subdir) so a blocked install
	// leaves nothing behind.
	cleanup := func() error {
		if err := removeAll(destFile); err != nil {
			return err
		}
		cleanEmptyInstallParents(destFile, opts.SourceDir)
		return nil
	}
	return auditInstalledResource(destFile, result, opts, scan, cleanup)
}

func cleanEmptyInstallParents(path, stopAt string) {
	stopAt = filepath.Clean(stopAt)
	dir := filepath.Dir(filepath.Clean(path))
	for dir != stopAt && dir != "." && dir != "/" {
		entries, err := os.ReadDir(dir)
		if err != nil || len(entries) > 0 {
			break
		}
		_ = os.Remove(dir)
		dir = filepath.Dir(dir)
	}
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
		scanResult, err = audit.ScanSkillForProject(repoPath, opts.AuditProjectRoot, opts.SourceFollow)
	} else {
		scanResult, err = audit.ScanSkillWithFollow(repoPath, opts.SourceFollow)
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
	acceptRoot, acceptPath := opts.auditAcceptTarget(repoPath)
	ApplyAcceptedFindings(acceptRoot, acceptPath, scanResult)
	scanResult.IsBlocked = scanResult.HasSeverityAtOrAbove(threshold)

	if len(scanResult.Findings) == 0 {
		return nil
	}

	for _, f := range scanResult.Findings {
		msg := fmt.Sprintf("audit %s: %s (%s:%d)", f.Severity, f.Message, f.File, f.Line)
		if f.Acknowledged {
			msg += " (accepted earlier)"
		}
		if f.Snippet != "" {
			msg += fmt.Sprintf("\n       %q", f.Snippet)
		}
		result.Warnings = append(result.Warnings, msg)
	}

	if scanResult.IsBlocked && !opts.AuditOverride {
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

	if scanResult.IsBlocked && opts.AuditOverride {
		result.Warnings = append(result.Warnings,
			fmt.Sprintf("audit findings at/above block threshold (%s); proceeding due to --force", threshold))
		result.Warnings = append(result.Warnings, recordAcceptedWarning(acceptRoot, acceptPath, scanResult, threshold)...)
	} else if !scanResult.IsBlocked {
		result.Warnings = append(result.Warnings,
			fmt.Sprintf("audit findings detected, but none at/above block threshold (%s)", threshold))
	}

	return nil
}

// RollbackState says what the audit gate did to the checkout when it blocked.
type RollbackState int

const (
	// RolledBack means the checkout is back at the pre-pull commit.
	RolledBack RollbackState = iota
	// RollbackUnavailable means no pre-pull commit was known.
	RollbackUnavailable
	// RollbackFailed means the reset failed and pulled content may remain.
	RollbackFailed
)

// AuditGate is the fail-closed audit of a git checkout that has just been
// pulled. Every blocked outcome resets the checkout to BeforeHash.
type AuditGate struct {
	SourceDir   string // root the accepted findings are stored under
	RepoPath    string
	AcceptPath  string // path the accepted findings are keyed on; RepoPath when empty
	BeforeHash  string // pre-pull commit; without it nothing is let through
	Threshold   string
	ProjectRoot string // non-empty scans with the project's audit rules
	Follow      *sourcewalk.Follow
	// Override lets findings at/above the threshold through (--force). A scan
	// that could not run stays blocked: "I accept these findings" is not the
	// same as "I could not be told any".
	Override bool
	// Confirm is asked once when findings would block and Override is unset.
	// Nil, false or an error blocks.
	Confirm func(*AuditGateResult) (bool, error)
}

// AuditGateResult is what the scan found. It is also returned next to an
// *AuditGateError when findings blocked.
type AuditGateResult struct {
	Audit           *audit.Result
	Threshold       string // normalized
	AcceptedSkipped int    // findings an earlier override accepted
	Overridden      bool   // blocking findings were let through
}

// AuditGateError reports a blocked update. It wraps audit.ErrBlocked.
type AuditGateError struct {
	ScanErr    error // the scan could not run; nil when findings blocked
	Threshold  string
	BeforeHash string
	Rollback   RollbackState
	ResetErr   error // set when Rollback is RollbackFailed
}

// RollbackNote describes the state of the checkout after the block.
func (e *AuditGateError) RollbackNote() string {
	switch e.Rollback {
	case RollbackUnavailable:
		return "rollback commit unavailable, update aborted and repository state is unknown"
	case RollbackFailed:
		return fmt.Sprintf("WARNING: rollback also failed: %v — malicious content may remain", e.ResetErr)
	}
	return "rolled back"
}

func (e *AuditGateError) Error() string {
	return e.message("security audit failed") + ": " + audit.ErrBlocked.Error()
}

// message words the block starting with subject, e.g. "security audit failed".
func (e *AuditGateError) message(subject string) string {
	head := fmt.Sprintf("%s — findings at/above %s detected", subject, e.Threshold)
	if e.ScanErr != nil {
		head = fmt.Sprintf("%s: %v", subject, e.ScanErr)
	}
	switch e.Rollback {
	case RollbackUnavailable:
		return subject + " — " + e.RollbackNote()
	case RollbackFailed:
		return head + "; " + e.RollbackNote()
	}
	return head + " — rolled back (use --skip-audit to bypass)"
}

func (e *AuditGateError) Unwrap() error { return audit.ErrBlocked }

// Run scans the checkout and decides whether the pulled content stays.
func (g AuditGate) Run() (*AuditGateResult, error) {
	threshold, err := audit.NormalizeThreshold(g.Threshold)
	if err != nil {
		threshold = audit.DefaultThreshold()
	}
	if g.BeforeHash == "" {
		return nil, &AuditGateError{Threshold: threshold, Rollback: RollbackUnavailable}
	}

	var scanResult *audit.Result
	if g.ProjectRoot != "" {
		scanResult, err = audit.ScanSkillForProject(g.RepoPath, g.ProjectRoot, g.Follow)
	} else {
		scanResult, err = audit.ScanSkillWithFollow(g.RepoPath, g.Follow)
	}
	if err != nil {
		return nil, g.block(&AuditGateError{ScanErr: err, Threshold: threshold})
	}

	res := &AuditGateResult{Audit: scanResult, Threshold: threshold}
	acceptPath := g.AcceptPath
	if acceptPath == "" {
		acceptPath = g.RepoPath
	}
	res.AcceptedSkipped = ApplyAcceptedFindings(g.SourceDir, acceptPath, scanResult)
	if !scanResult.HasSeverityAtOrAbove(threshold) {
		return res, nil
	}
	if g.Override {
		res.Overridden = true
		return res, nil
	}
	var confirmErr error
	if g.Confirm != nil {
		ok, err := g.Confirm(res)
		if err == nil && ok {
			res.Overridden = true
			return res, nil
		}
		confirmErr = err
	}
	blocked := g.block(&AuditGateError{Threshold: threshold})
	if confirmErr != nil {
		return res, fmt.Errorf("%w (confirmation failed: %v)", blocked, confirmErr)
	}
	return res, blocked
}

// block resets the checkout to the pre-pull commit and records the outcome.
func (g AuditGate) block(e *AuditGateError) *AuditGateError {
	e.BeforeHash = g.BeforeHash
	if err := gitResetHard(g.RepoPath, g.BeforeHash); err != nil {
		e.Rollback, e.ResetErr = RollbackFailed, err
	}
	return e
}

// blockedDetailsError is the wording install uses for findings that blocked
// and were rolled back: it lists them and names the override flags.
func blockedDetailsError(e *AuditGateError, res *AuditGateResult, subject string) error {
	return fmt.Errorf(
		"%s (rolled back to %s):\n%s\n\nUse --force to override or --skip-audit to bypass scanning: %w",
		subject,
		shortHash(e.BeforeHash),
		strings.Join(blockedFindingDetails(res.Audit.Findings, e.Threshold), "\n"),
		audit.ErrBlocked,
	)
}

// auditGateFailClosed gates a pulled non-tracked skill checkout (update and
// relock). The dashboard reads "post-update audit failed" as "force will not
// help", so this wording stays separate from AuditGateError's.
func auditGateFailClosed(sourceDir, repoPath, beforeHash, threshold, projectRoot string, auditOverride bool, follow ...*sourcewalk.Follow) (*audit.Result, error) {
	gate := AuditGate{
		SourceDir:   sourceDir,
		RepoPath:    repoPath,
		BeforeHash:  beforeHash,
		Threshold:   threshold,
		ProjectRoot: projectRoot,
		Override:    auditOverride,
	}
	if len(follow) > 0 {
		gate.Follow = follow[0]
	}
	res, err := gate.Run()
	var blocked *AuditGateError
	switch {
	case err == nil:
		return res.Audit, nil
	case !errors.As(err, &blocked):
		return nil, err
	case blocked.ScanErr != nil || blocked.Rollback == RollbackUnavailable:
		return nil, fmt.Errorf("%s: %w", blocked.message("post-update audit failed"), audit.ErrBlocked)
	case blocked.Rollback == RolledBack:
		return nil, blockedDetailsError(blocked, res,
			fmt.Sprintf("post-update audit failed — findings at/above %s detected", blocked.Threshold))
	}
	return nil, fmt.Errorf("post-update audit found findings at/above %s; %s: %w", blocked.Threshold, blocked.RollbackNote(), audit.ErrBlocked)
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

	acceptRoot, acceptPath := opts.auditAcceptTarget(repoPath)
	res, err := AuditGate{
		SourceDir:   acceptRoot,
		RepoPath:    repoPath,
		AcceptPath:  acceptPath,
		BeforeHash:  beforeHash,
		Threshold:   opts.AuditThreshold,
		ProjectRoot: opts.AuditProjectRoot,
		Follow:      opts.SourceFollow,
		Override:    opts.AuditOverride,
	}.Run()
	if err != nil {
		var blocked *AuditGateError
		// The CLI reads "tracked repository" from these to word its summary.
		if errors.As(err, &blocked) && blocked.ScanErr == nil {
			switch blocked.Rollback {
			case RolledBack:
				return blockedDetailsError(blocked, res,
					fmt.Sprintf("security audit failed — findings at/above %s detected in tracked repository", blocked.Threshold))
			case RollbackFailed:
				return fmt.Errorf("security audit found findings at/above %s in tracked repository — %s: %w",
					blocked.Threshold, blocked.RollbackNote(), audit.ErrBlocked)
			}
		}
		return err
	}
	threshold, scanResult := res.Threshold, res.Audit
	result.AuditThreshold = threshold
	result.AuditRiskScore = scanResult.RiskScore
	result.AuditRiskLabel = scanResult.RiskLabel
	if result.AuditRiskLabel == "" && len(scanResult.Findings) == 0 {
		result.AuditRiskLabel = "CLEAN"
	}

	if len(scanResult.Findings) == 0 {
		return nil
	}

	for _, f := range scanResult.Findings {
		msg := fmt.Sprintf("audit %s: %s (%s:%d)", f.Severity, f.Message, f.File, f.Line)
		if f.Acknowledged {
			msg += " (accepted earlier)"
		}
		if f.Snippet != "" {
			msg += fmt.Sprintf("\n       %q", f.Snippet)
		}
		result.Warnings = append(result.Warnings, msg)
	}

	if res.Overridden {
		result.Warnings = append(result.Warnings,
			fmt.Sprintf("audit findings at/above block threshold (%s); proceeding due to --force", threshold))
		result.Warnings = append(result.Warnings, recordAcceptedWarning(acceptRoot, acceptPath, scanResult, threshold)...)
	} else {
		result.Warnings = append(result.Warnings,
			fmt.Sprintf("audit findings detected, but none at/above block threshold (%s)", threshold))
	}

	return nil
}

// recordAcceptedWarning persists the findings a --force override just accepted
// and returns a warning line describing the outcome.
func recordAcceptedWarning(sourceDir, path string, res *audit.Result, threshold string) []string {
	n, err := RecordAcceptedFindings(sourceDir, path, res, threshold)
	if err != nil {
		return []string{fmt.Sprintf("failed to record accepted findings: %v", err)}
	}
	if n == 0 {
		return nil
	}
	return []string{fmt.Sprintf("recorded %d accepted finding(s); future updates won't block on them", n)}
}

func blockedFindingDetails(findings []audit.Finding, threshold string) []string {
	var details []string
	for _, f := range findings {
		if f.Acknowledged {
			continue
		}
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
