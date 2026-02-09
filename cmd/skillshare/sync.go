package main

import (
	"fmt"
	"os"
	"time"

	"skillshare/internal/backup"
	"skillshare/internal/config"
	"skillshare/internal/oplog"
	"skillshare/internal/sync"
	"skillshare/internal/trash"
	"skillshare/internal/ui"
	"skillshare/internal/utils"
)

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

	if mode == modeProject {
		err := cmdSyncProject(rest, cwd)
		logSyncOp(config.ProjectConfigPath(cwd), start, err)
		return err
	}

	dryRun := false
	force := false

	for _, arg := range rest {
		switch arg {
		case "--dry-run", "-n":
			dryRun = true
		case "--force", "-f":
			force = true
		}
	}

	cfg, err := config.Load()
	if err != nil {
		return err
	}

	// Ensure source exists
	if _, err := os.Stat(cfg.Source); os.IsNotExist(err) {
		return fmt.Errorf("source directory does not exist: %s", cfg.Source)
	}

	// Backup targets before sync (only if not dry-run)
	if !dryRun {
		backupTargetsBeforeSync(cfg)
	}

	// Check for name collisions before syncing
	discoveredSkills, discoverErr := sync.DiscoverSourceSkills(cfg.Source)
	if discoverErr == nil {
		collisions := sync.CheckNameCollisions(discoveredSkills)
		if len(collisions) > 0 {
			ui.Header("Name conflicts detected")
			for _, collision := range collisions {
				ui.Warning("Skill name '%s' is defined in multiple places:", collision.Name)
				for _, path := range collision.Paths {
					ui.Info("  - %s", path)
				}
			}
			ui.Info("CLI tools may not distinguish between them.")
			ui.Info("Suggestion: Rename one in SKILL.md (e.g., 'repo:skillname')")
			fmt.Println()
		}
	}

	ui.Header("Syncing skills")
	if dryRun {
		ui.Warning("Dry run mode - no changes will be made")
	}

	hasError := false
	for name, target := range cfg.Targets {
		if err := syncTarget(name, target, cfg, dryRun, force); err != nil {
			ui.Error("%s: %v", name, err)
			hasError = true
		}
	}

	var syncErr error
	if hasError {
		syncErr = fmt.Errorf("some targets failed to sync")
	}

	// Opportunistic cleanup of expired trash items
	if !dryRun {
		if n, _ := trash.Cleanup(trash.TrashDir(), 0); n > 0 {
			ui.Info("Cleaned up %d expired trash item(s)", n)
		}
	}

	logSyncOp(config.ConfigPath(), start, syncErr)
	return syncErr
}

func logSyncOp(cfgPath string, start time.Time, cmdErr error) {
	e := oplog.NewEntry("sync", statusFromErr(cmdErr), time.Since(start))
	if cmdErr != nil {
		e.Message = cmdErr.Error()
	}
	oplog.Write(cfgPath, oplog.OpsFile, e) //nolint:errcheck
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

	if mode == "merge" {
		return syncMergeMode(name, target, cfg.Source, dryRun, force)
	}

	return syncSymlinkMode(name, target, cfg.Source, dryRun, force)
}

func syncMergeMode(name string, target config.TargetConfig, source string, dryRun, force bool) error {
	result, err := sync.SyncTargetMerge(name, target, source, dryRun, force)
	if err != nil {
		return err
	}

	// Prune orphan links (skills that no longer exist in source)
	pruneResult, pruneErr := sync.PruneOrphanLinks(target.Path, source, dryRun)
	if pruneErr != nil {
		ui.Warning("%s: prune failed: %v", name, pruneErr)
	}

	// Report results
	linkedCount := len(result.Linked)
	updatedCount := len(result.Updated)
	skippedCount := len(result.Skipped)
	removedCount := 0
	if pruneResult != nil {
		removedCount = len(pruneResult.Removed)
	}

	if linkedCount > 0 || updatedCount > 0 || removedCount > 0 {
		ui.Success("%s: merged (%d linked, %d local, %d updated, %d pruned)",
			name, linkedCount, skippedCount, updatedCount, removedCount)
	} else if skippedCount > 0 {
		ui.Success("%s: merged (%d local skills preserved)", name, skippedCount)
	} else {
		ui.Success("%s: merged (no skills)", name)
	}

	// Show prune warnings
	if pruneResult != nil {
		for _, warn := range pruneResult.Warnings {
			ui.Warning("  %s", warn)
		}
	}

	return nil
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
