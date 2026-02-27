package install

import (
	"fmt"
	"path/filepath"
	"strings"
)

// BatchUpdateResult holds results for a batch of skills from the same repository
type BatchUpdateResult struct {
	RepoURL    string
	CommitHash string
	Results    map[string]*InstallResult // map[skillRelPath]result
	Errors     map[string]error          // map[skillRelPath]error
}

// UpdateSkillsFromRepo updates multiple skills that belong to the same git repository.
// It clones the repository once to a temporary directory and then installs/updates
// each requested skill from that local copy.
//
// skillTargets maps repo-internal subdir (meta.Subdir) → local absolute destination path.
// This avoids assuming the local path mirrors the repo structure.
func UpdateSkillsFromRepo(repoURL string, skillTargets map[string]string, opts InstallOptions) (*BatchUpdateResult, error) {
	if repoURL == "" {
		return nil, fmt.Errorf("repoURL is required")
	}

	source, err := ParseSource(repoURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse repo URL %q: %w", repoURL, err)
	}

	// 1. Discover skills from the repo (clones once)
	discovery, err := DiscoverFromGitWithProgress(source, opts.OnProgress)
	if err != nil {
		return nil, err
	}
	defer CleanupDiscovery(discovery)

	result := &BatchUpdateResult{
		RepoURL:    repoURL,
		CommitHash: discovery.CommitHash,
		Results:    make(map[string]*InstallResult),
		Errors:     make(map[string]error),
	}
	repoPath := filepath.Join(discovery.RepoPath, "repo")
	if result.CommitHash == "" {
		if hash, hashErr := getGitCommit(repoPath); hashErr == nil {
			result.CommitHash = hash
		}
	}

	// Create a map for quick lookup in discovery
	discoveryMap := make(map[string]SkillInfo)
	for _, s := range discovery.Skills {
		discoveryMap[s.Path] = s
	}
	treeHashCache := make(map[string]string)

	// 2. Install/Update each requested skill from the discovered repo
	for repoSubdir, destPath := range skillTargets {
		lookupKey := repoSubdir
		if lookupKey == "" {
			lookupKey = "."
		}
		skillInfo, ok := discoveryMap[lookupKey]
		if !ok {
			result.Errors[repoSubdir] = fmt.Errorf("skill path %q not found in repository", repoSubdir)
			continue
		}
		// Fast path for update --all on huge inventories:
		// if installed metadata already points at this exact repo commit/tree,
		// skip reinstall/copy entirely.
		if opts.Update && !opts.DryRun &&
			isSkillCurrentAtRepoState(destPath, repoSubdir, result.CommitHash, repoPath, treeHashCache) {
			continue
		}

		// Use a local copy of options to avoid side effects
		skillOpts := opts
		skillOpts.OnProgress = nil // Progress handled by clone stage

		installResult, err := InstallFromDiscovery(discovery, skillInfo, destPath, skillOpts)
		if err != nil {
			result.Errors[repoSubdir] = err
			continue
		}
		result.Results[repoSubdir] = installResult
	}

	return result, nil
}

// isSkillCurrentAtRepoState returns true when installed metadata already
// matches the latest fetched repo state for this skill path.
func isSkillCurrentAtRepoState(destPath, repoSubdir, commitHash, repoPath string, treeHashCache map[string]string) bool {
	if commitHash == "" || repoPath == "" {
		return false
	}
	meta, err := ReadMeta(destPath)
	if err != nil || meta == nil {
		return false
	}
	if strings.TrimSpace(meta.Version) != strings.TrimSpace(commitHash) {
		return false
	}

	subdir := strings.TrimSpace(repoSubdir)
	if subdir == "" || subdir == "." {
		// Root installs don't have a stable tree hash field; commit match is enough.
		return true
	}
	if strings.TrimSpace(meta.TreeHash) == "" {
		return false
	}

	treeHash, ok := treeHashCache[subdir]
	if !ok {
		treeHash = getSubdirTreeHash(repoPath, subdir)
		treeHashCache[subdir] = treeHash
	}
	return treeHash != "" && meta.TreeHash == treeHash
}
