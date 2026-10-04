// Package sourcewalk owns traversal of the skills source. Only the source root
// is resolved by default; an explicit FollowSet enables declared first-level links.
package sourcewalk

import (
	"io/fs"
	"os"
	"path/filepath"

	"skillshare/internal/utils"
)

// TargetDotDirs is the set of hidden directory names (e.g. ".claude", ".cursor")
// that hold target-synced copies rather than source skills. Set by the CLI
// entrypoint from config data to avoid a circular import.
var TargetDotDirs map[string]bool

// SkipTargetDotDir reports whether a directory found while walking the source
// should be skipped as a target dotdir. Only dotdirs nested below the source
// root are skipped: a skill installed from a repo that ships its own
// .claude/skills/ must not surface those as nested skills. Dotdirs directly
// under the root (e.g. ".cursor/skills/*") stay visible because host-style
// paths there are used to infer the skill's targets.
func SkipTargetDotDir(walkRoot, path string, info os.FileInfo) bool {
	if !info.IsDir() || !TargetDotDirs[info.Name()] {
		return false
	}
	return filepath.Dir(path) != walkRoot
}

// Options carries one operation's traversal policy. A nil Follow preserves
// existing traversal and resolved-root callback paths.
type Options struct{ Follow *FollowSet }

// Walk walks the resolved source root with filepath.Walk's callback, ordering,
// SkipDir, and error semantics. With Follow, paths retain the logical root and
// read failures are also recorded in FollowSet.Err; otherwise the base is resolved.
func Walk(root string, opts Options, fn filepath.WalkFunc) error {
	if opts.Follow != nil {
		if opts.Follow.declarationError != nil {
			return opts.Follow.declarationError
		}
		return walkFollow(root, opts.Follow, fn)
	}
	return filepath.Walk(utils.ResolveSymlink(root), fn)
}

// WalkDir is Walk's fs.DirEntry counterpart, with filepath.WalkDir semantics.
func WalkDir(root string, opts Options, fn fs.WalkDirFunc) error {
	if opts.Follow != nil {
		if opts.Follow.declarationError != nil {
			return opts.Follow.declarationError
		}
		return walkDirFollow(root, opts.Follow, fn)
	}
	return filepath.WalkDir(utils.ResolveSymlink(root), fn)
}

// ReadDir reads the resolved source root in filename order. Child links remain
// links, just as with os.ReadDir, unless Follow declares them as followed.
func ReadDir(root string, opts Options) ([]os.DirEntry, error) {
	if opts.Follow != nil {
		if opts.Follow.declarationError != nil {
			return nil, opts.Follow.declarationError
		}
		return readDirFollow(root, opts.Follow)
	}
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
