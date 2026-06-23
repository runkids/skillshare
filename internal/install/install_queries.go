package install

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"skillshare/internal/utils"
)

func getUpdatableSkillsImpl(sourceDir string) ([]string, error) {
	store, err := LoadMetadata(sourceDir)
	if err != nil {
		return nil, err
	}

	var skills []string
	for _, name := range store.List() {
		entry := store.Get(name)
		if entry == nil || entry.Source == "" {
			continue
		}
		// Skip tracked repos (they are handled separately)
		if entry.Tracked {
			continue
		}
		skills = append(skills, KeyToRelPath(name, entry))
	}
	return skills, nil
}

// TrackedRepoMeta describes a tracked repository declared in metadata.
type TrackedRepoMeta struct {
	Name   string
	Source string
	Branch string
}

// getMissingTrackedReposImpl returns tracked repositories declared in .metadata.json
// whose clone directories are absent or no longer contain a git checkout.
func getMissingTrackedReposImpl(sourceDir string) ([]TrackedRepoMeta, error) {
	store, err := LoadMetadata(sourceDir)
	if err != nil {
		return nil, err
	}

	existingRepos, err := GetTrackedRepos(sourceDir)
	if err != nil {
		return nil, err
	}
	existing := make(map[string]bool, len(existingRepos))
	for _, repo := range existingRepos {
		existing[filepath.ToSlash(repo)] = true
	}

	var missing []TrackedRepoMeta
	for _, key := range store.List() {
		entry := store.Get(key)
		if entry == nil || !entry.Tracked {
			continue
		}

		relPath := filepath.ToSlash(KeyToRelPath(key, entry))
		if !strings.HasPrefix(filepath.Base(relPath), "_") {
			continue
		}
		if existing[relPath] {
			continue
		}

		missing = append(missing, TrackedRepoMeta{
			Name:   relPath,
			Source: entry.Source,
			Branch: entry.Branch,
		})
	}
	return missing, nil
}

// RehydrateResult reports the outcome of rehydrating one missing tracked repo.
type RehydrateResult struct {
	Name   string `json:"name"`
	Action string `json:"action"` // "rehydrated" | "error"
	Error  string `json:"error,omitempty"`
}

// rehydrateMissingTrackedReposImpl re-clones tracked repos declared in metadata
// whose clone directories are absent on disk. It mirrors the tracked-repo phase
// of InstallFromConfig (bare `skillshare install`); repos already on disk are
// left untouched. See issue #212.
func rehydrateMissingTrackedReposImpl(sourceDir string, parseOpts ParseOptions, opts InstallOptions) ([]RehydrateResult, error) {
	store, err := LoadMetadata(sourceDir)
	if err != nil {
		return nil, err
	}
	existingRepos, err := GetTrackedRepos(sourceDir)
	if err != nil {
		return nil, err
	}
	existing := make(map[string]bool, len(existingRepos))
	for _, repo := range existingRepos {
		existing[filepath.ToSlash(repo)] = true
	}

	opts.Quiet = true
	opts.Update = false
	var results []RehydrateResult
	for _, key := range store.List() {
		entry := store.Get(key)
		if entry == nil || !entry.Tracked {
			continue
		}
		relPath := filepath.ToSlash(KeyToRelPath(key, entry))
		if !strings.HasPrefix(filepath.Base(relPath), "_") {
			continue
		}
		if existing[relPath] {
			continue
		}

		groupDir, bareName := splitTrackedRelPath(relPath)
		source, perr := ParseSourceWithOptions(entry.Source, parseOpts)
		if perr != nil {
			results = append(results, RehydrateResult{Name: relPath, Action: "error", Error: "invalid source: " + perr.Error()})
			continue
		}
		source.Name = bareName
		source.Branch = entry.Branch

		trackOpts := opts
		trackOpts.Name = bareName
		if groupDir != "" {
			trackOpts.Into = groupDir
		}
		if _, ierr := InstallTrackedRepo(source, sourceDir, trackOpts); ierr != nil {
			results = append(results, RehydrateResult{Name: relPath, Action: "error", Error: ierr.Error()})
			continue
		}
		results = append(results, RehydrateResult{Name: relPath, Action: "rehydrated"})
	}
	return results, nil
}

// splitTrackedRelPath splits a tracked repo's relative path into its parent
// group directory and base name (the base retains its leading "_").
func splitTrackedRelPath(relPath string) (group, name string) {
	relPath = strings.Trim(relPath, "/")
	if idx := strings.LastIndex(relPath, "/"); idx >= 0 {
		return relPath[:idx], relPath[idx+1:]
	}
	return "", relPath
}

// FindRepoInstalls scans sourceDir for skills whose meta repo_url matches
// cloneURL. Returns relative paths (e.g. "feature-radar/feature-radar-archive").
// Tracked repos (_-prefixed) are skipped.
func FindRepoInstalls(sourceDir, cloneURL string) []string {
	if cloneURL == "" {
		return nil
	}

	store, err := LoadMetadata(sourceDir)
	if err != nil {
		return nil
	}

	var matches []string
	for _, name := range store.List() {
		entry := store.Get(name)
		if entry == nil || entry.Tracked {
			continue
		}
		if repoURLsMatch(entry.RepoURL, cloneURL) {
			matches = append(matches, KeyToRelPath(name, entry))
		}
	}
	return matches
}

// CheckCrossPathDuplicate checks if a repo is already installed at a different
// location in sourceDir. Returns a user-facing error when duplicates are found
// outside targetPrefix, or nil if safe to proceed.
// Callers should skip this check when force is true or cloneURL is empty.
func CheckCrossPathDuplicate(sourceDir, cloneURL, targetPrefix string) error {
	existing := FindRepoInstalls(sourceDir, cloneURL)
	if len(existing) == 0 {
		return nil
	}
	var elsewhere []string
	for _, rel := range existing {
		sameLocation := false
		if targetPrefix == "" {
			sameLocation = !strings.Contains(rel, "/")
		} else {
			sameLocation = rel == targetPrefix || strings.HasPrefix(rel, targetPrefix+"/")
		}
		if !sameLocation {
			elsewhere = append(elsewhere, rel)
		}
	}
	if len(elsewhere) == 0 {
		return nil
	}
	loc := elsewhere[0]
	if len(elsewhere) > 1 {
		loc = fmt.Sprintf("%s (and %d more)", loc, len(elsewhere)-1)
	}
	return fmt.Errorf(
		"this repo is already installed at skills/%s\n"+
			"Use 'skillshare update' to refresh, or reinstall with --force to allow duplicates",
		loc)
}

// getTrackedReposImpl returns tracked repositories from the source directory.
// It walks subdirectories recursively so repos nested in organizational
// directories (e.g. category/_team-repo/) are found.

func getTrackedReposImpl(sourceDir string) ([]string, error) {
	var repos []string

	walkRoot := utils.ResolveSymlink(sourceDir)
	err := filepath.Walk(walkRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if path == walkRoot {
			return nil
		}
		// Skip .git directories
		if info.IsDir() && info.Name() == ".git" {
			return filepath.SkipDir
		}
		// Look for _-prefixed directories that are git repos
		if info.IsDir() && len(info.Name()) > 0 && info.Name()[0] == '_' {
			gitDir := filepath.Join(path, ".git")
			if _, statErr := os.Stat(gitDir); statErr == nil {
				relPath, relErr := filepath.Rel(walkRoot, path)
				if relErr == nil {
					repos = append(repos, relPath)
				}
				return filepath.SkipDir // Don't recurse into tracked repos
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return repos, nil
}
