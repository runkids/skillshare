package main

import (
	"fmt"
	"os"
	"time"

	"skillshare/internal/config"
	"skillshare/internal/sync"
	"skillshare/internal/ui"
)

// collectLocalSkills collects local skills from targets (non-symlinked).
func collectLocalSkills(targets map[string]config.TargetConfig, source, globalMode string, warn bool) []sync.LocalSkillInfo {
	var allLocalSkills []sync.LocalSkillInfo
	for name, target := range targets {
		sc := target.SkillsConfig()
		mode := sync.EffectiveMode(sc.Mode)
		if sc.Mode == "" && globalMode != "" {
			mode = globalMode
		}
		skills, err := sync.FindLocalSkills(sc.Path, source, mode)
		if err != nil {
			if warn {
				ui.Warning("%s: %v", name, err)
			}
			continue
		}
		for i := range skills {
			skills[i].TargetName = name
		}
		allLocalSkills = append(allLocalSkills, skills...)
	}
	return allLocalSkills
}

func skillDisplayItem(s sync.LocalSkillInfo) collectDisplayItem {
	return collectDisplayItem{Name: s.Name, TargetName: s.TargetName, Path: s.Path}
}

func cmdCollect(args []string) error {
	if wantsHelp(args) {
		printCollectHelp()
		return nil
	}

	start := time.Now()

	mode, rest, err := parseModeArgs(args)
	if err != nil {
		return err
	}

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("cannot determine working directory: %w", err)
	}

	mode = resolveAutoMode(mode, cwd)

	applyModeLabel(mode)

	kind, rest := parseKindArg(rest)
	opts := parseCollectOptions(rest)
	scope := "global"
	cfgPath := config.ConfigPath()
	if mode == modeProject {
		scope = "project"
		cfgPath = config.ProjectConfigPath(cwd)
	}

	summary := newCollectLogSummary(kind, scope, opts)

	switch mode {
	case modeProject:
		if kind == kindAgents {
			summary, err = cmdCollectProjectAgents(cwd, opts, start)
		} else {
			summary, err = cmdCollectProject(opts, cwd, start)
		}
	default:
		cfg, loadErr := config.LoadWithoutProjects()
		if loadErr != nil {
			err = collectCommandError(loadErr, opts.jsonOutput)
			logCollectOp(cfgPath, start, err, summary)
			return err
		}
		if kind == kindAgents {
			summary, err = cmdCollectAgents(cfg, opts, start)
		} else {
			summary, err = cmdCollectGlobal(cfg, opts, start)
		}
	}

	logCollectOp(cfgPath, start, err, summary)
	return err
}

func cmdCollectGlobal(cfg *config.Config, opts collectOptions, start time.Time) (collectLogSummary, error) {
	summary := newCollectLogSummary(kindSkills, "global", opts)

	targets, err := selectCollectTargets(cfg, opts.targetName, opts.collectAll, opts.jsonOutput)
	if err != nil {
		return summary, collectCommandError(err, opts.jsonOutput)
	}
	if targets == nil {
		return summary, nil
	}

	return runCollectPlan(collectPlan{
		kind: kindSkills, source: cfg.EffectiveSkillsSource(), follow: globalSkillFollowSet(cfg),
		scan: func(warn bool) collectResources {
			skills := collectLocalSkills(targets, cfg.EffectiveSkillsSource(), cfg.Mode, warn)
			return toCollectResources(skills, cfg.EffectiveSkillsSource(), skillDisplayItem, sync.PullSkills)
		},
	}, opts, start, "global")
}

func selectCollectTargets(cfg *config.Config, targetName string, collectAll, jsonOutput bool) (map[string]config.TargetConfig, error) {
	if targetName != "" {
		if t, exists := cfg.Targets[targetName]; exists {
			return map[string]config.TargetConfig{targetName: t}, nil
		}
		return nil, fmt.Errorf("target '%s' not found", targetName)
	}

	if len(cfg.Targets) == 0 {
		return cfg.Targets, nil
	}

	if collectAll || len(cfg.Targets) == 1 {
		return cfg.Targets, nil
	}

	if jsonOutput {
		return nil, fmt.Errorf("multiple targets found; specify a target name or use --all")
	}

	ui.Warning("Multiple targets found. Specify a target name or use --all")
	ui.Note("Available targets:")
	for name := range cfg.Targets {
		ui.Note("  - " + name)
	}
	return nil, nil
}

func printCollectHelp() {
	printHelp("skillshare collect [agents] [target] [options]", "Collect local skills or agents from target(s) to the source directory.",
		helpGroup{title: "Arguments", rows: []helpRow{
			{"[target]", "Target name to collect from (optional)"},
		}},
		helpGroup{title: "Options", rows: []helpRow{
			{"-a, --all", "Collect from all targets"},
			{"-n, --dry-run", "Preview changes without applying"},
			{"-f, --force", "Overwrite existing items in source and skip confirmation"},
			{"--json", "Output results as JSON (implies --force)"},
			{"-p, --project", "Use project-level config"},
			{"-g, --global", "Use global config"},
		}},
		helpExamples(
			helpRow{"skillshare collect claude", "Collect skills from the Claude target"},
			helpRow{"skillshare collect --all", "Collect skills from all targets"},
			helpRow{"skillshare collect --dry-run", "Preview what would be collected"},
			helpRow{"skillshare collect agents claude", "Collect agents from the Claude target"},
			helpRow{"skillshare collect agents --json", "Collect agents as JSON output"},
		),
	)
}
