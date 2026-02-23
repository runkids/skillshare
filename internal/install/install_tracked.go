package install

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func installTrackedRepoImpl(source *Source, sourceDir string, opts InstallOptions) (*TrackedRepoResult, error) {
	if !source.IsGit() {
		return nil, fmt.Errorf("--track requires a git repository source")
	}

	// Determine repo name: opts.Name > TrackName (owner-repo) > source.Name
	repoName := opts.Name
	if repoName == "" {
		repoName = source.TrackName()
	}
	if repoName == "" {
		repoName = source.Name
	}

	// Prefix with _ to indicate tracked repo (avoid double prefix if user already added _)
	trackedName := repoName
	if !strings.HasPrefix(repoName, "_") {
		trackedName = "_" + repoName
	}
	if err := validateTrackedRepoDirName(trackedName); err != nil {
		return nil, fmt.Errorf("invalid tracked repo name %q: %w", trackedName, err)
	}
	destBase := sourceDir
	if opts.Into != "" {
		destBase = filepath.Join(sourceDir, opts.Into)
		if err := os.MkdirAll(destBase, 0755); err != nil {
			return nil, fmt.Errorf("failed to create --into directory: %w", err)
		}
	}
	destPath := filepath.Join(destBase, trackedName)

	result := &TrackedRepoResult{
		RepoName: trackedName,
		RepoPath: destPath,
	}

	// Check if already exists
	if _, err := os.Stat(destPath); err == nil {
		if opts.Update {
			return updateTrackedRepo(destPath, result, opts)
		}
		if !opts.Force {
			return nil, fmt.Errorf("tracked repo '%s' already exists. To overwrite:\n       skillshare install %s --track --force", trackedName, source.Raw)
		}
		// Force mode - remove existing
		if !opts.DryRun {
			if err := os.RemoveAll(destPath); err != nil {
				return nil, fmt.Errorf("failed to remove existing repo: %w", err)
			}
		}
	}

	if opts.DryRun {
		result.Action = "would clone"
		return result, nil
	}

	// Clone tracked repos with a download-optimized strategy first, then
	// fallback to the legacy full clone for compatibility.
	if err := cloneTrackedRepo(source.CloneURL, source.Subdir, destPath, opts.OnProgress); err != nil {
		return nil, fmt.Errorf("failed to clone repository: %w", err)
	}

	// Discover skills in the cloned repo (exclude root for tracked repos)
	skills := discoverSkills(destPath, false)
	result.SkillCount = len(skills)
	for _, skill := range skills {
		result.Skills = append(result.Skills, skill.Name)
	}

	if len(skills) == 0 {
		result.Warnings = append(result.Warnings, "no SKILL.md files found in repository")
	}

	// Security audit on the entire tracked repo
	if err := auditTrackedRepo(destPath, result, opts); err != nil {
		return nil, err
	}

	// Auto-add to .gitignore to prevent committing tracked repo contents
	gitignoreEntry := trackedName
	if opts.Into != "" {
		gitignoreEntry = filepath.Join(opts.Into, trackedName)
	}
	if err := UpdateGitIgnore(sourceDir, gitignoreEntry); err != nil {
		result.Warnings = append(result.Warnings, fmt.Sprintf("failed to update .gitignore: %v", err))
	}

	result.Action = "cloned"
	return result, nil
}

// updateTrackedRepo performs git pull on an existing tracked repo
func updateTrackedRepo(repoPath string, result *TrackedRepoResult, opts InstallOptions) (*TrackedRepoResult, error) {
	if !isGitRepo(repoPath) {
		return nil, fmt.Errorf("'%s' is not a git repository", repoPath)
	}

	if opts.DryRun {
		result.Action = "would update (git pull)"
		return result, nil
	}

	// Record hash before pull for rollback on audit failure
	beforeHash, err := getGitFullHash(repoPath)
	if err != nil {
		return nil, fmt.Errorf("failed to determine rollback commit before update (aborting for safety): %w", err)
	}
	if beforeHash == "" {
		return nil, fmt.Errorf("failed to determine rollback commit before update (aborting for safety): empty commit hash")
	}

	if err := gitPull(repoPath, opts.OnProgress); err != nil {
		return nil, fmt.Errorf("failed to update: %w", err)
	}

	// Post-pull audit: rollback via git reset (not os.RemoveAll) to preserve repo.
	if err := auditTrackedRepoUpdate(repoPath, beforeHash, result, opts); err != nil {
		return nil, err
	}

	// Re-discover skills (exclude root for tracked repos)
	skills := discoverSkills(repoPath, false)
	result.SkillCount = len(skills)
	for _, skill := range skills {
		result.Skills = append(result.Skills, skill.Name)
	}

	result.Action = "updated"
	return result, nil
}

