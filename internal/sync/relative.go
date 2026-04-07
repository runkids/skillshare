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
	prefix := cleaned
	if prefix != string(filepath.Separator) {
		prefix += string(filepath.Separator)
	}
	src := filepath.Clean(sourcePath)
	tgt := filepath.Clean(targetPath)

	srcUnder := utils.PathHasPrefix(src, prefix) || utils.PathsEqual(src, cleaned)
	tgtUnder := utils.PathHasPrefix(tgt, prefix) || utils.PathsEqual(tgt, cleaned)
	return srcUnder && tgtUnder
}

// resolveReadlink converts a raw os.Readlink result to an absolute path,
// resolving relative targets against the link's parent directory (not CWD).
func resolveReadlink(dest, linkPath string) string {
	if filepath.IsAbs(dest) {
		return dest
	}
	return filepath.Clean(filepath.Join(filepath.Dir(linkPath), dest))
}

// linkNeedsReformat returns true if dest (the raw os.Readlink result)
// uses the wrong format (relative vs absolute) for the desired mode.
func linkNeedsReformat(dest string, wantRelative bool) bool {
	if dest == "" {
		return false
	}
	return wantRelative == filepath.IsAbs(dest)
}
