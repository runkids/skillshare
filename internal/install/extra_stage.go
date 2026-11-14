package install

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// stageFetchedTree validates the entire selection before materializing it in dst.
// The checkout and destination are private staging paths owned by the caller.
func stageFetchedTree(checkoutRoot, subdir, dst string) error {
	root, err := filepath.Abs(checkoutRoot)
	if err != nil {
		return err
	}
	rootInfo, err := os.Lstat(root)
	if err != nil {
		return fmt.Errorf("inspect %s: %w", root, err)
	}
	if rootInfo.Mode()&os.ModeSymlink != 0 || !rootInfo.IsDir() {
		return fmt.Errorf("checkout %s is not a real directory", root)
	}
	parent, err := filepath.EvalSymlinks(filepath.Dir(root))
	if err != nil {
		return fmt.Errorf("resolve checkout parent: %w", err)
	}
	root = filepath.Join(parent, filepath.Base(root))
	if subdir != "" {
		if err := validateRepoSubdir(subdir); err != nil {
			return err
		}
		for _, part := range strings.Split(subdir, "/") {
			if part == ".git" {
				return fmt.Errorf("selected path %q contains git metadata", subdir)
			}
		}
	}
	selected := filepath.Join(root, filepath.FromSlash(subdir))
	if !pathSameOrDescendant(selected, root) {
		return fmt.Errorf("selected path %q escapes checkout", subdir)
	}
	// Never follow a link in the selected path, including the checkout root itself.
	path := root
	parts := []string{}
	if subdir != "" {
		parts = strings.Split(filepath.FromSlash(subdir), string(filepath.Separator))
	}
	for _, part := range append([]string{""}, parts...) {
		path = filepath.Join(path, part)
		info, err := os.Lstat(path)
		if err != nil {
			return fmt.Errorf("inspect %s: %w", path, err)
		}
		if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
			return fmt.Errorf("selected directory %s is not a real directory", path)
		}
	}
	if pathSameOrDescendant(dst, selected) {
		return fmt.Errorf("staging destination is inside selected directory")
	}

	type entry struct {
		rel, source string
		mode        fs.FileMode
		dir         bool
	}
	var entries []entry
	err = filepath.WalkDir(selected, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return fmt.Errorf("inspect %s: %w", path, walkErr)
		}
		rel, err := filepath.Rel(selected, path)
		if err != nil {
			return err
		}
		if rel != "." && d.Name() == ".git" {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		info, err := os.Lstat(path)
		if err != nil {
			return fmt.Errorf("inspect %s: %w", path, err)
		}
		source := path
		if info.Mode()&os.ModeSymlink != 0 {
			target, err := os.Readlink(path)
			if err != nil {
				return fmt.Errorf("read link %s: %w", path, err)
			}
			if filepath.IsAbs(target) || filepath.VolumeName(target) != "" {
				return fmt.Errorf("link %s has absolute target", path)
			}
			lexical := filepath.Clean(filepath.Join(filepath.Dir(path), target))
			if !pathSameOrDescendant(lexical, selected) {
				return fmt.Errorf("link %s escapes selected directory", path)
			}
			source, err = filepath.EvalSymlinks(path)
			if err != nil {
				return fmt.Errorf("resolve link %s: %w", path, err)
			}
			if !pathSameOrDescendant(source, selected) {
				return fmt.Errorf("link %s resolves outside selected directory", path)
			}
			resolvedRel, err := filepath.Rel(selected, source)
			if err != nil {
				return err
			}
			for _, part := range strings.Split(resolvedRel, string(filepath.Separator)) {
				if part == ".git" {
					return fmt.Errorf("link %s resolves into git metadata", path)
				}
			}
			info, err = os.Stat(source)
			if err != nil {
				return fmt.Errorf("inspect link %s: %w", path, err)
			}
			if !info.Mode().IsRegular() {
				return fmt.Errorf("link %s does not point to a regular file", path)
			}
		} else if !info.IsDir() && !info.Mode().IsRegular() {
			return fmt.Errorf("unsupported file %s", path)
		}
		entries = append(entries, entry{rel: rel, source: source, mode: info.Mode(), dir: info.IsDir()})
		return nil
	})
	if err != nil {
		return err
	}
	for _, e := range entries {
		dest := filepath.Join(dst, e.rel)
		if e.dir {
			if err := os.Mkdir(dest, 0755); err != nil {
				return fmt.Errorf("stage %s: %w", e.rel, err)
			}
			continue
		}
		if err := copyStagedExtraFile(e.source, dest, e.mode); err != nil {
			return fmt.Errorf("stage %s: %w", e.rel, err)
		}
	}
	return nil
}

func copyStagedExtraFile(source, dest string, mode fs.FileMode) error {
	in, err := os.Open(source)
	if err != nil {
		return err
	}
	defer in.Close()
	info, err := in.Stat()
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("source is not a regular file")
	}
	out, err := os.OpenFile(dest, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode.Perm())
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(out, in)
	if copyErr == nil {
		copyErr = out.Chmod(mode.Perm())
	}
	if closeErr := out.Close(); copyErr == nil {
		copyErr = closeErr
	}
	return copyErr
}

// StageExtraTree gives removal the same validated, no-follow copy used by
// publication when retaining an intact source for in-process rollback.
func StageExtraTree(source, destination string) error {
	return stageFetchedTree(source, "", destination)
}
