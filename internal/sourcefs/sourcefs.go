// Package sourcefs is the write boundary of the skills source. Every write
// goes through an *os.Root opened at the source root, so a path that resolves
// outside the root through a link is refused by the handle itself. On top of
// that, every write first runs Lstat on each component of its path and
// refuses when one is a link: os.Root alone still acts on a link that is the
// last component, and it allows a relative link that stays inside the root.
//
// The package takes plain inputs and must not import internal/config or
// internal/sync.
package sourcefs

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"skillshare/internal/utils"
)

// ErrLink is matched by every error that refuses a path because one of its
// components is a link.
var ErrLink = errors.New("path is a link")

// LinkError reports the component of a path that is a link.
type LinkError struct {
	Path string // the link, as an absolute path under the source root
}

func (e *LinkError) Error() string {
	return fmt.Sprintf("%s is a link; edit its target directly", e.Path)
}

func (e *LinkError) Is(target error) bool { return target == ErrLink }

// Root is an open skills source. Names passed to its methods are relative to
// the root and use the OS separator; Rel converts an absolute path.
type Root struct {
	dir      string
	resolved string
	root     *os.Root
}

// Open opens the skills source at dir, which must exist. The root itself may
// be a link.
func Open(dir string) (*Root, error) {
	dir = filepath.Clean(dir)
	root, err := os.OpenRoot(dir)
	if err != nil {
		return nil, err
	}
	return &Root{dir: dir, resolved: utils.ResolveSymlink(dir), root: root}, nil
}

// Create creates the skills source directory at dir when it is missing, then
// opens it.
func Create(dir string) (*Root, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	return Open(dir)
}

// MkdirAllIn creates name below the source root dir, creating the root when
// it is missing. It is for callers that write nothing else.
func MkdirAllIn(dir, name string) error {
	r, err := Create(dir)
	if err != nil {
		return err
	}
	defer r.Close()
	return r.MkdirAll(name, 0o755)
}

// CheckMoveOut checks the in-source side of a move of path, below the source
// root dir, to a place outside it such as the trash. Such a move cannot use
// Root.Rename, so it is checked through the root first: a link above path is
// refused. Path itself may be a link; a rename moves the link, never its
// target.
func CheckMoveOut(dir, path string) error {
	r, err := Open(dir)
	if err != nil {
		return err
	}
	defer r.Close()
	rel, err := r.Rel(path)
	if err != nil {
		return err
	}
	return r.CheckParent(rel)
}

// Close releases the root.
func (r *Root) Close() error { return r.root.Close() }

// Dir returns the source root as it was opened.
func (r *Root) Dir() string { return r.dir }

// Rel returns the name of the absolute path p relative to the root. p may be
// spelled through the root as opened or through its resolved form, which is
// what discovery reports when the root is a link.
func (r *Root) Rel(p string) (string, error) {
	p = filepath.Clean(p)
	for _, base := range []string{r.dir, r.resolved} {
		rel, err := filepath.Rel(base, p)
		if err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel) {
			return rel, nil
		}
	}
	return "", fmt.Errorf("%s is outside the skills source %s", p, r.dir)
}

// CheckNoLink refuses name when any existing component of it, including the
// last, is a link. Components after the first missing one cannot exist, so
// the check stops there.
func (r *Root) CheckNoLink(name string) error {
	return r.checkComponents(name, true)
}

// CheckParent refuses name when any existing component above the last one is
// a link. A move out of the source (trash) uses it: renaming the last
// component never follows it, but a link above it would move content that
// lives outside the source.
func (r *Root) CheckParent(name string) error {
	return r.checkComponents(name, false)
}

func (r *Root) checkComponents(name string, includeLast bool) error {
	name = filepath.Clean(name)
	if name == "." {
		return nil
	}
	parts := strings.Split(name, string(filepath.Separator))
	if !includeLast {
		parts = parts[:len(parts)-1]
	}
	prefix := ""
	for _, part := range parts {
		prefix = filepath.Join(prefix, part)
		info, err := r.root.Lstat(prefix)
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		if err != nil {
			return err
		}
		abs := filepath.Join(r.dir, prefix)
		if utils.IsLinkMode(abs, info.Mode()) {
			return &LinkError{Path: abs}
		}
	}
	return nil
}

// Stat returns the file info of name, following a final link only when it
// stays inside the root.
func (r *Root) Stat(name string) (fs.FileInfo, error) { return r.root.Stat(name) }

// Lstat returns the file info of name without following a final link.
func (r *Root) Lstat(name string) (fs.FileInfo, error) { return r.root.Lstat(name) }

// WriteFile writes data to name, creating or truncating it.
func (r *Root) WriteFile(name string, data []byte, perm fs.FileMode) error {
	if err := r.CheckNoLink(name); err != nil {
		return err
	}
	return r.root.WriteFile(name, data, perm)
}

