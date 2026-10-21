package projects

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// Identity returns the lexical absolute identity of a project root.
// It uses only the supplied cwd and home, without resolving links or requiring
// the root to exist.
func Identity(arg, cwd, home string) (string, error) {
	if arg == "" || strings.ContainsRune(arg, 0) {
		return "", fmt.Errorf("invalid project root %q", arg)
	}
	path := arg
	if strings.HasPrefix(path, "~") {
		if len(path) > 1 && !os.IsPathSeparator(path[1]) {
			return "", fmt.Errorf("project root %q: ~user paths are not supported", arg)
		}
		if !filepath.IsAbs(home) || strings.ContainsRune(home, 0) {
			return "", fmt.Errorf("project root %q: home must be absolute", arg)
		}
		path = filepath.Join(home, path[1:])
	}
	if runtime.GOOS == "windows" && filepath.VolumeName(path) != "" && !filepath.IsAbs(path) {
		return "", fmt.Errorf("project root %q: drive-relative paths are not supported", arg)
	}
	if !filepath.IsAbs(path) {
		if !filepath.IsAbs(cwd) || strings.ContainsRune(cwd, 0) {
			return "", fmt.Errorf("project root %q: working directory must be absolute", arg)
		}
		if runtime.GOOS == "windows" && os.IsPathSeparator(path[0]) {
			// A rooted path without a volume uses cwd's drive, not its folder.
			path = filepath.VolumeName(cwd) + path
		} else {
			path = filepath.Join(cwd, path)
		}
	}
	return trimTrailingSeparators(filepath.Clean(path)), nil
}

func trimTrailingSeparators(path string) string {
	volume := filepath.VolumeName(path)
	minimum := len(volume)
	// A UNC share is already absolute without a trailing separator. Drive
	// roots need theirs: C:\ must never become the drive-relative C:.
	if !filepath.IsAbs(volume) && len(path) > minimum && os.IsPathSeparator(path[minimum]) {
		minimum++
	}
	for len(path) > minimum && os.IsPathSeparator(path[len(path)-1]) {
		path = path[:len(path)-1]
	}
	return path
}
