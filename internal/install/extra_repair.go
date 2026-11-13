package install

import (
	"fmt"
	"os"
	"path/filepath"
)

// repairRemoteExtra completes the journaled publication, never the new request.
// Failed repair leaves the marker in place for the next attempt.
func repairRemoteExtra(req RemoteExtraRequest, result *RemoteExtraResult) error {
	j, err := readExtraJournal(req.ExtrasRoot, req.Name)
	if err != nil {
		return fmt.Errorf("interrupted extra %q has missing or invalid marker; preserve staging paths: %w", req.Name, err)
	}
	source, err := ParseSourceWithOptions(j.Remote, req.Opts.ParseOptions)
	if err != nil || source == nil || !source.IsGit() {
		return fmt.Errorf("invalid journaled Git source for %q", req.Name)
	}
	source.Branch, source.Commit = j.Branch, j.Commit
	dest := filepath.Join(req.ExtrasRoot, req.Name)
	candidate, cleanup, err := prepareExtraCandidate(source, req.ExtrasRoot, dest, req.Opts)
	if err != nil {
		return fmt.Errorf("repair %q: %w", req.Name, err)
	}
	defer cleanup()
	candidateHash, err := fingerprintExtraTree(candidate.staged)
	if err != nil {
		return err
	}
	stage := filepath.Join(req.ExtrasRoot, extraStagingDir)
	incoming := filepath.Join(stage, "incoming-"+req.Name)
	previous := filepath.Join(stage, "previous-"+req.Name)
	liveHash, liveExists, err := existingExtraFingerprint(dest)
	if err != nil {
		return fmt.Errorf("repair live: %w", err)
	}
	previousHash, previousExists, err := existingExtraFingerprint(previous)
	if err != nil {
		return fmt.Errorf("repair previous: %w", err)
	}
	liveCandidate := liveExists && sameExtraFingerprint(liveHash, candidateHash)
	liveOld := liveExists && j.HadPrevious && sameExtraFingerprint(liveHash, j.Old)
	previousOld := previousExists && j.HadPrevious && sameExtraFingerprint(previousHash, j.Old)
	if (liveExists && !liveCandidate && !liveOld || previousExists && !previousOld) && !req.Opts.AuditOverride {
		return fmt.Errorf("interrupted extra %q has later edits; preserve them or explicitly use --force", req.Name)
	}
	if !liveCandidate {
		// The existing incoming tree has not been trusted; only replace it after
		// the journaled commit has passed fetch, staging, audit and comparisons.
		if err := extraCleanup(incoming); err != nil {
			return fmt.Errorf("repair incoming: %w", err)
		}
		if err := stageFetchedTree(candidate.staged, "", incoming); err != nil {
			return err
		}
		if liveExists {
			if !previousExists && liveOld {
				if err := extraRename(dest, previous); err != nil {
					return err
				}
				previousExists = true
			} else {
				if err := extraCleanup(dest); err != nil {
					return err
				}
			}
		}
		if err := extraRename(incoming, dest); err != nil {
			return err
		}
	}
	if err := extraBoundary("repair-live"); err != nil {
		return err
	}
	ignoreDir, ignoreEntries := extraIgnoreTarget(req)
	if req.ProjectRoot != "" && ignoreDir == "" {
		result.Warnings = append(result.Warnings, "extras root is outside the project; no ignore rules were written")
	}
	if ignoreDir != "" {
		if err := extraWriteIgnore(ignoreDir, ignoreEntries); err != nil {
			return err
		}
	}
	if err := writeExtraMetadata(req, candidate, j.Remote, j.Branch); err != nil {
		return err
	}
	if req.ProjectRoot != "" {
		if err := writeExtraPin(req, candidate, j.Remote); err != nil {
			return err
		}
	}
	if err := extraWriteConfig(func() error { return req.RepairConfig(j.Remote, j.Branch) }); err != nil {
		return err
	}
	if err := extraBoundary("repair-config"); err != nil {
		return err
	}
	if err := req.RefreshAfterRepair(); err != nil {
		return fmt.Errorf("refresh after repair: %w", err)
	}
	if err := extraCleanup(incoming); err != nil {
		return err
	}
	if previousExists {
		if err := extraCleanup(previous); err != nil {
			return err
		}
	}
	if err := os.Remove(extraJournalPath(req.ExtrasRoot, req.Name)); err != nil {
		return err
	}
	_ = os.Remove(stage)
	result.AuditSkipped = result.AuditSkipped || candidate.audit.Skipped
	result.AuditOverridden = result.AuditOverridden || candidate.audit.Overridden
	result.Warnings = append(result.Warnings, candidate.warnings...)
	return nil
}

func existingExtraFingerprint(dir string) (map[string]string, bool, error) {
	_, err := os.Lstat(dir)
	if os.IsNotExist(err) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	fp, err := fingerprintExtraTree(dir)
	return fp, true, err
}
