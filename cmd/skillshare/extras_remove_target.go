package main

import (
	"fmt"
	"os"
	"time"

	"skillshare/internal/config"
	"skillshare/internal/oplog"
	"skillshare/internal/ui"
)

func cmdExtrasRemoveTarget(args []string) error {
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

	var name, targetPath string
	for i := 0; i < len(rest); i++ {
		switch rest[i] {
		case "--target":
			if i+1 >= len(rest) {
				return fmt.Errorf("--target requires a path argument")
			}
			i++
			targetPath = rest[i]
		case "--help", "-h":
			printExtrasRemoveTargetHelp()
			return nil
		default:
			if name == "" {
				name = rest[i]
			} else {
				return fmt.Errorf("unexpected argument: %s", rest[i])
			}
		}
	}

	if name == "" {
		return fmt.Errorf("extras name is required: skillshare extras remove-target <name> --target <path>")
	}
	if targetPath == "" {
		return fmt.Errorf("--target is required")
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

	// Locate the extra.
	extraIdx := -1
	for i, e := range extras {
		if e.Name == name {
			extraIdx = i
			break
		}
	}
	if extraIdx == -1 {
		return fmt.Errorf("extra %q not found", name)
	}

	// Locate the target.
	targetIdx := -1
	for j, t := range extras[extraIdx].Targets {
		if t.Path == targetPath {
			targetIdx = j
			break
		}
	}
	if targetIdx == -1 {
		return fmt.Errorf("target %q not found in extra %q", targetPath, name)
	}

	// Splice out the target. NOTE: we deliberately do NOT touch files on disk —
	// already-synced files in the target directory remain. Same contract as the
	// HTTP DELETE handler.
	ts := extras[extraIdx].Targets
	extras[extraIdx].Targets = append(ts[:targetIdx], ts[targetIdx+1:]...)

	if err := saveFn(); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	ui.Success("Removed target from %s: %s", name, shortenPath(targetPath))

	e := oplog.NewEntry("extras-remove-target", "ok", time.Since(start))
	e.Args = map[string]any{"name": name, "target": targetPath}
	oplog.WriteWithLimit(configPath, oplog.OpsFile, e, logMaxEntries()) //nolint:errcheck

	return nil
}

func printExtrasRemoveTargetHelp() {
	fmt.Println(`Usage: skillshare extras remove-target <name> --target <path> [options]

Remove a target directory from an existing extra.

Files already synced to the target directory are NOT deleted — only the
config entry is removed. Run 'skillshare sync extras' afterwards to clean
up orphaned links.

Arguments:
  name                Name of the existing extra

Options:
  --target <path>     Target directory to remove (required)
  --project, -p       Use project mode (.skillshare/)
  --global, -g        Use global mode (~/.config/skillshare/)
  --help, -h          Show this help

Examples:
  skillshare extras remove-target rules --target ~/.cursor/rules
  skillshare extras remove-target agents --target .claude/agents -p`)
}
