package utils

import (
	"fmt"
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

// resolveExisting resolves symlinks in the longest existing prefix of path and
// appends the part that does not exist yet.
func resolveExisting(path string) string {
	path = filepath.Clean(path)
	rest := ""
	for {
		if resolved, err := filepath.EvalSymlinks(path); err == nil {
			return filepath.Join(resolved, rest)
		}
		parent := filepath.Dir(path)
		if parent == path {
			return filepath.Join(path, rest)
		}
		rest = filepath.Join(filepath.Base(path), rest)
		path = parent
	}
}

// ConfigWritePath returns the file a save of the config at path writes: the target
// of a symlink, so the link survives, including a link whose target does not exist
// yet. A project config (<root>/.skillshare/config.yaml) must resolve inside its
// project, so a cloned repository cannot point its config at a file elsewhere.
func ConfigWritePath(path string, project bool) (string, error) {
	root := ""
	if project {
		root = filepath.Dir(filepath.Dir(path))
	}
	return ProjectWritePath(path, root)
}

// ProjectWritePath is ConfigWritePath for any file a project config saves, such as
// an external source it names: with root set, the file must resolve inside root.
func ProjectWritePath(path, root string) (string, error) {
	dest, err := filepath.EvalSymlinks(path)
	if err != nil {
		// The file does not exist yet. Follow every link to the file a write would
		// create, so a link inside the project cannot hand the write to one outside.
		dest = path
		for hops := 0; ; hops++ {
			target, err := os.Readlink(dest)
			if err != nil {
				break
			}
			if hops == 40 {
				return "", fmt.Errorf("%s: too many levels of symbolic links", path)
			}
			if !filepath.IsAbs(target) {
				target = filepath.Join(resolveExisting(filepath.Dir(dest)), target)
			}
			dest = target
		}
		// Resolve every directory that exists, so a missing one cannot hide a link
		// above it that leaves the project.
		dest = resolveExisting(dest)
	}
	if root == "" {
		return dest, nil
	}
	root = resolveExisting(root)
	// A project at a volume root (/ or C:\) already ends with the separator.
	sep := string(filepath.Separator)
	if !PathHasPrefix(dest, strings.TrimSuffix(root, sep)+sep) {
		if info, err := os.Lstat(path); err == nil && info.Mode()&os.ModeSymlink != 0 {
			return "", fmt.Errorf("%s links outside the project to %s; replace the link with a regular file", path, dest)
		}
		return "", fmt.Errorf("%s is outside the project %s", path, root)
	}
	return dest, nil
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
