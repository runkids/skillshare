package plugin

import (
	"fmt"
	"io"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"
)

// maxPreview keeps a stray bundle or binary from being sent to the browser whole.
const maxPreview = 1 << 20

// snapshotDir is the reviewed copy of a package's source, "" when there is none: an
// imported plugin has no source, so Skillshare never copied it. Every target of one
// package snapshots the same source, so the first one present answers for all.
func (s *Service) snapshotDir(name string) (string, error) {
	d, err := s.load()
	if err != nil {
		return "", err
	}
	pack, ok := d.packages[name]
	if !ok {
		return "", fmt.Errorf("plugin %q is not managed here", name)
	}
	for _, target := range slices.Sorted(maps.Keys(pack.Bindings)) {
		b := pack.Bindings[target]
		if b.Source == "" {
			continue
		}
		dir := filepath.Join(s.snapshotPath(b, target), "content")
		if info, err := os.Stat(dir); err == nil && info.IsDir() {
			return dir, nil
		}
	}
	return "", nil
}

// Files lists the snapshot, slash separated and sorted. Nothing is read from the network.
func (s *Service) Files(name string) ([]string, error) {
	dir, err := s.snapshotDir(name)
	if err != nil || dir == "" {
		return []string{}, err
	}
	files := []string{}
	err = filepath.WalkDir(dir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			// Dependencies are not what the author wrote, and there can be tens of thousands of them.
			if entry.Name() == "node_modules" {
				return filepath.SkipDir
			}
			return nil
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		files = append(files, filepath.ToSlash(rel))
		return nil
	})
	slices.Sort(files)
	return files, err
}

// ReadFile reads one file of the snapshot. os.Root refuses any path, or symlink, that leaves it.
func (s *Service) ReadFile(name, rel string) ([]byte, error) {
	dir, err := s.snapshotDir(name)
	if err != nil {
		return nil, err
	}
	if dir == "" {
		return nil, fs.ErrNotExist
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	file, err := root.Open(filepath.FromSlash(rel))
	if err != nil {
		return nil, err
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, maxPreview+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxPreview {
		return nil, fmt.Errorf("%s is too large to preview", rel)
	}
	return data, nil
}
