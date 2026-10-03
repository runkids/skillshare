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
		effective := cfg.EffectiveExtrasSource()
		value := shortenPath(effective)
		if cfg.Sources.Extras == "" && cfg.ExtrasSource == "" {
			value += ui.DimText(" (default)")
		}
		ui.Row(ui.MarkNone, "extras_source", value, ui.RowWidth("extras_source"))
		return nil
	}

	// Set new value
	absPath, err := filepath.Abs(newPath)
	if err != nil {
		return fmt.Errorf("invalid path: %w", err)
	}

	// Write to whichever field the user already uses; default to legacy field
	// for fresh configs to preserve the existing on-disk shape. If the user
	// has migrated to sources.extras, update that to avoid being shadowed.
	if cfg.Sources.Extras != "" {
		cfg.Sources.Extras = absPath
	} else {
		cfg.ExtrasSource = absPath
	}
	if err := cfg.Save(); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	ui.Done(ui.MarkOK, "Set extras_source to "+shortenPath(absPath), 0)

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
