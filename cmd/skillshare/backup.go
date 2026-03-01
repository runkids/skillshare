package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/pterm/pterm"
	"golang.org/x/term"

	"skillshare/internal/backup"
	"skillshare/internal/config"
	"skillshare/internal/oplog"
	"skillshare/internal/trash"
	"skillshare/internal/ui"
)

func cmdBackup(args []string) error {
	start := time.Now()
	var targetName string
	doList := false
	doCleanup := false
	dryRun := false

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--list", "-l":
			doList = true
		case "--cleanup", "-c":
			doCleanup = true
		case "--dry-run", "-n":
			dryRun = true
		case "--target", "-t":
			if i+1 < len(args) {
				targetName = args[i+1]
				i++
			}
		default:
			targetName = args[i]
		}
	}

	if doList {
		return backupList()
	}

	if doCleanup {
		if dryRun {
			return backupCleanupDryRun()
		}
		return backupCleanup()
	}

	err := createBackup(targetName, dryRun)

	if !dryRun {
		e := oplog.NewEntry("backup", statusFromErr(err), time.Since(start))
		if targetName != "" {
			e.Args = map[string]any{"target": targetName}
		}
		if err != nil {
			e.Message = err.Error()
		}
		oplog.WriteWithLimit(config.ConfigPath(), oplog.OpsFile, e, logMaxEntries()) //nolint:errcheck
	}

	return err
}

func createBackup(targetName string, dryRun bool) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	targets := cfg.Targets
	if targetName != "" {
		if t, exists := cfg.Targets[targetName]; exists {
			targets = map[string]config.TargetConfig{targetName: t}
		} else {
			return fmt.Errorf("target '%s' not found", targetName)
		}
	}

	ui.Header("Creating backup")
	if dryRun {
		ui.Warning("Dry run mode - no backups will be created")
		for name, target := range targets {
			if err := previewBackup(name, target.Path); err != nil {
				ui.Warning("Failed to inspect %s: %v", name, err)
			}
		}
		return nil
	}

	created := 0
	skipped := 0
	for name, target := range targets {
		backupPath, err := backup.Create(name, target.Path)
		if err != nil {
			ui.Warning("Failed to backup %s: %v", name, err)
			continue
		}
		if backupPath != "" {
			ui.StepDone(name, backupPath)
			created++
		} else {
			ui.StepSkip(name, "nothing to backup (empty or symlink)")
			skipped++
		}
	}

	ui.OperationSummary("Backup", 0,
		ui.Metric{Label: "created", Count: created, HighlightColor: pterm.Green},
		ui.Metric{Label: "skipped", Count: skipped, HighlightColor: pterm.Yellow},
	)

	// List recent backups
	backups, _ := backup.List()
	if len(backups) > 0 {
		fmt.Println()
		ui.Header("Recent backups")
		limit := min(5, len(backups))
		for i := 0; i < limit; i++ {
			b := backups[i]
			ui.ListItem("info", b.Timestamp, fmt.Sprintf("%s (%s)", strings.Join(b.Targets, ", "), b.Path))
		}
	}

	return nil
}

func previewBackup(targetName, targetPath string) error {
	backupDir := backup.BackupDir()
	if backupDir == "" {
		return fmt.Errorf("cannot determine backup directory: home directory not found")
	}

	info, err := os.Lstat(targetPath)
	if err != nil {
		if os.IsNotExist(err) {
			ui.StepSkip(targetName, "nothing to backup (missing)")
			return nil
		}
		return err
	}

	if info.Mode()&os.ModeSymlink != 0 {
		ui.StepSkip(targetName, "nothing to backup (symlink)")
		return nil
	}

	entries, err := os.ReadDir(targetPath)
	if err != nil || len(entries) == 0 {
		ui.StepSkip(targetName, "nothing to backup (empty)")
		return nil
	}

	timestamp := time.Now().Format("2006-01-02_15-04-05")
	backupPath := filepath.Join(backupDir, timestamp, targetName)
	ui.Info("%s: would backup to %s", targetName, backupPath)

	return nil
}

