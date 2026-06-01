package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"skillshare/internal/config"
	"skillshare/internal/oplog"
	"skillshare/internal/ui"
)

func cmdExtrasAddTarget(args []string) error {
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

	var name, targetPath, syncMode string
	var flatten bool
	for i := 0; i < len(rest); i++ {
		switch rest[i] {
		case "--target":
			if i+1 >= len(rest) {
				return fmt.Errorf("--target requires a path argument")
			}
			i++
			targetPath = rest[i]
		case "--mode":
			if i+1 >= len(rest) {
				return fmt.Errorf("--mode requires an argument (merge/copy/symlink)")
			}
			i++
			syncMode = rest[i]
		case "--flatten":
			flatten = true
		case "--help", "-h":
			printExtrasAddTargetHelp()
			return nil
		default:
			if len(rest[i]) > 0 && rest[i][0] == '-' {
				return fmt.Errorf("unexpected flag: %s", rest[i])
			}
			if name == "" {
				name = rest[i]
			} else {
				return fmt.Errorf("unexpected argument: %s", rest[i])
			}
		}
	}

	if name == "" {
		return fmt.Errorf("extras name is required: skillshare extras add-target <name> --target <path>")
	}
	if targetPath == "" {
		return fmt.Errorf("--target is required")
	}
	targetPath = filepath.Clean(config.ExpandPath(targetPath))

	if err := config.ValidateExtraMode(syncMode); err != nil {
		return err
	}
	if flatten {
		if err := config.ValidateExtraFlatten(true, syncMode); err != nil {
			return err
		}
	}

	// Load config based on mode
	var extras []config.ExtraConfig
	var configPath string
	var saveFn func() error

	if mode == modeProject {
		projCfg, loadErr := config.LoadProject(cwd)
		if loadErr != nil {
			return loadErr
		}
		extras = projCfg.Extras
		configPath = config.ProjectConfigPath(cwd)
		saveFn = func() error { return projCfg.Save(cwd) }
	} else {
		cfg, loadErr := config.Load()
		if loadErr != nil {
			return loadErr
		}
		extras = cfg.Extras
		configPath = config.ConfigPath()
		saveFn = cfg.Save
	}

	// Locate the extra and verify target uniqueness.
	idx := -1
	for i, e := range extras {
		if e.Name == name {
			idx = i
			break
		}
	}
	if idx == -1 {
		return fmt.Errorf("extra %q not found", name)
	}
	for _, t := range extras[idx].Targets {
		if t.Path == targetPath {
			return fmt.Errorf("target %q already exists for extra %q", targetPath, name)
		}
	}

	newTarget := config.ExtraTargetConfig{Path: targetPath, Flatten: flatten}
	if syncMode != "" {
		newTarget.Mode = syncMode
	}
	extras[idx].Targets = append(extras[idx].Targets, newTarget)

	if err := saveFn(); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	ui.Success("Added target to %s: %s (mode=%s)", name, shortenPath(targetPath), syncMode)

	e := oplog.NewEntry("extras-add-target", "ok", time.Since(start))
	e.Args = map[string]any{"name": name, "target": targetPath, "mode": syncMode, "flatten": flatten}
	oplog.WriteWithLimit(configPath, oplog.OpsFile, e, logMaxEntries()) //nolint:errcheck

	return nil
}

func printExtrasAddTargetHelp() {
	fmt.Println(`Usage: skillshare extras add-target <name> --target <path> [options]

Add a new target directory to an existing extra.

Arguments:
  name                Name of the existing extra (e.g., rules, commands)

Options:
  --target <path>     Target directory to add (required)
  --mode <mode>       Sync mode: merge (default), copy, or symlink
  --flatten           Flatten files from subdirectories into target root
  --project, -p       Use project mode (.skillshare/)
  --global, -g        Use global mode (~/.config/skillshare/)
  --help, -h          Show this help

Examples:
  skillshare extras add-target rules --target ~/.cursor/rules
  skillshare extras add-target commands --target ~/.claude/commands --mode copy
  skillshare extras add-target agents --target ~/.claude/agents --flatten -p`)
}
