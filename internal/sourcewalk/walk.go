// Package sourcewalk owns traversal of the skills source. The source root is
// resolved; links inside the source are not followed, except first-level
// directory links when the operation's Options carry a Follow policy.
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
// paths there are used to infer the skill's targets. The root itself is never
// skipped, even when its name matches (e.g. a source named ".skillshare").
func SkipTargetDotDir(walkRoot, path string, info os.FileInfo) bool {
	if path == walkRoot || !info.IsDir() || !TargetDotDirs[info.Name()] {
		return false
	}
	return filepath.Dir(path) != walkRoot
}

// Options is the operation's traversal policy. The zero value follows no links.
type Options struct {
	// Follow, when set, follows directory links directly under the source root.
	Follow *Follow
}

// Walk walks the resolved source root with filepath.Walk's callback, ordering,
// SkipDir, and error semantics. Callback paths use the resolved root as base.
// A followed link is reported under its own name as a directory and its
// target's contents are walked under that logical path.
func Walk(root string, opts Options, fn filepath.WalkFunc) error {
	resolved := utils.ResolveSymlink(root)
	f := opts.Follow
	if f == nil {
		return filepath.Walk(resolved, fn)
	}
	return filepath.Walk(resolved, func(path string, info os.FileInfo, err error) error {
		if err != nil || !utils.IsLinkMode(path, info.Mode()) || !f.firstLevel(path) {
			return fn(path, info, err)
		}
		target, dirInfo, ok := f.check(path)
		if !ok {
			return fn(path, info, err)
		}
		if err := fn(path, dirInfo, nil); err != nil {
			if err == filepath.SkipDir {
				return nil
			}
			return err
		}
		var stop error
		err = filepath.Walk(target, func(p string, i os.FileInfo, e error) error {
			f.recordWalkError(path, target, p, e)
			if p == target {
				if e == nil {
					return nil // already reported under the link's name
				}
				i = dirInfo
			}
			r := fn(path+p[len(target):], i, e)
			if r == filepath.SkipAll {
				stop = r
			}
			return r
		})
		if stop != nil {
			return stop
		}
		return err
	})
}

// WalkDir is Walk's fs.DirEntry counterpart, with filepath.WalkDir semantics.
func WalkDir(root string, opts Options, fn fs.WalkDirFunc) error {
	resolved := utils.ResolveSymlink(root)
	f := opts.Follow
	if f == nil {
		return filepath.WalkDir(resolved, fn)
	}
	return filepath.WalkDir(resolved, func(path string, d fs.DirEntry, err error) error {
		if err != nil || !utils.IsLinkMode(path, d.Type()) || !f.firstLevel(path) {
			return fn(path, d, err)
		}
		target, dirInfo, ok := f.check(path)
		if !ok {
			return fn(path, d, err)
		}
		dirEntry := fs.FileInfoToDirEntry(dirInfo)
		if err := fn(path, dirEntry, nil); err != nil {
			if err == filepath.SkipDir {
				return nil
			}
			return err
		}
		var stop error
		err = filepath.WalkDir(target, func(p string, e fs.DirEntry, walkErr error) error {
			f.recordWalkError(path, target, p, walkErr)
			if p == target {
				if walkErr == nil {
					return nil // already reported under the link's name
				}
				e = dirEntry
			}
			r := fn(path+p[len(target):], e, walkErr)
			if r == filepath.SkipAll {
				stop = r
			}
			return r
		})
		if stop != nil {
			return stop
		}
		return err
	})
}

// ReadDir reads the resolved source root in filename order. Child links remain
// links, just as with os.ReadDir, unless opts follows them: a followed link is
// reported under its own name as a directory.
func ReadDir(root string, opts Options) ([]os.DirEntry, error) {
	resolved := utils.ResolveSymlink(root)
	entries, err := os.ReadDir(resolved)
	f := opts.Follow
	if f != nil {
		for i, entry := range entries {
			path := filepath.Join(resolved, entry.Name())
			if !utils.IsLinkMode(path, entry.Type()) || !f.firstLevel(path) {
				continue
			}
			if _, dirInfo, ok := f.check(path); ok {
				entries[i] = fs.FileInfoToDirEntry(dirInfo)
			}
		}
	}
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
