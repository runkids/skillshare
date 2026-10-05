package plugin

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Native install recursively removes its runtime destination before linking the
// cache. Require an absent destination, including undeclared/transitive packages.
func ompEmptyRuntime(path string) error {
	if err := ompSafePath(path); err != nil {
		return err
	}
	if _, err := os.Lstat(path); !os.IsNotExist(err) {
		return fmt.Errorf("OMP runtime destination is not verified empty: %s", path)
	}
	return nil
}

func (s *Service) prepareOMPInstall(ctx context.Context, h Host, c *Change) {
	check := func() error {
		ref := c.Binding.Commit
		if ref == "" {
			ref = c.Binding.SourceRef
		}
		root, source, cleanup, err := acquireRef(ctx, c.Binding.Source, ref)
		if err != nil {
			return err
		}
		defer cleanup()
		d, err := discoverRoot(root, source, c.Binding.Entry)
		if err != nil {
			return err
		}
		if d.Digest != c.Binding.Digest {
			return fmt.Errorf("OMP source changed; preview again")
		}
		for _, candidate := range d.Candidates {
			if candidate.Name != c.Binding.Plugin {
				continue
			}
			module, _, err := ompRemovalPackage(filepath.Join(root, candidate.pathFor("omp")), candidate.Name)
			if err != nil {
				return err
			}
			runtimeRoot := filepath.Dir(filepath.Dir(h.ompCacheRoot))
			if s.ProjectRoot != "" {
				runtimeRoot = filepath.Join(s.ProjectRoot, ".omp", "plugins")
			}
			c.ompRuntimePath = filepath.Join(runtimeRoot, "node_modules", filepath.FromSlash(module))
			return ompEmptyRuntime(c.ompRuntimePath)
		}
		return fmt.Errorf("OMP source no longer provides the reviewed plugin")
	}
	if err := check(); err != nil {
		c.Action, c.Message, c.MessageKey = "blocked", err.Error(), "plugins.error.ompRuntimeSafety"
	}
}

// Reinstalls use a new marketplace/cache identity instead of replacing the
// retained cache an invisible project can still use. Preview chooses the same
// first free identity until native state changes and invalidates its revision.
func ompFreshCache(h Host, c *Change) error {
	if err := ompSafePath(h.ompCacheRoot); err != nil {
		return err
	}
	if v := c.Binding.Version; v != "" && (filepath.Base(v) != v || strings.ContainsAny(v, "\\/\r\n\x00")) {
		return fmt.Errorf("unsafe OMP cache version")
	}
	original := c.ID
	name, _, ok := strings.Cut(original, "@")
	if !ok || !validOMPName(name) {
		return fmt.Errorf("invalid OMP plugin identity")
	}
	for attempt := 0; attempt < 100; attempt++ {
		candidate := original
		if attempt > 0 {
			candidate = name + "@skillshare-" + hash([]byte(fmt.Sprintf("%s\x00reinstall\x00%d", original, attempt)))[:16]
		}
		info, err := os.Lstat(ompCachePath(h.ompCacheRoot, candidate, c.Binding.Version))
		if os.IsNotExist(err) {
			if attempt > 0 {
				_, market, _ := strings.Cut(candidate, "@")
				if _, registered := h.Marketplaces[market]; registered {
					continue
				}
				c.ID, c.Binding.ID = candidate, candidate
				c.Message, c.MessageKey = "Installs into a new cache identity; retained cache used by other projects is not changed.", "plugins.note.ompFreshCache"
			}
			return nil
		}
		if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("unreadable or linked OMP cache destination")
		}
	}
	return fmt.Errorf("no free OMP cache identity")
}
