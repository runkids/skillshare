package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	"skillshare/internal/backup"
	"skillshare/internal/config"
	"skillshare/internal/oplog"
	"skillshare/internal/sync"
	"skillshare/internal/trash"
	"skillshare/internal/ui"
	"skillshare/internal/utils"
)

type syncLogStats struct {
	Targets      int
	Failed       int
	DryRun       bool
	Force        bool
	ProjectScope bool
}

// syncModeStats aggregates per-target sync results for UI summary.
type syncModeStats struct {
	linked, local, updated, pruned int
}

func cmdSync(args []string) error {
	start := time.Now()

	mode, rest, err := parseModeArgs(args)
	if err != nil {
		return err
	}

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("cannot determine working directory: %w", err)
	}

	if mode == modeAuto {
		if projectConfigExists(cwd) {
			mode = modeProject
		} else {
			mode = modeGlobal
		}
	}

	applyModeLabel(mode)

	dryRun, force := parseSyncFlags(rest)

	if mode == modeProject {
		stats, err := cmdSyncProject(cwd, dryRun, force)
		stats.ProjectScope = true
		logSyncOp(config.ProjectConfigPath(cwd), stats, start, err)
		return err
	}

	cfg, err := config.Load()
	if err != nil {
		return err
	}

	// Ensure source exists
	if _, err := os.Stat(cfg.Source); os.IsNotExist(err) {
		return fmt.Errorf("source directory does not exist: %s", cfg.Source)
	}

	// Phase 1: Discovery — spinner
	spinner := ui.StartSpinner("Discovering skills")
	discoveredSkills, discoverErr := discoveryCache.Discover(cfg.Source)
	if discoverErr != nil {
		spinner.Fail("Discovery failed")
		return discoverErr
	}
	spinner.Success(fmt.Sprintf("Discovered %d skills", len(discoveredSkills)))
	reportCollisions(discoveredSkills, cfg.Targets)

	// Backup targets before sync (only if not dry-run and there are skills)
	if !dryRun && len(discoveredSkills) > 0 {
		fmt.Println()
		backupTargetsBeforeSync(cfg)
	}

	// Phase 2: Per-target sync (parallel)
	ui.Header("Syncing skills")
	if dryRun {
		ui.Warning("Dry run mode - no changes will be made")
	}

	globalMode := cfg.Mode
	if globalMode == "" {
		globalMode = "merge"
	}
	var entries []syncTargetEntry
	for name, target := range cfg.Targets {
		mode := target.Mode
		if mode == "" {
			mode = globalMode
		}
		entries = append(entries, syncTargetEntry{name: name, target: target, mode: mode})
	}

	results, failedTargets := runParallelSync(entries, cfg.Source, discoveredSkills, dryRun, force)

	var totals syncModeStats
	for _, r := range results {
		totals.linked += r.stats.linked
		totals.local += r.stats.local
		totals.updated += r.stats.updated
		totals.pruned += r.stats.pruned
	}

	var syncErr error
	if failedTargets > 0 {
		syncErr = fmt.Errorf("some targets failed to sync")
	}

	// Phase 3: Summary
	ui.SyncSummary(ui.SyncStats{
		Targets:  len(cfg.Targets),
		Linked:   totals.linked,
		Local:    totals.local,
		Updated:  totals.updated,
		Pruned:   totals.pruned,
		Duration: time.Since(start),
	})

	// Opportunistic cleanup of expired trash items
	if !dryRun {
		if n, _ := trash.Cleanup(trash.TrashDir(), 0); n > 0 {
			ui.Info("Cleaned up %d expired trash item(s)", n)
		}
	}

	logSyncOp(config.ConfigPath(), syncLogStats{
		Targets: len(cfg.Targets),
		Failed:  failedTargets,
		DryRun:  dryRun,
		Force:   force,
	}, start, syncErr)
	return syncErr
}

func parseSyncFlags(args []string) (dryRun bool, force bool) {
	for _, arg := range args {
		switch arg {
		case "--dry-run", "-n":
			dryRun = true
		case "--force", "-f":
			force = true
		}
	}
	return dryRun, force
}

func logSyncOp(cfgPath string, stats syncLogStats, start time.Time, cmdErr error) {
	status := statusFromErr(cmdErr)
	if stats.Failed > 0 && stats.Failed < stats.Targets {
		status = "partial"
	}
	e := oplog.NewEntry("sync", status, time.Since(start))
	e.Args = map[string]any{
		"targets_total":  stats.Targets,
		"targets_failed": stats.Failed,
		"dry_run":        stats.DryRun,
		"force":          stats.Force,
		"scope":          "global",
	}
	if stats.ProjectScope {
		e.Args["scope"] = "project"
	}
	if cmdErr != nil {
		e.Message = cmdErr.Error()
	}
	oplog.WriteWithLimit(cfgPath, oplog.OpsFile, e, logMaxEntries()) //nolint:errcheck
}

func backupTargetsBeforeSync(cfg *config.Config) {
	backedUp := false
	for name, target := range cfg.Targets {
		backupPath, err := backup.Create(name, target.Path)
		if err != nil {
			ui.Warning("Failed to backup %s: %v", name, err)
		} else if backupPath != "" {
			if !backedUp {
				ui.Header("Backing up")
				backedUp = true
			}
			ui.Success("%s -> %s", name, backupPath)
		}
	}
}

