// Package sourcewalk owns traversal of the skills source. Only the source root
// is resolved; links encountered inside the source are never followed.
package sourcewalk

import (
	"io/fs"
	"os"
	"path/filepath"

	"skillshare/internal/utils"
)

// Options reserves the operation's traversal policy. The zero value preserves
// existing traversal; following entries is not implemented.
type Options struct{}

// Walk walks the resolved source root with filepath.Walk's callback, ordering,
// SkipDir, and error semantics. Callback paths use the resolved root as base.
func Walk(root string, _ Options, fn filepath.WalkFunc) error {
	return filepath.Walk(utils.ResolveSymlink(root), fn)
}

// WalkDir is Walk's fs.DirEntry counterpart, with filepath.WalkDir semantics.
func WalkDir(root string, _ Options, fn fs.WalkDirFunc) error {
	return filepath.WalkDir(utils.ResolveSymlink(root), fn)
}

// ReadDir reads the resolved source root in filename order. Child links remain
// links, just as with os.ReadDir.
func ReadDir(root string, _ Options) ([]os.DirEntry, error) {
	resolved := utils.ResolveSymlink(root)
	entries, err := os.ReadDir(resolved)
	if resolved == root {
		return entries, err
	}
	// ReadDir already follows a root link. Preserve its logical error paths,
	// including errors from a later DirEntry.Info call, for existing callers.
	for i, entry := range entries {
		entries[i] = sourceEntry{DirEntry: entry, path: filepath.Join(root, entry.Name())}
	}
	return entries, readErrorPath(err, root)
}

type sourceEntry struct {
	os.DirEntry
	path string
}

func (e sourceEntry) Info() (os.FileInfo, error) {
	info, err := e.DirEntry.Info()
	return info, readErrorPath(err, e.path)
}

func readErrorPath(err error, path string) error {
	if pathErr, ok := err.(*os.PathError); ok {
		copy := *pathErr
		copy.Path = path
		return &copy
	}
	return err
}
