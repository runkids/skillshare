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

// ReconcileOptions carries one operation's policy into a skills reconcile.
type ReconcileOptions struct {
	// Follow is the operation's .skillfollow snapshot; nil keeps legacy rules.
	Follow *sourcewalk.FollowSet
}

// reconcileSkillsWalk walks sourcePath for installed skills (those with metadata
// or tracked repos) and ensures they are present in the MetadataStore.
// onFound is called for each discovered installed skill; pass nil to skip.
// Names under a followed entry are marked live, but their metadata is never
// created or updated: that tree belongs to the user.
func reconcileSkillsWalk(sourcePath string, store *install.MetadataStore, onFound func(fullPath string), follow *sourcewalk.FollowSet) (reconcileResult, error) {
	result := reconcileResult{live: map[string]bool{}}

	walkRoot := utils.ResolveSymlink(sourcePath)
	err := sourcewalk.WalkDir(walkRoot, sourcewalk.Options{Follow: follow}, func(path string, d os.DirEntry, walkErr error) error {
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
		if follow != nil {
			if _, followed := follow.InFollowed(fullPath); followed {
				existing := store.GetByPath(fullPath)
				if (existing != nil && existing.Source != "") || isGitRepo(path) {
					result.live[fullPath] = true
					return filepath.SkipDir
				}
				return nil
			}
		}

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

// pruneStaleEntries removes store entries not present in the live set. Entries
// under an unavailable followed entry are kept: their absence proves nothing.
// Keys are compared as logical paths: a legacy basename key carries its group
// in the entry, and followed names are never migrated, so the raw key alone
// would not match the live set or the declaration.
func pruneStaleEntries(store *install.MetadataStore, live map[string]bool, follow *sourcewalk.FollowSet) bool {
	changed := false
	for _, name := range store.List() {
		rel := install.KeyToRelPath(name, store.Get(name))
		if follow != nil {
			if entry, declared := follow.InFollowed(rel); declared && entry.State != sourcewalk.Followed {
				continue
			}
		}
		if !live[rel] {
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
