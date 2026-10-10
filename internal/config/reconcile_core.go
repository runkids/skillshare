package config

import (
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"

	"skillshare/internal/install"
	"skillshare/internal/sourcewalk"
	"skillshare/internal/utils"
)

// reconcileResult holds the output of a reconcile walk.
type reconcileResult struct {
	live    map[string]bool
	changed bool
	// incomplete means a followed source link could not be read, so entries
	// absent from live may still exist and must not be removed.
	incomplete bool
	// moved maps the old source-relative path of each record that followed a
	// hand-moved copy to its new one.
	moved map[string]string
}

// reconcileSkillsWalk walks sourcePath for installed skills (those with metadata
// or tracked repos) and ensures they are present in the MetadataStore.
// onFound is called for each discovered installed skill; pass nil to skip.
// canMove limits which gone records may follow a moved copy; nil allows all.
func reconcileSkillsWalk(sourcePath string, walk sourcewalk.Options, store *install.MetadataStore, onFound func(fullPath string), canMove func(key string) bool) (reconcileResult, error) {
	result := reconcileResult{live: map[string]bool{}}
	moves := map[string][]string{} // gone record key -> candidate destinations
	walkFailed := false            // an unreadable directory may hide a copy

	walkRoot := utils.ResolveSymlink(sourcePath)
	err := sourcewalk.WalkDir(walkRoot, walk, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			walkFailed = true
			return nil
		}
		if path == walkRoot {
			return nil
		}
		if !d.IsDir() {
			return nil
		}
		if utils.IsHidden(d.Name()) {
			return filepath.SkipDir
		}
		if d.Name() == ".git" {
			return filepath.SkipDir
		}

		relPath, relErr := filepath.Rel(walkRoot, path)
		if relErr != nil {
			return nil
		}

		fullPath := filepath.ToSlash(relPath)

		group := ""
		if idx := strings.LastIndex(fullPath, "/"); idx >= 0 {
			group = fullPath[:idx]
		}

		var source string
		tracked := install.IsTrackedCheckout(path)

		// A basename key is this skill's record only when its group matches. A
		// top-level record found under another folder is a move candidate, and
		// MovedEntryKey below decides on gone path, hashes and uniqueness.
		existing := store.Get(fullPath)
		if existing == nil {
			if e := store.GetByPath(fullPath); e != nil && e.Group == group {
				existing = e
			}
		}
		// A skill moved with mv takes its record along once the walk shows a
		// single destination.
		if existing == nil && !tracked {
			key, hashErr := store.MovedEntryKey(walkRoot, fullPath, path, walk.Follow)
			if hashErr != nil {
				walkFailed = true
			}
			if key != "" && (canMove == nil || canMove(key)) {
				moves[key] = append(moves[key], fullPath)
				return filepath.SkipDir
			}
		}
		if existing != nil && existing.Source != "" {
			source = existing.Source
		} else if tracked {
			source = gitRemoteOrigin(path)
		}
		if source == "" {
			return nil
		}

		result.live[fullPath] = true

		var branch string
		if existing != nil && existing.Branch != "" {
			branch = existing.Branch
		} else if tracked {
			branch = gitCurrentBranch(path)
		}

		if existing != nil {
			if store.MigrateLegacyKey(fullPath, existing) {
				result.changed = true
			}
			if existing.Source != source {
				existing.Source = source
				result.changed = true
			}
			if existing.Tracked != tracked {
				existing.Tracked = tracked
				result.changed = true
			}
			if existing.Branch != branch {
				existing.Branch = branch
				result.changed = true
			}
			if existing.Group != group {
				existing.Group = group
				result.changed = true
			}
		} else {
			entry := &install.MetadataEntry{
				Source:  source,
				Tracked: tracked,
				Branch:  branch,
				Group:   group,
			}
			store.Set(fullPath, entry)
			result.changed = true
		}

		if tracked {
			if changed, hashErr := store.RefreshTrackedRootSkillHashes(fullPath, path, walk.Follow); hashErr == nil && changed {
				result.changed = true
			}
		}

		if onFound != nil {
			onFound(fullPath)
		}

		if tracked {
			return filepath.SkipDir
		}
		if existing != nil && existing.Source != "" {
			return filepath.SkipDir
		}

		return nil
	})

	for key, dests := range moves {
		// An unreadable source link may hide another copy, so wait for it.
		if len(dests) != 1 || walkFailed || walk.Follow.Incomplete() {
			continue
		}
		entry := store.Get(key)
		if result.moved == nil {
			result.moved = map[string]string{}
		}
		result.moved[filepath.ToSlash(install.KeyToRelPath(key, entry))] = dests[0]
		store.MoveEntry(key, dests[0])
		entry.Group = ""
		if idx := strings.LastIndex(dests[0], "/"); idx >= 0 {
			entry.Group = dests[0][:idx]
		}
		result.live[dests[0]] = true
		result.changed = true
		if onFound != nil {
			onFound(dests[0])
		}
	}

	keepNestedRecords(walkRoot, store, result.live)

	// A record left waiting on a failed walk must not be pruned as gone.
	if walkFailed && len(moves) > 0 {
		result.incomplete = true
	}

	if names := walk.Follow.Unavailable(); len(names) > 0 {
		// The walk missed whatever those links hold; that is not a removal.
		result.incomplete = true
		fmt.Fprintf(os.Stderr, "Warning: source link %s is unavailable; kept its install metadata\n", strings.Join(names, ", "))
	}
	return result, err
}

