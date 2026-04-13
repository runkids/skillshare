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

	if mode == modeAuto {
		if projectConfigExists(cwd) {
			mode = modeProject
		} else {
			mode = modeGlobal
		}
	}

	applyModeLabel(mode)

	kind, rest := parseKindArg(rest)
	if kind == kindAgents {
		if hasArg(rest, "--resources") {
			return fmt.Errorf("--resources cannot be used with agents")
		}

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
			summary, err = cmdCollectProjectAgents(cwd, opts, start)
		default:
			cfg, loadErr := config.Load()
			if loadErr != nil {
				err = collectCommandError(loadErr, opts.jsonOutput)
				logCollectOp(cfgPath, start, err, summary)
				return err
			}
			summary, err = cmdCollectAgents(cfg, opts, start)
		}

		logCollectOp(cfgPath, start, err, summary)
		return err
	}

	resources, rest, err := parseResourceFlags(rest, resourceFlagOptions{
		defaultSelection: resourceSelection{skills: true},
	})
	if err != nil {
		return err
	}

	opts := parseCollectOptions(rest)
	scope := "global"
	cfgPath := config.ConfigPath()
	if mode == modeProject {
		scope = "project"
		cfgPath = config.ProjectConfigPath(cwd)
	}

	summary := newCollectLogSummary(kindSkills, scope, opts)
	switch mode {
	case modeProject:
		summary, err = cmdCollectProject(opts, cwd, start, resources)
	default:
		if resources.onlyManaged() {
			summary, err = runManagedOnlyCollect(summary, opts, start, "", resources)
			break
		}
		cfg, loadErr := config.Load()
		if loadErr != nil {
			err = collectCommandError(loadErr, opts.jsonOutput)
			logCollectOp(cfgPath, start, err, summary)
			return err
		}
		summary, err = cmdCollectGlobal(cfg, opts, start, resources)
	}

	logCollectOp(cfgPath, start, err, summary)
	return err
}

func cmdCollectGlobal(cfg *config.Config, opts collectOptions, start time.Time, resources resourceSelection) (collectLogSummary, error) {
	summary := newCollectLogSummary(kindSkills, "global", opts)

	if resources.onlyManaged() {
		if opts.targetName != "" || opts.collectAll {
			return summary, collectCommandError(fmt.Errorf("target selection is only supported when collecting skills"), opts.jsonOutput)
		}
		return runManagedOnlyCollect(summary, opts, start, "", resources)
	}

	targets, err := selectCollectTargets(cfg, opts.targetName, opts.collectAll, opts.jsonOutput)
	if err != nil {
		return summary, collectCommandError(err, opts.jsonOutput)
	}
	if targets == nil {
		return summary, nil
	}

	if !resources.includesManaged() {
		return runCollectPlan(collectPlan{
			kind: kindSkills, source: cfg.Source,
			scan: func(warn bool) collectResources {
				skills := collectLocalSkills(targets, cfg.Source, cfg.Mode, warn)
				return toCollectResources(skills, cfg.Source, skillDisplayItem, sync.PullSkills)
			},
		}, opts, start, "global")
	}

	return runCombinedCollect(summary, opts, start, cfg.Source, cfg.Mode, targets, "", resources)
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
	fmt.Println("  Available targets:")
	for name := range cfg.Targets {
		fmt.Printf("    - %s\n", name)
	}
	return nil, nil
}

func runManagedOnlyCollect(summary collectLogSummary, opts collectOptions, start time.Time, projectRoot string, resources resourceSelection) (collectLogSummary, error) {
	if opts.jsonOutput {
		result, err := collectManagedResources(projectRoot, resources, opts.dryRun, opts.force)
		summary = accumulateCollectLogSummary(summary, result)
		return summary, collectOutputJSON(result, opts.dryRun, start, err)
	}

	result, err := collectManagedResources(projectRoot, resources, opts.dryRun, opts.force)
	summary = accumulateCollectLogSummary(summary, result)
	return summary, renderManagedCollectResult(projectRoot, resources, opts.dryRun, result, err)
}

