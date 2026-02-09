package main

import (
	"fmt"
	"os"
	"path/filepath"
)

func cmdCheckProject(root string, jsonOutput bool) error {
	if !projectConfigExists(root) {
		return fmt.Errorf("no project config found in %s", root)
	}

	sourcePath := filepath.Join(root, ".skillshare", "skills")
	if _, err := os.Stat(sourcePath); os.IsNotExist(err) {
		return fmt.Errorf("no project skills directory found")
	}

	return runCheck(sourcePath, jsonOutput)
}
