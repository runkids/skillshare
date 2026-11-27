package install

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// ErrLinkOutside marks a symlink that leads outside the repository it came
// from, or into its .git folder. No flag overrides it.
var ErrLinkOutside = errors.New("links outside the repository")

// checkoutLinksWithin checks the symlinks committed in the checkout at root,
// which stay links when installed: files the user added or ignored there do
// not count. A sparse checkout leaves links into the folders it excludes
// dangling; such a link reads nothing, so it passes when it points inside.
func checkoutLinksWithin(root string) error {
	out, err := gitOutput(root, "ls-files", "--stage", "-z")
	if err != nil {
		return fmt.Errorf("cannot list the files in %s: %w", root, err)
	}
	for _, entry := range strings.Split(out, "\x00") {
		meta, name, ok := strings.Cut(entry, "\t")
		if !ok || !strings.HasPrefix(meta, "120000 ") {
			continue
		}
		path := filepath.Join(root, filepath.FromSlash(name))
		if info, err := os.Lstat(path); err != nil || info.Mode()&os.ModeSymlink == 0 {
			continue // left out by a sparse checkout, or a file where links are off
		}
		err := resolveWithin(root, path)
		if errors.Is(err, fs.ErrNotExist) {
			err = danglingWithin(root, path, name)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

// danglingWithin fails unless the missing target of the link at path would
// stay inside root and out of its .git folder.
func danglingWithin(root, path, name string) error {
	resolvedRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return err
	}
	target, err := os.Readlink(path)
	if err != nil {
		return fmt.Errorf("cannot resolve %s: %w", name, err)
	}
	var missing *fs.PathError
	if _, err := filepath.EvalSymlinks(path); !errors.As(err, &missing) {
		return fmt.Errorf("cannot resolve %s: %w", name, err)
	}
	// The error names the first missing path with its parent as walked, which on
	// Windows can still hold 8.3 short names such as RUNNER~1; resolve the parent,
	// which exists, so it compares with resolvedRoot.
	parent, err := filepath.EvalSymlinks(filepath.Dir(missing.Path))
	if err != nil {
		return fmt.Errorf("cannot resolve %s: %w", name, err)
	}
	lexical := filepath.Join(filepath.Dir(path), target)
	if target == "" || filepath.IsAbs(target) || filepath.VolumeName(target) != "" || os.IsPathSeparator(target[0]) ||
		!pathSameOrDescendant(lexical, root) || pathSameOrDescendant(lexical, filepath.Join(root, ".git")) ||
		!pathSameOrDescendant(filepath.Join(parent, filepath.Base(missing.Path)), resolvedRoot) || inGitDir(resolvedRoot, parent) {
		return fmt.Errorf("%s %w", name, ErrLinkOutside)
	}
	return nil
}

// resolveWithin fails when path, after following every symlink, leads outside
// root or into its .git folder, whose config can hold a token in the remote URL.
func resolveWithin(root, path string) error {
	resolvedRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return err
	}
	name := path
	for _, base := range []string{root, resolvedRoot} {
		if rel, err := filepath.Rel(base, path); err == nil && !startsWithParentTraversal(rel) && rel != ".." {
			name = filepath.ToSlash(rel)
			break
		}
	}
	resolved, err := filepath.EvalSymlinks(path)
	if errors.Is(err, fs.ErrNotExist) {
		if outside := danglingWithin(root, path, name); outside != nil {
			return outside
		}
	}
	if err != nil {
		return fmt.Errorf("cannot resolve %s: %w", name, err)
	}
	if !pathSameOrDescendant(resolved, resolvedRoot) || inGitDir(resolvedRoot, resolved) {
		return fmt.Errorf("%s %w", name, ErrLinkOutside)
	}
	return nil
}

// inGitDir reports whether path, inside root, is root's .git folder or under
// it. It compares folders by identity, since .GIT names the same folder on a
// case-insensitive file system.
func inGitDir(root, path string) bool {
	gitDir, err := os.Stat(filepath.Join(root, ".git"))
	if err != nil {
		return false
	}
	for p := path; p != root && pathSameOrDescendant(p, root); p = filepath.Dir(p) {
		if info, err := os.Stat(p); err == nil && os.SameFile(info, gitDir) {
			return true
		}
	}
	return false
}
