package main

import (
	"fmt"
	"os"
	"path/filepath"
	"skillshare/internal/config"
	"skillshare/internal/ui"
)

func cmdCheckProject(root string, opts *checkOptions) error {
	if !projectConfigExists(root) {
		return fmt.Errorf("no project config found in %s", root)
	}

	sourcePath := filepath.Join(root, ".skillshare", "skills")
	if _, err := os.Stat(sourcePath); os.IsNotExist(err) {
		return fmt.Errorf("no project skills directory found")
	}

	var extraNames []string
	projectCfg, err := config.LoadProject(root)
	if err != nil {
		ui.Warning("Failed to load project config for target validation: %v", err)
	} else {
		for _, t := range projectCfg.Targets {
			if t.Name != "" {
				extraNames = append(extraNames, t.Name)
			}
		}
	}

	// No names and no groups → check all (existing behavior)
	if len(opts.names) == 0 && len(opts.groups) == 0 {
		return runCheck(sourcePath, opts.json, extraNames)
	}

	// Filtered check
	return runCheckFiltered(sourcePath, opts)
}
