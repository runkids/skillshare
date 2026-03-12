package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"skillshare/internal/config"
	"skillshare/internal/oplog"
	"skillshare/internal/ui"
)

func cmdExtrasRemove(args []string) error {
	start := time.Now()

	mode, rest, err := parseModeArgs(args)
	if err != nil {
		return err
	}

	cwd, _ := os.Getwd()
	if mode == modeAuto {
		if projectConfigExists(cwd) {
			mode = modeProject
		} else {
			mode = modeGlobal
		}
	}

	applyModeLabel(mode)

	// Parse flags
	var name string
	force := false
	for _, a := range rest {
		switch a {
		case "--force", "-f":
			force = true
		case "--help", "-h":
			printExtrasRemoveHelp()
			return nil
		default:
			if name == "" {
				name = a
			} else {
				return fmt.Errorf("unexpected argument: %s", a)
			}
		}
	}

	if name == "" {
		return fmt.Errorf("extras name is required: skillshare extras remove <name>")
	}

	if mode == modeProject {
		return extrasRemoveProject(cwd, name, force, start)
	}
	return extrasRemoveGlobal(name, force, start)
}

func extrasRemoveGlobal(name string, force bool, start time.Time) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	// Find the extra
	idx := -1
	for i, e := range cfg.Extras {
		if e.Name == name {
			idx = i
			break
		}
	}
	if idx == -1 {
		return fmt.Errorf("extra %q not found in config", name)
	}

	if !force {
		sourceDir := config.ExtrasSourceDir(cfg.Source, name)
		ui.Warning("This will remove %q from config.", name)
		ui.Info("Source files in %s will NOT be deleted.", shortenPath(sourceDir))
		ui.Info("Existing symlinks in targets will become orphaned.")
		fmt.Println()
		fmt.Print("Remove? [y/N]: ")
		var input string
		fmt.Scanln(&input)
		if input = strings.ToLower(strings.TrimSpace(input)); input != "y" && input != "yes" {
			ui.Info("Cancelled.")
			return nil
		}
	}

	// Remove from slice
	cfg.Extras = append(cfg.Extras[:idx], cfg.Extras[idx+1:]...)
	if err := cfg.Save(); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	ui.Success("Removed %q from extras config", name)

	sourceDir := config.ExtrasSourceDir(cfg.Source, name)
	cleanEmptyExtrasDir(sourceDir)

	ui.Info("Run 'skillshare sync extras' to clean up orphaned links.")

	// Oplog
	e := oplog.NewEntry("extras-remove", "ok", time.Since(start))
	e.Args = map[string]any{"name": name, "scope": "global"}
	oplog.WriteWithLimit(config.ConfigPath(), oplog.OpsFile, e, logMaxEntries()) //nolint:errcheck

	return nil
}

func extrasRemoveProject(cwd, name string, force bool, start time.Time) error {
	projCfg, err := config.LoadProject(cwd)
	if err != nil {
		return err
	}

	// Find the extra
	idx := -1
	for i, e := range projCfg.Extras {
		if e.Name == name {
			idx = i
			break
		}
	}
	if idx == -1 {
		return fmt.Errorf("extra %q not found in project config", name)
	}

	if !force {
		sourceDir := config.ExtrasSourceDirProject(cwd, name)
		ui.Warning("This will remove %q from project config.", name)
		ui.Info("Source files in %s will NOT be deleted.", shortenPath(sourceDir))
		ui.Info("Existing symlinks in targets will become orphaned.")
		fmt.Println()
		fmt.Print("Remove? [y/N]: ")
		var input string
		fmt.Scanln(&input)
		if input = strings.ToLower(strings.TrimSpace(input)); input != "y" && input != "yes" {
			ui.Info("Cancelled.")
			return nil
		}
	}

	// Remove from slice
	projCfg.Extras = append(projCfg.Extras[:idx], projCfg.Extras[idx+1:]...)
	if err := projCfg.Save(cwd); err != nil {
		return fmt.Errorf("failed to save project config: %w", err)
	}

	ui.Success("Removed %q from project extras config", name)

	sourceDir := config.ExtrasSourceDirProject(cwd, name)
	cleanEmptyExtrasDir(sourceDir)

	ui.Info("Run 'skillshare sync extras -p' to clean up orphaned links.")

	// Oplog
	cfgPath := config.ProjectConfigPath(cwd)
	e := oplog.NewEntry("extras-remove", "ok", time.Since(start))
	e.Args = map[string]any{"name": name, "scope": "project"}
	oplog.WriteWithLimit(cfgPath, oplog.OpsFile, e, logMaxEntries()) //nolint:errcheck

	return nil
}

// cleanEmptyExtrasDir removes the source directory if it exists and is empty.
// Also removes the parent extras/ directory if it becomes empty.
func cleanEmptyExtrasDir(dir string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return // doesn't exist or unreadable — nothing to do
	}
	if len(entries) == 0 {
		os.Remove(dir)
		ui.Info("Removed empty source directory %s", shortenPath(dir))
	} else {
		ui.Info("Source files preserved in %s (%d files)", shortenPath(dir), len(entries))
	}

	// Clean parent extras/ directory if empty
	removeEmptyDir(filepath.Dir(dir))
}

func printExtrasRemoveHelp() {
	fmt.Println(`Usage: skillshare extras remove <name> [options]

Remove an extra resource type from config.

Source files and target symlinks are NOT deleted.
Run 'skillshare sync extras' after removal to clean up orphaned links.

Arguments:
  name                Name of the extra to remove

Options:
  --force, -f         Skip confirmation prompt
  --project, -p       Remove from project config (.skillshare/)
  --global, -g        Remove from global config (~/.config/skillshare/)
  --help, -h          Show this help`)
}