func backupList() error {
	backups, err := backup.List()
	if err != nil {
		return err
	}

	if len(backups) == 0 {
		ui.Info("No backups found")
		return nil
	}

	totalSize, _ := backup.TotalSize()
	ui.Header(fmt.Sprintf("All backups (%s total)", formatBytes(totalSize)))

	for _, b := range backups {
		size := backup.Size(b.Path)
		fmt.Printf("  %s  %-20s  %8s  %s\n",
			b.Timestamp,
			strings.Join(b.Targets, ", "),
			formatBytes(size),
			b.Path)
	}

	return nil
}

func backupCleanup() error {
	ui.Header("Cleaning up old backups")

	// Show current state
	backups, err := backup.List()
	if err != nil {
		return err
	}

	if len(backups) == 0 {
		ui.Info("No backups to clean up")
		return nil
	}

	totalSize, _ := backup.TotalSize()
	ui.Info("Current: %d backups, %s total", len(backups), formatBytes(totalSize))

	// Use default cleanup config
	cfg := backup.DefaultCleanupConfig()
	removed, err := backup.Cleanup(cfg)
	if err != nil {
		return err
	}

	if removed > 0 {
		newSize, _ := backup.TotalSize()
		ui.Success("Removed %d old backups (freed %s)",
			removed,
			formatBytes(totalSize-newSize))
	} else {
		ui.Info("No backups needed to be removed")
	}

	return nil
}

func backupCleanupDryRun() error {
	ui.Header("Cleaning up old backups")

	backups, err := backup.List()
	if err != nil {
		return err
	}

	if len(backups) == 0 {
		ui.Info("No backups to clean up")
		return nil
	}

	totalSize, _ := backup.TotalSize()
	ui.Info("Current: %d backups, %s total", len(backups), formatBytes(totalSize))

	cfg := backup.DefaultCleanupConfig()
	removed, freed := planBackupCleanup(backups, cfg, time.Now())
	if removed > 0 {
		ui.Warning("Dry run - would remove %d old backups (free %s)", removed, formatBytes(freed))
	} else {
		ui.Info("Dry run - no backups needed to be removed")
	}

	return nil
}

func planBackupCleanup(backups []backup.BackupInfo, cfg backup.CleanupConfig, now time.Time) (int, int64) {
	removed := 0
	var removedSize int64
	var totalSize int64

	for i, backupInfo := range backups {
		shouldRemove := false

		if cfg.MaxAge > 0 && now.Sub(backupInfo.Date) > cfg.MaxAge {
			shouldRemove = true
		}

		if cfg.MaxCount > 0 && i >= cfg.MaxCount {
			shouldRemove = true
		}

		size := backup.Size(backupInfo.Path)
		totalSize += size
		if cfg.MaxSizeMB > 0 && totalSize > cfg.MaxSizeMB*1024*1024 {
			shouldRemove = true
		}

		if shouldRemove {
			removed++
			removedSize += size
		}
	}

	return removed, removedSize
}

func cmdRestore(args []string) error {
	start := time.Now()

	var targetName string
	var fromTimestamp string
	force := false
	dryRun := false
	noTUI := false

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--from", "-f":
			if i+1 < len(args) {
				fromTimestamp = args[i+1]
				i++
			}
		case "--force":
			force = true
		case "--dry-run", "-n":
			dryRun = true
		case "--no-tui":
			noTUI = true
		default:
			if targetName == "" {
				targetName = args[i]
			}
		}
	}

	// No target specified → TUI dispatch (or plain text fallback)
	if targetName == "" && fromTimestamp == "" && !dryRun {
		return restoreTUIDispatch(noTUI)
	}

	// Original CLI path (with target name)
	if targetName == "" {
		return fmt.Errorf("usage: skillshare restore <target> [--from <timestamp>] [--force] [--dry-run]")
	}

	cfg, err := config.Load()
	if err != nil {
		return err
	}

	target, exists := cfg.Targets[targetName]
	if !exists {
		return fmt.Errorf("target '%s' not found in config", targetName)
	}

	ui.Header(fmt.Sprintf("Restoring %s", targetName))

	if dryRun {
		ui.Warning("Dry run mode - no changes will be made")
	}

	opts := backup.RestoreOptions{Force: force}

	if dryRun {
		if fromTimestamp != "" {
			return previewRestoreFromTimestamp(targetName, target.Path, fromTimestamp, opts)
		}
		return previewRestoreFromLatest(targetName, target.Path, opts)
	}

	var restoreErr error
	if fromTimestamp != "" {
		restoreErr = restoreFromTimestamp(targetName, target.Path, fromTimestamp, opts)
	} else {
		restoreErr = restoreFromLatest(targetName, target.Path, opts)
	}

	e := oplog.NewEntry("restore", statusFromErr(restoreErr), time.Since(start))
	e.Args = map[string]any{"target": targetName}
	if fromTimestamp != "" {
		e.Args["from"] = fromTimestamp
	}
	if restoreErr != nil {
		e.Message = restoreErr.Error()
	}
	oplog.WriteWithLimit(config.ConfigPath(), oplog.OpsFile, e, logMaxEntries()) //nolint:errcheck

	return restoreErr
}