func runCombinedCollect(
	summary collectLogSummary,
	opts collectOptions,
	start time.Time,
	source string,
	globalMode string,
	targets map[string]config.TargetConfig,
	projectRoot string,
	resources resourceSelection,
) (collectLogSummary, error) {
	var sp *ui.Spinner
	if !opts.jsonOutput {
		ui.Header(ui.WithModeLabel("Collect"))
		sp = ui.StartSpinner("Scanning for local skills...")
	}

	allLocalSkills := collectLocalSkills(targets, source, globalMode, !opts.jsonOutput)
	if len(allLocalSkills) == 0 {
		if sp != nil {
			sp.Success("No local skills found")
		}
	} else if sp != nil {
		sp.Success(fmt.Sprintf("Found %d local skill(s)", len(allLocalSkills)))
		displayLocalCollectItems("Local skills found", skillCollectItems(allLocalSkills))
	}

	if opts.dryRun {
		skillResult := plannedSkillCollectResult(allLocalSkills)
		managedResult, managedErr := collectManagedResources(projectRoot, resources, true, opts.force)
		result := mergePullResults(skillResult, managedResult)
		summary = accumulateCollectLogSummary(summary, result)

		if opts.jsonOutput {
			return summary, collectOutputJSON(result, true, start, managedErr)
		}

		return summary, renderManagedCollectResult(projectRoot, resources, true, managedResult, managedErr)
	}

	if !opts.force && len(allLocalSkills) > 0 {
		if !confirmCollect("skills") {
			ui.Info("Cancelled")
			return summary, nil
		}
	}

	var skillResult *sync.PullResult
	var skillErr error
	if len(allLocalSkills) > 0 {
		skillResult, skillErr = sync.PullSkills(allLocalSkills, source, sync.PullOptions{
			DryRun: false,
			Force:  opts.force,
		})
		summary = accumulateCollectLogSummary(summary, skillResult)
		if !opts.jsonOutput {
			if skillErr == nil {
				skillErr = renderCollectResult("skills", skillResult, source)
			}
		} else {
			skillErr = combineCollectErrors(skillErr, collectResultError(skillResult))
		}
	}

	managedResult, managedErr := collectManagedResources(projectRoot, resources, false, opts.force)
	summary = accumulateCollectLogSummary(summary, managedResult)

	combinedErr := combineCollectErrors(skillErr, managedErr)
	if opts.jsonOutput {
		return summary, collectOutputJSON(mergePullResults(skillResult, managedResult), false, start, combinedErr)
	}

	if renderErr := renderManagedCollectResult(projectRoot, resources, false, managedResult, managedErr); renderErr != nil {
		combinedErr = combineCollectErrors(skillErr, renderErr)
	}
	return summary, combinedErr
}

func skillCollectItems(skills []sync.LocalSkillInfo) []collectDisplayItem {
	items := make([]collectDisplayItem, len(skills))
	for i, skill := range skills {
		items[i] = skillDisplayItem(skill)
	}
	return items
}

func plannedSkillCollectResult(skills []sync.LocalSkillInfo) *sync.PullResult {
	if len(skills) == 0 {
		return nil
	}
	names := make([]string, len(skills))
	for i, skill := range skills {
		names[i] = skill.Name
	}
	return &sync.PullResult{
		Pulled: names,
		Failed: make(map[string]error),
	}
}

func accumulateCollectLogSummary(summary collectLogSummary, result *sync.PullResult) collectLogSummary {
	if result == nil {
		return summary
	}
	summary.Pulled += len(result.Pulled)
	summary.Skipped += len(result.Skipped)
	summary.Failed += len(result.Failed)
	return summary
}

func mergePullResults(left, right *sync.PullResult) *sync.PullResult {
	switch {
	case left == nil:
		return right
	case right == nil:
		return left
	}

	merged := &sync.PullResult{
		Pulled:  append(append([]string{}, left.Pulled...), right.Pulled...),
		Skipped: append(append([]string{}, left.Skipped...), right.Skipped...),
		Failed:  make(map[string]error, len(left.Failed)+len(right.Failed)),
	}
	for name, err := range left.Failed {
		merged.Failed[name] = err
	}
	for name, err := range right.Failed {
		merged.Failed[name] = err
	}
	return merged
}

func collectResultError(result *sync.PullResult) error {
	if result == nil || len(result.Failed) == 0 {
		return nil
	}
	return fmt.Errorf("some skills failed to collect")
}

func combineCollectErrors(errs ...error) error {
	parts := make([]string, 0, len(errs))
	for _, err := range errs {
		if err == nil {
			continue
		}
		parts = append(parts, err.Error())
	}
	if len(parts) == 0 {
		return nil
	}
	return fmt.Errorf("%s", joinErrors(parts))
}

func joinErrors(parts []string) string {
	if len(parts) == 0 {
		return ""
	}
	out := parts[0]
	for i := 1; i < len(parts); i++ {
		out += "; " + parts[i]
	}
	return out
}

func hasArg(args []string, target string) bool {
	for _, arg := range args {
		if arg == target {
			return true
		}
	}
	return false
}

func printCollectHelp() {
	fmt.Println(`Usage: skillshare collect [agents] [target] [options]

Collect local skills, agents, or managed resources from target(s) to source.

Arguments:
  [target]          Target name to collect from (optional; skills only)

Options:
  --resources LIST  Collect only specific resources: skills,rules,hooks
  --all, -a         Collect from all targets
  --dry-run, -n     Preview changes without applying
  --force, -f       Overwrite existing items in source and skip confirmation
  --json            Output results as JSON (implies --force)
  --project, -p     Use project-level config
  --global, -g      Use global config
  --help, -h        Show this help

Examples:
  skillshare collect claude                     Collect skills from the Claude target
  skillshare collect --all                      Collect skills from all targets
  skillshare collect --resources rules,hooks    Collect managed rules and hooks
  skillshare collect --resources skills,hooks   Collect skills and managed hooks
  skillshare collect agents claude              Collect agents from the Claude target
  skillshare collect agents --json              Collect agents as JSON output`)
}
