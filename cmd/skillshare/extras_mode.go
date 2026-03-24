package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	"skillshare/internal/config"
	"skillshare/internal/oplog"
	"skillshare/internal/ui"
)

func cmdExtrasMode(args []string) error {
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
	var flattenSet bool
	var flattenVal bool
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
			flattenSet = true
			flattenVal = true
		case "--no-flatten":
			flattenSet = true
			flattenVal = false
		case "--help", "-h":
			printExtrasModeHelp()
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
		return fmt.Errorf("extras name is required: skillshare extras <name> --mode <mode> [--target <path>]")
	}
	if syncMode == "" && !flattenSet {
		return fmt.Errorf("--mode or --flatten/--no-flatten is required")
	}

	if syncMode != "" {
		if err := config.ValidateExtraMode(syncMode); err != nil {
			return err
		}
	}

	// Load config to resolve target when --target is omitted
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

	// Auto-resolve target when omitted
	if targetPath == "" {
		for _, extra := range extras {
			if extra.Name == name {
				switch len(extra.Targets) {
				case 0:
					return fmt.Errorf("extra %q has no targets", name)
				case 1:
					targetPath = extra.Targets[0].Path
				default:
					return fmt.Errorf("extra %q has %d targets — use --target to specify which one", name, len(extra.Targets))
				}
				break
			}
		}
		if targetPath == "" {
			return fmt.Errorf("extra %q not found", name)
		}
	}

	if syncMode != "" {
		if err := setExtraTargetMode(extras, name, targetPath, syncMode); err != nil {
			return err
		}
	}

	// Apply flatten if requested
	if flattenSet {
		effectiveMode := syncMode
		if effectiveMode == "" {
			// Look up current mode from config
			for _, extra := range extras {
				if extra.Name == name {
					for _, t := range extra.Targets {
						if t.Path == targetPath {
							effectiveMode = t.Mode
							break
						}
					}
					break
				}
			}
		}
		if err := config.ValidateExtraFlatten(flattenVal, effectiveMode); err != nil {
			return err
		}
		if err := setExtraTargetFlatten(extras, name, targetPath, flattenVal); err != nil {
			return err
		}
	}

	if err := saveFn(); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	// Build success message
	var parts []string
	if syncMode != "" {
		parts = append(parts, fmt.Sprintf("mode=%s", syncMode))
	}
	if flattenSet {
		parts = append(parts, fmt.Sprintf("flatten=%v", flattenVal))
	}
	ui.Success("Updated %s target %s: %s", name, shortenPath(targetPath), strings.Join(parts, ", "))

	e := oplog.NewEntry("extras-mode", "ok", time.Since(start))
	e.Args = map[string]any{"name": name, "target": targetPath, "mode": syncMode, "flatten": flattenVal}
	oplog.WriteWithLimit(configPath, oplog.OpsFile, e, logMaxEntries()) //nolint:errcheck

	return nil
}

// setExtraTargetMode finds an extra by name and sets the mode on a specific target.
// Operates on the extras slice in-place (caller must save config).
func setExtraTargetMode(extras []config.ExtraConfig, name, targetPath, mode string) error {
	for i, extra := range extras {
		if extra.Name != name {
			continue
		}
		for j, t := range extra.Targets {
			if t.Path == targetPath {
				extras[i].Targets[j].Mode = mode
				return nil
			}
		}
		return fmt.Errorf("target %q not found in extra %q", targetPath, name)
	}
	return fmt.Errorf("extra %q not found", name)
}

// setExtraTargetFlatten finds an extra by name and sets flatten on a specific target.
func setExtraTargetFlatten(extras []config.ExtraConfig, name, targetPath string, flatten bool) error {
	for i, extra := range extras {
		if extra.Name != name {
			continue
		}
		for j, t := range extra.Targets {
			if t.Path == targetPath {
				extras[i].Targets[j].Flatten = flatten
				return nil
			}
		}
		return fmt.Errorf("target %q not found in extra %q", targetPath, name)
	}
	return fmt.Errorf("extra %q not found", name)
}

func printExtrasModeHelp() {
	fmt.Println(`Usage: skillshare extras mode <name> --mode <mode> [--target <path>]
       skillshare extras <name> --mode <mode> [--target <path>]

Change the sync mode or flatten setting of an extra's target.

Arguments:
  name                Extra name (e.g., rules, commands)

Options:
  --mode <mode>       New sync mode: merge, copy, or symlink
  --flatten           Enable flatten (sync subdirectory files into target root)
  --no-flatten        Disable flatten
  --target <path>     Target directory path (optional if extra has only one target)
  --project, -p       Use project mode (.skillshare/)
  --global, -g        Use global mode (~/.config/skillshare/)
  --help, -h          Show this help

Examples:
  skillshare extras rules --mode copy
  skillshare extras agents --flatten
  skillshare extras mode rules --target ~/.claude/rules --mode copy
  skillshare extras mode agents --target ~/.claude/agents --flatten -p`)
}
