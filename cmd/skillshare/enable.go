package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"skillshare/internal/config"
	"skillshare/internal/oplog"
	"skillshare/internal/skillignore"
	"skillshare/internal/ui"
)

func cmdDisable(args []string) error {
	return cmdToggleSkill(args, false)
}

func cmdEnable(args []string) error {
	return cmdToggleSkill(args, true)
}

func cmdToggleSkill(args []string, enable bool) error {
	start := time.Now()
	action := "disable"
	if enable {
		action = "enable"
	}

	mode, rest, err := parseModeArgs(args)
	if err != nil {
		return err
	}

	var dryRun bool
	var patterns []string
	for _, arg := range rest {
		switch arg {
		case "--dry-run", "-n":
			dryRun = true
		case "--help", "-h":
			printToggleHelp(action)
			return nil
		default:
			if len(arg) > 0 && arg[0] == '-' {
				return fmt.Errorf("unknown flag: %s", arg)
			}
			patterns = append(patterns, arg)
		}
	}

	if len(patterns) == 0 {
		return fmt.Errorf("usage: skillshare %s <name|pattern>", action)
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

	var ignorePath string
	var cfgPath string
	if mode == modeProject {
		ignorePath = filepath.Join(cwd, ".skillshare", "skills", ".skillignore")
		cfgPath = config.ProjectConfigPath(cwd)
	} else {
		cfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}
		ignorePath = filepath.Join(cfg.Source, ".skillignore")
		cfgPath = config.ConfigPath()
	}

	for _, pattern := range patterns {
		if dryRun {
			if enable {
				ui.Info("Would remove %q from %s", pattern, ignorePath)
			} else {
				ui.Info("Would add %q to %s", pattern, ignorePath)
			}
			continue
		}

		if enable {
			removed, err := skillignore.RemovePattern(ignorePath, pattern)
			if err != nil {
				return fmt.Errorf("failed to update .skillignore: %w", err)
			}
			if !removed {
				ui.Warning("%s is not disabled", pattern)
				continue
			}
			ui.Success("Enabled: %s (removed from .skillignore)", pattern)
		} else {
			if skillignore.HasPattern(ignorePath, pattern) {
				ui.Warning("%s is already disabled", pattern)
				continue
			}
			if err := skillignore.AddPattern(ignorePath, pattern); err != nil {
				return fmt.Errorf("failed to update .skillignore: %w", err)
			}
			ui.Success("Disabled: %s (added to .skillignore)", pattern)
		}
	}

	if !dryRun {
		ui.Info("Run \"skillshare sync\" to apply changes.")

		e := oplog.NewEntry(action, "ok", time.Since(start))
		e.Args = map[string]any{
			"patterns": patterns,
		}
		oplog.Write(cfgPath, oplog.OpsFile, e)
	}

	return nil
}

func printToggleHelp(action string) {
	opposite := "enable"
	if action == "enable" {
		opposite = "disable"
	}
	fmt.Printf(`Usage: skillshare %s <name|pattern> [flags]

%s skills by adding/removing patterns from .skillignore.

Arguments:
  <name|pattern>  Skill name or glob pattern (e.g. "my-skill", "draft-*")

Flags:
  -p, --project   Use project-mode .skillignore
  -g, --global    Use global-mode .skillignore
  -n, --dry-run   Preview changes without writing
  -h, --help      Show this help

See also: skillshare %s
`, action, capitalizeFirst(action), opposite)
}

func capitalizeFirst(s string) string {
	if s == "" {
		return s
	}
	return string(s[0]-32) + s[1:]
}
