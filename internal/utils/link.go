package utils

import (
	"io/fs"
	"os"
	"path/filepath"
)

// IsSymlinkOrJunction checks whether path is a symlink or Windows junction.
func IsSymlinkOrJunction(path string) bool {
	info, err := os.Lstat(path)
	if err != nil {
		return false
	}
	return IsLinkMode(path, info.Mode())
}

// IsLinkMode reports whether the entry at path, whose Lstat or DirEntry mode
// is m, is a symlink or a Windows junction. Since Go 1.23 a junction is
// reported as ModeIrregular rather than ModeSymlink (and not as a directory),
// so only such entries cost an extra system call to read the reparse tag.
func IsLinkMode(path string, m fs.FileMode) bool {
	if m&fs.ModeSymlink != 0 {
		return true
	}
	return m&fs.ModeIrregular != 0 && isJunction(path)
}

// IsJunction reports whether path is a Windows junction, as opposed to a symlink.
// With GODEBUG=winsymlink=0 Go reports a junction as ModeSymlink, so check both.
func IsJunction(path string) bool {
	info, err := os.Lstat(path)
	return err == nil && info.Mode()&(fs.ModeIrregular|fs.ModeSymlink) != 0 && isJunction(path)
}

// isJunction reports whether path is a Windows junction (mount point).
// A variable so tests on other platforms can simulate one.
var isJunction = platformIsJunction

// ResolveLinkTarget resolves the target path of a symlink or junction.
func ResolveLinkTarget(path string) (string, error) {
	return resolveLinkTarget(path, true)
}

func resolveLinkTarget(path string, physicalParent bool) (string, error) {
	link, err := os.Readlink(path)
	if err == nil {
		if !filepath.IsAbs(link) {
			parent := filepath.Dir(path)
			if physicalParent {
				// The OS interprets a relative link from its physical directory,
				// not from an alias used to reach that directory.
				parent = CanonicalPath(parent)
			}
			link = filepath.Join(parent, link)
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
