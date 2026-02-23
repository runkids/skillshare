package install

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"skillshare/internal/utils"
)

func discoverFromGitWithProgressImpl(source *Source, onProgress ProgressCallback) (*DiscoveryResult, error) {
	if !isGitInstalled() {
		return nil, fmt.Errorf("git is not installed or not in PATH")
	}

	// Clone to temp directory
	tempDir, err := os.MkdirTemp("", "skillshare-discover-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp directory: %w", err)
	}

	repoPath := filepath.Join(tempDir, "repo")
	if err := cloneRepo(source.CloneURL, repoPath, true, onProgress); err != nil {
		os.RemoveAll(tempDir)
		return nil, fmt.Errorf("failed to clone repository: %w", err)
	}

	// Discover skills (include root to support single-skill-at-root repos)
	skills := discoverSkills(repoPath, true)

	// Fix root skill name: temp dir gives random name, use source.Name instead
	for i := range skills {
		if skills[i].Path == "." {
			skills[i].Name = source.Name
			break
		}
	}

	commitHash, _ := getGitCommit(repoPath)

	return &DiscoveryResult{
		RepoPath:   tempDir,
		Skills:     skills,
		Source:     source,
		CommitHash: commitHash,
	}, nil
}

// discoverFromGitImpl is the non-progress variant used by the public facade.
func discoverFromGitImpl(source *Source) (*DiscoveryResult, error) {
	return discoverFromGitWithProgressImpl(source, nil)
}

// resolveSubdir resolves a subdirectory path within a cloned repo.
// It first checks for an exact match. If not found, it scans the repo for
// SKILL.md files and looks for a skill whose name matches filepath.Base(subdir).
// Returns the resolved subdir path (may differ from input) or an error.

func resolveSubdir(repoPath, subdir string) (string, error) {
	// 1. Exact match — fast path
	exact := filepath.Join(repoPath, subdir)
	info, err := os.Stat(exact)
	if err == nil {
		if !info.IsDir() {
			return "", fmt.Errorf("'%s' is not a directory", subdir)
		}
		return subdir, nil
	}
	if !os.IsNotExist(err) {
		return "", fmt.Errorf("cannot access subdirectory: %w", err)
	}

	// 2. Fuzzy match — scan for SKILL.md files whose directory basename matches
	baseName := filepath.Base(subdir)
	skills := discoverSkills(repoPath, false) // exclude root
	var candidates []string
	for _, sk := range skills {
		if sk.Name == baseName {
			candidates = append(candidates, sk.Path)
		}
	}

	switch len(candidates) {
	case 0:
		return "", fmt.Errorf("subdirectory '%s' does not exist in repository", subdir)
	case 1:
		return candidates[0], nil
	default:
		return "", fmt.Errorf("subdirectory '%s' is ambiguous — multiple matches found:\n  %s",
			subdir, strings.Join(candidates, "\n  "))
	}
}

// readSkillIgnore reads a .skillignore file from the given directory.
// Returns a list of patterns (exact names or trailing-wildcard like "prefix-*").
// Lines starting with # and empty lines are skipped.
func readSkillIgnore(dir string) []string {
	data, err := os.ReadFile(filepath.Join(dir, ".skillignore"))
	if err != nil {
		return nil
	}
	var patterns []string
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		patterns = append(patterns, line)
	}
	return patterns
}

// matchSkillIgnore returns true if skillPath matches any pattern.
// Matching is path-based: exact path, group prefix (pattern matches a
// directory prefix of skillPath), and trailing wildcard ("prefix-*").
func matchSkillIgnore(skillPath string, patterns []string) bool {
	for _, p := range patterns {
		if strings.HasSuffix(p, "*") {
			if strings.HasPrefix(skillPath, strings.TrimSuffix(p, "*")) {
				return true
			}
		} else if skillPath == p || strings.HasPrefix(skillPath, p+"/") {
			return true
		}
	}
	return false
}

// discoverSkills finds directories containing SKILL.md
// If includeRoot is true, root-level SKILL.md is also included (with Path=".")
func discoverSkills(repoPath string, includeRoot bool) []SkillInfo {
	var skills []SkillInfo

	filepath.Walk(repoPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}

		// Skip .git directory
		if info.IsDir() && info.Name() == ".git" {
			return filepath.SkipDir
		}

		// Check if this is a SKILL.md file
		if !info.IsDir() && info.Name() == "SKILL.md" {
			skillDir := filepath.Dir(path)
			relPath, _ := filepath.Rel(repoPath, skillDir)
			license := utils.ParseFrontmatterField(path, "license")

			// Handle root level SKILL.md
			if relPath == "." {
				if includeRoot {
					skills = append(skills, SkillInfo{
						Name:    filepath.Base(repoPath),
						Path:    ".",
						License: license,
					})
				}
			} else {
				skills = append(skills, SkillInfo{
					Name:    filepath.Base(skillDir),
					Path:    strings.ReplaceAll(relPath, "\\", "/"),
					License: license,
				})
			}
		}

		return nil
	})

	// Apply .skillignore filtering
	patterns := readSkillIgnore(repoPath)
	if len(patterns) > 0 {
		filtered := skills[:0]
		for _, s := range skills {
			if !matchSkillIgnore(s.Path, patterns) {
				filtered = append(filtered, s)
			}
		}
		skills = filtered
	}

	return skills
}

