package check

import (
	"path/filepath"

	"skillshare/internal/git"
	"skillshare/internal/install"
	"skillshare/internal/resource"
	"skillshare/internal/utils"
)

// AgentCheckResult holds the check result for a single agent.
type AgentCheckResult struct {
	Name     string `json:"name"`
	Source   string `json:"source,omitempty"`
	Version  string `json:"version,omitempty"`
	RepoURL  string `json:"repoUrl,omitempty"`
	RepoPath string `json:"repoPath,omitempty"`
	Status   string `json:"status"` // "up_to_date", "drifted", "dirty", "local", "error", "update_available"
	Message  string `json:"message,omitempty"`
}

// CheckAgents scans the agents source directory for installed agents and
// compares their file hashes against metadata to detect drift.
// Uses resource.AgentKind{}.Discover() to recurse into subdirectories.
func CheckAgents(agentsDir string) []AgentCheckResult {
	discovered, err := resource.AgentKind{}.Discover(agentsDir)
	if err != nil {
		return nil
	}

	// Load centralized metadata store (auto-migrates any lingering sidecars).
	store, loadErr := install.LoadMetadata(agentsDir)

	var results []AgentCheckResult
	for _, d := range discovered {
		if loadErr != nil {
			// Surface corruption instead of silently treating all agents as local.
			key := d.RelPath[:len(d.RelPath)-len(".md")]
			results = append(results, AgentCheckResult{
				Name:    key,
				Status:  "error",
				Message: "invalid metadata: " + loadErr.Error(),
			})
			continue
		}
		result := checkOneAgent(store, d.SourcePath, d.RelPath)
		if d.RepoRelPath != "" {
			result = checkTrackedAgentRepo(agentsDir, d)
		}
		results = append(results, result)
	}

	return results
}

func checkTrackedAgentRepo(agentsDir string, d resource.DiscoveredResource) AgentCheckResult {
	result := AgentCheckResult{
		Name:     d.RelPath[:len(d.RelPath)-len(".md")],
		RepoPath: filepath.Join(agentsDir, filepath.FromSlash(d.RepoRelPath)),
	}

	if !install.IsGitRepo(result.RepoPath) {
		result.Status = "local"
		return result
	}

	repoURL, _ := git.GetRemoteURL(result.RepoPath)
	version, _ := git.GetCurrentFullHash(result.RepoPath)
	result.Source = repoURL
	result.RepoURL = repoURL
	result.Version = version

	if repoURL == "" || version == "" {
		result.Status = "local"
		return result
	}

	if isDirty, _ := git.IsDirty(result.RepoPath); isDirty {
		result.Status = "dirty"
		result.Message = "tracked repo has uncommitted changes"
		return result
	}

	result.Status = "up_to_date"
	return result
}

// checkOneAgent checks a single agent file against the centralized metadata store.
// sourcePath is the absolute path to the .md file; relPath is relative to the
// agents root (e.g. "demo/code-reviewer.md").
func checkOneAgent(store *install.MetadataStore, sourcePath, relPath string) AgentCheckResult {
	fileName := filepath.Base(relPath)
	key := relPath[:len(relPath)-len(".md")]
	result := AgentCheckResult{Name: key}

	entry := store.GetByPath(key)
	if entry == nil || entry.Source == "" {
		result.Status = "local"
		return result
	}

	result.Source = entry.Source
	result.Version = entry.Version
	result.RepoURL = entry.RepoURL

	// Compare file hash
	if entry.FileHashes == nil || entry.FileHashes[fileName] == "" {
		result.Status = "local"
		return result
	}

	currentHash, err := utils.FileHashFormatted(sourcePath)
	if err != nil {
		result.Status = "error"
		result.Message = "cannot hash file"
		return result
	}

	if currentHash == entry.FileHashes[fileName] {
		result.Status = "up_to_date"
	} else {
		result.Status = "drifted"
		result.Message = "file content changed since install"
	}

	return result
}

// EnrichAgentResultsWithRemote checks agents that have RepoURL + Version
// against their remote HEAD to detect available updates.
// Uses ParallelCheckURLs for efficient batched remote probing.
func EnrichAgentResultsWithRemote(results []AgentCheckResult, onDone func()) {
	// Collect unique repo URLs that have version info
	type agentRef struct {
		repoURL string
		version string
		indices []int
	}
	urlMap := make(map[string]*agentRef)
	for i, r := range results {
		if r.RepoURL == "" || r.Version == "" {
			continue
		}
		if ref, ok := urlMap[r.RepoURL]; ok {
			ref.indices = append(ref.indices, i)
		} else {
			urlMap[r.RepoURL] = &agentRef{
				repoURL: r.RepoURL,
				version: r.Version,
				indices: []int{i},
			}
		}
	}

	if len(urlMap) == 0 {
		return
	}

	// Build URL check inputs
	var inputs []URLCheckInput
	var refs []*agentRef
	for _, ref := range urlMap {
		inputs = append(inputs, URLCheckInput{RepoURL: ref.repoURL})
		refs = append(refs, ref)
	}

	outputs := ParallelCheckURLs(inputs, onDone)

	// Apply results
	for i, out := range outputs {
		ref := refs[i]
		if out.Err != nil {
			for _, idx := range ref.indices {
				results[idx].Status = "error"
				results[idx].Message = out.Err.Error()
			}
			continue
		}
		if out.RemoteHash != "" && out.RemoteHash != ref.version {
			for _, idx := range ref.indices {
				if results[idx].Status == "up_to_date" {
					results[idx].Status = "update_available"
					results[idx].Message = "newer version available"
				}
			}
		}
	}
}
