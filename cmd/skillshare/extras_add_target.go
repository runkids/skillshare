package main

import (
	"fmt"
	"os"
	"time"

	"skillshare/internal/config"
	"skillshare/internal/oplog"
	"skillshare/internal/ui"
)

// cmdExtrasAddTarget handles `skillshare extras <name> --add-target <path>
// [--mode <m>] [--flatten]`. It adds a new target to an existing extra.
// Config-only: it does not sync — the user runs `skillshare sync extras`.
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

	var name, addPath, syncMode string
	var flatten bool
	for i := 0; i < len(rest); i++ {
		switch rest[i] {
		case "--add-target":
			if i+1 >= len(rest) {
				return fmt.Errorf("--add-target requires a path argument")
			}
			i++
			addPath = rest[i]
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
			if name == "" {
				name = rest[i]
			} else {
				return fmt.Errorf("unexpected argument: %s", rest[i])
			}
		}
	}

	if name == "" {
		return fmt.Errorf("extras name is required: skillshare extras <name> --add-target <path>")
	}
	if addPath == "" {
		return fmt.Errorf("--add-target requires a path argument")
	}
	if err := config.ValidateExtraMode(syncMode); err != nil {
		return err
	}
	if err := config.ValidateExtraFlatten(flatten, syncMode); err != nil {
		return err
	}

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

	idx := -1
	for i := range extras {
		if extras[i].Name == name {
			idx = i
			break
		}
	}
	if idx == -1 {
		return fmt.Errorf("extra %q not found", name)
	}
	for _, t := range extras[idx].Targets {
		if extraTargetPathMatches(mode, cwd, t.Path, addPath) {
			return fmt.Errorf("target %q already exists on extra %q — use --mode/--flatten to change its settings", addPath, name)
		}
	}

	storedPath := storedExtraTargetPath(mode, cwd, addPath)
	et := config.ExtraTargetConfig{Path: storedPath, Flatten: flatten}
	if syncMode != "" {
		et.Mode = syncMode
	}
	extras[idx].Targets = append(extras[idx].Targets, et)

	if err := saveFn(); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	ui.Success("Added target %s to %s", shortenPath(addPath), name)
	ui.Info("Run 'skillshare sync extras%s' to sync the new target", projectSuffix(mode))

	e := oplog.NewEntry("extras-target", "ok", time.Since(start))
	e.Args = map[string]any{"name": name, "target": addPath, "action": "add", "mode": syncMode}
	oplog.WriteWithLimit(configPath, oplog.OpsFile, e, logMaxEntries()) //nolint:errcheck

	return nil
}

func printExtrasAddTargetHelp() {
	fmt.Println(`Usage: skillshare extras <name> --add-target <path> [options]

Add a new target directory to an existing extra. Config-only — run
'skillshare sync extras' afterwards to sync files into the new target.

Options:
  --add-target <path>   Target directory to add (required)
  --mode <mode>         Sync mode for the new target: merge (default), copy, symlink
  --flatten             Flatten subdirectory files into the target root
  --project, -p         Use project mode (.skillshare/)
  --global, -g          Use global mode (~/.config/skillshare/)
  --help, -h            Show this help

Examples:
  skillshare extras rules --add-target ~/.cursor/rules
  skillshare extras commands --add-target ~/.config/opencode/commands --mode copy`)
}

// projectSuffix returns " -p" in project mode for use in user-facing hints.
func projectSuffix(mode runMode) string {
	if mode == modeProject {
		return " -p"
	}
	return ""
}
