package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"skillshare/internal/backup"
	"skillshare/internal/config"
	"skillshare/internal/sync"
	"skillshare/internal/ui"
)

var version = "dev"

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	cmd := os.Args[1]
	args := os.Args[2:]

	var err error
	switch cmd {
	case "init":
		err = cmdInit(args)
	case "sync":
		err = cmdSync(args)
	case "status":
		err = cmdStatus(args)
	case "diff":
		err = cmdDiff(args)
	case "backup":
		err = cmdBackup(args)
	case "doctor":
		err = cmdDoctor(args)
	case "target":
		err = cmdTarget(args)
	case "version", "-v", "--version":
		fmt.Printf("skillshare %s\n", version)
	case "help", "-h", "--help":
		printUsage()
	default:
		ui.Error("Unknown command: %s", cmd)
		printUsage()
		os.Exit(1)
	}

	if err != nil {
		ui.Error("%v", err)
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println(`skillshare - Share skills across AI CLI tools

Usage:
  skillshare <command> [options]

Commands:
  init [--source PATH]       Initialize skillshare with a source directory
  sync [--dry-run] [--force] Sync skills to all targets
  status                     Show status of all targets
  diff [--target name]       Show differences between source and targets
  backup [--target name]     Create backup of target(s)
  doctor                     Check environment and diagnose issues
  target <name>              Show target info
  target <name> --mode MODE  Set target sync mode (merge|symlink)
  target add <name> <path>   Add a target
  target remove <name>       Unlink target and restore skills
  target remove --all        Unlink all targets
  target list                List all targets
  version                    Show version
  help                       Show this help

Examples:
  skillshare init --source ~/.skills
  skillshare target add claude ~/.claude/skills
  skillshare sync
  skillshare status`)
}