// restoreTUIDispatch handles the no-args TUI flow for restore.
func restoreTUIDispatch(noTUI bool) error {
	isTTY := term.IsTerminal(int(os.Stdout.Fd()))
	if noTUI || !isTTY {
		return backupList()
	}

	cfg, err := config.Load()
	if err != nil {
		return err
	}

	// Step 1: Source picker — Backup or Trash
	selected, err := runChecklistTUI(checklistConfig{
		title:        "Restore — choose source",
		singleSelect: true,
		itemName:     "source",
		items: []checklistItemData{
			{label: "Backup Restore", desc: "Restore a target from backup snapshot"},
			{label: "Trash Restore", desc: "Restore deleted skills from trash"},
		},
	})
	if err != nil {
		return err
	}
	if selected == nil {
		return nil // cancelled
	}

	switch selected[0] {
	case 0: // Backup Restore
		backupDir := backup.BackupDir()
		summaries, err := backup.ListTargetsWithBackups(backupDir)
		if err != nil {
			return err
		}
		if len(summaries) == 0 {
			ui.Info("No backups found")
			return nil
		}
		return runRestoreTUI(summaries, backupDir, cfg.Targets, config.ConfigPath())

	case 1: // Trash Restore
		cwd, _ := os.Getwd()
		mode := modeGlobal
		if projectConfigExists(cwd) {
			mode = modeProject
		}
		trashBase := resolveTrashBase(mode, cwd)
		items := trash.List(trashBase)
		if len(items) == 0 {
			ui.Info("Trash is empty")
			return nil
		}
		modeLabel := "global"
		if mode == modeProject {
			modeLabel = "project"
		}
		cfgPath := resolveTrashCfgPath(mode, cwd)
		destDir, err := resolveSourceDir(mode, cwd)
		if err != nil {
			return err
		}
		return runTrashTUI(items, trashBase, destDir, cfgPath, modeLabel)
	}

	return nil
}

func restoreFromTimestamp(targetName, targetPath, timestamp string, opts backup.RestoreOptions) error {
	backupInfo, err := backup.GetBackupByTimestamp(timestamp)
	if err != nil {
		return err
	}

	if err := backup.RestoreToPath(backupInfo.Path, targetName, targetPath, opts); err != nil {
		return err
	}
	ui.Success("Restored %s from backup %s", targetName, timestamp)
	return nil
}

func restoreFromLatest(targetName, targetPath string, opts backup.RestoreOptions) error {
	timestamp, err := backup.RestoreLatest(targetName, targetPath, opts)
	if err != nil {
		return err
	}
	ui.Success("Restored %s from latest backup (%s)", targetName, timestamp)
	return nil
}

func previewRestoreFromTimestamp(targetName, targetPath, timestamp string, opts backup.RestoreOptions) error {
	backupInfo, err := backup.GetBackupByTimestamp(timestamp)
	if err != nil {
		return err
	}

	if err := backup.ValidateRestore(backupInfo.Path, targetName, targetPath, opts); err != nil {
		return err
	}

	ui.Info("Would restore %s from backup %s", targetName, timestamp)
	return nil
}

func previewRestoreFromLatest(targetName, targetPath string, opts backup.RestoreOptions) error {
	backups, err := backup.FindBackupsForTarget(targetName)
	if err != nil {
		return err
	}

	if len(backups) == 0 {
		return fmt.Errorf("no backup found for target '%s'", targetName)
	}

	latest := backups[0]
	if err := backup.ValidateRestore(latest.Path, targetName, targetPath, opts); err != nil {
		return err
	}

	ui.Info("Would restore %s from latest backup (%s)", targetName, latest.Timestamp)
	return nil
}
