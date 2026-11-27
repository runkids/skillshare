package install

import (
	"errors"
	"fmt"
)

// scanExtraCandidate is the existing candidate audit, kept replaceable for
// deterministic scanner-error tests.
var scanExtraCandidate = (AuditGate).Run

// gateExtraCandidate scans an isolated extra without publishing or recording
// accepted findings. Only an explicit audit override accepts findings.
func gateExtraCandidate(staged, extrasRoot, destPath, commit string, opts InstallOptions) (*AuditGateResult, error) {
	// InstallRemoteExtra validates and normalizes this before any root or fetch work.
	threshold := opts.AuditThreshold
	if opts.SkipAudit {
		return &AuditGateResult{Threshold: threshold, Skipped: true}, nil
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
