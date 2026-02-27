package cache

import (
	"crypto/sha256"
	"encoding/gob"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
)

const diskCacheVersion = 1

// DiskCache is the on-disk manifest for cross-invocation discovery caching.
type DiskCache struct {
	Version int
	RootDir string
	Entries []DiskCacheEntry
}

// DiskCacheEntry represents a single cached skill.
type DiskCacheEntry struct {
	RelPath  string
	FlatName string
	IsInRepo bool
	Targets  []string // nil = all targets
	Mtime    int64    // SKILL.md mtime (unix nano)
	Size     int64    // SKILL.md size
}

// diskCachePath returns a per-source-path gob filename using sha256 hash.
func diskCachePath(cacheDir, sourcePath string) string {
	h := sha256.Sum256([]byte(sourcePath))
	return filepath.Join(cacheDir, "discovery-"+hex.EncodeToString(h[:6])+".gob")
}

func saveDiskCache(path string, dc *DiskCache) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("create cache dir: %w", err)
	}

	tmp := path + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	defer func() {
		f.Close()
		os.Remove(tmp)
	}()

	if err := gob.NewEncoder(f).Encode(dc); err != nil {
		return fmt.Errorf("encode: %w", err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("close: %w", err)
	}

	return os.Rename(tmp, path)
}

func loadDiskCache(path string) (*DiskCache, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var dc DiskCache
	if err := gob.NewDecoder(f).Decode(&dc); err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}

	if dc.Version != diskCacheVersion {
		return nil, fmt.Errorf("cache version mismatch: got %d, want %d", dc.Version, diskCacheVersion)
	}

	return &dc, nil
}

func deleteDiskCache(path string) {
	os.Remove(path)
}
