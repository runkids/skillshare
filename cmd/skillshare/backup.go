package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/pterm/pterm"

	"skillshare/internal/backup"
	"skillshare/internal/config"
	"skillshare/internal/oplog"
	"skillshare/internal/trash"
	"skillshare/internal/ui"
)

func cmdBackup(args []string) error {
	mode, args, err := parseModeArgs(args)
	if err != nil {
		return err
	}

	// Extract kind filter (e.g. "skillshare backup agents" or "--all").
	kind, args := parseKindArgWithAll(args)

	// Project mode is only supported for agents.
	if mode == modeProject && kind != kindAgents && kind != kindAll {
		return fmt.Errorf("backup is not supported in project mode (except for agents)")
	}

	cwd, _ := os.Getwd()
	if mode == modeAuto && kind == kindAgents && projectConfigExists(cwd) {
		mode = modeProject
	}
	applyModeLabel(mode)

	start := time.Now()
	var targetName string
	doList := false
	doCleanup := false
	dryRun := false

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--help", "-h":
			printBackupHelp()
			return nil
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

	if kind == kindAgents {
		err = createAgentBackup(mode, cwd, targetName, dryRun)
	} else {
		err = createBackup(targetName, dryRun)
	}

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
			if err := previewBackup(name, target.SkillsConfig().Path); err != nil {
				ui.Warning("Failed to inspect %s: %v", name, err)
			}
		}
		return nil
	}

	type backupResult struct {
		name       string
		backupPath string
		errMsg     string
	}

	spinner := ui.StartSpinner("Backing up targets...")
	var results []backupResult
	created := 0
	skipped := 0
	for name, target := range targets {
		spinner.Update(fmt.Sprintf("Backing up %s...", name))
		backupPath, err := backup.Create(name, target.SkillsConfig().Path)
		if err != nil {
			results = append(results, backupResult{name: name, errMsg: err.Error()})
			continue
		}
		if backupPath != "" {
			results = append(results, backupResult{name: name, backupPath: backupPath})
			created++
		} else {
			results = append(results, backupResult{name: name})
			skipped++
		}
	}
	spinner.Stop()

	for _, r := range results {
		if r.errMsg != "" {
			ui.Warning("Failed to backup %s: %s", r.name, r.errMsg)
		} else if r.backupPath != "" {
			ui.StepDone(r.name, r.backupPath)
		} else {
			ui.StepSkip(r.name, "nothing to backup (empty or symlink)")
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
			detail := fmt.Sprintf("%s (%s)", strings.Join(b.Targets, ", "), b.Path)
			arrow := pterm.NewStyle(pterm.FgCyan).Sprint("→")
			fmt.Printf("%s %-20s %s\n", arrow, b.Timestamp, ui.DimText(detail))
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

	timestamp := backup.NewTimestamp()
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
	mode, args, err := parseModeArgs(args)
	if err != nil {
		return err
	}

	// Extract kind filter (e.g. "skillshare restore agents").
	// Extract kind filter (e.g. "skillshare restore agents" or "--all").
	kind, args := parseKindArgWithAll(args)

	// Project mode is only supported for agents.
	if mode == modeProject && kind != kindAgents && kind != kindAll {
		return fmt.Errorf("restore is not supported in project mode (except for agents)")
	}

	cwd, _ := os.Getwd()
	if mode == modeAuto && kind == kindAgents && projectConfigExists(cwd) {
		mode = modeProject
	}
	applyModeLabel(mode)

	start := time.Now()
	_ = start // used below

	var targetName string
	var fromTimestamp string
	force := false
	dryRun := false
	noTUI := false

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--help", "-h":
			printRestoreHelp()
			return nil
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

	// Preserve the legacy "restore agents" alias for the canonical shared target
	// in global mode when no explicit agent target was requested.
	if kind == kindAgents && targetName == "" && mode != modeProject {
		cfg, cfgErr := config.Load()
		if cfgErr == nil {
			if _, _, resolveErr := resolveConfiguredRestoreTarget(cfg.Targets, "agents"); resolveErr == nil {
				kind = kindSkills
				targetName = "agents"
			}
		}
	}

	// Agent restore uses agent-specific backup entries (name suffixed with "-agents")
	if kind == kindAgents {
		return restoreAgentBackup(mode, cwd, targetName, fromTimestamp, force, dryRun)
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

	resolvedTargetName, target, err := resolveConfiguredRestoreTarget(cfg.Targets, targetName)
	if err != nil {
		return err
	}
	sc := target.SkillsConfig()
	if sc.Path == "" {
		return fmt.Errorf("target '%s' not found in config", targetName)
	}

	ui.Header(fmt.Sprintf("Restoring %s", targetName))

	if dryRun {
		ui.Warning("Dry run mode - no changes will be made")
	}

	opts := backup.RestoreOptions{Force: force}
	if dryRun {
		if fromTimestamp != "" {
			return previewRestoreFromTimestamp(resolvedTargetName, sc.Path, fromTimestamp, opts)
		}
		return previewRestoreFromLatest(resolvedTargetName, sc.Path, opts)
	}

	var restoreErr error
	if fromTimestamp != "" {
		restoreErr = restoreFromTimestamp(resolvedTargetName, sc.Path, fromTimestamp, opts)
	} else {
		restoreErr = restoreFromLatest(resolvedTargetName, sc.Path, opts)
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

func resolveConfiguredRestoreTarget(targets map[string]config.TargetConfig, requested string) (string, config.TargetConfig, error) {
	candidates := make([]string, 0, len(targets))
	for name := range targets {
		candidates = append(candidates, name)
	}

	resolvedName, ok, err := config.ResolveTargetNameCandidate(requested, candidates)
	if err != nil {
		return "", config.TargetConfig{}, err
	}
	if !ok {
		return "", config.TargetConfig{}, nil
	}
	return resolvedName, targets[resolvedName], nil
}

// restoreTUIDispatch handles the no-args TUI flow for restore.
func restoreTUIDispatch(noTUI bool) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	if !shouldLaunchTUI(noTUI, cfg) {
		return backupList()
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
		skillTrashBase := resolveTrashBase(mode, cwd, kindSkills)
		agentTrashBase := resolveTrashBase(mode, cwd, kindAgents)

		// Merge skill + agent trash
		var items []trash.TrashEntry
		for _, e := range trash.List(skillTrashBase) {
			e.Kind = "skill"
			items = append(items, e)
		}
		for _, e := range trash.List(agentTrashBase) {
			e.Kind = "agent"
			items = append(items, e)
		}
		if len(items) == 0 {
			ui.Info("Trash is empty")
			return nil
		}
		sort.Slice(items, func(i, j int) bool {
			return items[i].Date.After(items[j].Date)
		})

		modeLabel := "global"
		if mode == modeProject {
			modeLabel = "project"
		}
		cfgPath := resolveTrashCfgPath(mode, cwd)
		destDir, err := resolveSourceDir(mode, cwd, kindSkills)
		if err != nil {
			return err
		}
		agentDestDir, err := resolveSourceDir(mode, cwd, kindAgents)
		if err != nil {
			return err
		}
		return runTrashTUI(items, skillTrashBase, agentTrashBase, destDir, agentDestDir, cfgPath, modeLabel)
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

func restoreFromTimestampInDir(backupDir, targetName, targetPath, timestamp string, opts backup.RestoreOptions) error {
	backupInfo, err := backup.GetBackupByTimestampInDir(backupDir, timestamp)
	if err != nil {
		return err
	}

	if err := backup.RestoreToPath(backupInfo.Path, targetName, targetPath, opts); err != nil {
		return err
	}
	ui.Success("Restored %s from backup %s", targetName, timestamp)
	return nil
}

func restoreFromLatestInDir(backupDir, targetName, targetPath string, opts backup.RestoreOptions) error {
	timestamp, err := backup.RestoreLatestInDir(backupDir, targetName, targetPath, opts)
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

func printBackupHelp() {
	fmt.Println(`Usage: skillshare backup [agents] [target] [options]

Create a snapshot of target skill directories.
Without arguments, backs up all targets.

Arguments:
  target               Target name to backup (optional; backs up all if omitted)

Options:
  --all                Backup both skills and agents
  --project, -p        Use project mode (.skillshare/backups/); agents only
  --global, -g         Use global mode (default for skills)
  --list, -l           List all existing backups
  --cleanup, -c        Remove old backups based on retention policy
  --dry-run, -n        Preview what would be backed up or cleaned up
  --target, -t <name>  Specify target name (alternative to positional arg)
  --help, -h           Show this help

Examples:
  skillshare backup                         # Backup all targets
  skillshare backup claude                  # Backup only claude
  skillshare backup --list                  # List all backups
  skillshare backup --cleanup               # Remove old backups
  skillshare backup --cleanup --dry-run     # Preview cleanup
  skillshare backup agents                  # Backup all agent targets
  skillshare backup agents -p               # Backup project agent targets
  skillshare backup --all                   # Backup skills + agents`)
}

func printRestoreHelp() {
	fmt.Println(`Usage: skillshare restore [agents] [target] [options]

Restore target skills from a backup snapshot.
Without arguments, launches an interactive TUI.

Arguments:
  target               Target name to restore (optional)

Options:
  --all                Restore both skills and agents
  --project, -p        Use project mode (.skillshare/backups/); agents only
  --global, -g         Use global mode (default for skills)
  --from, -f <ts>      Restore from specific timestamp (e.g. 2024-01-15_14-30-45)
  --force              Overwrite non-empty target directory
  --dry-run, -n        Preview what would be restored without making changes
  --no-tui             Skip interactive TUI, show backup list instead
  --help, -h           Show this help

Examples:
  skillshare restore                        # Interactive TUI
  skillshare restore claude                 # Restore claude from latest backup
  skillshare restore claude --from 2024-01-15_14-30-45
  skillshare restore claude --dry-run       # Preview restore
  skillshare restore --no-tui               # List backups (no TUI)
  skillshare restore agents claude          # Restore agents claude target
  skillshare restore agents claude -p       # Restore project agents
  skillshare restore --all claude           # Restore skills + agents`)
}
