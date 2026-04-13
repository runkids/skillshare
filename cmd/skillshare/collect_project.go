package main

import (
	"fmt"
	"time"

	"skillshare/internal/config"
	"skillshare/internal/sync"
	"skillshare/internal/ui"
)

func cmdCollectProject(opts collectOptions, root string, start time.Time, resources resourceSelection) (collectLogSummary, error) {
	summary := newCollectLogSummary(kindSkills, "project", opts)

	if resources.onlyManaged() {
		if opts.targetName != "" || opts.collectAll {
			return summary, collectCommandError(fmt.Errorf("target selection is only supported when collecting skills"), opts.jsonOutput)
		}
		return runManagedOnlyCollect(summary, opts, start, root, resources)
	}

	runtime, err := loadProjectRuntime(root)
	if err != nil {
		return summary, collectCommandError(err, opts.jsonOutput)
	}

	targets, err := selectCollectProjectTargets(runtime, opts.targetName, opts.collectAll, opts.jsonOutput)
	if err != nil {
		return summary, collectCommandError(err, opts.jsonOutput)
	}
	if targets == nil {
		return summary, nil
	}

	if !resources.includesManaged() {
		return runCollectPlan(collectPlan{
			kind: kindSkills, source: runtime.sourcePath,
			scan: func(warn bool) collectResources {
				skills := collectLocalSkills(targets, runtime.sourcePath, "", warn)
				return toCollectResources(skills, runtime.sourcePath, skillDisplayItem, sync.PullSkills)
			},
		}, opts, start, "project")
	}

	return runCombinedCollect(summary, opts, start, runtime.sourcePath, "", targets, root, resources)
}

func selectCollectProjectTargets(runtime *projectRuntime, targetName string, collectAll, jsonOutput bool) (map[string]config.TargetConfig, error) {
	if targetName != "" {
		if t, ok := runtime.targets[targetName]; ok {
			return map[string]config.TargetConfig{targetName: t}, nil
		}
		return nil, fmt.Errorf("target '%s' not found in project config", targetName)
	}

	if len(runtime.targets) == 0 {
		return runtime.targets, nil
	}

	if collectAll || len(runtime.targets) == 1 {
		return runtime.targets, nil
	}

	if jsonOutput {
		return nil, fmt.Errorf("multiple targets found; specify a target name or use --all")
	}

	ui.Warning("Multiple targets found. Specify a target name or use --all")
	fmt.Println("  Available targets:")
	for _, entry := range runtime.config.Targets {
		fmt.Printf("    - %s\n", entry.Name)
	}
	return nil, nil
}
