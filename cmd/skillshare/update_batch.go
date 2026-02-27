package main

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"skillshare/internal/audit"
	"skillshare/internal/config"
	"skillshare/internal/install"
	"skillshare/internal/trash"
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
	var staleNames []string
	var prunedNames []string

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

	// Phase 1: tracked repos (git pull)
	for _, t := range trackedRepos {
		progressBar.UpdateTitle(fmt.Sprintf("Updating %s", t.name))
		updated, auditResult, err := updateTrackedRepoQuick(uc, t.path)
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
				subdir := meta.Subdir
				if subdir == "" {
					subdir = "."
				}
				skillTargetMap[subdir] = t.path
				pathToTarget[subdir] = t
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
				if isStaleError(err) {
					if uc.opts.prune {
						if pruneErr := pruneSkill(t.path, t.name, uc); pruneErr == nil {
							prunedNames = append(prunedNames, t.name)
							result.pruned++
						} else {
							result.skipped++
						}
					} else {
						staleNames = append(staleNames, t.name)
						result.skipped++
					}
				} else if isSecurityError(err) {
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
		updated, installRes, err := updateSkillFromMeta(uc, t.path)
		if err != nil {
			if isStaleError(err) {
				if uc.opts.prune {
					if pruneErr := pruneSkill(t.path, t.name, uc); pruneErr == nil {
						prunedNames = append(prunedNames, t.name)
						result.pruned++
					} else {
						result.skipped++
					}
				} else {
					staleNames = append(staleNames, t.name)
					result.skipped++
				}
			} else if isSecurityError(err) {
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

	// Registry cleanup for pruned skills
	if len(prunedNames) > 0 {
		pruneRegistry(prunedNames, uc)
	}

	// Render results
	if !uc.opts.dryRun {
		displayUpdateBlockedSection(blockedEntries)
		displayPrunedSection(prunedNames)
		displayStaleWarning(staleNames)
		displayUpdateAuditResults(auditEntries, uc.opts.auditVerbose)
		fmt.Println()
		parts := []string{fmt.Sprintf("Updated %d, skipped %d", result.updated, result.skipped)}
		if result.pruned > 0 {
			parts = append(parts, fmt.Sprintf("pruned %d", result.pruned))
		}
		ui.SuccessMsg("%s of %d skill(s)", strings.Join(parts, ", "), total)
		if result.securityFailed > 0 {
			ui.Warning("Blocked: %d (security)", result.securityFailed)
		}
	}

	if (result.updated > 0 || result.pruned > 0) && !uc.opts.dryRun {
		ui.SectionLabel("Next Steps")
		ui.Info("Run 'skillshare sync' to distribute changes")
	}

	if result.securityFailed > 0 {
		return result, fmt.Errorf("%d repo(s) blocked by security audit", result.securityFailed)
	}
	return result, nil
}

// isStaleError returns true if the error indicates a skill path was deleted
// from the upstream repository.
func isStaleError(err error) bool {
	msg := err.Error()
	return strings.Contains(msg, "not found in repository") ||
		strings.Contains(msg, "does not exist in repository")
}

// pruneSkill moves a stale skill to the trash directory.
func pruneSkill(skillPath, name string, uc *updateContext) error {
	var trashDir string
	if uc.isProject() {
		trashDir = trash.ProjectTrashDir(uc.projectRoot)
	} else {
		trashDir = trash.TrashDir()
	}
	_, err := trash.MoveToTrash(skillPath, name, trashDir)
	return err
}

// pruneRegistry removes pruned skill entries from the registry.
func pruneRegistry(prunedNames []string, uc *updateContext) {
	var regDir string
	if uc.isProject() {
		regDir = filepath.Join(uc.projectRoot, ".skillshare")
	} else {
		regDir = filepath.Dir(config.ConfigPath())
	}

	reg, err := config.LoadRegistry(regDir)
	if err != nil || len(reg.Skills) == 0 {
		return
	}

	removedSet := make(map[string]bool, len(prunedNames))
	for _, n := range prunedNames {
		removedSet[n] = true
	}

	updated := make([]config.SkillEntry, 0, len(reg.Skills))
	for _, s := range reg.Skills {
		if !removedSet[s.FullName()] {
			updated = append(updated, s)
		}
	}

	if len(updated) != len(reg.Skills) {
		reg.Skills = updated
		if saveErr := reg.Save(regDir); saveErr != nil {
			ui.Warning("Failed to update registry after prune: %v", saveErr)
		}
	}
}
