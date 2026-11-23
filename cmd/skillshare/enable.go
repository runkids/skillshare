package main

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"skillshare/internal/config"
	"skillshare/internal/oplog"
	"skillshare/internal/toggle"
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

	mode, rest, err := parseModeArgs(args, "--kind")
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

	src := toggle.Source{Agent: isAgent}
	var cfgPath string
	if mode == modeProject {
		projectCfg, loadErr := config.LoadProject(cwd)
		if loadErr != nil {
			return fmt.Errorf("failed to load project config: %w", loadErr)
		}
		if isAgent {
			src.Root = projectCfg.EffectiveAgentsSource(cwd)
		} else {
			src.Root, src.Walk = projectCfg.EffectiveSkillsSource(cwd), projectSkillsWalk(cwd, projectCfg)
		}
		cfgPath = config.ProjectConfigPath(cwd)
	} else {
		cfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}
		if isAgent {
			src.Root = cfg.EffectiveAgentsSource()
		} else {
			src.Root, src.Walk = cfg.EffectiveSkillsSource(), cfg.SkillsWalk()
		}
		cfgPath = config.ConfigPath()
	}
	ignorePath := src.IgnorePath()
	ignoreLabel := filepath.Base(ignorePath)

	set, err := src.Discover()
	if err != nil {
		return err
	}
	argLines := make([][]string, len(patterns))
	argItems := make([][]toggle.Item, len(patterns))
	var lines, relPaths []string
	for i, pattern := range patterns {
		argLines[i], argItems[i] = set.Lines(pattern, enable)
		lines = append(lines, argLines[i]...)
		for _, it := range argItems[i] {
			relPaths = append(relPaths, it.RelPath)
		}
	}

	if dryRun {
		width := ui.RowWidth(lines...)
		for _, line := range lines {
			if enable {
				ui.Row(ui.MarkNone, line, "would be removed from "+shortenPath(ignorePath), width)
			} else {
				ui.Row(ui.MarkNone, line, "would be added to "+shortenPath(ignorePath), width)
			}
		}
		fmt.Println()
		ui.DryRun()
		return nil
	}

	res, err := src.Apply(enable, lines, relPaths)
	if err != nil {
		return err
	}

	done, unchanged := "added to "+ignoreLabel, "already disabled"
	if enable {
		done, unchanged = "removed from "+ignoreLabel, "not disabled"
	}
	type row struct{ mark, label, value string }
	var rows []row
	var failed []row
	printed := map[string]bool{}
	applied := 0
	ok := func(line string) {
		if !printed[line] {
			printed[line] = true
			applied++
			rows = append(rows, row{ui.MarkOK, line, done})
		}
	}
	next := 0 // res.Items follows relPaths, argument by argument
	for i := range patterns {
		if len(argItems[i]) == 0 {
			if line := argLines[i][0]; res.Changed[line] {
				ok(line)
			} else {
				rows = append(rows, row{ui.MarkWarn, line, unchanged})
			}
			continue
		}
		for range argItems[i] {
			out := res.Items[next]
			next++
			switch {
			case out.Blocked != "":
				r := row{ui.MarkFail, out.RelPath, out.Blocked}
				rows, failed = append(rows, r), append(failed, r)
			case len(out.Lines) > 0:
				for _, line := range out.Lines {
					ok(line)
				}
			default:
				rows = append(rows, row{ui.MarkWarn, out.RelPath, unchanged})
			}
		}
	}

	// A single failure is reported once, by the final error line, which Herdr reads.
	var cmdErr error
	switch len(failed) {
	case 0:
	case 1:
		cmdErr = fmt.Errorf("could not %s %s: %s", action, failed[0].label, failed[0].value)
		rows = slices.DeleteFunc(rows, func(r row) bool { return r == failed[0] })
	default:
		names := make([]string, len(failed))
		for i, r := range failed {
			names[i] = r.label
		}
		cmdErr = fmt.Errorf("could not %s %s", action, strings.Join(names, ", "))
	}
	labels := make([]string, len(rows))
	for i, r := range rows {
		labels[i] = r.label
	}
	width := ui.RowWidth(labels...)
	for _, r := range rows {
		ui.Row(r.mark, r.label, r.value, width)
	}

	if applied > 0 {
		ui.Next("skillshare sync", "apply the change")
	}
	if applied > 0 || cmdErr != nil {
		status := statusFromErr(cmdErr)
		if cmdErr != nil && applied > 0 {
			status = "partial"
		}
		e := oplog.NewEntry(action, status, time.Since(start))
		e.Args = map[string]any{
			"patterns": patterns,
			"kind":     kind.String(),
		}
		if cmdErr != nil {
			e.Message = cmdErr.Error()
		}
		oplog.Write(cfgPath, oplog.OpsFile, e)
	}
	return cmdErr
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
