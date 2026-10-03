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

func cmdExtrasInit(args []string) error {
	start := time.Now()

	mode, rest, err := parseModeArgs(args)
	if err != nil {
		return err
	}

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("cannot determine working directory: %w", err)
	}

	mode = resolveAutoMode(mode, cwd)

	applyModeLabel(mode)

	// Parse flags
	var name string
	var targets []string
	var syncMode string
	var sourceOverride string
	var file, as string
	var force bool
	var noTUI bool
	var flatten bool
	for i := 0; i < len(rest); i++ {
		switch rest[i] {
		case "--target":
			if i+1 >= len(rest) {
				return fmt.Errorf("--target requires a path argument")
			}
			i++
			targets = append(targets, rest[i])
		case "--mode":
			if i+1 >= len(rest) {
				return fmt.Errorf("--mode requires an argument (merge/copy/symlink/import)")
			}
			i++
			syncMode = rest[i]
		case "--source":
			if i+1 >= len(rest) {
				return fmt.Errorf("--source requires a path argument")
			}
			i++
			sourceOverride = rest[i]
		case "--file":
			if i+1 >= len(rest) {
				return fmt.Errorf("--file requires a filename")
			}
			i++
			file = rest[i]
		case "--as":
			if i+1 >= len(rest) {
				return fmt.Errorf("--as requires a filename")
			}
			i++
			as = rest[i]
		case "--flatten":
			flatten = true
		case "--force":
			force = true
		case "--no-tui":
			noTUI = true
		case "--help", "-h":
			printExtrasInitHelp()
			return nil
		default:
			if name == "" {
				name = rest[i]
			} else {
				return fmt.Errorf("unexpected argument: %s", rest[i])
			}
		}
	}

	// No arguments at all → ask for each setting
	if name == "" && len(targets) == 0 && syncMode == "" && file == "" && as == "" && shouldLaunchTUI(noTUI, nil) {
		return cmdExtrasInitPrompt(mode, cwd)
	}
	if name == "" {
		return fmt.Errorf("extras name is required: skillshare extras init <name> --target <path>")
	}
	if len(targets) == 0 {
		return fmt.Errorf("at least one --target is required")
	}

	initTargets := make([]extrasInitTarget, 0, len(targets))
	for _, t := range targets {
		initTargets = append(initTargets, extrasInitTarget{path: t, mode: syncMode, flatten: flatten, as: as})
	}
	opts := extrasInitOptions{name: name, source: sourceOverride, file: file, targets: initTargets, force: force}
	if err := validateExtrasInit(opts); err != nil {
		return err
	}

	if mode == modeProject {
		if err := config.ValidateProjectExtraSource(sourceOverride); err != nil {
			return err
		}
		return extrasInitProject(cwd, opts, start)
	}
	return extrasInitGlobal(opts, start)
}

// extrasInitOptions describes the extra that init creates. A non-empty file
// makes it a single-file extra.
type extrasInitOptions struct {
	name    string
	source  string // relative to the project root in project mode
	file    string
	targets []extrasInitTarget
	force   bool
}

func (o extrasInitOptions) extra() config.ExtraConfig {
	extra := config.ExtraConfig{Name: o.name, Source: o.source, File: o.file}
	for _, t := range o.targets {
		et := config.ExtraTargetConfig{Path: t.path, Flatten: t.flatten, As: t.as}
		if t.mode != "" {
			et.Mode = t.mode
		}
		extra.Targets = append(extra.Targets, et)
	}
	return extra
}

// validateExtrasInit checks the flags before anything is written.
func validateExtrasInit(o extrasInitOptions) error {
	if err := config.ValidateExtraName(o.name); err != nil {
		return err
	}
	for _, t := range o.targets {
		if err := config.ValidateExtraMode(t.mode); err != nil {
			return err
		}
		if err := config.ValidateExtraFlatten(t.flatten, t.mode); err != nil {
			return err
		}
		if o.file == "" && t.as != "" {
			return fmt.Errorf("--as requires --file")
		}
		if o.file == "" && t.mode == "import" {
			return fmt.Errorf("import mode requires a single-file extra: add --file <filename>")
		}
	}
	return config.ValidateExtraConfig(o.extra())
}

