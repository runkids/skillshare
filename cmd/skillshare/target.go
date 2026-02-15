package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"skillshare/internal/backup"
	"skillshare/internal/config"
	"skillshare/internal/oplog"
	"skillshare/internal/sync"
	"skillshare/internal/ui"
	"skillshare/internal/utils"
	"skillshare/internal/validate"
)

func cmdTarget(args []string) error {
	mode, rest, err := parseModeArgs(args)
	if err != nil {
		return err
	}

	if len(rest) < 1 {
		return fmt.Errorf("usage: skillshare target <add|remove|list|name> [options]")
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

	subcmd := rest[0]
	subargs := rest[1:]

	switch subcmd {
	case "help", "--help", "-h":
		printTargetHelp()
		return nil
	case "add":
		if mode == modeProject {
			return targetAddProject(subargs, cwd)
		}
		return targetAdd(subargs)
	case "remove", "rm":
		if mode == modeProject {
			return targetRemoveProject(subargs, cwd)
		}
		return targetRemove(subargs)
	case "list", "ls":
		if mode == modeProject {
			return targetListProject(cwd)
		}
		return targetList()
	default:
		// Assume it's a target name - show info or modify settings
		if mode == modeProject {
			return targetInfoProject(subcmd, subargs, cwd)
		}
		return targetInfo(subcmd, subargs)
	}
}

func printTargetHelp() {
	fmt.Println(`Usage: skillshare target <add|remove|list|name> [options]

Manage target skill directories.

Subcommands:
  add <name> [path]      Add a target (path optional for known project targets)
  remove <name>          Remove a target
  remove --all           Remove all targets
  list                   List configured targets
  <name>                 Show target info or modify settings

Options:
  --project, -p          Use project-level config in current directory
  --global, -g           Use global config (~/.config/skillshare)

Target Settings:
  <name> --mode <mode>              Set sync mode (merge or symlink)
  <name> --add-include <pattern>    Add an include filter pattern
  <name> --add-exclude <pattern>    Add an exclude filter pattern
  <name> --remove-include <pattern> Remove an include filter pattern
  <name> --remove-exclude <pattern> Remove an exclude filter pattern

Examples:
  skillshare target add cursor
  skillshare target add my-ide .my-ide/skills
  skillshare target remove cursor
  skillshare target list
  skillshare target cursor
  skillshare target claude --add-include "team-*"
  skillshare target claude --remove-include "team-*"
  skillshare target claude --add-exclude "_legacy*"

Project mode:
  skillshare target add claude-code -p
  skillshare target claude-code --add-include "team-*" -p
  skillshare target list -p`)
}

func targetAdd(args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("usage: skillshare target add <name> <path>")
	}

	name := args[0]
	path := args[1]

	// Validate target name
	if err := validate.TargetName(name); err != nil {
		return fmt.Errorf("invalid target name: %w", err)
	}

	// Expand ~
	if utils.HasTildePrefix(path) {
		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("cannot expand path: %w", err)
		}
		path = filepath.Join(home, path[1:])
	}

	// Validate target path and get warnings
	warnings, err := validate.TargetPath(path)
	if err != nil {
		return fmt.Errorf("invalid path: %w", err)
	}

	// Show warnings to user
	for _, w := range warnings {
		ui.Warning("%s", w)
	}

	// If path doesn't look like a skills directory, ask for confirmation
	if !validate.IsLikelySkillsPath(path) {
		ui.Warning("Path doesn't appear to be a skills directory")
		fmt.Print("  Continue anyway? [y/N]: ")
		var input string
		fmt.Scanln(&input)
		input = strings.ToLower(strings.TrimSpace(input))
		if input != "y" && input != "yes" {
			ui.Info("Cancelled")
			return nil
		}
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
	ui.Info("Run 'skillshare sync' to sync skills to this target")
	return nil
}

// targetRemoveOptions holds parsed options for target remove
type targetRemoveOptions struct {
	name      string
	removeAll bool
	dryRun    bool
}

// parseTargetRemoveArgs parses target remove arguments
func parseTargetRemoveArgs(args []string) (*targetRemoveOptions, error) {
	opts := &targetRemoveOptions{}

	for _, arg := range args {
		switch arg {
		case "--all", "-a":
			opts.removeAll = true
		case "--dry-run", "-n":
			opts.dryRun = true
		default:
			opts.name = arg
		}
	}

	if !opts.removeAll && opts.name == "" {
		return nil, fmt.Errorf("usage: skillshare target remove <name> or --all")
	}

	return opts, nil
}

// resolveTargetsToRemove determines which targets to remove
func resolveTargetsToRemove(cfg *config.Config, opts *targetRemoveOptions) ([]string, error) {
	if opts.removeAll {
		var toRemove []string
		for n := range cfg.Targets {
			toRemove = append(toRemove, n)
		}
		return toRemove, nil
	}

	if _, exists := cfg.Targets[opts.name]; !exists {
		return nil, fmt.Errorf("target '%s' not found", opts.name)
	}
	return []string{opts.name}, nil
}

// backupTargets creates backups for targets before removal
func backupTargets(cfg *config.Config, toRemove []string) {
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
}

// unlinkTarget unlinks a single target
func unlinkTarget(targetName string, target config.TargetConfig, sourcePath string) error {
	info, err := os.Lstat(target.Path)
	if err != nil {
		return nil // Target doesn't exist, OK to remove from config
	}

	if info.Mode()&os.ModeSymlink != 0 {
		if err := unlinkSymlinkMode(target.Path, sourcePath); err != nil {
			return err
		}
		ui.Success("%s: unlinked and restored", targetName)
	} else if info.IsDir() {
		if err := unlinkMergeMode(target.Path, sourcePath); err != nil {
			return err
		}
		ui.Success("%s: skill symlinks removed", targetName)
	}

	return nil
}

