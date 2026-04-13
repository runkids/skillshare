package config

import "path/filepath"

// ManagedRulesDir returns the managed rules directory for global or project mode.
func ManagedRulesDir(projectRoot string) string {
	if projectRoot == "" {
		return filepath.Join(BaseDir(), "rules")
	}
	return filepath.Join(projectRoot, ".skillshare", "rules")
}

// ManagedHooksDir returns the managed hooks directory for global or project mode.
func ManagedHooksDir(projectRoot string) string {
	if projectRoot == "" {
		return filepath.Join(BaseDir(), "hooks")
	}
	return filepath.Join(projectRoot, ".skillshare", "hooks")
}
