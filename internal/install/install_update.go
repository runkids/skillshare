package install

import (
	"fmt"
	"os"
	"path/filepath"

	"skillshare/internal/audit"
)

func handleUpdate(source *Source, destPath string, result *InstallResult, opts InstallOptions) (*InstallResult, error) {
	result.SkillPath = destPath

	// For git repos without subdir, use git pull for true single-skill repo
	// installs. Legacy installs of repo-root orchestrators (root SKILL.md plus
	// child skills) used to be full repository clones; migrate those through the
	// discovery installer so the root skill becomes SKILL.md-only.
	if source.IsGit() && !source.HasSubdir() && IsGitRepo(destPath) {
		if hasDescendantSkillFile(destPath) {
			migrated, err := updateRepoRootOrchestrator(source, destPath, result, opts)
			if migrated || err != nil {
				return result, err
			}
		}

		if opts.DryRun {
			result.Action = "would update (git pull)"
			return result, nil
		}

		threshold, err := audit.NormalizeThreshold(opts.AuditThreshold)
		if err != nil {
			threshold = audit.DefaultThreshold()
		}
		result.AuditThreshold = threshold

		// Record hash before pull for rollback
		beforeHash, err := getGitFullHash(destPath)
		if err != nil {
			return nil, fmt.Errorf("failed to determine rollback commit before update (aborting for safety): %w", err)
		}
		if beforeHash == "" {
			return nil, fmt.Errorf("failed to determine rollback commit before update (aborting for safety): empty commit hash")
		}

		if err := gitPull(destPath, opts.OnProgress); err != nil {
			return nil, fmt.Errorf("failed to update: %w", err)
		}

		// Post-pull audit gate: rollback on findings at/above threshold unless skipped.
		if !opts.SkipAudit {
			afterHash, _ := getGitFullHash(destPath)
			if afterHash != beforeHash {
				scanResult, err := auditGateFailClosed(destPath, beforeHash, threshold, opts.AuditProjectRoot)
				if err != nil {
					return nil, err
				}
				result.AuditRiskScore = scanResult.RiskScore
				result.AuditRiskLabel = scanResult.RiskLabel
			}
		} else {
			result.AuditSkipped = true
		}

		// Update centralized metadata timestamp and file hashes.
		meta, _ := ReadMeta(destPath)
		if meta == nil {
			meta = NewMetaFromSource(source)
		}
		if hash, err := getGitCommit(destPath); err == nil {
			meta.Version = hash
		}
		if meta.Subdir != "" {
			meta.TreeHash = getSubdirTreeHash(destPath, meta.Subdir)
		}
		if hashes, hashErr := ComputeFileHashes(destPath); hashErr == nil {
			meta.FileHashes = hashes
		}
		if err := WriteMetaToStore(opts.SourceDir, destPath, meta); err != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("failed to write metadata: %v", err))
		}

		result.Action = "updated"
		return result, nil
	}

	// For other cases (e.g., git with subdir), reinstall automatically
	// --update implies willingness to reinstall when git pull is not possible

	if opts.DryRun {
		result.Action = "would reinstall from source"
		return result, nil
	}

	// Safe update: install to temp first, then swap
	tempDir, err := os.MkdirTemp("", "skillshare-update-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(tempDir)

	tempDest := filepath.Join(tempDir, "skill")

	// Install to temp location first.
	// Force is NOT set: tempDest is fresh, so no overwrite needed.
	// This lets auditInstalledSkill properly gate on findings at/above threshold,
	// consistent with auditGateAfterPull for tracked repos.
	innerResult, err := Install(source, tempDest, InstallOptions{
		Name:             opts.Name,
		Force:            false,
		DryRun:           false,
		Update:           false,
		OnProgress:       opts.OnProgress,
		SkipAudit:        opts.SkipAudit,
		AuditThreshold:   opts.AuditThreshold,
		AuditProjectRoot: opts.AuditProjectRoot,
	})
	if err != nil {
		// Installation failed - original skill is preserved
		return nil, err
	}

	// Propagate audit results and warnings from inner install
	if innerResult != nil {
		result.AuditRiskScore = innerResult.AuditRiskScore
		result.AuditRiskLabel = innerResult.AuditRiskLabel
		result.AuditSkipped = innerResult.AuditSkipped
		result.AuditThreshold = innerResult.AuditThreshold
		result.Warnings = append(result.Warnings, innerResult.Warnings...)
	}

	// Installation succeeded - now safe to remove original and move new
	if err := os.RemoveAll(destPath); err != nil {
		return nil, fmt.Errorf("failed to remove existing skill: %w", err)
	}

	if err := os.Rename(tempDest, destPath); err != nil {
		// Rename failed (possibly cross-device), try copy instead
		if err := copyDir(tempDest, destPath); err != nil {
			return nil, fmt.Errorf("failed to move updated skill: %w", err)
		}
	}

	result.Action = "reinstalled"
	return result, nil
}