func cmdInit(args []string) error {
	home, _ := os.UserHomeDir()
	sourcePath := "" // Will be determined

	// Parse args
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--source", "-s":
			if i+1 >= len(args) {
				return fmt.Errorf("--source requires a path argument")
			}
			sourcePath = args[i+1]
			i++
		}
	}

	// Expand ~ in path
	if len(sourcePath) > 0 && sourcePath[0] == '~' {
		sourcePath = filepath.Join(home, sourcePath[1:])
	}

	// Check if already initialized
	if _, err := os.Stat(config.ConfigPath()); err == nil {
		return fmt.Errorf("already initialized. Config at: %s", config.ConfigPath())
	}

	// Detect existing CLI skills directories
	ui.Header("Detecting CLI skills directories")
	defaultTargets := config.DefaultTargets()

	type detectedDir struct {
		name       string
		path       string
		skillCount int
		hasSkills  bool
		exists     bool // true if skills dir exists, false if only parent exists
	}
	var detected []detectedDir

	for name, target := range defaultTargets {
		if info, err := os.Stat(target.Path); err == nil && info.IsDir() {
			// Skills directory exists - count skills
			entries, _ := os.ReadDir(target.Path)
			skillCount := 0
			for _, e := range entries {
				if e.IsDir() && e.Name()[0] != '.' {
					skillCount++
				}
			}
			detected = append(detected, detectedDir{
				name:       name,
				path:       target.Path,
				skillCount: skillCount,
				hasSkills:  skillCount > 0,
				exists:     true,
			})
			if skillCount > 0 {
				ui.Success("Found: %s (%d skills)", name, skillCount)
			} else {
				ui.Info("Found: %s (empty)", name)
			}
		} else {
			// Skills directory doesn't exist - check if parent exists (CLI installed)
			parent := filepath.Dir(target.Path)
			if _, err := os.Stat(parent); err == nil {
				detected = append(detected, detectedDir{
					name:       name,
					path:       target.Path,
					skillCount: 0,
					hasSkills:  false,
					exists:     false,
				})
				ui.Info("Found: %s (not initialized)", name)
			}
		}
	}

	// Default source path (same location as config)
	if sourcePath == "" {
		sourcePath = filepath.Join(home, ".config", "skillshare", "skills")
	}

	// Find directories with skills to potentially copy from
	var withSkills []detectedDir
	for _, d := range detected {
		if d.hasSkills {
			withSkills = append(withSkills, d)
		}
	}

	// Ask user if they want to initialize from existing skills
	var copyFrom string
	var copyFromName string
	if len(withSkills) > 0 {
		ui.Header("Initialize from existing skills?")
		fmt.Println("  Copy skills from an existing directory to the shared source?")
		fmt.Println()

		for i, d := range withSkills {
			fmt.Printf("  [%d] Copy from %s (%d skills)\n", i+1, d.name, d.skillCount)
		}
		fmt.Printf("  [%d] Start fresh (empty source)\n", len(withSkills)+1)
		fmt.Println()

		fmt.Print("  Enter choice [1]: ")
		var input string
		fmt.Scanln(&input)

		choice := 1
		if input != "" {
			fmt.Sscanf(input, "%d", &choice)
		}

		if choice >= 1 && choice <= len(withSkills) {
			copyFrom = withSkills[choice-1].path
			copyFromName = withSkills[choice-1].name
			ui.Success("Will copy skills from %s", copyFromName)
		} else {
			ui.Info("Starting with empty source")
		}
	}

	// Create source directory if needed
	if err := os.MkdirAll(sourcePath, 0755); err != nil {
		return fmt.Errorf("failed to create source directory: %w", err)
	}

	// Copy skills from selected directory
	if copyFrom != "" {
		ui.Info("Copying skills to %s...", sourcePath)
		entries, _ := os.ReadDir(copyFrom)
		copied := 0
		for _, entry := range entries {
			if !entry.IsDir() || entry.Name()[0] == '.' {
				continue
			}
			srcPath := filepath.Join(copyFrom, entry.Name())
			dstPath := filepath.Join(sourcePath, entry.Name())

			// Skip if already exists
			if _, err := os.Stat(dstPath); err == nil {
				continue
			}

			// Copy directory
			if err := copyDir(srcPath, dstPath); err != nil {
				ui.Warning("Failed to copy %s: %v", entry.Name(), err)
				continue
			}
			copied++
		}
		ui.Success("Copied %d skills to source", copied)
	}

	// Build targets list - only add the directory user chose to copy from
	targets := make(map[string]config.TargetConfig)
	if copyFromName != "" {
		targets[copyFromName] = config.TargetConfig{Path: copyFrom}
	}

	// Find other available targets (detected directories)
	var otherTargets []string
	for _, d := range detected {
		if d.name == copyFromName {
			continue // Already added
		}
		otherTargets = append(otherTargets, d.name)
	}

	// Ask if user wants to add other targets
	if len(otherTargets) > 0 {
		ui.Header("Add other CLI targets?")
		fmt.Println("  Other CLI tools detected on your system:")
		for _, name := range otherTargets {
			fmt.Printf("    - %s\n", name)
		}
		fmt.Println()
		fmt.Print("  Add these targets? [Y/n]: ")
		var input string
		fmt.Scanln(&input)
		input = strings.ToLower(strings.TrimSpace(input))

		if input == "" || input == "y" || input == "yes" {
			for _, name := range otherTargets {
				targets[name] = defaultTargets[name]
			}
			ui.Success("Added %d additional targets", len(otherTargets))
		} else {
			ui.Info("Skipped additional targets")
		}
	}

	if len(targets) == 0 {
		ui.Warning("No CLI skills directories detected.")
	}

	// Create config
	cfg := &config.Config{
		Source:  sourcePath,
		Mode:    "merge",
		Targets: targets,
		Ignore: []string{
			"**/.DS_Store",
			"**/.git/**",
		},
	}

	if err := cfg.Save(); err != nil {
		return err
	}

	ui.Header("Initialized successfully")
	ui.Success("Source: %s", sourcePath)
	ui.Success("Config: %s", config.ConfigPath())
	ui.Info("Run 'skillshare sync' to sync your skills")

	return nil
}

