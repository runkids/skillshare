package config

import (
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
}

// reconcileSkillsWalk walks sourcePath for installed skills (those with metadata
// or tracked repos) and ensures they are present in the MetadataStore.
// onFound is called for each discovered installed skill; pass nil to skip.
func reconcileSkillsWalk(sourcePath string, store *install.MetadataStore, onFound func(fullPath string)) (reconcileResult, error) {
	result := reconcileResult{live: map[string]bool{}}

	walkRoot := utils.ResolveSymlink(sourcePath)
	err := sourcewalk.WalkDir(walkRoot, sourcewalk.Options{}, func(path string, d os.DirEntry, walkErr error) error {
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
		tracked := isGitRepo(path)

		existing := store.GetByPath(fullPath)
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
			if changed, hashErr := store.RefreshTrackedRootSkillHashes(fullPath, path); hashErr == nil && changed {
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

// isGitRepo checks if the given path is a git repository (has .git/ directory or file).
func isGitRepo(path string) bool {
	_, err := os.Stat(filepath.Join(path, ".git"))
	return err == nil
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
