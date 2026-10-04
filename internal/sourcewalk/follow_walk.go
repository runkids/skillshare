package sourcewalk

import (
	"io/fs"
	"os"
	"path/filepath"

	"skillshare/internal/utils"
)

// namedInfo keeps the logical entry name when its physical target has another name.
type namedInfo struct {
	os.FileInfo
	name string
}

func (i namedInfo) Name() string { return i.name }

type followedDirEntry struct{ os.FileInfo }

func (d followedDirEntry) Type() fs.FileMode          { return d.Mode().Type() }
func (d followedDirEntry) Info() (os.FileInfo, error) { return d.FileInfo, nil }

func (s *FollowSet) followedChild(name string) (Entry, bool) {
	for _, entry := range s.entries {
		if sameEntryName(entry.Name, name) && entry.State == Followed {
			return entry, true
		}
	}
	return Entry{}, false
}

func readDirFollow(root string, set *FollowSet) ([]os.DirEntry, error) {
	entries, err := readFollowDir(root)
	if err != nil {
		return entries, err
	}
	canonicalRoot, canonErr := Canonicalize(root)
	if canonErr != nil {
		return nil, readErrorPath(canonErr, root)
	}
	if !utils.PathsEqual(canonicalRoot, set.canonicalRoot) {
		return entries, nil
	}
	for i, d := range entries {
		entry, ok := set.followedChild(d.Name())
		if !ok {
			continue
		}
		info, err := os.Stat(entry.ResolvedTarget)
		if err == nil && !info.IsDir() {
			err = &os.PathError{Op: "readdir", Path: filepath.Join(root, entry.Name), Err: fs.ErrInvalid}
		}
		if err != nil {
			err = readErrorPath(err, filepath.Join(root, entry.Name))
			set.markMissing(entry.Name, err)
			return nil, err
		}
		entries[i] = followedDirEntry{namedInfo{info, entry.Name}}
	}
	return entries, nil
}

func finishWalk(err error) error {
	if err == filepath.SkipDir || err == filepath.SkipAll {
		return nil
	}
	return err
}

func walkFollow(root string, set *FollowSet, fn filepath.WalkFunc) error {
	logical := filepath.Clean(root)
	physical, err := Canonicalize(root)
	if err != nil {
		return finishWalk(fn(logical, nil, readErrorPath(err, logical)))
	}
	info, err := os.Lstat(physical)
	if err != nil {
		return finishWalk(fn(logical, nil, readErrorPath(err, logical)))
	}
	return finishWalk(walkFollowNode(logical, physical, namedInfo{info, filepath.Base(logical)}, "", 0, set, fn))
}

func walkFollowNode(logical, physical string, info os.FileInfo, owner string, depth int, set *FollowSet, fn filepath.WalkFunc) error {
	if !info.IsDir() || utils.IsLinkMode(physical, info.Mode()) {
		return fn(logical, info, nil)
	}
	// filepath.Walk reads a directory before invoking its callback.
	children, readErr := readFollowDir(physical)
	readErr = readErrorPath(readErr, logical)
	if readErr != nil && owner != "" {
		set.markMissing(owner, readErr)
	}
	if err := fn(logical, info, readErr); readErr != nil || err != nil {
		return err
	}
	atRoot := depth == 0 && utils.PathsEqual(physical, set.canonicalRoot)
	for _, child := range children {
		childLogical, childPhysical := filepath.Join(logical, child.Name()), filepath.Join(physical, child.Name())
		childOwner := owner
		var childInfo os.FileInfo
		var err error
		entry, ok := Entry{}, false
		if atRoot {
			entry, ok = set.followedChild(child.Name())
		}
		if ok {
			childPhysical, childOwner = entry.ResolvedTarget, entry.Name
			childInfo, err = followedInfo(childPhysical)
		} else {
			childInfo, err = os.Lstat(childPhysical)
		}
		if err != nil {
			err = readErrorPath(err, childLogical)
			if childOwner != "" {
				set.markMissing(childOwner, err)
			}
			if err = fn(childLogical, nil, err); err != nil && err != filepath.SkipDir {
				return err
			}
			continue
		}
		childInfo = namedInfo{childInfo, child.Name()}
		err = walkFollowNode(childLogical, childPhysical, childInfo, childOwner, depth+1, set, fn)
		if err != nil && (err != filepath.SkipDir || !childInfo.IsDir()) {
			return err
		}
	}
	return nil
}

func walkDirFollow(root string, set *FollowSet, fn fs.WalkDirFunc) error {
	logical := filepath.Clean(root)
	physical, err := Canonicalize(root)
	if err != nil {
		return finishWalk(fn(logical, nil, readErrorPath(err, logical)))
	}
	info, err := os.Lstat(physical)
	if err != nil {
		return finishWalk(fn(logical, nil, readErrorPath(err, logical)))
	}
	entry := fs.FileInfoToDirEntry(namedInfo{info, filepath.Base(logical)})
	return finishWalk(walkDirFollowNode(logical, physical, entry, "", 0, set, fn))
}

func walkDirFollowNode(logical, physical string, entry fs.DirEntry, owner string, depth int, set *FollowSet, fn fs.WalkDirFunc) error {
	if err := fn(logical, entry, nil); err != nil || !entry.IsDir() || utils.IsLinkMode(physical, entry.Type()) {
		if err == filepath.SkipDir && entry.IsDir() {
			return nil
		}
		return err
	}
	children, err := readFollowDir(physical)
	if err != nil {
		err = readErrorPath(err, logical)
		if owner != "" {
			set.markMissing(owner, err)
		}
		if err = fn(logical, entry, err); err != nil {
			if err == filepath.SkipDir && entry.IsDir() {
				return nil
			}
			return err
		}
	}
	atRoot := depth == 0 && utils.PathsEqual(physical, set.canonicalRoot)
	for _, child := range children {
		childLogical, childPhysical := filepath.Join(logical, child.Name()), filepath.Join(physical, child.Name())
		childOwner := owner
		followed, ok := Entry{}, false
		if atRoot {
			followed, ok = set.followedChild(child.Name())
		}
		if ok {
			childPhysical, childOwner = followed.ResolvedTarget, followed.Name
			info, err := followedInfo(childPhysical)
			if err != nil {
				err = readErrorPath(err, childLogical)
				set.markMissing(childOwner, err)
				if err = fn(childLogical, nil, err); err != nil {
					return err
				}
				continue
			}
			child = fs.FileInfoToDirEntry(namedInfo{info, child.Name()})
		} else {
			child = sourceEntry{DirEntry: child, path: childLogical}
		}
		if err := walkDirFollowNode(childLogical, childPhysical, child, childOwner, depth+1, set, fn); err != nil {
			if err == filepath.SkipDir {
				break
			}
			return err
		}
	}
	return nil
}

func followedInfo(path string) (os.FileInfo, error) {
	info, err := os.Stat(path)
	if err == nil && !info.IsDir() {
		err = &os.PathError{Op: "readdir", Path: path, Err: fs.ErrInvalid}
	}
	return info, err
}