// checkExtraSourceFile rejects a single-file extra whose source path is a
// directory. A missing file is fine: the user creates it before syncing.
func checkExtraSourceFile(sourceDir, file string) error {
	if file == "" {
		return nil
	}
	path := filepath.Join(sourceDir, file)
	if info, err := os.Stat(path); err == nil && info.IsDir() {
		return fmt.Errorf("source %s is a directory, not a file", path)
	}
	return nil
}

func extrasInitGlobal(o extrasInitOptions, start time.Time) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	if o.force {
		cfg.Extras = removeExtraByName(cfg.Extras, o.name)
	} else if err := config.ValidateExtraNameUnique(o.name, cfg.Extras); err != nil {
		return err
	}

	extra := o.extra()
	sourceDir := config.ResolveExtrasSourceDir(extra, cfg.EffectiveExtrasSource(), cfg.EffectiveSkillsSource())
	if err := checkExtraSourceFile(sourceDir, o.file); err != nil {
		return err
	}
	cfg.Extras = append(cfg.Extras, extra)
	if o.file != "" {
		if err := cfg.ValidateExtras(o.name); err != nil {
			return err
		}
	}

	// Create source directory
	if err := os.MkdirAll(sourceDir, 0755); err != nil {
		return fmt.Errorf("failed to create extras source directory: %w", err)
	}

	if err := cfg.Save(); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	if o.file != "" {
		printSingleFileExtraCreated(extra, sourceDir, "")
	} else {
		ui.Done(ui.MarkOK, fmt.Sprintf("Created extras/%s/ with %s", o.name, plural(len(o.targets), "target")), 0)
		ui.Next("skillshare sync extras", fmt.Sprintf("after adding files to extras/%s/", o.name))
	}

	// Oplog
	e := oplog.NewEntry("extras-init", "ok", time.Since(start))
	e.Args = map[string]any{"name": o.name, "targets": len(o.targets), "scope": "global"}
	if o.file != "" {
		e.Args["file"] = o.file
	}
	oplog.WriteWithLimit(config.ConfigPath(), oplog.OpsFile, e, logMaxEntries()) //nolint:errcheck

	return nil
}

func extrasInitProject(cwd string, o extrasInitOptions, start time.Time) error {
	projCfg, err := config.LoadProject(cwd)
	if err != nil {
		return err
	}

	if o.force {
		projCfg.Extras = removeExtraByName(projCfg.Extras, o.name)
	} else if err := config.ValidateExtraNameUnique(o.name, projCfg.Extras); err != nil {
		return err
	}

	extra := o.extra()
	sourceDir := config.ResolveExtrasSourceDirProject(extra, projCfg.EffectiveExtrasSource(cwd), cwd)
	if err := checkExtraSourceFile(sourceDir, o.file); err != nil {
		return err
	}
	projCfg.Extras = append(projCfg.Extras, extra)
	if o.file != "" {
		if err := projCfg.ValidateExtras(cwd, o.name); err != nil {
			return err
		}
	}

	// Create source directory
	if err := os.MkdirAll(sourceDir, 0755); err != nil {
		return fmt.Errorf("failed to create extras source directory: %w", err)
	}

	if err := projCfg.Save(cwd); err != nil {
		return fmt.Errorf("failed to save project config: %w", err)
	}

	if o.file != "" {
		printSingleFileExtraCreated(extra, sourceDir, " -p")
	} else {
		ui.Done(ui.MarkOK, fmt.Sprintf("Created .skillshare/extras/%s/ with %s", o.name, plural(len(o.targets), "target")), 0)
		ui.Next("skillshare sync extras -p", fmt.Sprintf("after adding files to .skillshare/extras/%s/", o.name))
	}

	// Oplog
	cfgPath := config.ProjectConfigPath(cwd)
	e := oplog.NewEntry("extras-init", "ok", time.Since(start))
	e.Args = map[string]any{"name": o.name, "targets": len(o.targets), "scope": "project"}
	if o.file != "" {
		e.Args["file"] = o.file
	}
	oplog.WriteWithLimit(cfgPath, oplog.OpsFile, e, logMaxEntries()) //nolint:errcheck

	return nil
}

