package main

import (
	"fmt"

	"skillshare/internal/config"
	"skillshare/internal/ui"
)

func cmdTUIToggle(args []string) error {
	if len(args) > 0 && (args[0] == "-h" || args[0] == "--help") {
		printTUIToggleUsage()
		return nil
	}

	cfg, err := config.Load()
	if err != nil {
		return err
	}

	if len(args) == 0 {
		// Show current status
		if cfg.TUI == nil {
			ui.Info("TUI: on (default)")
		} else if *cfg.TUI {
			ui.Info("TUI: on")
		} else {
			ui.Info("TUI: off")
		}
		return nil
	}

	switch args[0] {
	case "on":
		v := true
		cfg.TUI = &v
		if err := cfg.Save(); err != nil {
			return fmt.Errorf("failed to save config: %w", err)
		}
		ui.Success("TUI enabled")
	case "off":
		v := false
		cfg.TUI = &v
		if err := cfg.Save(); err != nil {
			return fmt.Errorf("failed to save config: %w", err)
		}
		ui.Success("TUI disabled")
	default:
		return fmt.Errorf("unknown argument %q: use 'tui on' or 'tui off'", args[0])
	}

	return nil
}

func printTUIToggleUsage() {
	fmt.Println("Usage: skillshare tui [on|off]")
	fmt.Println()
	fmt.Println("Toggle interactive TUI mode globally.")
	fmt.Println()
	fmt.Println("  tui        Show current TUI status")
	fmt.Println("  tui on     Enable TUI for all commands")
	fmt.Println("  tui off    Disable TUI for all commands (plain text output)")
	fmt.Println()
	fmt.Println("The --no-tui flag on individual commands always takes priority.")
}
