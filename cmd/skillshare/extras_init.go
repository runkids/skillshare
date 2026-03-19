package main

import (
	"fmt"
	"os"
	"time"

	"skillshare/internal/config"
	"skillshare/internal/oplog"
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
	var targets []string
	var syncMode string
	var sourceOverride string
	var force bool
	var noTUI bool
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
				return fmt.Errorf("--mode requires an argument (merge/copy/symlink)")
			}
			i++
			syncMode = rest[i]
		case "--source":
			if i+1 >= len(rest) {
				return fmt.Errorf("--source requires a path argument")
			}
			i++
			sourceOverride = rest[i]
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

	// No arguments at all → launch interactive TUI wizard
	if name == "" && len(targets) == 0 && syncMode == "" && shouldLaunchTUI(noTUI, nil) {
		return cmdExtrasInitTUI(mode, cwd)
	}
	if name == "" {
		return fmt.Errorf("extras name is required: skillshare extras init <name> --target <path>")
	}
	if len(targets) == 0 {
		return fmt.Errorf("at least one --target is required")
	}

	// Validate name
	if err := config.ValidateExtraName(name); err != nil {
		return err
	}

	// Validate sync mode
	if err := config.ValidateExtraMode(syncMode); err != nil {
		return err
	}

	if mode == modeProject {
		if sourceOverride != "" {
			return fmt.Errorf("--source is not supported in project mode (source is always .skillshare/extras/<name>/)")
		}
		return extrasInitProject(cwd, name, targets, syncMode, force, start)
	}
	return extrasInitGlobal(name, targets, syncMode, sourceOverride, force, start)
}

func extrasInitGlobal(name string, targets []string, syncMode string, sourceOverride string, force bool, start time.Time) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	if force {
		cfg.Extras = removeExtraByName(cfg.Extras, name)
	} else if err := config.ValidateExtraNameUnique(name, cfg.Extras); err != nil {
		return err
	}

	// Backfill extras_source if not set
	if cfg.ExtrasSource == "" {
		cfg.ExtrasSource = config.ExtrasParentDir(cfg.Source)
	}

	// Build extra config
	extra := config.ExtraConfig{Name: name, Source: sourceOverride}
	for _, t := range targets {
		et := config.ExtraTargetConfig{Path: t}
		if syncMode != "" {
			et.Mode = syncMode
		}
		extra.Targets = append(extra.Targets, et)
	}

	// Create source directory
	sourceDir := config.ResolveExtrasSourceDir(extra, cfg.ExtrasSource, cfg.Source)
	if err := os.MkdirAll(sourceDir, 0755); err != nil {
		return fmt.Errorf("failed to create extras source directory: %w", err)
	}

	// Add to config and save
	cfg.Extras = append(cfg.Extras, extra)
	if err := cfg.Save(); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	ui.Success("Created extras/%s/ with %d target(s)", name, len(targets))
	ui.Info("Add files to extras/%s/ then run 'skillshare sync extras'", name)

	// Oplog
	e := oplog.NewEntry("extras-init", "ok", time.Since(start))
	e.Args = map[string]any{"name": name, "targets": len(targets), "scope": "global"}
	oplog.WriteWithLimit(config.ConfigPath(), oplog.OpsFile, e, logMaxEntries()) //nolint:errcheck

	return nil
}

func extrasInitProject(cwd, name string, targets []string, syncMode string, force bool, start time.Time) error {
	projCfg, err := config.LoadProject(cwd)
	if err != nil {
		return err
	}

	if force {
		projCfg.Extras = removeExtraByName(projCfg.Extras, name)
	} else if err := config.ValidateExtraNameUnique(name, projCfg.Extras); err != nil {
		return err
	}

	extra := config.ExtraConfig{Name: name}
	for _, t := range targets {
		et := config.ExtraTargetConfig{Path: t}
		if syncMode != "" {
			et.Mode = syncMode
		}
		extra.Targets = append(extra.Targets, et)
	}

	// Create source directory
	sourceDir := config.ExtrasSourceDirProject(cwd, name)
	if err := os.MkdirAll(sourceDir, 0755); err != nil {
		return fmt.Errorf("failed to create extras source directory: %w", err)
	}

	// Add to config and save
	projCfg.Extras = append(projCfg.Extras, extra)
	if err := projCfg.Save(cwd); err != nil {
		return fmt.Errorf("failed to save project config: %w", err)
	}

	ui.Success("Created .skillshare/extras/%s/ with %d target(s)", name, len(targets))
	ui.Info("Add files to .skillshare/extras/%s/ then run 'skillshare sync extras -p'", name)

	// Oplog
	cfgPath := config.ProjectConfigPath(cwd)
	e := oplog.NewEntry("extras-init", "ok", time.Since(start))
	e.Args = map[string]any{"name": name, "targets": len(targets), "scope": "project"}
	oplog.WriteWithLimit(cfgPath, oplog.OpsFile, e, logMaxEntries()) //nolint:errcheck

	return nil
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
	fmt.Println(`Usage: skillshare extras init <name> [options]

Create a new extra resource type.

Arguments:
  name                Name for the extra (e.g., rules, commands, prompts)

Options:
  --target <path>     Target directory (repeatable)
  --source <path>     Custom source directory (overrides extras_source and default; global mode only)
  --mode <mode>       Sync mode: merge (default), copy, symlink
  --force             Overwrite if extra already exists
  --project, -p       Create in project mode (.skillshare/)
  --global, -g        Create in global mode (~/.config/skillshare/)
  --no-tui            Skip interactive wizard
  --help, -h          Show this help

Examples:
  skillshare extras init rules --target ~/.claude/rules --target ~/.cursor/rules
  skillshare extras init commands --target ~/.claude/commands --mode copy
  skillshare extras init rules --source ~/company-shared/rules --target ~/.claude/rules
  skillshare extras init rules --target ~/.claude/rules --force
  skillshare extras init prompts --target .claude/prompts -p`)
}
