package main

import (
	"fmt"
	"path/filepath"
	"strings"

	"skillshare/internal/config"
	"skillshare/internal/sync"
)

// resolveExtension resolves a target's extension value into a spec.
// A bare name resolves under extensionsDir; a path (contains a separator or
// starts with ".") is used directly. Empty value resolves to (nil, nil).
func resolveExtension(ext, extensionsDir string) (*sync.ExtensionSpec, error) {
	if ext == "" {
		return nil, nil
	}
	var execPath string
	if strings.ContainsAny(ext, "/\\") || strings.HasPrefix(ext, ".") {
		execPath = config.ExpandPath(ext)
	} else {
		execPath = filepath.Join(extensionsDir, ext)
	}
	return sync.LoadExtensionSpec(execPath, ext)
}

// validateExtensionMode ensures a target's mode is compatible with a transform.
// Transforms require copy semantics. Returns the effective mode or an error.
func validateExtensionMode(rawMode string) (string, error) {
	switch rawMode {
	case "", "copy":
		return "copy", nil
	default:
		return "", fmt.Errorf("extension requires copy mode, but mode %q was set on the target", rawMode)
	}
}

// globalExtensionsDir returns the global extensions directory (~/.config/skillshare/extensions).
func globalExtensionsDir() string {
	return filepath.Join(filepath.Dir(config.ConfigPath()), "extensions")
}

// projectExtensionsDir returns the project extensions directory (.skillshare/extensions).
func projectExtensionsDir(projectRoot string) string {
	return filepath.Join(filepath.Dir(config.ProjectConfigPath(projectRoot)), "extensions")
}
