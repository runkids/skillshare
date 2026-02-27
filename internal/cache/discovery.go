package cache

import (
	"os"
	"path/filepath"
	gosync "sync"
	"time"

	"skillshare/internal/sync"
)

// DiscoveryCache provides two-layer caching for skill discovery:
//   - L1: in-process mutex-guarded maps (separate full/lite to prevent pollution)
//   - L2: on-disk gob manifest (per-source-path, validated by per-entry stat sweep)
type DiscoveryCache struct {
	mu       gosync.Mutex
	full     map[string]*fullEntry // key: sourcePath
	lite     map[string]*liteEntry // key: sourcePath
	cacheDir string
}

type fullEntry struct {
	skills   []sync.DiscoveredSkill
	loadedAt time.Time
}

type liteEntry struct {
	skills   []sync.DiscoveredSkill
	repos    []string
	loadedAt time.Time
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
	c.mu.Lock()
	defer c.mu.Unlock()

	// L1: in-process hit (full only — never returns lite data)
	if entry, ok := c.full[sourcePath]; ok {
		return entry.skills, nil
	}

	// L2: disk cache hit (per-entry stat sweep)
	if skills, ok := c.tryDiskCache(sourcePath); ok {
		c.full[sourcePath] = &fullEntry{skills: skills, loadedAt: time.Now()}
		return skills, nil
	}

	// Cache miss: full walk
	skills, err := sync.DiscoverSourceSkills(sourcePath)
	if err != nil {
		return nil, err
	}

	c.full[sourcePath] = &fullEntry{skills: skills, loadedAt: time.Now()}
	c.writeDiskCache(sourcePath, skills)

	return skills, nil
}

// DiscoverLite returns skills without frontmatter parsing + tracked repos.
// Uses L1 lite cache only (not persisted to L2 — Lite lacks Targets data).
func (c *DiscoveryCache) DiscoverLite(sourcePath string) ([]sync.DiscoveredSkill, []string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if entry, ok := c.lite[sourcePath]; ok {
		return entry.skills, entry.repos, nil
	}

	skills, repos, err := sync.DiscoverSourceSkillsLite(sourcePath)
	if err != nil {
		return nil, nil, err
	}

	c.lite[sourcePath] = &liteEntry{skills: skills, repos: repos, loadedAt: time.Now()}
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

// tryDiskCache loads and validates disk cache via per-entry stat sweep.
func (c *DiscoveryCache) tryDiskCache(sourcePath string) ([]sync.DiscoveredSkill, bool) {
	dc, err := loadDiskCache(diskCachePath(c.cacheDir, sourcePath))
	if err != nil {
		return nil, false
	}

	if dc.RootDir != sourcePath {
		return nil, false
	}

	// Per-entry stat sweep: stat every SKILL.md, compare (mtime, size)
	currentSkills := quickStatWalk(sourcePath)
	if len(currentSkills) != len(dc.Entries) {
		return nil, false // skill count changed
	}

	cached := make(map[string]DiskCacheEntry, len(dc.Entries))
	for _, e := range dc.Entries {
		cached[e.RelPath] = e
	}

	for relPath, cur := range currentSkills {
		old, ok := cached[relPath]
		if !ok {
			return nil, false // new skill
		}
		if cur.Mtime != old.Mtime || cur.Size != old.Size {
			return nil, false // content changed
		}
	}

	// All match — convert
	skills := make([]sync.DiscoveredSkill, len(dc.Entries))
	for i, e := range dc.Entries {
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

type statEntry struct {
	Mtime int64
	Size  int64
}

// quickStatWalk walks source dir collecting SKILL.md stat info only (no file reads).
func quickStatWalk(sourcePath string) map[string]statEntry {
	result := make(map[string]statEntry)

	filepath.Walk(sourcePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() && info.Name() == ".git" {
			return filepath.SkipDir
		}
		if !info.IsDir() && info.Name() == "SKILL.md" {
			skillDir := filepath.Dir(path)
			relPath, relErr := filepath.Rel(sourcePath, skillDir)
			if relErr != nil || relPath == "." {
				return nil
			}
			relPath = filepath.ToSlash(relPath)
			result[relPath] = statEntry{
				Mtime: info.ModTime().UnixNano(),
				Size:  info.Size(),
			}
		}
		return nil
	})

	return result
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
