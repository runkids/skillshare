package main

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"skillshare/internal/audit"
	"skillshare/internal/install"
	"skillshare/internal/sourcefs"
	"skillshare/internal/trash"
	"skillshare/internal/ui"
)

// updateContext holds mode-specific configuration for update operations.
// projectRoot == "" means global mode.
type updateContext struct {
	sourcePath  string
	registryDir string // cached SourceRoot result for global mode
	projectRoot string
	opts        *updateOptions
	parseOpts   install.ParseOptions
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
		AuditOverride:  uc.opts.force,
		Update:         true,
		SkipAudit:      uc.opts.skipAudit,
		AuditThreshold: uc.opts.threshold,
		SourceDir:      uc.sourcePath,
	}
	if uc.isProject() {
		opts.AuditProjectRoot = uc.projectRoot
	}
	return opts
}

// executeBatchUpdate runs the 3-phase batch update loop shared by global and
// project modes. Returns combined updateResult and any security error.
func executeBatchUpdate(uc *updateContext, targets []updateTarget) (updateResult, error) {
	total := len(targets)
	start := time.Now()

	var result updateResult
	var auditEntries []batchAuditEntry
	var blockedEntries []batchBlockedEntry
	var staleNames []string
	var prunedNames []string
	var missingRepoNames []string
	var statusFailedEntries []batchBlockedEntry

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
			// Group by URL+branch: skills installed from a non-default branch
			// must be re-fetched from that branch, not the remote HEAD.
			key := urlBranchKey(t.meta.RepoURL, t.meta.Branch)
			repoGroups[key] = append(repoGroups[key], t)
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

	// Create a single progress bar up front — phase headers are rendered
	// inline via SetHeader so the bar never duplicates on screen.
	progressBar := ui.StartProgress("Updating skills", total)

	// Phase 1: tracked repos (git pull)
	if len(trackedRepos) > 0 {
		phaseCurrent++
		if phaseTotal > 1 {
			progressBar.SetHeader(ui.FormatPhaseHeader(phaseCurrent, phaseTotal, "Pulling %s...", plural(len(trackedRepos), "tracked repo")))
		}
	}
	for _, t := range trackedRepos {
		progressBar.UpdateTitle(fmt.Sprintf("Updating %s", t.name))
		// Tracked repo declared in metadata but clone dir is absent (issue #212):
		// surface it instead of silently skipping (mirrors the single-target path).
		if _, statErr := os.Stat(t.path); os.IsNotExist(statErr) {
			result.skipped++
			missingRepoNames = append(missingRepoNames, t.name)
			result.items = append(result.items, updateJSONItem{Name: t.name, Type: "repo", Status: "skipped", Error: missingTrackedRepoShortReason})
			progressBar.Increment()
			continue
		}
		updated, auditResult, err := updateTrackedRepoQuick(uc, t.path)
		var statusErr *gitStatusError
		if err != nil {
			if isSecurityError(err) {
				result.securityFailed++
				blockedEntries = append(blockedEntries, batchBlockedEntry{name: t.name, errMsg: err.Error()})
				result.items = append(result.items, updateJSONItem{Name: t.name, Type: "repo", Status: "security_blocked", Error: err.Error()})
			} else if errors.As(err, &statusErr) {
				statusFailedEntries = append(statusFailedEntries, batchBlockedEntry{name: t.name, errMsg: err.Error()})
				result.items = append(result.items, updateJSONItem{Name: t.name, Type: "repo", Status: "failed", Error: err.Error()})
			} else {
				result.skipped++
				result.items = append(result.items, updateJSONItem{Name: t.name, Type: "repo", Status: "skipped", Error: updateTrackedRepoErrorMessage(t, err)})
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
	result.missingTrackedRepos = missingRepoNames

	// Phase 2: grouped skills (one clone per repo)
	if len(repoGroups) > 0 {
		phaseCurrent++
		groupedCount := 0
		for _, g := range repoGroups {
			groupedCount += len(g)
		}
		if phaseTotal > 1 {
			progressBar.SetHeader(ui.FormatPhaseHeader(phaseCurrent, phaseTotal, "Updating %s from %s...", plural(groupedCount, "grouped skill"), plural(len(repoGroups), "repo")))
		}
	}
	for groupKey, groupTargets := range repoGroups {
		repoURL, branch := splitURLBranch(groupKey)
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
			if t.meta == nil {
				ui.Warning("Skipping %s: missing metadata", t.name)
				result.skipped++
				continue
			}
			subdir := t.meta.Subdir
			if subdir == "" {
				subdir = "."
			}
			skillTargetMap[subdir] = t.path
			pathToTarget[subdir] = t
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

		batchResult, err := install.UpdateSkillsFromRepo(repoURL, branch, skillTargetMap, batchOpts)
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

		// Non-TTY: increment progress bar for each skill in this group.
		// In TTY mode, handleGroupedBatchProgress already increments via OnProgress.
		if !ui.IsTTY() {
			for range skillTargetMap {
				progressBar.Increment()
			}
		}
	}

	// Phase 3: standalone skills
	if len(standaloneSkills) > 0 {
		phaseCurrent++
		if phaseTotal > 1 {
			progressBar.SetHeader(ui.FormatPhaseHeader(phaseCurrent, phaseTotal, "Updating %s...", plural(len(standaloneSkills), "standalone skill")))
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

	progressBar.Stop()

	// Registry cleanup for pruned skills
	if len(prunedNames) > 0 {
		pruneRegistry(prunedNames, uc)
	}

	// Render results
	if !uc.opts.dryRun {
		displayUpdateBlockedSection(blockedEntries)
		for _, e := range statusFailedEntries {
			ui.Error("%s: %s", e.name, e.errMsg)
		}
		displayPrunedSection(prunedNames)
		displayStaleWarning(staleNames)
		displayMissingTrackedReposWarning(missingRepoNames)
		displayUpdateAuditResults(auditEntries, uc.opts.auditVerbose)
		ui.UpdateSummary(ui.UpdateStats{
			Updated:        result.updated,
			Skipped:        result.skipped,
			Pruned:         result.pruned,
			SecurityFailed: result.securityFailed,
			Duration:       time.Since(start),
		})
	} else {
		repos := 0
		for _, t := range targets {
			if t.isRepo {
				repos++
				printUpdateRow(ui.MarkNone, t.name, "would run git pull", 0)
			} else {
				printUpdateRow(ui.MarkNone, t.name, "would reinstall", 0)
			}
		}
		var parts []string
		if repos > 0 {
			parts = append(parts, plural(repos, "tracked repo"))
		}
		if skills := total - repos; skills > 0 {
			parts = append(parts, plural(skills, "skill"))
		}
		fmt.Println()
		ui.Done(ui.MarkNone, "Would update "+strings.Join(parts, ", "), 0)
		ui.DryRun()
	}

	if (result.updated > 0 || result.pruned > 0) && !uc.opts.dryRun {
		ui.Next("skillshare sync", "link the changes into your targets")
	}

	var errs []error
	if result.securityFailed > 0 {
		errs = append(errs, fmt.Errorf("%d repo(s) blocked by security audit", result.securityFailed))
	}
	if len(statusFailedEntries) > 0 {
		errs = append(errs, fmt.Errorf("%d repo(s) failed to check git status", len(statusFailedEntries)))
	}
	return result, errors.Join(errs...)
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
	if err := sourcefs.CheckMoveOut(uc.sourcePath, skillPath); err != nil {
		return err
	}
	_, err := trash.MoveToTrash(skillPath, name, trashDir)
	return err
}

// pruneRegistry removes pruned skill entries from the metadata store.
func pruneRegistry(prunedNames []string, uc *updateContext) {
	store, err := install.LoadMetadata(uc.sourcePath)
	if err != nil {
		return
	}

	changed := false
	for _, name := range prunedNames {
		if store.Has(name) {
			store.Remove(name)
			changed = true
		}
	}

	if changed {
		if saveErr := store.Save(uc.sourcePath); saveErr != nil {
			ui.Warning("Failed to update metadata after prune: %v", saveErr)
		}
	}
}
