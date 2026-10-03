package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"skillshare/internal/config"
	"skillshare/internal/oplog"
	"skillshare/internal/sync"
	"skillshare/internal/ui"
)

func cmdExtrasRemove(args []string) error {
	start := time.Now()

	mode, rest, err := parseModeArgs(args)
	if err != nil {
		return err
	}

	cwd, _ := os.Getwd()
	mode = resolveAutoMode(mode, cwd)

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

func removeExtraFromGlobalConfig(cfg *config.Config, name string) (string, error) {
	idx := -1
	var removed config.ExtraConfig
	for i, e := range cfg.Extras {
		if e.Name == name {
			idx = i
			removed = e
			break
		}
	}
	if idx == -1 {
		return "", fmt.Errorf("extra %q not found in config", name)
	}

	sourceDir := config.ResolveExtrasSourceDir(removed, cfg.EffectiveExtrasSource(), cfg.EffectiveSkillsSource())

	restored, restoreErr := restoreExtraFileTargets(removed, sourceDir, modeGlobal, "")
	if restoreErr == nil {
		cfg.Extras = append(cfg.Extras[:idx], cfg.Extras[idx+1:]...)
		if err := cfg.Save(); err != nil {
			return "", fmt.Errorf("failed to save config: %w", err)
		}
	}

	e := oplog.NewEntry("extras-remove", statusFromErr(restoreErr), 0)
	e.Args = map[string]any{"name": name, "scope": "global", "restored": restored}
	if restoreErr != nil {
		e.Message = restoreErr.Error()
	}
	oplog.WriteWithLimit(config.ConfigPath(), oplog.OpsFile, e, logMaxEntries()) //nolint:errcheck

	return sourceDir, restoreErr
}

func removeExtraFromProjectConfig(projCfg *config.ProjectConfig, cwd, name string) (string, error) {
	idx, removed := findExtraByName(projCfg.Extras, name)
	if idx == -1 {
		return "", fmt.Errorf("extra %q not found in project config", name)
	}

	sourceDir := config.ResolveExtrasSourceDirProject(removed, projCfg.EffectiveExtrasSource(cwd), cwd)
	restored, restoreErr := restoreExtraFileTargets(removed, sourceDir, modeProject, cwd)
	if restoreErr == nil {
		projCfg.Extras = append(projCfg.Extras[:idx], projCfg.Extras[idx+1:]...)
		if err := projCfg.Save(cwd); err != nil {
			return "", fmt.Errorf("failed to save project config: %w", err)
		}
	}

	cfgPath := config.ProjectConfigPath(cwd)
	e := oplog.NewEntry("extras-remove", statusFromErr(restoreErr), 0)
	e.Args = map[string]any{"name": name, "scope": "project", "restored": restored}
	if restoreErr != nil {
		e.Message = restoreErr.Error()
	}
	oplog.WriteWithLimit(cfgPath, oplog.OpsFile, e, logMaxEntries()) //nolint:errcheck

	return sourceDir, restoreErr
}

func extraRemoveTargetNote(extra config.ExtraConfig) string {
	if extra.File != "" {
		return "Target files will be restored to how they were before skillshare replaced them."
	}
	return "Existing symlinks in targets will become orphaned."
}

// restoreExtraFileTargets undoes the targets of a removed single-file extra:
// its links, copies, or import lines go, and files it replaced come back.
// Directory extras keep their targets (sync extras cleans orphans).
func restoreExtraFileTargets(extra config.ExtraConfig, sourceDir string, mode runMode, cwd string) (int, error) {
	return sync.RestoreExtraFileTargets(extra, sourceDir, func(path string) string {
		return canonicalExtraTargetPath(mode, cwd, path)
	})
}

func extrasRemoveGlobal(name string, force bool, start time.Time) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	// Pre-resolve source path for confirmation and cleanup (before removal mutates the slice).
	idx, found := findExtraByName(cfg.Extras, name)
	if idx == -1 {
		return fmt.Errorf("extra %q not found in config", name)
	}
	sourceDir := config.ResolveExtrasSourceDir(found, cfg.EffectiveExtrasSource(), cfg.EffectiveSkillsSource())

	if !force {
		ui.Warning("This will remove %q from config.", name)
		ui.Info("Source files in %s will NOT be deleted.", shortenPath(sourceDir))
		ui.Info("%s", extraRemoveTargetNote(found))
		fmt.Println()
		ok, err := ui.ConfirmAction("Remove?", false)
		if err != nil {
			return err
		}
		if !ok {
			ui.Cancelled("removed")
			return nil
		}
	}

	sourceDir, err = removeExtraFromGlobalConfig(cfg, name)
	if err != nil {
		return err
	}

	ui.Success("Removed %q from extras config", name)
	cleanEmptyExtrasDir(sourceDir)
	if found.File == "" {
		ui.Info("Run 'skillshare sync extras' to clean up orphaned links.")
	}
	_ = start
	return nil
}

func extrasRemoveProject(cwd, name string, force bool, start time.Time) error {
	projCfg, err := config.LoadProject(cwd)
	if err != nil {
		return err
	}

	_, found := findExtraByName(projCfg.Extras, name)
	if !force {
		sourceDir := config.ResolveExtrasSourceDirProject(found, projCfg.EffectiveExtrasSource(cwd), cwd)
		ui.Warning("This will remove %q from project config.", name)
		ui.Info("Source files in %s will NOT be deleted.", shortenPath(sourceDir))
		ui.Info("%s", extraRemoveTargetNote(found))
		fmt.Println()
		ok, err := ui.ConfirmAction("Remove?", false)
		if err != nil {
			return err
		}
		if !ok {
			ui.Cancelled("removed")
			return nil
		}
	}

	sourceDir, err := removeExtraFromProjectConfig(projCfg, cwd, name)
	if err != nil {
		return err
	}

	ui.Success("Removed %q from project extras config", name)
	cleanEmptyExtrasDir(sourceDir)
	if found.File == "" {
		ui.Info("Run 'skillshare sync extras -p' to clean up orphaned links.")
	}
	_ = start
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

// cleanEmptyExtrasDirQuiet is the quiet variant of cleanEmptyExtrasDir for TUI use (no stdout writes).
func cleanEmptyExtrasDirQuiet(dir string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	if len(entries) == 0 {
		os.Remove(dir)
	}
	removeEmptyDir(filepath.Dir(dir))
}

// findExtraByName returns the index and a copy of the ExtraConfig with the given name.
// Returns -1 and zero-value if not found.
func findExtraByName(extras []config.ExtraConfig, name string) (int, config.ExtraConfig) {
	for i, e := range extras {
		if e.Name == name {
			return i, e
		}
	}
	return -1, config.ExtraConfig{}
}

func printExtrasRemoveHelp() {
	fmt.Println(`Usage: skillshare extras remove <name> [options]

Remove an extra resource type from config.

Source files are NOT deleted. Single-file targets are restored to their
pre-attach state. Directory extras leave target files in place; run
'skillshare sync extras' to clean up their orphaned links.

Arguments:
  name                Name of the extra to remove

Options:
  --force, -f         Skip confirmation prompt
  --project, -p       Remove from project config (.skillshare/)
  --global, -g        Remove from global config (~/.config/skillshare/)
  --help, -h          Show this help`)
}
