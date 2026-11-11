package check

import (
	"path/filepath"
	"strings"

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
	Branch   string `json:"branch,omitempty"`
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
	if branch, err := git.GetCurrentBranch(result.RepoPath); err == nil && branch != "HEAD" {
		result.Branch = branch
	}

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
	result.Branch = entry.Branch

	if entry.AgentSourceLost() {
		result.Status = "error"
		result.Message = install.ErrAgentSourceLost.Error()
		return result
	}

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
// against the remote head of the branch they were installed from.
// Each remote branch is probed once; every agent is compared with its own version.
func EnrichAgentResultsWithRemote(results []AgentCheckResult, onDone func(), parseOpts install.ParseOptions) {
	groups := make(map[remoteKey][]int)
	var refs []remoteKey
	for i, r := range results {
		if r.RepoURL == "" || r.Version == "" {
			continue
		}
		ref := remoteKey{url: r.RepoURL, branch: r.Branch}
		if _, ok := groups[ref]; !ok {
			refs = append(refs, ref)
		}
		groups[ref] = append(groups[ref], i)
	}

	if len(refs) == 0 {
		return
	}

	inputs := make([]URLCheckInput, len(refs))
	for i, ref := range refs {
		inputs[i] = URLCheckInput{RepoURL: ref.url, Branch: ref.branch, ParseOptions: parseOpts}
	}

	outputs := ParallelCheckURLs(inputs, onDone)

	for i, out := range outputs {
		for _, idx := range groups[refs[i]] {
			if out.Err != nil {
				results[idx].Status = "error"
				results[idx].Message = out.Err.Error()
				continue
			}
			// The remote hash is abbreviated; a tracked repo records the full one.
			if out.RemoteHash != "" && !strings.HasPrefix(results[idx].Version, out.RemoteHash) && results[idx].Status == "up_to_date" {
				results[idx].Status = "update_available"
				results[idx].Message = "newer version available"
			}
		}
	}
}