func cmdSync(args []string) error {
	dryRun := false
	force := false

	for _, arg := range args {
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

	ui.Header("Syncing skills")
	if dryRun {
		ui.Warning("Dry run mode - no changes will be made")
	}

	hasError := false
	for name, target := range cfg.Targets {
		// Determine mode: target-specific > global > default
		mode := target.Mode
		if mode == "" {
			mode = cfg.Mode
		}
		if mode == "" {
			mode = "merge"
		}

		if mode == "merge" {
			// Merge mode: create individual skill symlinks
			result, err := sync.SyncTargetMerge(name, target, cfg.Source, dryRun)
			if err != nil {
				ui.Error("%s: %v", name, err)
				hasError = true
				continue
			}

			if len(result.Linked) > 0 || len(result.Updated) > 0 {
				ui.Success("%s: merged (%d linked, %d local, %d updated)",
					name, len(result.Linked), len(result.Skipped), len(result.Updated))
			} else if len(result.Skipped) > 0 {
				ui.Success("%s: merged (%d local skills preserved)", name, len(result.Skipped))
			} else {
				ui.Success("%s: merged (no skills)", name)
			}
			continue
		}

		// Symlink mode (default)
		status := sync.CheckStatus(target.Path, cfg.Source)

		// Handle conflicts
		if status == sync.StatusConflict && !force {
			link, _ := os.Readlink(target.Path)
			ui.Error("%s: conflict - symlink points to %s (use --force to override)", name, link)
			hasError = true
			continue
		}

		if status == sync.StatusConflict && force {
			if !dryRun {
				os.Remove(target.Path)
			}
		}

		if err := sync.SyncTarget(name, target, cfg.Source, dryRun); err != nil {
			ui.Error("%s: %v", name, err)
			hasError = true
			continue
		}

		switch status {
		case sync.StatusLinked:
			ui.Success("%s: already linked", name)
		case sync.StatusNotExist:
			ui.Success("%s: symlink created", name)
		case sync.StatusHasFiles:
			ui.Success("%s: files migrated and linked", name)
		case sync.StatusBroken:
			ui.Success("%s: broken link fixed", name)
		case sync.StatusConflict:
			ui.Success("%s: conflict resolved (forced)", name)
		}
	}

	if hasError {
		return fmt.Errorf("some targets failed to sync")
	}

	return nil
}

func cmdStatus(args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	ui.Header("Source")
	if info, err := os.Stat(cfg.Source); err == nil {
		entries, _ := os.ReadDir(cfg.Source)
		skillCount := 0
		for _, e := range entries {
			if e.IsDir() && e.Name()[0] != '.' {
				skillCount++
			}
		}
		ui.Success("%s (%d skills, %s)", cfg.Source, skillCount, info.ModTime().Format("2006-01-02 15:04"))
	} else {
		ui.Error("%s (not found)", cfg.Source)
	}

	ui.Header("Targets")
	for name, target := range cfg.Targets {
		// Determine mode
		mode := target.Mode
		if mode == "" {
			mode = cfg.Mode
		}
		if mode == "" {
			mode = "merge"
		}

		var statusStr, detail string

		if mode == "merge" {
			status, linkedCount, localCount := sync.CheckStatusMerge(target.Path, cfg.Source)
			if status == sync.StatusMerged {
				statusStr = "merged"
				detail = fmt.Sprintf("[%s] %s (%d shared, %d local)", mode, target.Path, linkedCount, localCount)
			} else if status == sync.StatusLinked {
				// Configured as merge but actually using symlink - needs resync
				statusStr = "linked"
				detail = fmt.Sprintf("[%s→needs sync] %s", mode, target.Path)
			} else {
				statusStr = status.String()
				detail = fmt.Sprintf("[%s] %s (%d local)", mode, target.Path, localCount)
			}
		} else {
			status := sync.CheckStatus(target.Path, cfg.Source)
			statusStr = status.String()
			detail = fmt.Sprintf("[%s] %s", mode, target.Path)

			if status == sync.StatusConflict {
				link, _ := os.Readlink(target.Path)
				detail = fmt.Sprintf("[%s] %s -> %s", mode, target.Path, link)
			} else if status == sync.StatusMerged {
				// Configured as symlink but actually using merge - needs resync
				detail = fmt.Sprintf("[%s→needs sync] %s", mode, target.Path)
			}
		}

		ui.Status(name, statusStr, detail)
	}

	return nil
}

func cmdTarget(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: skillshare target <add|remove|list|name> [options]")
	}

	subcmd := args[0]
	subargs := args[1:]

	switch subcmd {
	case "add":
		return targetAdd(subargs)
	case "remove", "rm":
		return targetRemove(subargs)
	case "list", "ls":
		return targetList()
	default:
		// Assume it's a target name - show info or modify settings
		return targetInfo(subcmd, subargs)
	}
}