func hasDescendantSkillFile(root string) bool {
	found := false
	_ = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || found {
			return nil
		}
		if info.IsDir() {
			if path != root && info.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		if info.Name() == "SKILL.md" && filepath.Dir(path) != root {
			found = true
		}
		return nil
	})
	return found
}

func updateRepoRootOrchestrator(source *Source, destPath string, result *InstallResult, opts InstallOptions) (bool, error) {
	discovery, err := DiscoverFromGitWithProgress(source, opts.OnProgress)
	if err != nil {
		return false, fmt.Errorf("failed to discover repository for orchestrator update: %w", err)
	}
	defer CleanupDiscovery(discovery)

	rootSkill, ok := repoRootOrchestratorRoot(discovery)
	if !ok {
		return false, nil
	}

	if opts.DryRun {
		result.Action = "would reinstall root skill"
		return true, nil
	}

	tempDir, err := os.MkdirTemp("", "skillshare-update-*")
	if err != nil {
		return true, fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(tempDir)

	tempDest := filepath.Join(tempDir, "skill")
	innerResult, err := installFromDiscoveryInternal(discovery, rootSkill, tempDest, InstallOptions{
		Name:             opts.Name,
		Force:            false,
		DryRun:           false,
		Update:           false,
		OnProgress:       opts.OnProgress,
		SkipAudit:        opts.SkipAudit,
		AuditThreshold:   opts.AuditThreshold,
		AuditProjectRoot: opts.AuditProjectRoot,
	}, false)
	if err != nil {
		return true, err
	}
	if innerResult != nil {
		result.AuditRiskScore = innerResult.AuditRiskScore
		result.AuditRiskLabel = innerResult.AuditRiskLabel
		result.AuditSkipped = innerResult.AuditSkipped
		result.AuditThreshold = innerResult.AuditThreshold
		result.Warnings = append(result.Warnings, innerResult.Warnings...)
	}

	if err := os.RemoveAll(destPath); err != nil {
		return true, fmt.Errorf("failed to remove existing skill: %w", err)
	}
	if err := os.Rename(tempDest, destPath); err != nil {
		if err := copyDir(tempDest, destPath); err != nil {
			return true, fmt.Errorf("failed to move updated skill: %w", err)
		}
	}

	fullSource, fullSubdir := discoveredSkillSourceParts(discovery, rootSkill)
	if err := writeDiscoveredSkillMetadata(discovery, rootSkill, destPath, opts.SourceDir, fullSource, fullSubdir); err != nil {
		result.Warnings = append(result.Warnings, fmt.Sprintf("failed to write metadata: %v", err))
	}

	result.Action = "reinstalled"
	return true, nil
}

func repoRootOrchestratorRoot(discovery *DiscoveryResult) (SkillInfo, bool) {
	if discovery == nil || discovery.Source == nil || discovery.Source.HasSubdir() {
		return SkillInfo{}, false
	}
	for _, skill := range discovery.Skills {
		if skill.Path == "." && descendantSkillPaths(discovery, skill) != nil {
			return skill, true
		}
	}
	return SkillInfo{}, false
}
