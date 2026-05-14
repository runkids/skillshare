package utils

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// PathsEqual compares two paths for equality.
// On Windows, paths are compared case-insensitively since NTFS is case-insensitive.
// On Unix systems, paths are compared exactly.
func PathsEqual(a, b string) bool {
	if runtime.GOOS == "windows" {
		return strings.EqualFold(a, b)
	}
	return a == b
}

// PathHasPrefix checks if path starts with prefix.
// On Windows, comparison is case-insensitive.
// On Unix systems, comparison is exact.
// ResolveSymlink resolves symlinks on a path so filepath.Walk enters
// symlinked directories. Falls back to the original path on error.
func ResolveSymlink(path string) string {
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		return resolved
	}
	return path
}

func PathHasPrefix(path, prefix string) bool {
	if runtime.GOOS == "windows" {
		return strings.HasPrefix(strings.ToLower(path), strings.ToLower(prefix))
	}
	return strings.HasPrefix(path, prefix)
}

// FoldHomePath folds an absolute path under the user's home directory back to
// the ~ form for stable, machine-agnostic YAML serialization.
//
// Returns the original path unchanged when:
//   - input is empty
//   - user home cannot be determined
//   - path is not under the home prefix (strict match on home + separator)
//   - path already starts with ~
//
// On Windows ~ is increasingly accepted but not universally honored by shells;
// we still fold to keep behavior parallel across OSes (downstream consumers
// already expand via ExpandPath/normalizeTargetPath).
//
// fork patch: preserve ~ in saved config paths (iFwu/skillshare).
func FoldHomePath(path string) string {
	if path == "" || HasTildePrefix(path) {
		return path
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return path
	}
	return FoldHomePathWith(path, home)
}

// FoldHomePathWith is the home-explicit form of FoldHomePath. Callers that
// fold many paths in a tight loop should resolve the home directory once
// (it can involve a syscall on Windows) and pass it here directly.
func FoldHomePathWith(path, home string) string {
	if path == "" || HasTildePrefix(path) || home == "" {
		return path
	}
	if PathsEqual(path, home) {
		return "~"
	}
	sep := string(filepath.Separator)
	if PathHasPrefix(path, home+sep) {
		return "~" + path[len(home):]
	}
	return path
}