func targetAdd(args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("usage: skillshare target add <name> <path>")
	}

	name := args[0]
	path := args[1]

	// Expand ~
	if len(path) > 0 && path[0] == '~' {
		home, _ := os.UserHomeDir()
		path = filepath.Join(home, path[1:])
	}

	cfg, err := config.Load()
	if err != nil {
		return err
	}

	if _, exists := cfg.Targets[name]; exists {
		return fmt.Errorf("target '%s' already exists", name)
	}

	cfg.Targets[name] = config.TargetConfig{Path: path}
	if err := cfg.Save(); err != nil {
		return err
	}

	ui.Success("Added target: %s -> %s", name, path)
	return nil
}

func targetRemove(args []string) error {
	// Check for --all flag
	removeAll := false
	var name string
	for _, arg := range args {
		if arg == "--all" || arg == "-a" {
			removeAll = true
		} else {
			name = arg
		}
	}

	if !removeAll && name == "" {
		return fmt.Errorf("usage: skillshare target remove <name> or --all")
	}

	cfg, err := config.Load()
	if err != nil {
		return err
	}

	var toRemove []string
	if removeAll {
		for n := range cfg.Targets {
			toRemove = append(toRemove, n)
		}
	} else {
		if _, exists := cfg.Targets[name]; !exists {
			return fmt.Errorf("target '%s' not found", name)
		}
		toRemove = []string{name}
	}

	// Backup before removing
	ui.Header("Backing up before unlink")
	for _, targetName := range toRemove {
		target := cfg.Targets[targetName]
		backupPath, err := backup.Create(targetName, target.Path)
		if err != nil {
			ui.Warning("Failed to backup %s: %v", targetName, err)
		} else if backupPath != "" {
			ui.Success("%s -> %s", targetName, backupPath)
		}
	}

	ui.Header("Unlinking targets")
	for _, targetName := range toRemove {
		target := cfg.Targets[targetName]

		// Check if it's a symlink (symlink mode) or has symlinked skills (merge mode)
		info, err := os.Lstat(target.Path)
		if err != nil {
			// Target doesn't exist, just remove from config
			delete(cfg.Targets, targetName)
			ui.Success("%s: removed from config", targetName)
			continue
		}

		if info.Mode()&os.ModeSymlink != 0 {
			// Symlink mode: remove symlink and copy source contents
			if err := unlinkSymlinkMode(target.Path, cfg.Source); err != nil {
				ui.Error("%s: %v", targetName, err)
				continue
			}
			ui.Success("%s: unlinked and restored", targetName)
		} else if info.IsDir() {
			// Merge mode: remove individual skill symlinks
			if err := unlinkMergeMode(target.Path, cfg.Source); err != nil {
				ui.Error("%s: %v", targetName, err)
				continue
			}
			ui.Success("%s: skill symlinks removed", targetName)
		}

		delete(cfg.Targets, targetName)
	}

	if err := cfg.Save(); err != nil {
		return err
	}

	return nil
}

// unlinkSymlinkMode removes symlink and copies source contents back
func unlinkSymlinkMode(targetPath, sourcePath string) error {
	// Remove the symlink
	if err := os.Remove(targetPath); err != nil {
		return fmt.Errorf("failed to remove symlink: %w", err)
	}

	// Copy source contents to target
	if err := copyDir(sourcePath, targetPath); err != nil {
		return fmt.Errorf("failed to copy skills: %w", err)
	}

	return nil
}

