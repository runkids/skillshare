package main

import (
	"fmt"
	"time"

	"skillshare/internal/config"
	"skillshare/internal/sync"
	"skillshare/internal/ui"
)

func collectLocalAgents(targets map[string]string, source string, warn bool) []sync.LocalAgentInfo {
	var allLocalAgents []sync.LocalAgentInfo
	for name, targetPath := range targets {
		agents, err := sync.FindLocalAgents(targetPath, source)
		if err != nil {
			if warn {
				ui.Warning("%s: %v", name, err)
			}
			continue
		}
		for i := range agents {
			agents[i].TargetName = name
		}
		allLocalAgents = append(allLocalAgents, agents...)
	}
	return allLocalAgents
}

func agentDisplayItem(a sync.LocalAgentInfo) collectDisplayItem {
	return collectDisplayItem{Name: a.Name, TargetName: a.TargetName, Path: a.Path}
}

func cmdCollectAgents(cfg *config.Config, opts collectOptions, start time.Time) (collectLogSummary, error) {
	summary := newCollectLogSummary(kindAgents, "global", opts)

	targets, err := selectCollectAgentTargets(cfg, opts.targetName, opts.collectAll, opts.jsonOutput)
	if err != nil {
		return summary, collectCommandError(err, opts.jsonOutput)
	}
	if targets == nil {
		return summary, nil
	}

	source := cfg.EffectiveAgentsSource()
	return runCollectPlan(collectPlan{
		kind: kindAgents, source: source,
		scan: func(warn bool) collectResources {
			agents := collectLocalAgents(targets, source, warn)
			return toCollectResources(agents, source, agentDisplayItem, sync.PullAgents)
		},
	}, opts, start, "global")
}

func selectCollectAgentTargets(cfg *config.Config, targetName string, collectAll, jsonOutput bool) (map[string]string, error) {
	builtinAgents := config.DefaultAgentTargets()

	if targetName != "" {
		target, ok := cfg.Targets[targetName]
		if !ok {
			return nil, fmt.Errorf("target '%s' not found", targetName)
		}
		agentPath := resolveAgentTargetPath(target, builtinAgents, targetName)
		if agentPath == "" {
			return nil, fmt.Errorf("target '%s' does not support agents", targetName)
		}
		return map[string]string{targetName: agentPath}, nil
	}

	targets := make(map[string]string)
	for name := range cfg.Targets {
		agentPath := resolveAgentTargetPath(cfg.Targets[name], builtinAgents, name)
		if agentPath == "" {
			continue
		}
		targets[name] = agentPath
	}

	if len(targets) == 0 {
		return targets, nil
	}

	if collectAll || len(targets) <= 1 {
		return targets, nil
	}

	if jsonOutput {
		return nil, fmt.Errorf("multiple targets found; specify a target name or use --all")
	}

	ui.Warning("Multiple targets found. Specify a target name or use --all")
	fmt.Println("  Available targets:")
	for name := range targets {
		fmt.Printf("    - %s\n", name)
	}
	return nil, nil
}

func cmdCollectProjectAgents(projectRoot string, opts collectOptions, start time.Time) (collectLogSummary, error) {
	summary := newCollectLogSummary(kindAgents, "project", opts)

	projCfg, err := config.LoadProject(projectRoot)
	if err != nil {
		return summary, collectCommandError(fmt.Errorf("cannot load project config: %w", err), opts.jsonOutput)
	}

	targets, err := selectCollectProjectAgentTargets(projCfg, projectRoot, opts.targetName, opts.collectAll, opts.jsonOutput)
	if err != nil {
		return summary, collectCommandError(err, opts.jsonOutput)
	}
	if targets == nil {
		return summary, nil
	}

	source := projCfg.EffectiveAgentsSource(projectRoot)
	return runCollectPlan(collectPlan{
		kind: kindAgents, source: source,
		scan: func(warn bool) collectResources {
			agents := collectLocalAgents(targets, source, warn)
			return toCollectResources(agents, source, agentDisplayItem, sync.PullAgents)
		},
	}, opts, start, "project")
}

func selectCollectProjectAgentTargets(projCfg *config.ProjectConfig, projectRoot, targetName string, collectAll, jsonOutput bool) (map[string]string, error) {
	builtinAgents := config.ProjectAgentTargets()

	if targetName != "" {
		for _, entry := range projCfg.Targets {
			if entry.Name != targetName {
				continue
			}
			agentPath := resolveProjectAgentTargetPath(entry, builtinAgents, projectRoot)
			if agentPath == "" {
				return nil, fmt.Errorf("target '%s' does not support agents in project config", targetName)
			}
			return map[string]string{targetName: agentPath}, nil
		}
		return nil, fmt.Errorf("target '%s' not found in project config", targetName)
	}

	targets := make(map[string]string)
	for _, entry := range projCfg.Targets {
		agentPath := resolveProjectAgentTargetPath(entry, builtinAgents, projectRoot)
		if agentPath == "" {
			continue
		}
		targets[entry.Name] = agentPath
	}

	if len(targets) == 0 {
		return targets, nil
	}

	if collectAll || len(targets) <= 1 {
		return targets, nil
	}

	if jsonOutput {
		return nil, fmt.Errorf("multiple targets found; specify a target name or use --all")
	}

	ui.Warning("Multiple targets found. Specify a target name or use --all")
	fmt.Println("  Available targets:")
	for _, entry := range projCfg.Targets {
		if _, ok := targets[entry.Name]; ok {
			fmt.Printf("    - %s\n", entry.Name)
		}
	}
	return nil, nil
}