// anyRecordGone reports whether a plain record's directory is missing, the
// only state a hand move leaves behind. Adoption skips the walk otherwise.
func anyRecordGone(sourcePath string, store *install.MetadataStore) bool {
	for _, key := range store.List() {
		entry := store.Get(key)
		if entry.Tracked || len(entry.FileHashes) == 0 {
			continue
		}
		if _, err := os.Lstat(filepath.Join(sourcePath, filepath.FromSlash(install.KeyToRelPath(key, entry)))); os.IsNotExist(err) {
			return true
		}
	}
	return false
}

// keepNestedRecords marks the records below a live plain install as live while
// their directory still exists. The walk stops at an installed skill, so it
// never reaches a nested record; pruning it would lose its audit acceptances.
// A tracked checkout is left out: its skills are not recorded by path.
func keepNestedRecords(walkRoot string, store *install.MetadataStore, live map[string]bool) {
	for _, key := range store.List() {
		if live[key] {
			continue
		}
		rel := filepath.ToSlash(install.KeyToRelPath(key, store.Get(key)))
		for parent := path.Dir(rel); parent != "."; parent = path.Dir(parent) {
			owner := store.Get(parent)
			if !live[parent] || owner == nil || owner.Tracked || owner.Source == "" {
				continue
			}
			if info, err := os.Stat(filepath.Join(walkRoot, filepath.FromSlash(rel))); err == nil && info.IsDir() {
				live[key], live[rel] = true, true
			}
			break
		}
	}
}

// pruneStaleEntries removes store entries not present in the live set.
func pruneStaleEntries(store *install.MetadataStore, live map[string]bool) bool {
	changed := false
	for _, name := range store.List() {
		if !live[name] {
			store.Remove(name)
			changed = true
		}
	}
	return changed
}

// gitCurrentBranch returns the current branch name for a git repo, or "" on failure.
func gitCurrentBranch(repoPath string) string {
	cmd := exec.Command("git", "-C", repoPath, "rev-parse", "--abbrev-ref", "HEAD")
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// gitRemoteOrigin returns the "origin" remote URL for a git repo, or "" on failure.
func gitRemoteOrigin(repoPath string) string {
	cmd := exec.Command("git", "-C", repoPath, "remote", "get-url", "origin")
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}
