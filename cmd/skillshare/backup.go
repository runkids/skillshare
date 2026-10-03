package main

import (
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"time"

	"skillshare/internal/backup"
	"skillshare/internal/config"
	"skillshare/internal/oplog"
	"skillshare/internal/theme"
	"skillshare/internal/trash"
	"skillshare/internal/ui"
	"skillshare/internal/utils"
)

func cmdBackup(args []string) error {
	mode, args, err := parseModeArgs(args)
	if err != nil {
		return err
	}

	if len(args) > 0 && args[0] == "files" {
		return cmdBackupFiles(mode, args[1:])
	}

	cwd, _ := os.Getwd()

	// --delete works on either mode's snapshot folders, so it is handled
	// before the project-mode restriction below.
	for i := 0; i < len(args); i++ {
		if args[i] == "--delete" {
			if i+1 >= len(args) || strings.HasPrefix(args[i+1], "-") {
				return fmt.Errorf("--delete requires a backup timestamp (see backup --list)")
			}
			applyModeLabel(mode)
			return backupDelete(mode, cwd, args[i+1], hasFlag(args, "--dry-run") || hasFlag(args, "-n"))
		}
	}

	// Extract kind filter (e.g. "skillshare backup agents" or "--all").
	kind, args := parseKindArgWithAll(args)

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

	if mode == modeAuto && kind == kindAgents && projectConfigExists(cwd) {
		mode = modeProject
	}
	applyModeLabel(mode)

	// Listing and cleanup work on the project's snapshots too (agents only).
	if doList || doCleanup {
		backupDir := backup.BackupDir()
		if mode == modeProject {
			backupDir = backup.ProjectBackupDir(cwd)
		}
		if doList {
			return backupList(backupDir)
		}
		// Retention limits come from the global config; project snapshots keep the defaults.
		retention := backup.DefaultCleanupConfig()
		if mode != modeProject {
			if cfg, err := config.Load(); err == nil {
				retention = backup.RetentionConfig(cfg)
			}
		}
		if dryRun {
			return backupCleanupDryRun(backupDir, retention)
		}
		return backupCleanup(backupDir, retention)
	}

	// Project mode is only supported for agents.
	if mode == modeProject && kind != kindAgents && kind != kindAll {
		return fmt.Errorf("backup is not supported in project mode (except for agents)")
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

	names := slices.Sorted(maps.Keys(targets))
	width := ui.RowWidth(names...)
	if dryRun {
		for _, name := range names {
			target := targets[name]
			if err := previewBackup(name, target.SkillsConfig().Path, width); err != nil {
				ui.Row(ui.MarkWarn, name, fmt.Sprintf("could not inspect: %v", err), width)
			}
		}
		fmt.Println()
		ui.DryRun()
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
	failed := 0
	for _, name := range names {
		spinner.Update(fmt.Sprintf("Backing up %s...", name))
		target := targets[name]
		backupPath, err := backup.Create(name, target.SkillsConfig().Path)
		if err != nil {
			results = append(results, backupResult{name: name, errMsg: err.Error()})
			failed++
			continue
		}
		if backupPath != "" {
			results = append(results, backupResult{name: name, backupPath: backupPath})
			created++
		} else {
			results = append(results, backupResult{name: name})
		}
	}
	spinner.Stop()

	for _, r := range results {
		if r.errMsg != "" {
			ui.Row(ui.MarkFail, r.name, r.errMsg, width)
		} else if r.backupPath != "" {
			ui.Row(ui.MarkOK, r.name, utils.FoldHomePath(r.backupPath), width)
		} else {
			ui.Row(ui.MarkNone, r.name, ui.DimText("nothing to back up (empty or symlink)"), width)
		}
	}

	fmt.Println()
	printBackupDone(created, failed, "", spinner.Started())
	return nil
}

// printBackupDone closes a backup run; what names the backed-up folders
// ("" for skills, "agents of " for agent backups).
func printBackupDone(created, failed int, what string, start time.Time) {
	switch {
	case failed > 0:
		ui.Done(ui.MarkFail, fmt.Sprintf("Backed up %s%s, %d failed", what, plural(created, "target"), failed), time.Since(start))
	case created == 0:
		ui.Done(ui.MarkNone, "Nothing to back up", 0)
	default:
		ui.Done(ui.MarkOK, fmt.Sprintf("Backed up %s%s", what, plural(created, "target")), time.Since(start))
		ui.Next("skillshare restore <target>", "roll a target back")
	}
}

func previewBackup(targetName, targetPath string, width int) error {
	backupDir := backup.BackupDir()
	if backupDir == "" {
		return fmt.Errorf("cannot determine backup directory: home directory not found")
	}

	info, err := os.Lstat(targetPath)
	if err != nil {
		if os.IsNotExist(err) {
			ui.Row(ui.MarkNone, targetName, ui.DimText("nothing to back up (missing)"), width)
			return nil
		}
		return err
	}

	if utils.IsLinkMode(targetPath, info.Mode()) {
		ui.Row(ui.MarkNone, targetName, ui.DimText("nothing to back up (symlink)"), width)
		return nil
	}

	entries, err := os.ReadDir(targetPath)
	if err != nil || len(entries) == 0 {
		ui.Row(ui.MarkNone, targetName, ui.DimText("nothing to back up (empty)"), width)
		return nil
	}

	timestamp := time.Now().Format("2006-01-02_15-04-05")
	backupPath := filepath.Join(backupDir, timestamp, targetName)
	ui.Row(ui.MarkNone, targetName, "would back up to "+utils.FoldHomePath(backupPath), width)

	return nil
}

func backupList(backupDir string) error {
	backups, err := backup.ListInDir(backupDir)
	if err != nil {
		return err
	}

	if len(backups) == 0 {
		ui.Done(ui.MarkNone, "No backups found", 0)
		return nil
	}

	fmt.Println(theme.Primary().Bold(true).Render("Backups") + "  " + utils.FoldHomePath(backupDir))
	width := ui.RowWidth(backups[0].Timestamp)
	for _, b := range backups {
		ui.Row(ui.MarkNone, b.Timestamp, strings.Join(b.Targets, ", ")+ui.DimText(" · "+formatBytes(backup.Size(b.Path))), width)
	}
	totalSize, _ := backup.TotalSizeInDir(backupDir)
	fmt.Println()
	ui.Done(ui.MarkNone, fmt.Sprintf("%s, %s", plural(len(backups), "backup"), formatBytes(totalSize)), 0)
	ui.Next("skillshare restore <target> --from <timestamp>", "roll a target back")
	return nil
}

// backupDelete removes one snapshot folder: the global one, or the project's
// (agents only) with -p.
func backupDelete(mode runMode, cwd, timestamp string, dryRun bool) error {
	start := time.Now()
	backupDir := backup.BackupDir()
	if mode == modeProject {
		backupDir = backup.ProjectBackupDir(cwd)
	}
	if !backup.ValidTimestamp(timestamp) {
		return fmt.Errorf("invalid backup timestamp %q (expected e.g. 2024-01-15_14-30-45)", timestamp)
	}
	info, err := backup.GetBackupByTimestampInDir(backupDir, timestamp)
	if err != nil {
		return err
	}
	size := backup.Size(info.Path)
	if dryRun {
		ui.Done(ui.MarkNone, "Would delete backup "+timestamp, 0)
		ui.Note(fmt.Sprintf("%s · %s", strings.Join(info.Targets, ", "), formatBytes(size)))
		fmt.Println()
		ui.DryRun()
		return nil
	}

	err = backup.DeleteInDir(backupDir, timestamp)
	e := oplog.NewEntry("backup", statusFromErr(err), time.Since(start))
	e.Args = map[string]any{"action": "delete", "timestamp": timestamp}
	if err != nil {
		e.Message = err.Error()
	}
	oplog.WriteWithLimit(config.ConfigPath(), oplog.OpsFile, e, logMaxEntries()) //nolint:errcheck
	if err != nil {
		return err
	}
	ui.Done(ui.MarkOK, fmt.Sprintf("Deleted backup %s, freed %s", timestamp, formatBytes(size)), time.Since(start))
	return nil
}

func backupCleanup(backupDir string, cfg backup.CleanupConfig) error {
	start := time.Now()
	backups, err := backup.ListInDir(backupDir)
	if err != nil {
		return err
	}

	if len(backups) == 0 {
		ui.Done(ui.MarkNone, "No backups to clean up", 0)
		return nil
	}

	totalSize, _ := backup.TotalSizeInDir(backupDir)
	removed, err := backup.CleanupInDir(backupDir, cfg)
	if err != nil {
		return err
	}

	if removed > 0 {
		newSize, _ := backup.TotalSizeInDir(backupDir)
		ui.Done(ui.MarkOK, fmt.Sprintf("Removed %s, freed %s", plural(removed, "old backup"), formatBytes(totalSize-newSize)), time.Since(start))
		ui.Note(fmt.Sprintf("%s left, %s", plural(len(backups)-removed, "backup"), formatBytes(newSize)))
	} else {
		ui.Done(ui.MarkNone, "No old backups to remove", 0)
		ui.Note(fmt.Sprintf("%s, %s", plural(len(backups), "backup"), formatBytes(totalSize)))
	}

	return nil
}

func backupCleanupDryRun(backupDir string, cfg backup.CleanupConfig) error {
	backups, err := backup.ListInDir(backupDir)
	if err != nil {
		return err
	}

	if len(backups) == 0 {
		ui.Done(ui.MarkNone, "No backups to clean up", 0)
		return nil
	}

	totalSize, _ := backup.TotalSizeInDir(backupDir)
	removed, freed := planBackupCleanup(backups, cfg, time.Now())
	if removed > 0 {
		ui.Done(ui.MarkNone, fmt.Sprintf("Would remove %s, freeing %s", plural(removed, "old backup"), formatBytes(freed)), 0)
	} else {
		ui.Done(ui.MarkNone, "No old backups to remove", 0)
	}
	ui.Note(fmt.Sprintf("%s, %s", plural(len(backups), "backup"), formatBytes(totalSize)))
	fmt.Println()
	ui.DryRun()
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
		if !shouldRemove && cfg.MaxSizeMB > 0 && i > 0 &&
			totalSize+size > cfg.MaxSizeMB*1024*1024 {
			shouldRemove = true
		}

		if shouldRemove {
			removed++
			removedSize += size
			continue
		}
		totalSize += size
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

	target, exists := cfg.Targets[targetName]
	if !exists {
		return fmt.Errorf("target '%s' not found in config", targetName)
	}

	opts := backup.RestoreOptions{Force: force}

	sc := target.SkillsConfig()
	if dryRun {
		if fromTimestamp != "" {
			return previewRestoreFromTimestamp(targetName, sc.Path, fromTimestamp, opts)
		}
		return previewRestoreFromLatest(targetName, sc.Path, opts)
	}

	var restoreErr error
	if fromTimestamp != "" {
		restoreErr = restoreFromTimestamp(targetName, sc.Path, fromTimestamp, opts)
	} else {
		restoreErr = restoreFromLatest(targetName, sc.Path, opts)
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
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	if !shouldLaunchTUI(noTUI, cfg) {
		return backupList(backup.BackupDir())
	}

	// Step 1: Source picker — Backup or Trash
	selected, err := runChecklistTUI(checklistConfig{
		title:        "Restore from",
		singleSelect: true,
		itemName:     "source",
		items: []checklistItemData{
			{label: "Backup", desc: "restore a target from a snapshot"},
			{label: "Trash", desc: "restore deleted skills"},
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
			ui.Done(ui.MarkNone, "No backups found", 0)
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
			ui.Done(ui.MarkNone, "Trash is empty", 0)
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
	ui.Done(ui.MarkOK, fmt.Sprintf("Restored %s from backup %s", targetName, timestamp), 0)
	return nil
}

func restoreFromLatest(targetName, targetPath string, opts backup.RestoreOptions) error {
	timestamp, err := backup.RestoreLatest(targetName, targetPath, opts)
	if err != nil {
		return err
	}
	ui.Done(ui.MarkOK, fmt.Sprintf("Restored %s from the latest backup, %s", targetName, timestamp), 0)
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
	ui.Done(ui.MarkOK, fmt.Sprintf("Restored %s from backup %s", targetName, timestamp), 0)
	return nil
}

func restoreFromLatestInDir(backupDir, targetName, targetPath string, opts backup.RestoreOptions) error {
	timestamp, err := backup.RestoreLatestInDir(backupDir, targetName, targetPath, opts)
	if err != nil {
		return err
	}
	ui.Done(ui.MarkOK, fmt.Sprintf("Restored %s from the latest backup, %s", targetName, timestamp), 0)
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

	ui.Done(ui.MarkNone, fmt.Sprintf("Would restore %s from backup %s", targetName, timestamp), 0)
	fmt.Println()
	ui.DryRun()
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

	ui.Done(ui.MarkNone, fmt.Sprintf("Would restore %s from the latest backup, %s", targetName, latest.Timestamp), 0)
	fmt.Println()
	ui.DryRun()
	return nil
}

func printBackupHelp() {
	printHelp("skillshare backup [agents] [target] [options]\n       skillshare backup files [list|show|restore] ...", "Create a snapshot of target skill directories.\nWithout arguments, backs up all targets.",
		helpGroup{title: "Arguments", rows: []helpRow{
			{"target", "Target name to backup (optional; backs up all if omitted)"},
		}},
		helpGroup{title: "Options", rows: []helpRow{
			{"--all", "Backup both skills and agents"},
			{"-p, --project", "Use project mode (.skillshare/backups/); agents only"},
			{"-g, --global", "Use global mode (default for skills)"},
			{"-l, --list", "List all existing backups"},
			{"-c, --cleanup", "Remove old backups based on retention policy"},
			{"--delete <ts>", "Delete one backup (timestamp from --list); with -p,\nfrom the project's .skillshare/backups/"},
			{"-n, --dry-run", "Preview what would be backed up, cleaned up, or deleted"},
			{"-t, --target <name>", "Specify target name (alternative to positional arg)"},
		}},
		helpExamples(
			helpRow{"skillshare backup", "Backup all targets"},
			helpRow{"skillshare backup claude", "Backup only claude"},
			helpRow{"skillshare backup --list", "List all backups"},
			helpRow{"skillshare backup --cleanup", "Remove old backups"},
			helpRow{"skillshare backup --cleanup --dry-run", "Preview cleanup"},
			helpRow{"skillshare backup --list -p", "List this project's (agents) backups"},
			helpRow{"skillshare backup --delete 2024-01-15_14-30-45", ""},
			helpRow{"skillshare backup files", "Files skillshare saved before rewriting them"},
			helpRow{"skillshare backup agents", "Backup all agent targets"},
			helpRow{"skillshare backup agents -p", "Backup project agent targets"},
			helpRow{"skillshare backup --all", "Backup skills + agents"},
		),
	)
}

func printRestoreHelp() {
	printHelp("skillshare restore [agents] [target] [options]", "Restore target skills from a backup snapshot.\nWithout arguments, launches an interactive TUI.",
		helpGroup{title: "Arguments", rows: []helpRow{
			{"target", "Target name to restore (optional)"},
		}},
		helpGroup{title: "Options", rows: []helpRow{
			{"--all", "Restore both skills and agents"},
			{"-p, --project", "Use project mode (.skillshare/backups/); agents only"},
			{"-g, --global", "Use global mode (default for skills)"},
			{"-f, --from <ts>", "Restore from specific timestamp (e.g. 2024-01-15_14-30-45)"},
			{"--force", "Overwrite non-empty target directory"},
			{"-n, --dry-run", "Preview what would be restored without making changes"},
			{"--no-tui", "Skip interactive TUI, show backup list instead"},
		}},
		helpExamples(
			helpRow{"skillshare restore", "Interactive TUI"},
			helpRow{"skillshare restore claude", "Restore claude from latest backup"},
			helpRow{"skillshare restore claude --from 2024-01-15_14-30-45", ""},
			helpRow{"skillshare restore claude --dry-run", "Preview restore"},
			helpRow{"skillshare restore --no-tui", "List backups (no TUI)"},
			helpRow{"skillshare restore agents claude", "Restore agents claude target"},
			helpRow{"skillshare restore agents claude -p", "Restore project agents"},
			helpRow{"skillshare restore --all claude", "Restore skills + agents"},
		),
	)
}
