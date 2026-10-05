package plugin

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

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
