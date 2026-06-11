package main

import (
	"path/filepath"

	"skillshare/internal/config"
)

func canonicalExtraTargetPath(mode runMode, cwd, path string) string {
	if path == "" {
		return ""
	}
	if mode == modeProject {
		return filepath.Clean(resolveProjectPath(cwd, path))
	}
	return filepath.Clean(config.ExpandPath(path))
}

func storedExtraTargetPath(mode runMode, cwd, path string) string {
	if mode == modeProject {
		return path
	}
	return canonicalExtraTargetPath(mode, cwd, path)
}

func extraTargetPathMatches(mode runMode, cwd, left, right string) bool {
	return canonicalExtraTargetPath(mode, cwd, left) == canonicalExtraTargetPath(mode, cwd, right)
}
