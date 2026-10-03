package main

import (
	"fmt"
	"time"

	"skillshare/internal/config"
	"skillshare/internal/oplog"
	"skillshare/internal/ui"
)

func cmdTUIToggle(args []string) error {
	if wantsHelp(args) {
		printTUIToggleUsage()
		return nil
	}

	cfg, err := config.Load()
	if err != nil {
		return err
	}

	if len(args) == 0 {
		// Show current status
		width := ui.RowWidth("TUI")
		if cfg.TUI == nil {
			ui.Row(ui.MarkNone, "TUI", "on"+ui.DimText(" · default"), width)
		} else if *cfg.TUI {
			ui.Row(ui.MarkNone, "TUI", "on", width)
		} else {
			ui.Row(ui.MarkNone, "TUI", "off", width)
		}
		return nil
	}

	start := time.Now()
	var setErr error

	switch args[0] {
	case "on":
		cfg.TUI = boolPtr(true)
		if err := cfg.Save(); err != nil {
			setErr = fmt.Errorf("failed to save config: %w", err)
		} else {
			ui.Done(ui.MarkOK, "TUI enabled", 0)
		}
	case "off":
		cfg.TUI = boolPtr(false)
		if err := cfg.Save(); err != nil {
			setErr = fmt.Errorf("failed to save config: %w", err)
		} else {
			ui.Done(ui.MarkOK, "TUI disabled", 0)
		}
	default:
		return fmt.Errorf("unknown argument %q: use 'tui on' or 'tui off'", args[0])
	}

	e := oplog.NewEntry("tui", statusFromErr(setErr), time.Since(start))
	e.Args = map[string]any{"value": args[0]}
	oplog.WriteWithLimit(config.ConfigPath(), oplog.OpsFile, e, logMaxEntries()) //nolint:errcheck

	return setErr
}

func printTUIToggleUsage() {
	printHelp("skillshare tui [on|off]", "Toggle interactive TUI mode globally. With no command, shows the current setting.",
		helpGroup{title: "Commands", rows: []helpRow{
			{"on", "Enable TUI for all commands"},
			{"off", "Disable TUI for all commands (plain text output)"},
		}},
		helpNotes("Notes", "The --no-tui flag on individual commands always takes priority."),
	)
}