// printSingleFileExtraCreated reports a new single-file extra: its source file
// and the file each target will get, as configured. suffix is appended to the
// sync hint.
func printSingleFileExtraCreated(extra config.ExtraConfig, sourceDir, suffix string) {
	src := filepath.Join(sourceDir, extra.File)
	_, err := os.Stat(src)
	width := ui.RowWidth("Source", "Target")
	if err != nil {
		ui.Row(ui.MarkWarn, "Source", shortenPath(src)+ui.DimText(" · not found"), width)
	} else {
		ui.Row(ui.MarkNone, "Source", shortenPath(src), width)
	}
	for _, t := range extra.Targets {
		ui.Row(ui.MarkNone, "Target", shortenPath(singleFileTargetPath(extra, t))+ui.DimText(" · "+sync.EffectiveMode(t.Mode)), width)
	}
	fmt.Println()
	ui.Done(ui.MarkOK, fmt.Sprintf("Created extra %s (single file)", extra.Name), 0)
	if err != nil {
		ui.Next("skillshare sync extras"+suffix, "after creating the source file")
		return
	}
	ui.Next("skillshare sync extras"+suffix, "sync it")
}

// singleFileTargetPath returns the file a single-file extra's target writes,
// <path>/<as or file>, with the path as configured (relative project paths
// stay relative).
func singleFileTargetPath(extra config.ExtraConfig, t config.ExtraTargetConfig) string {
	name := t.As
	if name == "" {
		name = extra.File
	}
	return filepath.Join(config.ExpandPath(t.Path), name)
}

// removeExtraByName returns a new slice with the named extra removed.
// If no extra matches, the original slice is returned unchanged.
func removeExtraByName(extras []config.ExtraConfig, name string) []config.ExtraConfig {
	result := make([]config.ExtraConfig, 0, len(extras))
	for _, e := range extras {
		if e.Name != name {
			result = append(result, e)
		}
	}
	return result
}

func printExtrasInitHelp() {
	printHelp("skillshare extras init <name> [options]", "Create a new extra resource type: a folder of files, or one file (--file).",
		helpGroup{title: "Arguments", rows: []helpRow{
			{"name", "Name for the extra (e.g., rules, commands, prompts)"},
		}},
		helpGroup{title: "Options", rows: []helpRow{
			{"--target <path>", "Target directory (repeatable)"},
			{"--source <path>", "Custom source directory (overrides extras_source and default;\nrelative to the project root in project mode)"},
			{"--file <filename>", "Sync only this file from the source directory (single-file extra)"},
			{"--as <filename>", "Target filename for every --target (requires --file; default: the --file name)"},
			{"--mode <mode>", "Sync mode: merge (default), copy, symlink; import for single-file extras"},
			{"--flatten", "Flatten files from subdirectories into target root (folder extras only)"},
			{"--force", "Overwrite if extra already exists"},
			{"-p, --project", "Create in project mode (.skillshare/)"},
			{"-g, --global", "Create in global mode (~/.config/skillshare/)"},
			{"--no-tui", "Skip interactive wizard"},
		}},
		helpExamples(
			helpRow{"skillshare extras init rules --target ~/.claude/rules --target ~/.cursor/rules", ""},
			helpRow{"skillshare extras init commands --target ~/.claude/commands --mode copy", ""},
			helpRow{"skillshare extras init rules --source ~/company-shared/rules --target ~/.claude/rules", ""},
			helpRow{"skillshare extras init rules --target ~/.claude/rules --force", ""},
			helpRow{"skillshare extras init agents --target ~/.claude/agents --flatten", ""},
			helpRow{"skillshare extras init prompts --target .claude/prompts -p", ""},
			helpRow{"skillshare extras init review -p --source .skillshare/extras/prompts \\", ""},
			helpRow{"  --file review.md --target .claude/commands", ""},
			helpRow{"skillshare extras init pi-prompt --source ~/dotfiles/prompts --file system.md \\", ""},
			helpRow{"  --target ~/.pi/agent --as APPEND_SYSTEM.md", ""},
		),
	)
}
