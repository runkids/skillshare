package install

import (
	"errors"
	"fmt"

	"skillshare/internal/audit"
)

// scanExtraCandidate is the existing candidate audit, kept replaceable for
// deterministic scanner-error tests.
var scanExtraCandidate = (AuditGate).Run

// gateExtraCandidate scans an isolated extra without publishing or recording
// accepted findings. Only an explicit audit override accepts findings.
func gateExtraCandidate(staged, extrasRoot, destPath, commit string, opts InstallOptions) (*AuditGateResult, error) {
	threshold, err := audit.NormalizeThreshold(opts.AuditThreshold)
	if err != nil {
		return nil, fmt.Errorf("invalid audit threshold: %w", err)
	}
	if opts.SkipAudit {
		return &AuditGateResult{Threshold: threshold, Skipped: true}, nil
	}
	// Preflight global rules before entering the candidate scan.
	if opts.AuditProjectRoot == "" {
		if _, err := audit.Rules(); err != nil {
			return nil, &AuditGateError{ScanErr: fmt.Errorf("load global rules: %w", err), Threshold: threshold, BeforeHash: commit, Rollback: CheckoutUnchanged}
		}
	}
	if opts.OnProgress != nil {
		opts.OnProgress("Auditing extra candidate...")
	}
	gate := AuditGate{
		SourceDir:        extrasRoot,
		RepoPath:         staged,
		AcceptPath:       destPath,
		BeforeHash:       commit,
		Candidate:        true,
		ReadOnlyMetadata: true,
		Threshold:        threshold,
		ProjectRoot:      opts.AuditProjectRoot,
		Override:         opts.AuditOverride,
	}
	result, err := scanExtraCandidate(gate)
	var blocked *AuditGateError
	if errors.As(err, &blocked) && blocked.ScanErr == nil && blocked.Rollback == CheckoutUnchanged && result != nil && result.Audit != nil {
		return result, blockedDetailsError(blocked, result, fmt.Sprintf("security audit failed — findings at/above %s detected", blocked.Threshold))
	}
	return result, err
}
