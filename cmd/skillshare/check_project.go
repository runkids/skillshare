package main

import (
	"fmt"
	"os"
	"path/filepath"
	"skillshare/internal/config"
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
	if projectCfg, err := config.LoadProject(root); err == nil {
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
