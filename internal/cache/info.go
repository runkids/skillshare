package cache

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// CacheItem describes a single discovery cache file on disk.
type CacheItem struct {
	Path       string    // absolute gob file path
	RootDir    string    // source path stored in gob
	EntryCount int       // number of cached skills
	Size       int64     // file size in bytes
	ModTime    time.Time // file modification time
	Orphan     bool      // true if RootDir no longer exists on disk
	Error      string    // decode error (corrupt/version mismatch)
}

// ListDiskCaches scans cacheDir for discovery-*.gob files and returns
// metadata for each. Results are sorted by path for stable output.
func ListDiskCaches(cacheDir string) []CacheItem {
	pattern := filepath.Join(cacheDir, "discovery-*.gob")
	matches, err := filepath.Glob(pattern)
	if err != nil || len(matches) == 0 {
		return nil
	}

	items := make([]CacheItem, 0, len(matches))
	for _, path := range matches {
		item := CacheItem{Path: path}

		fi, err := os.Stat(path)
		if err != nil {
			item.Error = err.Error()
			items = append(items, item)
			continue
		}
		item.Size = fi.Size()
		item.ModTime = fi.ModTime()

		dc, err := loadDiskCache(path)
		if err != nil {
			item.Error = err.Error()
			items = append(items, item)
			continue
		}

		item.RootDir = dc.RootDir
		item.EntryCount = len(dc.Entries)

		// Check if the source directory is accessible. Any stat error
		// (not-exist, permission denied, broken symlink) marks as orphan
		// because cache is rebuildable — safe to be conservative.
		if _, statErr := os.Stat(dc.RootDir); statErr != nil {
			item.Orphan = true
		}

		items = append(items, item)
	}

	sort.Slice(items, func(i, j int) bool {
		return items[i].Path < items[j].Path
	})
	return items
}

// RemoveDiskCache deletes a single cache file by absolute path.
// Returns an error if the path doesn't look like a discovery gob file
// or if removal fails.
func RemoveDiskCache(path string) error {
	base := filepath.Base(path)
	if !strings.HasPrefix(base, "discovery-") || !strings.HasSuffix(base, ".gob") {
		return fmt.Errorf("not a discovery cache file: %s", base)
	}
	err := os.Remove(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// ClearAllDiskCaches removes all discovery-*.gob files from cacheDir.
// Returns the number of files removed and the first error encountered.
func ClearAllDiskCaches(cacheDir string) (int, error) {
	pattern := filepath.Join(cacheDir, "discovery-*.gob")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return 0, err
	}

	removed := 0
	for _, path := range matches {
		if err := os.Remove(path); err != nil {
			if os.IsNotExist(err) {
				continue // race: file already removed by concurrent process
			}
			return removed, fmt.Errorf("remove %s: %w", filepath.Base(path), err)
		}
		removed++
	}
	return removed, nil
}
