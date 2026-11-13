package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"skillshare/internal/config"
	"skillshare/internal/extraops"
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

func removeExtraFromGlobalConfig(cfg *config.Config, name string) (string, extraops.RemoveResult, error) {
	_, extra := findExtraByName(cfg.Extras, name)
	source := config.ResolveExtrasSourceDir(extra, cfg.EffectiveExtrasSource(), cfg.EffectiveSkillsSource())
	baseline := cloneExtraConfig(&extra)
	result, err := extraops.Remove(extraops.RemoveRequest{Extras: cfg.Extras, Name: name, SourceDir: source, ExtrasRoot: cfg.EffectiveExtrasSource(), TargetPath: func(p string) string { return canonicalExtraTargetPath(modeGlobal, "", p) },
		Load: func() ([]config.ExtraConfig, error) {
			fresh, err := config.LoadReadOnly()
			if err != nil {
				return nil, err
			}
			_, e := findExtraByName(fresh.Extras, name)
			baseline = cloneExtraConfig(&e)
			return fresh.Extras, nil
		},
		Save: func(remaining []config.ExtraConfig) error {
			if extra.Remote == "" {
				cfg.Extras = remaining
				return cfg.Save()
			}
			fresh, err := config.LoadReadOnly()
			if err != nil {
				return err
			}
			fresh.Extras, err = extraops.WithoutUnchangedExtra(fresh.Extras, *baseline)
			if err != nil {
				return err
			}
			candidate, err := config.PrepareSave(config.ReadOnlyConfigPath(), fresh)
			if err != nil {
				return err
			}
			return candidate.Write()
		}})
	logExtraRemoval(config.ReadOnlyConfigPath(), name, "global", result, err)
	return source, result, err
}

func removeExtraFromProjectConfig(cfg *config.ProjectConfig, cwd, name string) (string, extraops.RemoveResult, error) {
	_, extra := findExtraByName(cfg.Extras, name)
	source := config.ResolveExtrasSourceDirProject(extra, cfg.EffectiveExtrasSource(cwd), cwd)
	baseline := cloneExtraConfig(&extra)
	result, err := extraops.Remove(extraops.RemoveRequest{Extras: cfg.Extras, Name: name, SourceDir: source, ExtrasRoot: cfg.EffectiveExtrasSource(cwd), ProjectRoot: cwd, TargetPath: func(p string) string { return canonicalExtraTargetPath(modeProject, cwd, p) },
		Load: func() ([]config.ExtraConfig, error) {
			fresh, err := config.LoadProjectReadOnly(cwd)
			if err != nil {
				return nil, err
			}
			_, e := findExtraByName(fresh.Extras, name)
			baseline = cloneExtraConfig(&e)
			return fresh.Extras, nil
		},
		Save: func(remaining []config.ExtraConfig) error {
			if extra.Remote == "" {
				cfg.Extras = remaining
				return cfg.Save(cwd)
			}
			fresh, err := config.LoadProjectReadOnly(cwd)
			if err != nil {
				return err
			}
			fresh.Extras, err = extraops.WithoutUnchangedExtra(fresh.Extras, *baseline)
			if err != nil {
				return err
			}
			return fresh.Save(cwd)
		}})
	logExtraRemoval(config.ProjectConfigPath(cwd), name, "project", result, err)
	return source, result, err
}

func logExtraRemoval(configPath, name, scope string, result extraops.RemoveResult, err error) {
	entry := oplog.NewEntry("extras-remove", statusFromErr(err), 0)
	entry.Args = map[string]any{"name": name, "scope": scope, "restored": result.Restored, "deleted_dir": result.DeletedDir, "missing_dir": result.MissingDir, "kept_edited_dir": result.KeptEditedDir, "kept_outputs": result.KeptOutputs}
	if result.Remote != "" {
		entry.Args["remote"] = result.Remote
		entry.Args["commit"] = result.Commit
	}
	if err != nil {
		entry.Message = err.Error()
	}
	oplog.WriteWithLimit(configPath, oplog.OpsFile, entry, logMaxEntries()) //nolint:errcheck
}

func extraRemovalMessage(result extraops.RemoveResult, source string) string {
	if result.Remote == "" {
		return ""
	}
	commit := result.Commit
	if len(commit) > 7 {
		commit = commit[:7]
	}
	if result.MissingDir {
		return "Fetched source was already missing: " + shortenPath(source)
	}
	if result.DeletedDir {
		return fmt.Sprintf("Deleted fetched copy from %s at %s", result.Remote, commit)
	}
	return "Kept edited or unknown source in " + shortenPath(source)
}

func reportExtraRemoval(result extraops.RemoveResult, source string) {
	if result.Remote == "" {
		cleanEmptyExtrasDir(source)
		return
	}
	ui.Note(extraRemovalMessage(result, source))
	for _, warning := range result.Warnings {
		ui.Warning("%s", warning)
	}
	for _, output := range result.KeptOutputs {
		ui.Note("Kept output " + shortenPath(output))
	}
}

func extraRemoveTargetNote(extra config.ExtraConfig) string {
	if extra.Remote != "" {
		return "An unedited fetched copy is deleted; an edited copy stays. Owned links are cleaned; copied or transformed outputs stay."
	}
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
		ui.Warning("This removes %q from the config", name)
		if found.Remote == "" {
			ui.Note(fmt.Sprintf("Source files in %s stay", shortenPath(sourceDir)))
		}
		ui.Note(extraRemoveTargetNote(found))
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

	sourceDir, result, err := removeExtraFromGlobalConfig(cfg, name)
	if err != nil {
		return err
	}

	ui.Done(ui.MarkOK, fmt.Sprintf("Removed %q from the extras config", name), 0)
	reportExtraRemoval(result, sourceDir)
	if found.File == "" && found.Remote == "" {
		ui.Next("skillshare sync extras", "clean up orphaned links")
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
		ui.Warning("This removes %q from the project config", name)
		if found.Remote == "" {
			ui.Note(fmt.Sprintf("Source files in %s stay", shortenPath(sourceDir)))
		}
		ui.Note(extraRemoveTargetNote(found))
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

	sourceDir, result, err := removeExtraFromProjectConfig(projCfg, cwd, name)
	if err != nil {
		return err
	}

	ui.Done(ui.MarkOK, fmt.Sprintf("Removed %q from the project extras config", name), 0)
	reportExtraRemoval(result, sourceDir)
	if found.File == "" && found.Remote == "" {
		ui.Next("skillshare sync extras -p", "clean up orphaned links")
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
		ui.Note("Removed the empty source directory " + shortenPath(dir))
	} else {
		ui.Note(fmt.Sprintf("Kept %s in %s", plural(len(entries), "file"), shortenPath(dir)))
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
	printHelp("skillshare extras remove <name> [options]", "Remove an extra resource type from config.\n\nRemote extras delete an unedited fetched copy; edited copies stay. Owned\nlinks are cleaned, while copied or transformed outputs stay. Local source\nfiles stay; single-file targets are restored to their pre-attach state.\nRun 'skillshare sync extras' to clean up local directory orphan links.",
		helpGroup{title: "Arguments", rows: []helpRow{
			{"name", "Name of the extra to remove"},
		}},
		helpGroup{title: "Options", rows: []helpRow{
			{"-f, --force", "Skip confirmation prompt"},
			{"-p, --project", "Remove from project config (.skillshare/)"},
			{"-g, --global", "Remove from global config (~/.config/skillshare/)"},
		}},
	)
}
