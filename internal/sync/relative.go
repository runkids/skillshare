package sync

import (
	"path/filepath"

	"skillshare/internal/utils"
)

// shouldUseRelative returns true if both sourcePath and targetPath
// are under the given projectRoot, meaning a relative symlink
// between them would be portable across machines.
// Returns false if projectRoot is empty (global mode).
func shouldUseRelative(projectRoot, sourcePath, targetPath string) bool {
	if projectRoot == "" {
		return false
	}
	cleaned := filepath.Clean(projectRoot)
	prefix := cleaned + string(filepath.Separator)
	src := filepath.Clean(sourcePath)
	tgt := filepath.Clean(targetPath)

	srcUnder := utils.PathHasPrefix(src, prefix) || utils.PathsEqual(src, cleaned)
	tgtUnder := utils.PathHasPrefix(tgt, prefix) || utils.PathsEqual(tgt, cleaned)
	return srcUnder && tgtUnder
}
