package sync

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"skillshare/internal/utils"
)

// FileLinkFallbackWarning is reported when a mode that links single files
// syncs copies instead.
const FileLinkFallbackWarning = "file links need Windows Developer Mode; copying instead"

// canCreateFileLink reports whether this system can create symlinks to files.
// Windows junctions only work for directories, so without Developer Mode file
// link modes fall back to copy. A variable so tests can simulate that.
var canCreateFileLink = platformCanCreateFileLink

// CanCreateFileLink reports whether this system can create symlinks to files
// (false on Windows without Developer Mode). The probe runs once.
func CanCreateFileLink() bool { return canCreateFileLink() }

// SetFileLinksForTest overrides whether file links can be created and returns
// a func that restores the probe. Only for tests in other packages.
func SetFileLinksForTest(ok bool) (restore func()) {
	prev := canCreateFileLink
	canCreateFileLink = func() bool { return ok }
	return func() { canCreateFileLink = prev }
}

// EffectiveAgentMode returns the mode agents actually sync with. Merge links
// each agent file, so it copies when file links are unavailable; symlink mode
// links the whole directory and is unaffected.
func EffectiveAgentMode(mode string) string {
	m := EffectiveMode(mode)
	if m == "merge" && !canCreateFileLink() {
		return "copy"
	}
	return m
}

// ExtraTargetMode returns the mode an extras target actually syncs with.
// Merge links each file, and symlink mode of a single-file extra links that
// file, so both copy when file links are unavailable. Symlink mode of a
// directory extra links the directory and is unaffected.
func ExtraTargetMode(mode string, singleFile bool) string {
	m := EffectiveMode(mode)
	if !canCreateFileLink() && (m == "merge" || (singleFile && m == "symlink")) {
		return "copy"
	}
	return m
}

// copyTracker records files copied in place of file links in the target's
// manifest, so later syncs can tell them from the user's own files. A tracked
// file that still matches its recorded hash belongs to skillshare and may be
// updated, relinked, or pruned; an edited one is left alone.
type copyTracker struct {
	dir     string
	m       *Manifest
	changed bool
}

func loadCopyTracker(dir string) *copyTracker {
	m, err := ReadManifest(dir)
	if err != nil {
		m = &Manifest{Managed: make(map[string]string), Mtimes: make(map[string]int64)}
	}
	return &copyTracker{dir: dir, m: m}
}

// owns reports whether rel is a tracked copy the user has not edited.
func (t *copyTracker) owns(rel string) bool {
	sum, ok := t.m.Managed[filepath.ToSlash(rel)]
	if !ok {
		return false
	}
	h, err := utils.FileHash(filepath.Join(t.dir, rel))
	return err == nil && h == sum
}

func (t *copyTracker) record(rel string) {
	key := filepath.ToSlash(rel)
	if h, err := utils.FileHash(filepath.Join(t.dir, rel)); err == nil && t.m.Managed[key] != h {
		t.m.Managed[key] = h
		t.changed = true
	}
}

// recordSource remembers which source content rel was converted from, so
// status can tell a stale extension output from a current one without
// running the extension.
func (t *copyTracker) recordSource(rel, srcPath string) {
	key := filepath.ToSlash(rel)
	h, err := utils.FileHash(srcPath)
	if err != nil || t.m.Sources[key] == h {
		return
	}
	if t.m.Sources == nil {
		t.m.Sources = make(map[string]string)
	}
	t.m.Sources[key] = h
	t.changed = true
}

// sourceMatches reports whether rel was converted from srcPath's current
// content. An output recorded before source fingerprints existed has no
// entry and is taken as current; the next sync records it.
func (t *copyTracker) sourceMatches(rel, srcPath string) bool {
	sum, ok := t.m.Sources[filepath.ToSlash(rel)]
	if !ok {
		return true
	}
	h, err := utils.FileHash(srcPath)
	return err == nil && h == sum
}

func (t *copyTracker) forget(rel string) {
	key := filepath.ToSlash(rel)
	if _, ok := t.m.Managed[key]; ok {
		delete(t.m.Managed, key)
		t.changed = true
	}
	if _, ok := t.m.Sources[key]; ok {
		delete(t.m.Sources, key)
		t.changed = true
	}
}

// pruneOrphans removes tracked copies whose name is not in expected, keeping
// edited ones, and forgets them. It returns the removed names.
func (t *copyTracker) pruneOrphans(expected map[string]bool, dryRun bool) ([]string, error) {
	var removed []string
	var errs []error
	for key := range t.m.Managed {
		rel := filepath.FromSlash(key)
		if expected[rel] {
			continue
		}
		if t.owns(rel) {
			if !dryRun {
				if err := os.Remove(filepath.Join(t.dir, rel)); err != nil {
					errs = append(errs, fmt.Errorf("failed to remove orphaned copy %s: %w", rel, err))
					continue
				}
			}
			removed = append(removed, rel)
		}
		if !dryRun {
			t.forget(rel)
		}
	}
	return removed, errors.Join(errs...)
}

func (t *copyTracker) save() error {
	if !t.changed {
		return nil
	}
	if len(t.m.Managed) == 0 {
		return RemoveManifest(t.dir)
	}
	return WriteManifest(t.dir, t.m)
}