// unlinkMergeMode removes individual skill symlinks and copies them back
func unlinkMergeMode(targetPath, sourcePath string) error {
	entries, err := os.ReadDir(targetPath)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		skillPath := filepath.Join(targetPath, entry.Name())
		info, err := os.Lstat(skillPath)
		if err != nil {
			continue
		}

		// Check if it's a symlink pointing to source
		if info.Mode()&os.ModeSymlink != 0 {
			link, _ := os.Readlink(skillPath)
			sourceSkillPath := filepath.Join(sourcePath, entry.Name())

			// Check if symlink points to our source
			absLink, _ := filepath.Abs(link)
			absSource, _ := filepath.Abs(sourceSkillPath)

			if absLink == absSource {
				// Remove symlink and copy the skill back
				os.Remove(skillPath)
				if err := copyDir(sourceSkillPath, skillPath); err != nil {
					return fmt.Errorf("failed to copy %s: %w", entry.Name(), err)
				}
			}
		}
	}

	return nil
}

func targetList() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	ui.Header("Configured Targets")
	for name, target := range cfg.Targets {
		mode := target.Mode
		if mode == "" {
			mode = "merge"
		}
		fmt.Printf("  %-12s %s (%s)\n", name, target.Path, mode)
	}

	return nil
}

func targetInfo(name string, args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	target, exists := cfg.Targets[name]
	if !exists {
		return fmt.Errorf("target '%s' not found. Use 'skillshare target list' to see available targets", name)
	}

	// Parse flags
	var newMode string
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--mode", "-m":
			if i+1 >= len(args) {
				return fmt.Errorf("--mode requires a value (merge or symlink)")
			}
			newMode = args[i+1]
			i++
		}
	}

	// If --mode is provided, update the mode
	if newMode != "" {
		if newMode != "merge" && newMode != "symlink" {
			return fmt.Errorf("invalid mode '%s'. Use 'merge' or 'symlink'", newMode)
		}

		oldMode := target.Mode
		if oldMode == "" {
			oldMode = cfg.Mode
			if oldMode == "" {
				oldMode = "merge"
			}
		}

		target.Mode = newMode
		cfg.Targets[name] = target
		if err := cfg.Save(); err != nil {
			return err
		}

		ui.Success("Changed %s mode: %s -> %s", name, oldMode, newMode)
		ui.Info("Run 'skillshare sync' to apply the new mode")
		return nil
	}

	// Show target info
	mode := target.Mode
	if mode == "" {
		mode = cfg.Mode
		if mode == "" {
			mode = "merge"
		}
		mode = mode + " (default)"
	}

	status := sync.CheckStatus(target.Path, cfg.Source)

	ui.Header(fmt.Sprintf("Target: %s", name))
	fmt.Printf("  Path:   %s\n", target.Path)
	fmt.Printf("  Mode:   %s\n", mode)
	fmt.Printf("  Status: %s\n", status)

	return nil
}

// copyDir copies a directory recursively
func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, _ := filepath.Rel(src, path)
		dstPath := filepath.Join(dst, relPath)

		if info.IsDir() {
			return os.MkdirAll(dstPath, info.Mode())
		}

		return copyFile(path, dstPath)
	})
}

// copyFile copies a single file
func copyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	srcInfo, err := srcFile.Stat()
	if err != nil {
		return err
	}

	dstFile, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, srcInfo.Mode())
	if err != nil {
		return err
	}
	defer dstFile.Close()

	_, err = io.Copy(dstFile, srcFile)
	return err
}

