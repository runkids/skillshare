package install

import (
	"fmt"
	"path/filepath"
	"strings"
)

// BatchUpdateProgressPrefix marks per-skill grouped update progress messages
// emitted through InstallOptions.OnProgress.
const BatchUpdateProgressPrefix = "skillshare:batch-update:"

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
	totalSkills := len(skillTargets)
	processedSkills := 0

	// 2. Install/Update each requested skill from the discovered repo
	for repoSubdir, destPath := range skillTargets {
		lookupKey := repoSubdir
		if lookupKey == "" {
			lookupKey = "."
		}
		skillInfo, ok := discoveryMap[lookupKey]
		if !ok {
			result.Errors[repoSubdir] = fmt.Errorf("skill path %q not found in repository", repoSubdir)
			processedSkills++
			emitBatchSkillProgress(opts.OnProgress, processedSkills, totalSkills, repoSubdir)
			continue
		}
		// Fast path for update --all on huge inventories:
		// if installed metadata already points at this exact repo commit/tree,
		// skip reinstall/copy entirely.
		if opts.Update && !opts.DryRun &&
			isSkillCurrentAtRepoState(destPath, repoSubdir, result.CommitHash, repoPath, treeHashCache) {
			// Best-effort metadata refresh:
			// when skipping by tree-hash match on a moved HEAD, keep meta.Version
			// aligned to latest commit to avoid stale version reporting.
			_ = refreshSkillMetaVersionIfNeeded(destPath, result.CommitHash)
			processedSkills++
			emitBatchSkillProgress(opts.OnProgress, processedSkills, totalSkills, repoSubdir)
			continue
		}

		// Use a local copy of options to avoid side effects
		skillOpts := opts
		skillOpts.OnProgress = nil // Progress handled by clone stage

		installResult, err := InstallFromDiscovery(discovery, skillInfo, destPath, skillOpts)
		if err != nil {
			result.Errors[repoSubdir] = err
			processedSkills++
			emitBatchSkillProgress(opts.OnProgress, processedSkills, totalSkills, repoSubdir)
			continue
		}
		result.Results[repoSubdir] = installResult
		processedSkills++
		emitBatchSkillProgress(opts.OnProgress, processedSkills, totalSkills, repoSubdir)
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

	subdir := strings.TrimSpace(repoSubdir)
	metaVersion := strings.TrimSpace(meta.Version)
	commitHash = strings.TrimSpace(commitHash)

	// Fast path: exact repo commit match means installed content is already current.
	// This avoids per-skill git subtree lookups on very large batches.
	if metaVersion != "" && metaVersion == commitHash {
		return true
	}

	if subdir == "" || subdir == "." {
		// Root installs don't have a stable tree hash field; require commit match.
		return false
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

func emitBatchSkillProgress(onProgress ProgressCallback, done, total int, repoSubdir string) {
	if onProgress == nil || total <= 0 {
		return
	}
	label := strings.TrimSpace(repoSubdir)
	if label == "" {
		label = "."
	}
	onProgress(fmt.Sprintf("%s%d/%d %s", BatchUpdateProgressPrefix, done, total, label))
}

func refreshSkillMetaVersionIfNeeded(destPath, commitHash string) error {
	commitHash = strings.TrimSpace(commitHash)
	if commitHash == "" {
		return nil
	}
	meta, err := ReadMeta(destPath)
	if err != nil || meta == nil {
		return err
	}
	if strings.TrimSpace(meta.Version) == commitHash {
		return nil
	}
	meta.Version = commitHash
	return WriteMeta(destPath, meta)
}
