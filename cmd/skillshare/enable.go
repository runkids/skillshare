package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
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

	// Extract --kind flag before parsing other args
	kind, rest, err := parseKindFlag(rest)
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

	mode = resolveAutoMode(mode, cwd)
	applyModeLabel(mode)

	isAgent := kind == kindAgents

	var ignorePath string
	var cfgPath string
	if mode == modeProject {
		projectCfg, loadErr := config.LoadProject(cwd)
		if loadErr != nil {
			return fmt.Errorf("failed to load project config: %w", loadErr)
		}
		if isAgent {
			ignorePath = filepath.Join(projectCfg.EffectiveAgentsSource(cwd), ".agentignore")
		} else {
			ignorePath = filepath.Join(projectCfg.EffectiveSkillsSource(cwd), ".skillignore")
		}
		cfgPath = config.ProjectConfigPath(cwd)
	} else {
		cfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}
		if isAgent {
			ignorePath = filepath.Join(cfg.EffectiveAgentsSource(), ".agentignore")
		} else {
			ignorePath = filepath.Join(cfg.EffectiveSkillsSource(), ".skillignore")
		}
		cfgPath = config.ConfigPath()
	}

	ignoreLabel := ".skillignore"
	if isAgent {
		ignoreLabel = ".agentignore"
	}

	width := ui.RowWidth(patterns...)
	changed := false
	for _, pattern := range patterns {
		if dryRun {
			if enable {
				ui.Row(ui.MarkNone, pattern, "would be removed from "+shortenPath(ignorePath), width)
			} else {
				ui.Row(ui.MarkNone, pattern, "would be added to "+shortenPath(ignorePath), width)
			}
			continue
		}

		if enable {
			removed, err := skillignore.RemovePattern(ignorePath, pattern)
			if err != nil {
				return fmt.Errorf("failed to update %s: %w", ignoreLabel, err)
			}
			if !removed {
				ui.Row(ui.MarkWarn, pattern, "not disabled", width)
				continue
			}
			changed = true
			ui.Row(ui.MarkOK, pattern, "removed from "+ignoreLabel, width)
		} else {
			added, err := skillignore.AddPattern(ignorePath, pattern)
			if err != nil {
				return fmt.Errorf("failed to update %s: %w", ignoreLabel, err)
			}
			if !added {
				ui.Row(ui.MarkWarn, pattern, "already disabled", width)
				continue
			}
			changed = true
			ui.Row(ui.MarkOK, pattern, "added to "+ignoreLabel, width)
		}
	}

	if dryRun {
		fmt.Println()
		ui.DryRun()
	}
	if !dryRun && changed {
		ui.Next("skillshare sync", "apply the change")

		e := oplog.NewEntry(action, "ok", time.Since(start))
		e.Args = map[string]any{
			"patterns": patterns,
			"kind":     kind.String(),
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
	printHelp("skillshare "+action+" <name|pattern> [options]",
		strings.ToUpper(action[:1])+action[1:]+" skills by adding/removing patterns from .skillignore.",
		helpGroup{title: "Arguments", rows: []helpRow{
			{"<name|pattern>", "Skill name or glob pattern (e.g. \"my-skill\", \"draft-*\")"},
		}},
		helpGroup{title: "Options", rows: []helpRow{
			{"-p, --project", "Use project-mode .skillignore"},
			{"-g, --global", "Use global-mode .skillignore"},
			{"-n, --dry-run", "Preview changes without writing"},
		}},
		helpNotes("See also", "skillshare "+opposite),
	)
}
