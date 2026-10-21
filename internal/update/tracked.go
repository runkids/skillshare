// Package update holds update operations shared by the CLI and the dashboard.
package update

import (
	"fmt"
	"path/filepath"

	"skillshare/internal/audit"
	"skillshare/internal/git"
	"skillshare/internal/install"
	"skillshare/internal/sourcewalk"
)

// Status is how far TrackedRepo got when it returned without an error.
type Status string

const (
	StatusUpdated  Status = "updated"
	StatusUpToDate Status = "up-to-date"
	// StatusDirty means uncommitted changes stopped the update before the pull.
	StatusDirty Status = "dirty"
	// StatusDryRun means the pull was not run.
	StatusDryRun Status = "dry-run"
)

// Stage names the step of TrackedRepo that failed before the audit gate.
type Stage string

const (
	StageStatus  Stage = "status"
	StageDiscard Stage = "discard"
	StagePull    Stage = "pull"
)

// Error is a failure before the audit gate. A blocked audit is reported as
// *install.AuditGateError instead.
type Error struct {
	Stage Stage
	Err   error
}

func (e *Error) Error() string {
	switch e.Stage {
	case StageStatus:
		return fmt.Sprintf("failed to check git status: %v", e.Err)
	case StageDiscard:
		return fmt.Sprintf("failed to discard changes: %v", e.Err)
	}
	return fmt.Sprintf("git pull failed: %v", e.Err)
}

func (e *Error) Unwrap() error { return e.Err }

// TrackedRepoOptions configures one tracked-repo update.
type TrackedRepoOptions struct {
	// SourceDir is the source root that holds the repo's metadata and accepted
	// findings.
	SourceDir string
	// Force discards uncommitted changes, pulls with a hard reset and lets
	// audit findings through. It never overrides a scan that could not run.
	Force       bool
	DryRun      bool
	SkipAudit   bool
	Threshold   string
	ProjectRoot string // non-empty audits with the project's rules
	Follow      *sourcewalk.Follow
	// Confirm is asked once when findings would block and Force is unset. It
	// receives the result so far (Info, Audit, Threshold, AcceptedSkipped).
	// Nil, false or an error blocks and rolls back.
	Confirm    func(*TrackedRepoResult) (bool, error)
	OnProgress func(string)
}

// TrackedRepoResult is always returned, also next to an error, filled as far
// as the update got.
type TrackedRepoResult struct {
	Status    Status
	Discarded bool            // Force discarded uncommitted changes (in a dry run: would)
	Info      *git.UpdateInfo // set once the pull succeeded
	Audit     *audit.Result   // nil when the audit was skipped or could not scan
	Threshold string          // normalized block threshold of the audit

	AcceptedSkipped int      // findings ignored because they were accepted earlier
	Overridden      bool     // blocking findings were let through by Force or Confirm
	Recorded        int      // findings newly recorded as accepted
	RecordErr       error    // recording accepted findings failed; the update stays applied
	MetadataChanged bool     // the repo's entry under SourceDir was rewritten
	MetadataErr     error    // refreshing metadata failed; the update stays applied
	Warnings        []string // git submodules the pulled checkout leaves empty
}

// TrackedRepo updates one tracked repo: it refuses or discards uncommitted
// changes, pulls, audits what was pulled, resets to the pre-pull commit when
// the audit blocks, and refreshes the repo's metadata.
func TrackedRepo(repoPath string, opts TrackedRepoOptions) (*TrackedRepoResult, error) {
	res := &TrackedRepoResult{}

	// Force resets the checkout anyway, so it does not need a readable status.
	dirty, err := git.IsDirty(repoPath)
	if err != nil && !opts.Force {
		return res, &Error{Stage: StageStatus, Err: err}
	}
	if dirty {
		if !opts.Force {
			res.Status = StatusDirty
			return res, nil
		}
		res.Discarded = true
		if !opts.DryRun {
			if err := git.Restore(repoPath); err != nil {
				return res, &Error{Stage: StageDiscard, Err: err}
			}
		}
	}
	if opts.DryRun {
		res.Status = StatusDryRun
		return res, nil
	}

	pull := git.PullWithProgress
	if opts.Force {
		pull = git.ForcePullWithProgress
	}
	info, err := pull(repoPath, git.AuthEnvForRepo(repoPath), opts.OnProgress)
	if err != nil {
		return res, &Error{Stage: StagePull, Err: err}
	}
	res.Info = info

	if !info.UpToDate && !opts.SkipAudit {
		if err := auditPulled(repoPath, res, opts); err != nil {
			return res, err
		}
	}

	if !info.UpToDate {
		res.Warnings = install.SubmoduleWarnings(repoPath, git.AuthEnvForRepo(repoPath))
	}

	res.MetadataChanged, res.MetadataErr = refreshMetadata(repoPath, opts)
	res.Status = StatusUpdated
	if info.UpToDate {
		res.Status = StatusUpToDate
	}
	return res, nil
}

func auditPulled(repoPath string, res *TrackedRepoResult, opts TrackedRepoOptions) error {
	fill := func(g *install.AuditGateResult) {
		res.Audit, res.Threshold, res.AcceptedSkipped, res.Overridden = g.Audit, g.Threshold, g.AcceptedSkipped, g.Overridden
	}
	gate := install.AuditGate{
		SourceDir:   opts.SourceDir,
		RepoPath:    repoPath,
		BeforeHash:  res.Info.BeforeHash,
		Threshold:   opts.Threshold,
		ProjectRoot: opts.ProjectRoot,
		Follow:      opts.Follow,
		Override:    opts.Force,
	}
	if opts.Confirm != nil {
		gate.Confirm = func(g *install.AuditGateResult) (bool, error) {
			fill(g)
			return opts.Confirm(res)
		}
	}
	g, err := gate.Run()
	if g != nil {
		fill(g)
	}
	if err != nil {
		return err
	}
	if g.Overridden {
		res.Recorded, res.RecordErr = install.RecordAcceptedFindings(opts.SourceDir, repoPath, g.Audit, g.Threshold)
	}
	return nil
}

func refreshMetadata(repoPath string, opts TrackedRepoOptions) (bool, error) {
	rel, err := filepath.Rel(opts.SourceDir, repoPath)
	if err != nil {
		return false, err
	}
	return install.RefreshTrackedRepoMetadata(opts.SourceDir, rel, repoPath, opts.Follow)
}