// DiscoverFromGitSubdir clones a repo and discovers skills within a subdirectory
// Unlike DiscoverFromGit, this includes root-level SKILL.md of the subdir

func discoverFromGitSubdirWithProgressImpl(source *Source, onProgress ProgressCallback) (*DiscoveryResult, error) {
	if !isGitInstalled() {
		return nil, fmt.Errorf("git is not installed or not in PATH")
	}

	if !source.HasSubdir() {
		return nil, fmt.Errorf("source has no subdirectory specified")
	}

	// Prepare temporary repo directory
	tempDir, err := os.MkdirTemp("", "skillshare-discover-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp directory: %w", err)
	}
	repoPath := filepath.Join(tempDir, "repo")
	var warnings []string
	var commitHash string
	var subdirPath string

	// Fast path 1: GitHub/GHE Contents API
	if isGitHubAPISource(source) {
		owner, repo := source.GitHubOwner(), source.GitHubRepo()
		subdirPath = filepath.Join(repoPath, source.Subdir)
		hash, dlErr := downloadGitHubDir(owner, repo, source.Subdir, subdirPath, source, onProgress)
		if dlErr == nil {
			commitHash = hash
			skills := discoverSkills(subdirPath, true)
			return &DiscoveryResult{
				RepoPath:   tempDir,
				Skills:     skills,
				Source:     source,
				CommitHash: commitHash,
			}, nil
		}
		warnings = append(warnings, fmt.Sprintf("GitHub API discovery fallback: %v", dlErr))
		_ = os.RemoveAll(repoPath)
	}

	// Fast path 2: sparse checkout fallback when API path is unavailable
	// or unsupported (works for GitHub and non-GitHub hosts).
	if gitSupportsSparseCheckout() {
		if err := sparseCloneSubdir(source.CloneURL, source.Subdir, repoPath, authEnv(source.CloneURL), onProgress); err == nil {
			subdirPath = filepath.Join(repoPath, source.Subdir)
			if info, statErr := os.Stat(subdirPath); statErr == nil && info.IsDir() {
				if hash, hashErr := getGitCommit(repoPath); hashErr == nil {
					commitHash = hash
				}
				skills := discoverSkills(subdirPath, true)
				return &DiscoveryResult{
					RepoPath:   tempDir,
					Skills:     skills,
					Source:     source,
					CommitHash: commitHash,
					Warnings:   warnings,
				}, nil
			}
			warnings = append(warnings, "sparse checkout discovery fallback: subdirectory missing after checkout")
			_ = os.RemoveAll(repoPath)
		} else {
			warnings = append(warnings, fmt.Sprintf("sparse checkout discovery fallback: %v", err))
			_ = os.RemoveAll(repoPath)
		}
	}

	// Fallback: full clone + fuzzy subdir resolution
	_ = os.RemoveAll(repoPath)
	if onProgress != nil {
		onProgress("Cloning repository...")
	}
	if err := cloneRepo(source.CloneURL, repoPath, true, onProgress); err != nil {
		os.RemoveAll(tempDir)
		return nil, fmt.Errorf("failed to clone repository: %w", err)
	}

	resolved, err := resolveSubdir(repoPath, source.Subdir)
	if err != nil {
		os.RemoveAll(tempDir)
		return nil, err
	}
	if resolved != source.Subdir {
		source.Subdir = resolved
		source.Name = filepath.Base(resolved)
	}
	subdirPath = filepath.Join(repoPath, resolved)
	if hash, hashErr := getGitCommit(repoPath); hashErr == nil {
		commitHash = hash
	}

	skills := discoverSkills(subdirPath, true)
	return &DiscoveryResult{
		RepoPath:   tempDir,
		Skills:     skills,
		Source:     source,
		CommitHash: commitHash,
		Warnings:   warnings,
	}, nil
}

// discoverFromGitSubdirImpl is the non-progress variant used by the public facade.
func discoverFromGitSubdirImpl(source *Source) (*DiscoveryResult, error) {
	return discoverFromGitSubdirWithProgressImpl(source, nil)
}

// CleanupDiscovery removes the temporary directory from discovery

func cleanupDiscoveryImpl(result *DiscoveryResult) {
	if result != nil && result.RepoPath != "" {
		os.RemoveAll(result.RepoPath)
	}
}

// InstallFromDiscovery installs a skill from a discovered repository
