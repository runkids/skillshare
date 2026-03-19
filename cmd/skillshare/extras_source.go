package main

import (
	"fmt"
	"path/filepath"
	"time"

	"skillshare/internal/config"
	"skillshare/internal/oplog"
	"skillshare/internal/ui"
)

func cmdExtrasSource(args []string) error {
	start := time.Now()

	mode, rest, err := parseModeArgs(args)
	if err != nil {
		return err
	}

	if mode == modeAuto {
		mode = modeGlobal
	}

	applyModeLabel(mode)

	// Project mode does not use extras_source.
	if mode == modeProject {
		return fmt.Errorf("extras source is not supported in project mode (source is always .skillshare/extras/)")
	}

	// Parse args
	var newPath string
	for i := 0; i < len(rest); i++ {
		switch rest[i] {
		case "--help", "-h":
			printExtrasSourceHelp()
			return nil
		default:
			if newPath == "" {
				newPath = rest[i]
			} else {
				return fmt.Errorf("unexpected argument: %s", rest[i])
			}
		}
	}

	cfg, err := config.Load()
	if err != nil {
		return err
	}

	// No argument → show current value
	if newPath == "" {
		current := cfg.ExtrasSource
		if current == "" {
			current = config.ExtrasParentDir(cfg.Source)
			ui.Info("extras_source: %s (default)", shortenPath(current))
		} else {
			ui.Info("extras_source: %s", shortenPath(current))
		}
		return nil
	}

	// Set new value
	absPath, err := filepath.Abs(newPath)
	if err != nil {
		return fmt.Errorf("invalid path: %w", err)
	}

	cfg.ExtrasSource = absPath
	if err := cfg.Save(); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	ui.Success("Set extras_source to %s", shortenPath(absPath))

	e := oplog.NewEntry("extras-source", "ok", time.Since(start))
	e.Args = map[string]any{"path": absPath}
	oplog.WriteWithLimit(config.ConfigPath(), oplog.OpsFile, e, logMaxEntries()) //nolint:errcheck

	return nil
}

func printExtrasSourceHelp() {
	fmt.Println(`Usage: skillshare extras source [path]

Show or set the global extras_source directory.

Without arguments, shows the current extras_source path.
With a path argument, sets extras_source in the global config.

This setting is global-only. Project mode always uses .skillshare/extras/.

Options:
  --help, -h          Show this help

Examples:
  skillshare extras source                          Show current extras_source
  skillshare extras source ~/company-shared/extras  Set extras_source`)
}
