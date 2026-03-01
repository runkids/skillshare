package cache

import (
	"io/fs"
	"os"
	"path/filepath"
	gosync "sync"

	"github.com/charlievieth/fastwalk"

	"skillshare/internal/sync"
)

// DiscoveryCache provides two-layer caching for skill discovery:
//   - L1: in-process mutex-guarded maps (separate full/lite to prevent pollution)
//   - L2: on-disk gob manifest (per-source-path, validated by per-entry stat sweep)
type DiscoveryCache struct {
	mu       gosync.RWMutex
	full     map[string]*fullEntry // key: sourcePath
	lite     map[string]*liteEntry // key: sourcePath
	cacheDir string
}

type fullEntry struct {
	skills []sync.DiscoveredSkill
}

type liteEntry struct {
	skills []sync.DiscoveredSkill
	repos  []string
}

// New creates a DiscoveryCache. cacheDir is typically config.CacheDir().
func New(cacheDir string) *DiscoveryCache {
	return &DiscoveryCache{
		full:     make(map[string]*fullEntry),
		lite:     make(map[string]*liteEntry),
		cacheDir: cacheDir,
	}
}

// Discover returns discovered skills with Targets parsed.
// Checks L1 full → L2 disk → full walk. Never returns Lite data.
func (c *DiscoveryCache) Discover(sourcePath string) ([]sync.DiscoveredSkill, error) {
	// Fast path: L1 hit with read lock (concurrent readers allowed)
	c.mu.RLock()
	if entry, ok := c.full[sourcePath]; ok {
		c.mu.RUnlock()
		return entry.skills, nil
	}
	c.mu.RUnlock()

	// Slow path: L2 or full walk with exclusive lock
	c.mu.Lock()
	defer c.mu.Unlock()

	// Re-check L1 after acquiring write lock (another goroutine may have populated it)
	if entry, ok := c.full[sourcePath]; ok {
		return entry.skills, nil
	}

	// L2: disk cache hit (per-entry stat sweep)
	if skills, ok := c.tryDiskCache(sourcePath); ok {
		c.full[sourcePath] = &fullEntry{skills: skills}
		return skills, nil
	}

	// Cache miss: full walk
	skills, err := sync.DiscoverSourceSkills(sourcePath)
	if err != nil {
		return nil, err
	}

	c.full[sourcePath] = &fullEntry{skills: skills}
	c.writeDiskCache(sourcePath, skills)

	return skills, nil
}

// DiscoverLite returns skills without frontmatter parsing + tracked repos.
// Uses L1 lite cache only (not persisted to L2 — Lite lacks Targets data).
func (c *DiscoveryCache) DiscoverLite(sourcePath string) ([]sync.DiscoveredSkill, []string, error) {
	// Fast path: L1 hit with read lock
	c.mu.RLock()
	if entry, ok := c.lite[sourcePath]; ok {
		c.mu.RUnlock()
		return entry.skills, entry.repos, nil
	}
	c.mu.RUnlock()

	// Slow path: walk with exclusive lock
	c.mu.Lock()
	defer c.mu.Unlock()

	// Re-check after acquiring write lock
	if entry, ok := c.lite[sourcePath]; ok {
		return entry.skills, entry.repos, nil
	}

	skills, repos, err := sync.DiscoverSourceSkillsLite(sourcePath)
	if err != nil {
		return nil, nil, err
	}

	c.lite[sourcePath] = &liteEntry{skills: skills, repos: repos}
	return skills, repos, nil
}

// Invalidate clears both L1 (full + lite) and L2 for the given source path.
func (c *DiscoveryCache) Invalidate(sourcePath string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	delete(c.full, sourcePath)
	delete(c.lite, sourcePath)
	deleteDiskCache(diskCachePath(c.cacheDir, sourcePath))
}

// tryDiskCache loads and validates disk cache via count guard + targeted stats.
// 1. Count guard: lightweight fastwalk to detect added/removed skills
// 2. Targeted stat: stat only cached SKILL.md files (O(N) on skills, no frontmatter parsing)
func (c *DiscoveryCache) tryDiskCache(sourcePath string) ([]sync.DiscoveredSkill, bool) {
	dc, err := loadDiskCache(diskCachePath(c.cacheDir, sourcePath))
	if err != nil {
		return nil, false
	}

	if dc.RootDir != sourcePath {
		return nil, false
	}

	// Count guard: detect skills added or removed since cache was written.
	// This is a lightweight fastwalk (no stat, no file reads — just counts SKILL.md).
	if countSkillMDs(sourcePath) != len(dc.Entries) {
		return nil, false
	}

	// Targeted stat: verify each cached SKILL.md is unchanged.
	skills := make([]sync.DiscoveredSkill, len(dc.Entries))
	for i, e := range dc.Entries {
		skillMDPath := filepath.Join(sourcePath, e.RelPath, "SKILL.md")
		fi, err := os.Stat(skillMDPath)
		if err != nil {
			return nil, false // skill removed or inaccessible
		}
		if fi.ModTime().UnixNano() != e.Mtime || fi.Size() != e.Size {
			return nil, false // content changed
		}
		skills[i] = sync.DiscoveredSkill{
			SourcePath: filepath.Join(sourcePath, e.RelPath),
			RelPath:    e.RelPath,
			FlatName:   e.FlatName,
			IsInRepo:   e.IsInRepo,
			Targets:    e.Targets,
		}
	}

	return skills, true
}

// countSkillMDs counts SKILL.md files under sourcePath using fastwalk.
// No stat collection, no file reads — just filename matching.
func countSkillMDs(sourcePath string) int {
	var mu gosync.Mutex
	count := 0

	fastwalk.Walk(nil, sourcePath, func(path string, d fs.DirEntry, err error) error { //nolint:errcheck
		if err != nil {
			return nil
		}
		if d.IsDir() && d.Name() == ".git" {
			return fastwalk.SkipDir
		}
		if !d.IsDir() && d.Name() == "SKILL.md" {
			dir := filepath.Dir(path)
			if rel, relErr := filepath.Rel(sourcePath, dir); relErr == nil && rel != "." {
				mu.Lock()
				count++
				mu.Unlock()
			}
		}
		return nil
	})

	return count
}

// writeDiskCache persists Full discovery results to disk.
func (c *DiscoveryCache) writeDiskCache(sourcePath string, skills []sync.DiscoveredSkill) {
	dc := &DiskCache{
		Version: diskCacheVersion,
		RootDir: sourcePath,
		Entries: make([]DiskCacheEntry, len(skills)),
	}

	for i, s := range skills {
		var mtime, size int64
		skillMDPath := filepath.Join(s.SourcePath, "SKILL.md")
		if fi, err := os.Stat(skillMDPath); err == nil {
			mtime = fi.ModTime().UnixNano()
			size = fi.Size()
		}

		dc.Entries[i] = DiskCacheEntry{
			RelPath:  s.RelPath,
			FlatName: s.FlatName,
			IsInRepo: s.IsInRepo,
			Targets:  s.Targets,
			Mtime:    mtime,
			Size:     size,
		}
	}

	_ = saveDiskCache(diskCachePath(c.cacheDir, sourcePath), dc)
}