// cmdDiff shows differences between source and targets
func cmdDiff(args []string) error {
	var targetName string
	for i := 0; i < len(args); i++ {
		if args[i] == "--target" || args[i] == "-t" {
			if i+1 < len(args) {
				targetName = args[i+1]
				i++
			}
		} else {
			targetName = args[i]
		}
	}

	cfg, err := config.Load()
	if err != nil {
		return err
	}

	// Get source skills
	sourceSkills := make(map[string]bool)
	entries, _ := os.ReadDir(cfg.Source)
	for _, e := range entries {
		if e.IsDir() && e.Name()[0] != '.' {
			sourceSkills[e.Name()] = true
		}
	}

	targets := cfg.Targets
	if targetName != "" {
		if t, exists := cfg.Targets[targetName]; exists {
			targets = map[string]config.TargetConfig{targetName: t}
		} else {
			return fmt.Errorf("target '%s' not found", targetName)
		}
	}

	for name, target := range targets {
		ui.Header(fmt.Sprintf("Diff: %s", name))

		// Check if target is a symlink (symlink mode)
		targetInfo, err := os.Lstat(target.Path)
		if err != nil {
			ui.Warning("Cannot access target: %v", err)
			continue
		}

		if targetInfo.Mode()&os.ModeSymlink != 0 {
			// Symlink mode - entire directory is linked
			link, _ := os.Readlink(target.Path)
			absLink, _ := filepath.Abs(link)
			absSource, _ := filepath.Abs(cfg.Source)
			if absLink == absSource {
				ui.Success("Fully synced (symlink mode)")
			} else {
				ui.Warning("Symlink points to different location: %s", link)
			}
			continue
		}

		// Merge mode - check individual skills
		targetSkills := make(map[string]bool)
		targetSymlinks := make(map[string]bool)
		entries, err := os.ReadDir(target.Path)
		if err != nil {
			ui.Warning("Cannot read target: %v", err)
			continue
		}

		for _, e := range entries {
			if e.Name()[0] == '.' {
				continue
			}
			skillPath := filepath.Join(target.Path, e.Name())
			info, _ := os.Lstat(skillPath)
			if info != nil && info.Mode()&os.ModeSymlink != 0 {
				targetSymlinks[e.Name()] = true
			}
			targetSkills[e.Name()] = true
		}

		// Compare
		hasChanges := false

		// Skills only in source (not synced)
		for skill := range sourceSkills {
			if !targetSkills[skill] {
				fmt.Printf("  %s+ %s%s (in source, not in target)\n", ui.Green, skill, ui.Reset)
				hasChanges = true
			} else if !targetSymlinks[skill] {
				fmt.Printf("  %s~ %s%s (local copy, not linked)\n", ui.Yellow, skill, ui.Reset)
				hasChanges = true
			}
		}

		// Skills only in target (local only)
		for skill := range targetSkills {
			if !sourceSkills[skill] && !targetSymlinks[skill] {
				fmt.Printf("  %s- %s%s (local only, not in source)\n", ui.Cyan, skill, ui.Reset)
				hasChanges = true
			}
		}

		if !hasChanges {
			ui.Success("Fully synced (merge mode)")
		}
	}

	return nil
}

// cmdBackup creates a manual backup
func cmdBackup(args []string) error {
	var targetName string
	for i := 0; i < len(args); i++ {
		if args[i] == "--target" || args[i] == "-t" {
			if i+1 < len(args) {
				targetName = args[i+1]
				i++
			}
		} else {
			targetName = args[i]
		}
	}

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
	created := 0
	for name, target := range targets {
		backupPath, err := backup.Create(name, target.Path)
		if err != nil {
			ui.Warning("Failed to backup %s: %v", name, err)
			continue
		}
		if backupPath != "" {
			ui.Success("%s -> %s", name, backupPath)
			created++
		} else {
			ui.Info("%s: nothing to backup (empty or symlink)", name)
		}
	}

	if created == 0 {
		ui.Info("No backups created")
	}

	// List recent backups
	backups, _ := backup.List()
	if len(backups) > 0 {
		ui.Header("Recent backups")
		limit := 5
		if len(backups) < limit {
			limit = len(backups)
		}
		for i := 0; i < limit; i++ {
			b := backups[i]
			fmt.Printf("  %s %s (%s)\n", b.Timestamp, ui.Gray+strings.Join(b.Targets, ", ")+ui.Reset, b.Path)
		}
	}

	return nil
}