// OpenFile opens name with the given flags. Any flag that can change the
// file runs the link check first.
func (r *Root) OpenFile(name string, flag int, perm fs.FileMode) (*os.File, error) {
	if flag&(os.O_WRONLY|os.O_RDWR|os.O_CREATE|os.O_TRUNC|os.O_APPEND) != 0 {
		if err := r.CheckNoLink(name); err != nil {
			return nil, err
		}
	}
	return r.root.OpenFile(name, flag, perm)
}

// MkdirAll creates name and any missing parents.
func (r *Root) MkdirAll(name string, perm fs.FileMode) error {
	if err := r.CheckNoLink(name); err != nil {
		return err
	}
	return r.root.MkdirAll(name, perm)
}

// Remove removes the file or empty directory name.
func (r *Root) Remove(name string) error {
	if err := r.CheckNoLink(name); err != nil {
		return err
	}
	return r.root.Remove(name)
}

// RemoveAll removes name and everything below it. Links below name are
// removed, never followed.
func (r *Root) RemoveAll(name string) error {
	if err := r.CheckNoLink(name); err != nil {
		return err
	}
	return r.root.RemoveAll(name)
}

// Rename renames oldname to newname, both inside the root.
func (r *Root) Rename(oldname, newname string) error {
	if err := r.CheckNoLink(oldname); err != nil {
		return err
	}
	if err := r.CheckNoLink(newname); err != nil {
		return err
	}
	return r.root.Rename(oldname, newname)
}

// WriteFileAtomic replaces name through a temporary file in the same folder.
// The temporary file is created and renamed inside the root, so the first
// filesystem change is already constrained by it.
func (r *Root) WriteFileAtomic(name string, data []byte, perm fs.FileMode) error {
	if err := r.CheckNoLink(name); err != nil {
		return err
	}
	temp := filepath.Join(filepath.Dir(name), ".skillshare-"+filepath.Base(name)+"."+strconv.FormatInt(time.Now().UnixNano(), 36))
	f, err := r.root.OpenFile(temp, os.O_WRONLY|os.O_CREATE|os.O_EXCL, perm)
	if err != nil {
		return fmt.Errorf("create temp: %w", err)
	}
	defer func() { _ = r.root.Remove(temp) }()
	if _, err = f.Write(data); err == nil {
		err = f.Chmod(perm)
	}
	if err == nil {
		err = f.Sync()
	}
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return fmt.Errorf("write temp: %w", err)
	}
	if err := r.root.Rename(temp, name); err != nil {
		return fmt.Errorf("rename temp: %w", err)
	}
	return nil
}

// MoveIn renames src, which lives outside the root, to name. The move
// crosses the root's edge, so it cannot use Root.Rename: the in-source side
// is checked through the root first, and name must not exist yet.
func (r *Root) MoveIn(src, name string) error {
	if err := r.CheckNoLink(name); err != nil {
		return err
	}
	if _, err := r.root.Lstat(name); err == nil {
		return fmt.Errorf("%s already exists", filepath.Join(r.dir, name))
	}
	if _, err := r.root.Stat(filepath.Dir(filepath.Clean(name))); err != nil {
		return err
	}
	return os.Rename(src, filepath.Join(r.dir, name))
}

// CopyIn copies the directory tree src, which lives outside the root, to
// name. It is the fallback for MoveIn when the rename crosses filesystems
// (see IsCrossDevice) and keeps its contract: name must not exist yet, and
// every directory and file is created through the root, so a link that
// appears at or above name after the check is still refused by the handle.
// Links inside src are copied as the content they point at.
func (r *Root) CopyIn(src, name string) error {
	if err := r.CheckNoLink(name); err != nil {
		return err
	}
	if _, err := r.root.Lstat(name); err == nil {
		return fmt.Errorf("%s already exists", filepath.Join(r.dir, name))
	}
	return filepath.Walk(src, func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		dst := filepath.Join(name, rel)
		if info.IsDir() {
			return r.root.Mkdir(dst, info.Mode().Perm())
		}
		return r.copyFileIn(path, dst)
	})
}

func (r *Root) copyFileIn(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	info, err := in.Stat()
	if err != nil {
		return err
	}
	out, err := r.root.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_EXCL, info.Mode().Perm())
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

// Unlink removes entry, a first-level link, without touching its target.
// It is the only way to remove a link in the source.
func (r *Root) Unlink(entry string) error {
	if entry == "" || entry != filepath.Base(entry) || entry == "." || entry == ".." {
		return fmt.Errorf("%q is not a first-level entry", entry)
	}
	info, err := r.root.Lstat(entry)
	if err != nil {
		return err
	}
	if !utils.IsLinkMode(filepath.Join(r.dir, entry), info.Mode()) {
		return fmt.Errorf("%s is not a link", filepath.Join(r.dir, entry))
	}
	return r.root.Remove(entry)
}