func syncTarget(name string, target config.TargetConfig, cfg *config.Config, dryRun, force bool) error {
	// Determine mode: target-specific > global > default
	mode := target.Mode
	if mode == "" {
		mode = cfg.Mode
	}
	if mode == "" {
		mode = "merge"
	}

	switch mode {
	case "merge":
		return syncMergeMode(name, target, cfg.Source, dryRun, force)
	case "copy":
		return syncCopyMode(name, target, cfg.Source, dryRun, force)
	default:
		return syncSymlinkMode(name, target, cfg.Source, dryRun, force)
	}
}

func syncTargetWithSkills(name string, target config.TargetConfig, cfg *config.Config, skills []sync.DiscoveredSkill, dryRun, force bool) error {
	_, err := syncTargetWithSkillsStats(name, target, cfg, skills, dryRun, force)
	return err
}

func syncTargetWithSkillsStats(name string, target config.TargetConfig, cfg *config.Config, skills []sync.DiscoveredSkill, dryRun, force bool) (syncModeStats, error) {
	mode := target.Mode
	if mode == "" {
		mode = cfg.Mode
	}
	if mode == "" {
		mode = "merge"
	}

	switch mode {
	case "merge":
		return syncMergeModeWithSkills(name, target, cfg.Source, skills, dryRun, force)
	case "copy":
		return syncCopyModeWithSkills(name, target, cfg.Source, skills, dryRun, force)
	default:
		err := syncSymlinkMode(name, target, cfg.Source, dryRun, force)
		return syncModeStats{}, err
	}
}

func syncMergeMode(name string, target config.TargetConfig, source string, dryRun, force bool) error {
	result, err := sync.SyncTargetMerge(name, target, source, dryRun, force)
	if err != nil {
		return err
	}

	pruneResult, pruneErr := sync.PruneOrphanLinks(target.Path, source, target.Include, target.Exclude, name, dryRun, force)
	if pruneErr != nil {
		ui.Warning("%s: prune failed: %v", name, pruneErr)
	}

	reportMergeResult(name, target, result, pruneResult)
	return nil
}

func syncMergeModeWithSkills(name string, target config.TargetConfig, source string, skills []sync.DiscoveredSkill, dryRun, force bool) (syncModeStats, error) {
	result, err := sync.SyncTargetMergeWithSkills(name, target, skills, source, dryRun, force)
	if err != nil {
		return syncModeStats{}, err
	}

	pruneResult, pruneErr := sync.PruneOrphanLinksWithSkills(sync.PruneOptions{
		TargetPath: target.Path, SourcePath: source, Skills: skills,
		Include: target.Include, Exclude: target.Exclude, TargetName: name,
		DryRun: dryRun, Force: force,
	})
	if pruneErr != nil {
		ui.Warning("%s: prune failed: %v", name, pruneErr)
	}

	reportMergeResult(name, target, result, pruneResult)
	return mergeStats(result, pruneResult), nil
}

func reportMergeResult(name string, target config.TargetConfig, result *sync.MergeResult, pruneResult *sync.PruneResult) {
	linkedCount := len(result.Linked)
	updatedCount := len(result.Updated)
	skippedCount := len(result.Skipped)
	removedCount := 0
	if pruneResult != nil {
		removedCount = len(pruneResult.Removed)
		skippedCount += len(pruneResult.LocalDirs)
	}

	if linkedCount > 0 || updatedCount > 0 || removedCount > 0 {
		ui.Success("%s: merged (%d linked, %d local, %d updated, %d pruned)",
			name, linkedCount, skippedCount, updatedCount, removedCount)
	} else if skippedCount > 0 {
		ui.Success("%s: merged (%d local skills preserved)", name, skippedCount)
	} else {
		ui.Success("%s: merged (no skills)", name)
	}

	if len(target.Include) > 0 {
		ui.Info("  include: %s", strings.Join(target.Include, ", "))
	}
	if len(target.Exclude) > 0 {
		ui.Info("  exclude: %s", strings.Join(target.Exclude, ", "))
	}

	if pruneResult != nil {
		for _, warn := range pruneResult.Warnings {
			ui.Warning("  %s", warn)
		}
	}
}

func syncCopyMode(name string, target config.TargetConfig, source string, dryRun, force bool) error {
	result, err := sync.SyncTargetCopy(name, target, source, dryRun, force)
	if err != nil {
		return err
	}

	pruneResult, pruneErr := sync.PruneOrphanCopies(target.Path, source, target.Include, target.Exclude, name, dryRun)
	if pruneErr != nil {
		ui.Warning("%s: prune failed: %v", name, pruneErr)
	}

	reportCopyResult(name, target, result, pruneResult)
	return nil
}

