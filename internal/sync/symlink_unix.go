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
		// Resolve real paths: the OS resolves relative symlinks from
		// the real parent directory, not the lexical one.
		linkDir := evalOrClean(filepath.Dir(linkPath))
		src := evalOrClean(sourcePath)
		if rel, err := filepath.Rel(linkDir, src); err == nil {
			target = rel
		}
	}
	return os.Symlink(target, linkPath)
}

// canCreateRelativeLink returns true on Unix where os.Symlink always works.
func canCreateRelativeLink() bool { return true }

// isJunctionOrSymlink checks if path is a symlink
func isJunctionOrSymlink(path string) bool {
	info, err := os.Lstat(path)
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeSymlink != 0
}
