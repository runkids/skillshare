//go:build !windows

package sync

import (
	"os"
	"path/filepath"
)

// createLink creates a symlink on Unix systems.
// If relative is true, the symlink stores a relative path from linkPath's
// directory to sourcePath. Falls back to absolute if filepath.Rel fails.
func createLink(linkPath, sourcePath string, relative bool) error {
	target := sourcePath
	if relative {
		rel, err := filepath.Rel(filepath.Dir(linkPath), sourcePath)
		if err == nil {
			target = rel
		}
	}
	return os.Symlink(target, linkPath)
}

// isJunctionOrSymlink checks if path is a symlink
func isJunctionOrSymlink(path string) bool {
	info, err := os.Lstat(path)
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeSymlink != 0
}