// cloneRepoFull performs a full git clone (quiet mode for cleaner output)
func cloneRepoFull(url, destPath string, onProgress ProgressCallback) error {
	args := []string{"clone", "--quiet", url, destPath}
	if onProgress != nil {
		args = []string{"clone", "--progress", url, destPath}
	}
	return runGitCommandWithProgress(args, "", authEnv(url), onProgress)
}

// cloneTrackedRepo clones a tracked repository with an optimized payload first
// and falls back to full clone when the remote does not support partial/shallow
// capabilities.
//
// When subdir is provided, sparse checkout is attempted first to reduce payload
// while preserving .git for future tracked updates.
func cloneTrackedRepo(url, subdir, destPath string, onProgress ProgressCallback) error {
	subdir = strings.TrimSpace(subdir)
	if subdir != "" && gitSupportsSparseCheckout() {
		if onProgress != nil {
			onProgress("Preparing sparse checkout...")
		}
		if err := sparseCloneSubdir(url, subdir, destPath, authEnv(url), onProgress); err == nil {
			return nil
		} else if shouldFallbackSparseTrackedClone(err) {
			// sparseCloneSubdir may have already created destPath. Clean it before
			// falling back to a standard clone strategy.
			if cleanupErr := removeAll(destPath); cleanupErr != nil {
				return fmt.Errorf("sparse checkout failed (%v), and cleanup failed: %w", err, cleanupErr)
			}
			if onProgress != nil {
				onProgress("Sparse checkout unavailable; retrying standard clone...")
			}
		} else {
			return err
		}
	}

	args := []string{
		"clone",
		"--filter=blob:none",
		"--depth", "1",
		"--single-branch",
		"--quiet",
		url,
		destPath,
	}
	if onProgress != nil {
		args = []string{
			"clone",
			"--filter=blob:none",
			"--depth", "1",
			"--single-branch",
			"--progress",
			url,
			destPath,
		}
	}

	err := runGitCommandWithProgress(args, "", authEnv(url), onProgress)
	if err == nil {
		return nil
	}
	if !shouldFallbackTrackedClone(err) {
		return err
	}
	if onProgress != nil {
		onProgress("Remote lacks partial clone support; retrying standard clone...")
	}
	return cloneRepoFull(url, destPath, onProgress)
}

func shouldFallbackSparseTrackedClone(err error) bool {
	if err == nil {
		return false
	}

	s := strings.ToLower(err.Error())
	if strings.Contains(s, "authentication failed") ||
		strings.Contains(s, "could not read username") ||
		strings.Contains(s, "terminal prompts disabled") ||
		strings.Contains(s, "permission denied") ||
		strings.Contains(s, "repository not found") {
		return false
	}

	// For compatibility, fallback to the legacy tracked clone flow for
	// capability, sparse-path, and server-specific sparse checkout errors.
	return true
}

func shouldFallbackTrackedClone(err error) bool {
	if err == nil {
		return false
	}

	s := strings.ToLower(err.Error())
	if strings.Contains(s, "authentication failed") ||
		strings.Contains(s, "could not read username") ||
		strings.Contains(s, "terminal prompts disabled") ||
		strings.Contains(s, "permission denied") ||
		strings.Contains(s, "repository not found") {
		return false
	}

	capabilityHints := []string{
		"does not support",
		"not support",
		"filter",
		"shallow",
		"depth",
		"single-branch",
		"partial clone",
		"dumb http",
	}
	for _, hint := range capabilityHints {
		if strings.Contains(s, hint) {
			return true
		}
	}
	return false
}

// GetUpdatableSkills returns skill names that have metadata with a remote source.
// It walks subdirectories recursively so nested skills are found.
