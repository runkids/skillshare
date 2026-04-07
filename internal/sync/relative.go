package sync

import (
	"path/filepath"
	"strings"
)

// shouldUseRelative returns true if both sourcePath and targetPath
// are under the given projectRoot, meaning a relative symlink
// between them would be portable across machines.
// Returns false if projectRoot is empty (global mode).
func shouldUseRelative(projectRoot, sourcePath, targetPath string) bool {
	if projectRoot == "" {
		return false
	}
	root := filepath.Clean(projectRoot) + string(filepath.Separator)
	src := filepath.Clean(sourcePath)
	tgt := filepath.Clean(targetPath)

	return (strings.HasPrefix(src, root) || src == filepath.Clean(projectRoot)) &&
		(strings.HasPrefix(tgt, root) || tgt == filepath.Clean(projectRoot))
}
