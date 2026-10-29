package plugin

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// maxPreview keeps a stray bundle or binary from being sent to the browser whole.
const maxPreview = 1 << 20

// snapshotPackage verifies only this package's unresolved source ownership in
// memory. Current local receipts need no native queries; nothing is migrated.
func (s *Service) snapshotPackage(ctx context.Context, name string) (Package, error) {
	d, err := s.load()
	if err != nil {
		return Package{}, err
	}
	pack, ok := d.packages[name]
	if !ok {
		return Package{}, fmt.Errorf("plugin %q is not managed here", name)
	}
	targets := map[string]bool{}
	for target := range pack.Bindings {
		local := d.state.Bindings[name][target]
		candidate := d.legacy[name].Bindings[target]
		if local.Legacy != nil {
			candidate = local.Legacy.Binding
		}
		if local.Applied == nil && candidate.Source != "" {
			targets[target] = true
		}
	}
	if len(targets) == 0 {
		return pack, nil
	}
	hosts := map[string]Host{}
	s.adoptLegacy(d, func(target string) Host {
		if !targets[target] {
			return Host{Error: "outside requested plugin snapshot"}
		}
		if h, ok := hosts[target]; ok {
			return h
		}
		h := s.host(ctx, target)
		hosts[target] = h
		return h
	}, name)
	return d.packages[name], nil
}

// snapshotDir is the reviewed copy of a package's source, "" when there is none: an
// imported plugin has no source, so Skillshare never copied it. Each binding of a
// package can use a different source; the first locally owned snapshot answers.
func (s *Service) snapshotDir(name string) (string, error) {
	pack, err := s.snapshotPackage(context.Background(), name)
	if err != nil {
		return "", err
	}
	for _, target := range slices.Sorted(maps.Keys(pack.Bindings)) {
		b := pack.Bindings[target]
		if b.Source == "" || b.ID == "" || b.Cleanup || b.ContextChanged || !s.ownedSnapshot(b, target) {
			continue
		}
		dir := filepath.Join(s.snapshotPath(b, target), "content")
		if info, err := os.Stat(dir); err == nil && info.IsDir() {
			return dir, nil
		}
	}
	return "", nil
}

// discoverSnapshot reads a remote source from the snapshot of a binding made from it, nil
// when there is none. A snapshot whose digest no longer matches was edited, so it does not answer.
func (s *Service) discoverSnapshot(ctx context.Context, name, source, ref, entry string) *Discovery {
	if name == "" || !strings.HasPrefix(source, "https://") {
		return nil
	}
	pack, err := s.snapshotPackage(ctx, name)
	if err != nil {
		return nil
	}
	for _, target := range slices.Sorted(maps.Keys(pack.Bindings)) {
		b := pack.Bindings[target]
		if b.Source != source || b.SourceRef != ref || b.Digest == "" || b.Cleanup || b.ContextChanged || !s.ownedSnapshot(b, target) {
			continue
		}
		found, err := discoverRoot(filepath.Join(s.snapshotPath(b, target), "content"), b.Source, entry)
		if err != nil || found.Digest != b.Digest {
			continue
		}
		found.SourceRef, found.Commit = b.SourceRef, b.Commit
		return found
	}
	return nil
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
