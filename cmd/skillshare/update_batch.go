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
	start := time.Now()
	fmt.Println()
	// Progress bar is started lazily — after the first phase header prints,
	// so the header appears above the bar rather than below it.
	var progressBar *ui.ProgressBar

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
		if t.meta != nil && t.meta.RepoURL != "" {
			repoGroups[t.meta.RepoURL] = append(repoGroups[t.meta.RepoURL], t)
		} else {
			standaloneSkills = append(standaloneSkills, t)
		}
	}

	// Count non-empty phases for dynamic numbering
	phaseTotal := 0
	if len(trackedRepos) > 0 {
		phaseTotal++
	}
	if len(repoGroups) > 0 {
		phaseTotal++
	}
	if len(standaloneSkills) > 0 {
		phaseTotal++
	}
	phaseCurrent := 0

	// Phase 1: tracked repos (git pull)
	if len(trackedRepos) > 0 {
		phaseCurrent++
		if phaseTotal > 1 {
			ui.PhaseHeader(phaseCurrent, phaseTotal, "Pulling %d tracked repo(s)...", len(trackedRepos))
		}
		if progressBar == nil {
			progressBar = ui.StartProgress("Updating skills", total)
		}
	}
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
	if len(repoGroups) > 0 {
		phaseCurrent++
		groupedCount := 0
		for _, g := range repoGroups {
			groupedCount += len(g)
		}
		if phaseTotal > 1 {
			ui.PhaseHeader(phaseCurrent, phaseTotal, "Updating %d grouped skill(s) from %d repo(s)...", groupedCount, len(repoGroups))
		}
		if progressBar == nil {
			progressBar = ui.StartProgress("Updating skills", total)
		}
	}
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
			if t.meta != nil {
				subdir := t.meta.Subdir
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
				if handleGroupedBatchProgress(progressBar, line) {
					return
				}
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
		}
	}

	// Phase 3: standalone skills
	if len(standaloneSkills) > 0 {
		phaseCurrent++
		if phaseTotal > 1 {
			ui.PhaseHeader(phaseCurrent, phaseTotal, "Updating %d standalone skill(s)...", len(standaloneSkills))
		}
		if progressBar == nil {
			progressBar = ui.StartProgress("Updating skills", total)
		}
	}
	for _, t := range standaloneSkills {
		progressBar.UpdateTitle(fmt.Sprintf("Updating %s", t.name))
		updated, installRes, err := updateSkillFromMeta(uc, t.path, t.meta)
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

	if progressBar != nil {
		progressBar.Stop()
	}

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
		ui.UpdateSummary(ui.UpdateStats{
			Updated:        result.updated,
			Skipped:        result.skipped,
			Pruned:         result.pruned,
			SecurityFailed: result.securityFailed,
			Duration:       time.Since(start),
		})
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

func handleGroupedBatchProgress(progressBar *ui.ProgressBar, line string) bool {
	if !strings.HasPrefix(line, install.BatchUpdateProgressPrefix) {
		return false
	}
	payload := strings.TrimSpace(strings.TrimPrefix(line, install.BatchUpdateProgressPrefix))
	if payload == "" {
		progressBar.UpdateTitle("Updating skills")
		progressBar.Increment()
		return true
	}

	parts := strings.SplitN(payload, " ", 2)
	if len(parts) == 2 && strings.TrimSpace(parts[1]) != "" {
		progressBar.UpdateTitle(fmt.Sprintf("Updating %s", strings.TrimSpace(parts[1])))
	} else {
		progressBar.UpdateTitle("Updating skills")
	}
	progressBar.Increment()
	return true
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
