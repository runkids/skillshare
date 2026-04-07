package sync

import (
	"fmt"
	"os"
	"path/filepath"

	"skillshare/internal/utils"
)

// shouldUseRelative returns true if both sourcePath and targetPath
// are under the given projectRoot, meaning a relative symlink
// between them would be portable across machines.
// Returns false if projectRoot is empty (global mode).
// Paths are resolved through EvalSymlinks so that symlinked ancestors
// do not fool the prefix check.
func shouldUseRelative(projectRoot, sourcePath, targetPath string) bool {
	if projectRoot == "" {
		return false
	}
	cleaned := evalOrClean(projectRoot)
	prefix := cleaned
	if prefix != string(filepath.Separator) {
		prefix += string(filepath.Separator)
	}
	src := evalOrClean(sourcePath)
	tgt := evalOrClean(targetPath)

	srcUnder := utils.PathHasPrefix(src, prefix) || utils.PathsEqual(src, cleaned)
	tgtUnder := utils.PathHasPrefix(tgt, prefix) || utils.PathsEqual(tgt, cleaned)
	return srcUnder && tgtUnder
}

// evalOrClean resolves symlinks in path, falling back to filepath.Clean
// when the path does not exist on disk (e.g. during tests).
func evalOrClean(p string) string {
	if resolved, err := filepath.EvalSymlinks(p); err == nil {
		return resolved
	}
	return filepath.Clean(p)
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
	if wantRelative && !canCreateRelativeLink() {
		// Platform cannot create relative symlinks (e.g. Windows without
		// Developer Mode falls back to junctions, which are always absolute).
		// Skip reformat to avoid remove→recreate loop on every sync.
		return false
	}
	return wantRelative == filepath.IsAbs(dest)
}

// reformatLink atomically replaces an existing symlink with a new one
// using the specified format. It creates a temp link and renames it
// over the original so the link is never missing. Falls back to
// remove→create when rename fails (e.g. Windows junctions).
func reformatLink(linkPath, sourcePath string, relative bool) error {
	tmpPath := linkPath + ".ss-reformat"
	os.Remove(tmpPath) // clean up stale temp
	if err := createLink(tmpPath, sourcePath, relative); err != nil {
		return err
	}
	if err := os.Rename(tmpPath, linkPath); err == nil {
		return nil
	}
	// Rename failed (Windows junction, cross-device, etc.); fall back
	os.Remove(tmpPath)
	if err := os.Remove(linkPath); err != nil {
		return fmt.Errorf("failed to remove old link: %w", err)
	}
	return createLink(linkPath, sourcePath, relative)
}
