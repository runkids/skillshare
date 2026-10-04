package install

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"skillshare/internal/sourcewalk"
	"skillshare/internal/utils"
)

// GetUpdatableSkillsWithOptions returns the metadata paths of installed skills
// with a remote source, excluding tracked repositories. The list comes from
// metadata, but it fails like discovery when the operation's snapshot is
// incomplete, so callers cannot act on a partial view of the source.
func GetUpdatableSkillsWithOptions(sourceDir string, opts sourcewalk.Options) ([]string, error) {
	if opts.Follow.Err() != nil {
		return nil, opts.Follow.Err()
	}
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

// GetMissingTrackedReposWithOptions returns tracked repositories declared in
// .metadata.json whose clone directories are absent or no longer contain a git
// checkout. It uses the same ownership snapshot as discovery.
func GetMissingTrackedReposWithOptions(sourceDir string, opts sourcewalk.Options) ([]TrackedRepoMeta, error) {
	store, err := LoadMetadata(sourceDir)
	if err != nil {
		return nil, err
	}

	existingRepos, err := GetTrackedReposWithOptions(sourceDir, opts)
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
		// A declared entry's content belongs to its external owner, even while
		// the link is offline; recreating it would materialize the boundary.
		if opts.Follow != nil {
			if _, followed := opts.Follow.InFollowed(relPath); followed {
				continue
			}
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
	missing, err := GetMissingTrackedReposWithOptions(sourceDir, sourcewalk.Options{Follow: opts.Follow})
	if err != nil {
		return nil, err
	}

	opts.Quiet = true
	opts.Update = false
	var results []RehydrateResult
	for _, repo := range missing {
		relPath := repo.Name
		groupDir, bareName := splitTrackedRelPath(relPath)
		source, perr := ParseSourceWithOptions(repo.Source, parseOpts)
		if perr != nil {
			results = append(results, RehydrateResult{Name: relPath, Action: "error", Error: "invalid source: " + perr.Error()})
			continue
		}
		source.Name = bareName
		source.ApplyRecordedBranch(repo.Branch)

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

// GetTrackedReposWithOptions returns tracked repositories in the source. It walks
// subdirectories recursively so repos nested in organizational directories
// (e.g. category/_team-repo/) are found. Callers pass the operation's FollowSet;
// a nil Follow keeps plain traversal.
func GetTrackedReposWithOptions(sourceDir string, opts sourcewalk.Options) ([]string, error) {
	var repos []string

	walkRoot := utils.ResolveSymlink(sourceDir)
	if opts.Follow != nil {
		walkRoot = filepath.Clean(sourceDir)
	}
	err := sourcewalk.Walk(walkRoot, opts, func(path string, info os.FileInfo, err error) error {
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
	if opts.Follow.Err() != nil {
		return nil, opts.Follow.Err()
	}
	return repos, nil
}
