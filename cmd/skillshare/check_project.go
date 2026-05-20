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
	if _, err := os.Stat(sourcePath); os.IsNotExist(err) {
		return fmt.Errorf("no project skills directory found")
	}

	// No names and no groups → check all (existing behavior)
	if len(opts.names) == 0 && len(opts.groups) == 0 {
		return runCheck(sourcePath, opts.json, extraNames)
	}

	// Filtered check
	return runCheckFiltered(sourcePath, opts)
}