// cmdDoctor checks the environment and diagnoses issues
func cmdDoctor(args []string) error {
	ui.Header("Checking environment")
	issues := 0

	// Check config exists
	if _, err := os.Stat(config.ConfigPath()); os.IsNotExist(err) {
		ui.Error("Config not found: run 'skillshare init' first")
		return nil
	}
	ui.Success("Config: %s", config.ConfigPath())

	cfg, err := config.Load()
	if err != nil {
		ui.Error("Config error: %v", err)
		return nil
	}

	// Check source exists
	if info, err := os.Stat(cfg.Source); err != nil {
		ui.Error("Source not found: %s", cfg.Source)
		issues++
	} else if !info.IsDir() {
		ui.Error("Source is not a directory: %s", cfg.Source)
		issues++
	} else {
		entries, _ := os.ReadDir(cfg.Source)
		skillCount := 0
		for _, e := range entries {
			if e.IsDir() && e.Name()[0] != '.' {
				skillCount++
			}
		}
		ui.Success("Source: %s (%d skills)", cfg.Source, skillCount)
	}

	// Check symlink support
	testSymlink := filepath.Join(os.TempDir(), "skillshare_symlink_test")
	testTarget := filepath.Join(os.TempDir(), "skillshare_symlink_target")
	os.Remove(testSymlink)
	os.Remove(testTarget)
	os.MkdirAll(testTarget, 0755)
	defer os.Remove(testSymlink)
	defer os.RemoveAll(testTarget)

	if err := os.Symlink(testTarget, testSymlink); err != nil {
		ui.Error("Symlink not supported: %v", err)
		issues++
	} else {
		ui.Success("Symlink support: OK")
	}

	// Check each target
	ui.Header("Checking targets")
	for name, target := range cfg.Targets {
		// Determine mode
		mode := target.Mode
		if mode == "" {
			mode = cfg.Mode
		}
		if mode == "" {
			mode = "merge"
		}

		targetIssues := []string{}

		// Check path exists
		info, err := os.Lstat(target.Path)
		if err != nil {
			if os.IsNotExist(err) {
				// Check parent is writable
				parent := filepath.Dir(target.Path)
				if _, err := os.Stat(parent); err != nil {
					targetIssues = append(targetIssues, "parent directory not found")
				}
			} else {
				targetIssues = append(targetIssues, fmt.Sprintf("access error: %v", err))
			}
		} else {
			// Check if it's a symlink
			if info.Mode()&os.ModeSymlink != 0 {
				link, _ := os.Readlink(target.Path)
				absLink, _ := filepath.Abs(link)
				absSource, _ := filepath.Abs(cfg.Source)
				if absLink != absSource {
					targetIssues = append(targetIssues, fmt.Sprintf("symlink points to wrong location: %s", link))
				}
			}
		}

		// Check write permission
		if info != nil && info.IsDir() {
			testFile := filepath.Join(target.Path, ".skillshare_write_test")
			if f, err := os.Create(testFile); err != nil {
				targetIssues = append(targetIssues, "not writable")
			} else {
				f.Close()
				os.Remove(testFile)
			}
		}

		if len(targetIssues) > 0 {
			ui.Error("%s [%s]: %s", name, mode, strings.Join(targetIssues, ", "))
			issues++
		} else {
			var statusStr string
			needsSync := false

			if mode == "merge" {
				status, linkedCount, localCount := sync.CheckStatusMerge(target.Path, cfg.Source)
				if status == sync.StatusMerged {
					statusStr = fmt.Sprintf("merged (%d shared, %d local)", linkedCount, localCount)
				} else if status == sync.StatusLinked {
					statusStr = "linked (needs sync to apply merge mode)"
					needsSync = true
				} else {
					statusStr = status.String()
				}
			} else {
				status := sync.CheckStatus(target.Path, cfg.Source)
				statusStr = status.String()
				if status == sync.StatusMerged {
					statusStr = "merged (needs sync to apply symlink mode)"
					needsSync = true
				}
			}

			if needsSync {
				ui.Warning("%s [%s]: %s", name, mode, statusStr)
			} else {
				ui.Success("%s [%s]: %s", name, mode, statusStr)
			}
		}
	}

	// Summary
	ui.Header("Summary")
	if issues == 0 {
		ui.Success("All checks passed!")
	} else {
		ui.Warning("%d issue(s) found", issues)
	}

	return nil
}
