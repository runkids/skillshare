package main

import (
	"fmt"
	"slices"
	"time"

	"skillshare/internal/config"
	"skillshare/internal/oplog"
	"skillshare/internal/ui"
	versioncheck "skillshare/internal/version"
)

func cmdUpdates(args []string) (err error) {
	if wantsHelp(args) {
		printUpdatesHelp()
		return nil
	}
	jsonMode := slices.Contains(args, "--json")
	defer func() {
		if jsonMode && err != nil {
			err = writeJSONError(err)
		}
	}()
	action := ""
	for _, arg := range args {
		if arg == "--json" {
			continue
		}
		if action != "" {
			return fmt.Errorf("unexpected argument: %s", arg)
		}
		action = arg
	}
	switch action {
	case "", "on", "off":
		if action != "" {
			start := time.Now()
			err := config.SetAutoUpdateCheck(action == "on")
			e := oplog.NewEntry("updates", statusFromErr(err), time.Since(start))
			e.Args = map[string]any{"auto_check": action == "on"}
			if err != nil {
				e.Message = err.Error()
			}
			oplog.WriteWithLimit(config.ConfigPath(), oplog.OpsFile, e, logMaxEntries()) //nolint:errcheck
			if err != nil {
				return err
			}
		}
		p, err := config.ReadUpdatePolicy()
		if err != nil {
			return err
		}
		if jsonMode {
			return writeJSON(p)
		}
		state := "off"
		if p.Enabled {
			state = "on"
		}
		if p.EnvOverride {
			state += " · overridden by " + config.AutoUpdateCheckEnv
		}
		ui.Row(ui.MarkNone, "Automatic update checks", state, ui.RowWidth("Automatic update checks"))
		return nil
	case "check":
		result, err := versioncheck.CheckNow(version, versioncheck.DetectCurrentInstallMethod())
		if err != nil {
			return err
		}
		if jsonMode {
			return writeJSON(result)
		}
		if result.UpdateAvailable {
			ui.UpdateNotification(result.CurrentVersion, result.LatestVersion, result.InstallMethod.UpgradeCommand(), versioncheck.NixConfigUpgradeHint)
		} else {
			ui.Success("CLI is up to date (%s)", result.CurrentVersion)
		}
		return nil
	default:
		return fmt.Errorf("unknown updates action %q (use on, off or check)", action)
	}
}

func printUpdatesHelp() {
	printHelp("skillshare updates [on|off|check] [--json]", "Control automatic version checks globally. Manual checks and upgrades remain available.",
		helpGroup{title: "Actions", rows: []helpRow{
			{"on / off", "Save updates.auto_check in the global config"},
			{"check", "Check the CLI version now, even when automatic checks are off"},
			{"(no action)", "Show the effective preference"},
		}},
		helpGroup{title: "Options", rows: []helpRow{{"--json", "Output JSON"}, {"-h, --help", "Show help"}}},
	)
}
