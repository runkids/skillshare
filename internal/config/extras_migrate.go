package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// MigrateExtrasDir moves legacy extras directories from configDir/name/ to configDir/extras/name/.
// Only names listed in extras config are migrated. Returns warnings for cases needing user attention.
func MigrateExtrasDir(configDir string, extras []ExtraConfig) []string {
	var warnings []string

	for _, extra := range extras {
		name := extra.Name
		oldPath := filepath.Join(configDir, name)
		newPath := filepath.Join(configDir, "extras", name)

		oldExists := dirExists(oldPath)
		newExists := dirExists(newPath)

		if !oldExists {
			// Nothing to migrate (neither exists, or only new exists)
			continue
		}

		if oldExists && newExists {
			warnings = append(warnings, fmt.Sprintf("both legacy %s/ and extras/%s/ exist; using new", name, name))
			continue
		}

		// Old exists, new does not — perform migration
		if err := os.MkdirAll(filepath.Join(configDir, "extras"), 0755); err != nil {
			warnings = append(warnings, fmt.Sprintf("failed to create extras/ dir for %s: %v; using legacy path", name, err))
			continue
		}

		if err := os.Rename(oldPath, newPath); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				// Already migrated by another process — idempotent no-op
				continue
			}
			warnings = append(warnings, fmt.Sprintf("failed to migrate %s/ to extras/%s/: %v; using legacy path", name, name, err))
			continue
		}

		fmt.Printf("  Migrated %s/ → extras/%s/\n", name, name)
	}

	return warnings
}

// dirExists reports whether path exists and is a directory.
func dirExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.IsDir()
}
