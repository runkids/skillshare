package utils

import (
	"os"
	"path/filepath"
)

// IsSymlinkOrJunction checks whether path is a symlink or Windows junction.
func IsSymlinkOrJunction(path string) bool {
	info, err := os.Lstat(path)
	if err != nil {
		return false
	}

	if info.Mode()&os.ModeSymlink != 0 {
		return true
	}

	return isWindowsReparsePoint(path)
}

// ResolveLinkTarget resolves the target path of a symlink or junction.
func ResolveLinkTarget(path string) (string, error) {
	link, err := os.Readlink(path)
	if err == nil {
		if !filepath.IsAbs(link) {
			link = filepath.Join(filepath.Dir(path), link)
		}
		return filepath.Abs(link)
	}

	// On Windows, Readlink may fail for junctions. EvalSymlinks still resolves.
	resolved, evalErr := filepath.EvalSymlinks(path)
	if evalErr != nil {
		return "", err
	}
	return filepath.Abs(resolved)
}
