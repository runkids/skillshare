package main

import (
	"fmt"
	"time"

	"skillshare/internal/audit"
	"skillshare/internal/install"
	"skillshare/internal/ui"
)

// updateContext holds mode-specific configuration for update operations.
// projectRoot == "" means global mode.
type updateContext struct {
	sourcePath  string
	projectRoot string
	opts        *updateOptions
}

func (uc *updateContext) isProject() bool {
	return uc.projectRoot != ""
}

func (uc *updateContext) auditScanFn() auditScanFunc {
	if uc.isProject() {
		return func(path string) (*audit.Result, error) {
			return audit.ScanSkillForProject(path, uc.projectRoot)
		}
	}
	return audit.ScanSkill
}

func (uc *updateContext) makeInstallOpts() install.InstallOptions {
	opts := install.InstallOptions{
		Force:          true,
		Update:         true,
		SkipAudit:      uc.opts.skipAudit,
		AuditThreshold: uc.opts.threshold,
	}
	if uc.isProject() {
		opts.AuditProjectRoot = uc.projectRoot
	}
	return opts
}

// executeBatchUpdate runs the 3-phase batch update loop shared by global and
// project modes. Caller is responsible for header/dry-run message before calling.
// Returns combined updateResult and any security error.
func executeBatchUpdate(uc *updateContext, targets []updateTarget) (updateResult, error) {
	total := len(targets)
	fmt.Println()
	progressBar := ui.StartProgress("Updating skills", total)

	var result updateResult
	var auditEntries []batchAuditEntry
	var blockedEntries []batchBlockedEntry

	// Group skills by RepoURL to optimize updates
	repoGroups := make(map[string][]updateTarget)
	var standaloneSkills []updateTarget
	var trackedRepos []updateTarget

	for _, t := range targets {
		if t.isRepo {
			trackedRepos = append(trackedRepos, t)
			continue
		}
		meta, err := install.ReadMeta(t.path)
		if err == nil && meta != nil && meta.RepoURL != "" {
			repoGroups[meta.RepoURL] = append(repoGroups[meta.RepoURL], t)
		} else {
			standaloneSkills = append(standaloneSkills, t)
		}
	}

	scanFn := uc.auditScanFn()

	// Phase 1: tracked repos (git pull)
	for _, t := range trackedRepos {
		progressBar.UpdateTitle(fmt.Sprintf("Updating %s", t.name))
		updated, auditResult, err := updateTrackedRepoQuick(
			t.name, t.path, uc.opts.dryRun, uc.opts.force,
			uc.opts.skipAudit, uc.opts.threshold, scanFn)
		if err != nil {
			if isSecurityError(err) {
				result.securityFailed++
				blockedEntries = append(blockedEntries, batchBlockedEntry{name: t.name, errMsg: err.Error()})
			} else {
				result.skipped++
			}
		} else if updated {
			result.updated++
		} else {
			result.skipped++
		}
		if auditResult != nil {
			auditEntries = append(auditEntries, batchAuditEntryFromAuditResult(t.name, auditResult, uc.opts.skipAudit))
		}
		progressBar.Increment()
	}

	// Phase 2: grouped skills (one clone per repo)
	for repoURL, groupTargets := range repoGroups {
		if uc.opts.dryRun {
			for _, t := range groupTargets {
				progressBar.UpdateTitle(fmt.Sprintf("Updating %s", t.name))
				progressBar.Increment()
				result.skipped++
			}
			continue
		}

		progressBar.UpdateTitle(fmt.Sprintf("Updating %d skills from %s", len(groupTargets), repoURL))

		skillTargetMap := make(map[string]string)
		pathToTarget := make(map[string]updateTarget)
		for _, t := range groupTargets {
			meta, _ := install.ReadMeta(t.path)
			if meta != nil {
				skillTargetMap[meta.Subdir] = t.path
				pathToTarget[meta.Subdir] = t
			}
		}

		batchOpts := uc.makeInstallOpts()
		if ui.IsTTY() {
			batchOpts.OnProgress = func(line string) {
				progressBar.UpdateTitle(line)
			}
		}

		batchResult, err := install.UpdateSkillsFromRepo(repoURL, skillTargetMap, batchOpts)
		if err != nil {
			for _, t := range groupTargets {
				progressBar.UpdateTitle(fmt.Sprintf("Failed %s: %v", t.name, err))
				result.skipped++
				progressBar.Increment()
			}
			continue
		}

		for subdir := range skillTargetMap {
			t := pathToTarget[subdir]
			progressBar.UpdateTitle(fmt.Sprintf("Updating %s", t.name))

			if ui.IsTTY() {
				time.Sleep(50 * time.Millisecond)
			}

			if err := batchResult.Errors[subdir]; err != nil {
				if isSecurityError(err) {
					result.securityFailed++
					blockedEntries = append(blockedEntries, batchBlockedEntry{name: t.name, errMsg: err.Error()})
				} else {
					result.skipped++
				}
			} else if res := batchResult.Results[subdir]; res != nil {
				result.updated++
				auditEntries = append(auditEntries, batchAuditEntryFromInstallResult(t.name, res))
			} else {
				result.skipped++
			}
			progressBar.Increment()
		}
	}

	// Phase 3: standalone skills
	for _, t := range standaloneSkills {
		progressBar.UpdateTitle(fmt.Sprintf("Updating %s", t.name))
		installOpts := uc.makeInstallOpts()
		updated, installRes, err := updateSkillFromMeta(t.path, uc.opts.dryRun, installOpts)
		if err != nil {
			if isSecurityError(err) {
				result.securityFailed++
				blockedEntries = append(blockedEntries, batchBlockedEntry{name: t.name, errMsg: err.Error()})
			} else {
				result.skipped++
			}
		} else if updated {
			result.updated++
		} else {
			result.skipped++
		}
		if installRes != nil {
			auditEntries = append(auditEntries, batchAuditEntryFromInstallResult(t.name, installRes))
		}
		progressBar.Increment()
	}

	progressBar.Stop()

	// Render results
	if !uc.opts.dryRun {
		displayUpdateBlockedSection(blockedEntries)
		displayUpdateAuditResults(auditEntries, uc.opts.auditVerbose)
		fmt.Println()
		ui.SuccessMsg("Updated %d, skipped %d of %d skill(s)", result.updated, result.skipped, total)
		if result.securityFailed > 0 {
			ui.Warning("Blocked: %d (security)", result.securityFailed)
		}
	}

	if result.updated > 0 && !uc.opts.dryRun {
		ui.SectionLabel("Next Steps")
		ui.Info("Run 'skillshare sync' to distribute changes")
	}

	if result.securityFailed > 0 {
		return result, fmt.Errorf("%d repo(s) blocked by security audit", result.securityFailed)
	}
	return result, nil
}
