package config

import (
	"fmt"
	"os"
	"os/exec"
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
}

// reconcileSkillsWalk walks sourcePath for installed skills (those with metadata
// or tracked repos) and ensures they are present in the MetadataStore.
// onFound is called for each discovered installed skill; pass nil to skip.
func reconcileSkillsWalk(sourcePath string, walk sourcewalk.Options, store *install.MetadataStore, onFound func(fullPath string)) (reconcileResult, error) {
	result := reconcileResult{live: map[string]bool{}}
	moves := map[string][]string{} // gone record key -> candidate destinations

	walkRoot := utils.ResolveSymlink(sourcePath)
	err := sourcewalk.WalkDir(walkRoot, walk, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
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

		existing := store.Get(fullPath)
		if existing == nil || !tracked {
			existing = store.GetByPath(fullPath)
			if tracked && existing != nil && existing.Group != group {
				existing = nil
			}
		}
		// A skill moved with mv takes its record along once the walk shows a
		// single destination.
		if existing == nil && !tracked {
			if key := store.MovedEntryKey(walkRoot, fullPath, path, walk.Follow); key != "" {
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
		if len(dests) != 1 {
			continue
		}
		entry := store.Get(key)
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

	if names := walk.Follow.Unavailable(); len(names) > 0 {
		// The walk missed whatever those links hold; that is not a removal.
		result.incomplete = true
		fmt.Fprintf(os.Stderr, "Warning: source link %s is unavailable; kept its install metadata\n", strings.Join(names, ", "))
	}
	return result, err
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
