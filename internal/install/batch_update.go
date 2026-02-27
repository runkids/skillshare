package install

import (
	"fmt"
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

	// Create a map for quick lookup in discovery
	discoveryMap := make(map[string]SkillInfo)
	for _, s := range discovery.Skills {
		discoveryMap[s.Path] = s
	}

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
