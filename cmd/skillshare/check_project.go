package main

import (
	"fmt"
	"os"
	"skillshare/internal/config"
)

func cmdCheckProject(root string, opts *checkOptions) error {
	if !projectConfigExists(root) {
		return fmt.Errorf("no project config found in %s", root)
	}

	projectCfg, err := config.LoadProject(root)
	if err != nil {
		return fmt.Errorf("failed to load project config: %w", err)
	}

	var extraNames []string
	for _, t := range projectCfg.Targets {
		if t.Name != "" {
			extraNames = append(extraNames, t.Name)
		}
	}

	sourcePath := projectCfg.EffectiveSkillsSource(root)
	// --all still has the agents to check when the project has no skills.
	if _, err := os.Stat(sourcePath); os.IsNotExist(err) && !opts.allKinds {
		return fmt.Errorf("no project skills directory found")
	}

	opts.parseOpts = parseOptsFromProjectConfig(projectCfg)

	// No names and no groups → check all (existing behavior)
	if len(opts.names) == 0 && len(opts.groups) == 0 {
		agentsDir := ""
		if opts.allKinds {
			agentsDir = projectCfg.EffectiveAgentsSource(root)
		}
		return runCheck(sourcePath, agentsDir, root, opts, extraNames, projectSkillsWalk(root, projectCfg))
	}

	// Filtered check
	return runCheckFiltered(sourcePath, root, opts, projectSkillsWalk(root, projectCfg))
}