func targetRemove(args []string) error {
	opts, err := parseTargetRemoveArgs(args)
	if err != nil {
		return err
	}

	cfg, err := config.Load()
	if err != nil {
		return err
	}

	toRemove, err := resolveTargetsToRemove(cfg, opts)
	if err != nil {
		return err
	}

	if opts.dryRun {
		return targetRemoveDryRun(cfg, toRemove)
	}

	backupTargets(cfg, toRemove)

	ui.Header("Unlinking targets")
	for _, targetName := range toRemove {
		target := cfg.Targets[targetName]
		if err := unlinkTarget(targetName, target, cfg.Source); err != nil {
			ui.Error("%s: %v", targetName, err)
			continue
		}
		delete(cfg.Targets, targetName)
	}

	return cfg.Save()
}

func targetRemoveDryRun(cfg *config.Config, toRemove []string) error {
	ui.Warning("Dry run mode - no changes will be made")

	ui.Header("Backing up before unlink")
	for _, targetName := range toRemove {
		ui.Info("%s: would attempt backup", targetName)
	}

	ui.Header("Unlinking targets")
	for _, targetName := range toRemove {
		target := cfg.Targets[targetName]
		info, err := os.Lstat(target.Path)
		if err != nil {
			if os.IsNotExist(err) {
				ui.Info("%s: would remove from config (path missing)", targetName)
				continue
			}
			ui.Warning("%s: %v", targetName, err)
			continue
		}

		if info.Mode()&os.ModeSymlink != 0 {
			ui.Info("%s: would unlink symlink and restore contents", targetName)
		} else if info.IsDir() {
			ui.Info("%s: would remove skill symlinks", targetName)
		}

		ui.Info("%s: would remove from config", targetName)
	}

	return nil
}

// unlinkSymlinkMode removes symlink and copies source contents back.
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

// unlinkMergeMode removes individual skill symlinks pointing to source and
// copies the skill contents back so the target retains real files.
func unlinkMergeMode(targetPath, sourcePath string) error {
	entries, err := os.ReadDir(targetPath)
	if err != nil {
		return err
	}

	absSource, err := filepath.Abs(sourcePath)
	if err != nil {
		return fmt.Errorf("failed to resolve source path: %w", err)
	}
	absSourcePrefix := absSource + string(filepath.Separator)

	for _, entry := range entries {
		skillPath := filepath.Join(targetPath, entry.Name())

		if !utils.IsSymlinkOrJunction(skillPath) {
			continue // Not a symlink — preserve local skills
		}

		absLink, err := utils.ResolveLinkTarget(skillPath)
		if err != nil {
			continue // Can't resolve — skip
		}

		// Check if symlink points to anywhere under source directory
		if !utils.PathHasPrefix(absLink, absSourcePrefix) {
			continue // Not managed by skillshare — skip
		}

		// Remove symlink and copy the skill back if source still exists
		os.Remove(skillPath)
		if _, statErr := os.Stat(absLink); statErr == nil {
			if err := copyDir(absLink, skillPath); err != nil {
				return fmt.Errorf("failed to copy %s: %w", entry.Name(), err)
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
	// Parse filter flags first, pass remaining to mode parsing
	filterOpts, remaining, err := parseFilterFlags(args)
	if err != nil {
		return err
	}

	cfg, err := config.Load()
	if err != nil {
		return err
	}

	target, exists := cfg.Targets[name]
	if !exists {
		return fmt.Errorf("target '%s' not found. Use 'skillshare target list' to see available targets", name)
	}

	// Parse --mode from remaining args
	var newMode string
	for i := 0; i < len(remaining); i++ {
		switch remaining[i] {
		case "--mode", "-m":
			if i+1 >= len(remaining) {
				return fmt.Errorf("--mode requires a value (merge or symlink)")
			}
			newMode = remaining[i+1]
			i++
		}
	}

	// Apply filter updates if any
	if filterOpts.hasUpdates() {
		start := time.Now()
		changes, fErr := applyFilterUpdates(&target.Include, &target.Exclude, filterOpts)
		if fErr != nil {
			return fErr
		}
		cfg.Targets[name] = target
		if err := cfg.Save(); err != nil {
			return err
		}
		for _, change := range changes {
			ui.Success("%s: %s", name, change)
		}
		if len(changes) > 0 {
			ui.Info("Run 'skillshare sync' to apply filter changes")
		}

		e := oplog.NewEntry("target", statusFromErr(nil), time.Since(start))
		e.Args = map[string]any{
			"action":  "filter",
			"name":    name,
			"changes": changes,
		}
		oplog.Write(config.ConfigPath(), oplog.OpsFile, e) //nolint:errcheck
		return nil
	}

	// If --mode is provided, update the mode
	if newMode != "" {
		return updateTargetMode(cfg, name, target, newMode)
	}

	// Show target info
	return showTargetInfo(cfg, name, target)
}

func updateTargetMode(cfg *config.Config, name string, target config.TargetConfig, newMode string) error {
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

func showTargetInfo(cfg *config.Config, name string, target config.TargetConfig) error {
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
	fmt.Printf("  Path:    %s\n", target.Path)
	fmt.Printf("  Mode:    %s\n", mode)
	fmt.Printf("  Status:  %s\n", status)
	fmt.Printf("  Include: %s\n", formatFilterList(target.Include))
	fmt.Printf("  Exclude: %s\n", formatFilterList(target.Exclude))

	return nil
}