func syncCopyModeWithSkills(name string, target config.TargetConfig, source string, skills []sync.DiscoveredSkill, dryRun, force bool) (syncModeStats, error) {
	// Copy mode is slow (checksum + file copy per skill) — show a spinner with progress
	spinner := ui.StartSpinner(fmt.Sprintf("%s: copying skills", name))
	onProgress := func(cur, total int, skill string) {
		spinner.Update(fmt.Sprintf("%s: %d/%d %s", name, cur, total, skill))
	}

	result, err := sync.SyncTargetCopyWithSkills(name, target, skills, source, dryRun, force, onProgress)
	if err != nil {
		spinner.Fail(fmt.Sprintf("%s: copy failed", name))
		return syncModeStats{}, err
	}
	spinner.Stop()

	pruneResult, pruneErr := sync.PruneOrphanCopiesWithSkills(target.Path, skills, target.Include, target.Exclude, name, dryRun)
	if pruneErr != nil {
		ui.Warning("%s: prune failed: %v", name, pruneErr)
	}

	reportCopyResult(name, target, result, pruneResult)
	return copyStats(result, pruneResult), nil
}

func mergeStats(result *sync.MergeResult, prune *sync.PruneResult) syncModeStats {
	s := syncModeStats{
		linked:  len(result.Linked),
		local:   len(result.Skipped),
		updated: len(result.Updated),
	}
	if prune != nil {
		s.pruned = len(prune.Removed)
		s.local += len(prune.LocalDirs)
	}
	return s
}

func copyStats(result *sync.CopyResult, prune *sync.PruneResult) syncModeStats {
	s := syncModeStats{
		linked:  len(result.Copied),
		local:   len(result.Skipped),
		updated: len(result.Updated),
	}
	if prune != nil {
		s.pruned = len(prune.Removed)
	}
	return s
}

func reportCopyResult(name string, target config.TargetConfig, result *sync.CopyResult, pruneResult *sync.PruneResult) {
	copiedCount := len(result.Copied)
	updatedCount := len(result.Updated)
	skippedCount := len(result.Skipped)
	removedCount := 0
	if pruneResult != nil {
		removedCount = len(pruneResult.Removed)
	}

	if copiedCount > 0 || updatedCount > 0 || removedCount > 0 {
		ui.Success("%s: copied (%d new, %d skipped, %d updated, %d pruned)",
			name, copiedCount, skippedCount, updatedCount, removedCount)
	} else if skippedCount > 0 {
		ui.Success("%s: copied (%d skipped, up to date)", name, skippedCount)
	} else {
		ui.Success("%s: copied (no skills)", name)
	}

	if len(target.Include) > 0 {
		ui.Info("  include: %s", strings.Join(target.Include, ", "))
	}
	if len(target.Exclude) > 0 {
		ui.Info("  exclude: %s", strings.Join(target.Exclude, ", "))
	}

	if pruneResult != nil {
		for _, warn := range pruneResult.Warnings {
			ui.Warning("  %s", warn)
		}
	}
}

func reportCollisions(skills []sync.DiscoveredSkill, targets map[string]config.TargetConfig) {
	global, perTarget := sync.CheckNameCollisionsForTargets(skills, targets)
	if len(global) == 0 {
		return
	}

	if len(perTarget) > 0 {
		// Real per-target collisions — actionable warning
		ui.Header("Name conflicts detected")
		for _, tc := range perTarget {
			for _, c := range tc.Collisions {
				ui.Warning("Target '%s': skill name '%s' is defined in multiple places:", tc.TargetName, c.Name)
				for _, p := range c.Paths {
					ui.Info("  - %s", p)
				}
			}
		}
		ui.Info("Rename one in SKILL.md or adjust include/exclude filters")
		fmt.Println()
	} else {
		// Global collision exists but filters isolate them — single summary line
		fmt.Println()
		ui.Info("%d duplicate skill names (isolated by target filters)", len(global))
	}
}

func syncSymlinkMode(name string, target config.TargetConfig, source string, dryRun, force bool) error {
	status := sync.CheckStatus(target.Path, source)

	// Handle conflicts
	if status == sync.StatusConflict && !force {
		link, err := utils.ResolveLinkTarget(target.Path)
		if err != nil {
			link = "(unable to resolve target)"
		}
		return fmt.Errorf("conflict - symlink points to %s (use --force to override)", link)
	}

	if status == sync.StatusConflict && force {
		if !dryRun {
			os.Remove(target.Path)
		}
	}

	if err := sync.SyncTarget(name, target, source, dryRun); err != nil {
		return err
	}

	switch status {
	case sync.StatusLinked:
		ui.Success("%s: already linked", name)
	case sync.StatusNotExist:
		ui.Success("%s: symlink created", name)
		ui.Warning("  Symlink mode: deleting files in %s will delete from source!", target.Path)
		ui.Info("  Use 'skillshare target remove %s' to safely unlink", name)
	case sync.StatusHasFiles:
		ui.Success("%s: files migrated and linked", name)
		ui.Warning("  Symlink mode: deleting files in %s will delete from source!", target.Path)
		ui.Info("  Use 'skillshare target remove %s' to safely unlink", name)
	case sync.StatusBroken:
		ui.Success("%s: broken link fixed", name)
	case sync.StatusConflict:
		ui.Success("%s: conflict resolved (forced)", name)
	}

	return nil
}
