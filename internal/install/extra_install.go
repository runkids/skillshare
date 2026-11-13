package install

import (
	"fmt"
	"os"
	"path/filepath"

	"skillshare/internal/audit"
	"skillshare/internal/utils"
)

// RemoteExtraRequest provides the caller's config-specific operations. They run
// while the extras root lock is held, but do not lock unrelated config writers.
type RemoteExtraRequest struct {
	Name, RawSource, Branch                      string
	Source                                       *Source
	ExtrasRoot, ProjectRoot, LockDir, ConfigPath string
	Opts                                         InstallOptions
	Recheck, WriteConfig, RevertConfig           func() error
	RepairConfig                                 func(remote, branch string) error
	RefreshAfterRepair                           func() error
	PinBranch                                    func() string // followed branch from the refreshed config baseline
}

type RemoteExtraResult struct {
	Name, Dir, Remote, Branch, Commit, Action  string
	Interrupted, AuditSkipped, AuditOverridden bool
	Warnings                                   []string
}

// InstallRemoteExtra prepares and publishes a remote extra under the root lock.
func InstallRemoteExtra(req RemoteExtraRequest) (*RemoteExtraResult, error) {
	if err := ValidateExtraName(req.Name); err != nil {
		return nil, err
	}
	if _, err := audit.NormalizeThreshold(req.Opts.AuditThreshold); err != nil {
		return nil, fmt.Errorf("invalid audit threshold: %w", err)
	}
	if err := RejectSourceCredentials(req.RawSource); err != nil {
		return nil, err
	}
	if req.Source == nil || !req.Source.IsGit() {
		return nil, fmt.Errorf("remote extra requires a Git source")
	}
	if req.Source.Raw != req.RawSource {
		return nil, fmt.Errorf("remote extra source does not match its identity")
	}
	if err := RejectSourceCredentials(req.Source.CloneURL); err != nil {
		return nil, err
	}
	parsed, err := ParseSourceWithOptions(req.RawSource, req.Opts.ParseOptions)
	if err != nil || parsed == nil || !parsed.IsGit() || parsed.CloneURL != req.Source.CloneURL || parsed.Subdir != req.Source.Subdir {
		return nil, fmt.Errorf("remote extra source does not match its identity")
	}
	if req.ExtrasRoot == "" || req.ConfigPath == "" || (req.ProjectRoot != "" && req.LockDir == "") || req.Recheck == nil || req.WriteConfig == nil || req.RevertConfig == nil || req.RepairConfig == nil || req.RefreshAfterRepair == nil || req.PinBranch == nil {
		return nil, fmt.Errorf("remote extra request is missing required state or callbacks")
	}
	dest := filepath.Join(req.ExtrasRoot, req.Name)
	result := &RemoteExtraResult{Name: req.Name, Dir: dest, Remote: req.RawSource, Branch: req.Branch}
	if err := refuseExtraDestinationLink(dest); err != nil {
		return nil, err
	}
	if req.Opts.DryRun {
		state := InspectRemoteExtra(req.ExtrasRoot, req.Name)
		if state.Err != nil {
			return nil, state.Err
		}
		result.Interrupted, result.Action = state.Interrupted, "would install"
		if result.Branch == "" {
			result.Branch = req.PinBranch()
		}
		if !state.Interrupted {
			if err := req.Recheck(); err != nil {
				return nil, err
			}
			if !state.Missing && state.Modified && !req.Opts.AuditOverride {
				return nil, fmt.Errorf("extra %q has local edits or unknown ownership; use --force explicitly", req.Name)
			}
		}
		return result, nil
	}
	unlock, err := LockExtrasRoot(req.ExtrasRoot)
	if err != nil {
		return nil, err
	}
	defer unlock()
	if err := refuseExtraDestinationLink(dest); err != nil {
		return nil, err
	}
	state := InspectRemoteExtra(req.ExtrasRoot, req.Name)
	if state.Err != nil {
		return nil, state.Err
	}
	if state.Interrupted {
		if err := repairRemoteExtra(req, result); err != nil {
			return nil, err
		}
		result.Interrupted = true
	}
	if err := req.Recheck(); err != nil {
		return nil, err
	}
	state = InspectRemoteExtra(req.ExtrasRoot, req.Name)
	if state.Err != nil {
		return nil, state.Err
	}
	if !state.Missing && state.Modified && !req.Opts.AuditOverride {
		return nil, fmt.Errorf("extra %q has local edits or unknown ownership; use --force explicitly", req.Name)
	}
	store, _, err := ReadMetadata(req.ExtrasRoot)
	if err != nil {
		return nil, err
	}
	entry := store.GetExact(req.Name)
	if !state.Missing && entry != nil && entry.Source != "" && entry.Source != req.RawSource {
		return nil, fmt.Errorf("extra %q has a different origin; remove it before reinstalling", req.Name)
	}
	// Repair may have changed the declaration. Read its branch only after
	// RefreshAfterRepair and Recheck, including when local metadata is absent.
	pinnedBranch := req.PinBranch()
	if req.Branch == "" {
		req.Branch = pinnedBranch
	}
	result.Branch = req.Branch
	source := *req.Source
	source.Branch = req.Branch
	source.Commit = ""
	if !req.Opts.Update && pinnedBranch == req.Branch {
		if req.ProjectRoot != "" {
			lock, err := LoadLock(req.LockDir)
			if err != nil {
				return nil, err
			}
			source.Commit = lock.ExtraCommitFor(req.Name, req.RawSource)
		} else if entry != nil && entry.Source == req.RawSource && entry.Branch == req.Branch && len(entry.Commit) == 40 && IsCommitSHA(entry.Commit) {
			source.Commit = entry.Commit
		}
	}
	candidate, cleanup, err := prepareExtraCandidate(&source, req.ExtrasRoot, dest, req.Opts)
	if err != nil {
		return nil, err
	}
	defer cleanup()
	if err := req.Recheck(); err != nil {
		return nil, err
	}
	if err := refuseExtraDestinationLink(dest); err != nil {
		return nil, err
	}
	latest := InspectRemoteExtra(req.ExtrasRoot, req.Name)
	if latest.Err != nil {
		return nil, latest.Err
	}
	if latest.Interrupted {
		return nil, fmt.Errorf("extra %q became interrupted during fetch", req.Name)
	}
	if !latest.Missing && latest.Modified && !req.Opts.AuditOverride {
		return nil, fmt.Errorf("extra %q has local edits or unknown ownership; use --force explicitly", req.Name)
	}
	if err := publishRemoteExtra(req, candidate); err != nil {
		return nil, err
	}
	result.Commit = candidate.fetch.Commit
	result.AuditSkipped = result.AuditSkipped || candidate.audit.Skipped
	result.AuditOverridden = result.AuditOverridden || candidate.audit.Overridden
	result.Warnings = append(result.Warnings, candidate.warnings...)
	if state.Missing {
		result.Action = "installed"
	} else {
		result.Action = "reinstalled"
	}
	if req.ProjectRoot != "" {
		if dir, _ := extraIgnoreTarget(req); dir == "" {
			result.Warnings = append(result.Warnings, "extras root is outside the project; no ignore rules were written")
		}
	}
	return result, nil
}

func refuseExtraDestinationLink(path string) error {
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if utils.IsLinkMode(path, info.Mode()) || !info.IsDir() {
		return fmt.Errorf("unsafe extra destination %s", path)
	}
	return nil
}
