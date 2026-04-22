package install

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"skillshare/internal/skillignore"
	"skillshare/internal/utils"
)

// TargetDotDirs is the set of hidden directory names (e.g. ".claude", ".cursor")
// to skip during skill discovery. Set by the CLI entrypoint from config data
// to avoid a circular import between install and config packages.
var TargetDotDirs map[string]bool

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
	if err := cloneRepo(source.CloneURL, repoPath, source.Branch, true, onProgress); err != nil {
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

	// Discover agents (agents/ dir or pure agent repo fallback)
	agents := discoverAgents(repoPath, len(skills) > 0)
	skills, agents, err = constrainDiscoveryToExplicitSkill(source, skills, agents)
	if err != nil {
		_ = os.RemoveAll(tempDir)
		return nil, err
	}

	commitHash, _ := getGitCommit(repoPath)

	return &DiscoveryResult{
		RepoPath:   tempDir,
		Skills:     skills,
		Agents:     agents,
		Source:     source,
		CommitHash: commitHash,
	}, nil
}

// discoverFromGitImpl is the non-progress variant used by the public facade.
func discoverFromGitImpl(source *Source) (*DiscoveryResult, error) {
	return discoverFromGitWithProgressImpl(source, nil)
}

func discoverLocalImpl(source *Source) (*DiscoveryResult, error) {
	if source == nil {
		return nil, fmt.Errorf("source is required")
	}
	if source.Type != SourceTypeLocalPath {
		return nil, fmt.Errorf("source is not a local path")
	}

	info, err := os.Stat(source.Path)
	if err != nil {
		return nil, fmt.Errorf("cannot access source path: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("source path is not a directory: %s", source.Path)
	}

	skills := discoverSkills(source.Path, true)
	for i := range skills {
		if skills[i].Path == "." {
			skills[i].Name = source.Name
			break
		}
	}

	agents := discoverAgents(source.Path, len(skills) > 0)
	skills, agents, err = constrainDiscoveryToExplicitSkill(source, skills, agents)
	if err != nil {
		return nil, err
	}

	return &DiscoveryResult{
		RepoPath: source.Path,
		Skills:   skills,
		Agents:   agents,
		Source:   source,
	}, nil
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

func constrainDiscoveryToExplicitSkill(source *Source, skills []SkillInfo, agents []AgentInfo) ([]SkillInfo, []AgentInfo, error) {
	if source == nil || !source.TargetsExplicitSkill() {
		return skills, agents, nil
	}

	for _, skill := range skills {
		if skill.Path == "." {
			return []SkillInfo{skill}, nil, nil
		}
	}

	return nil, nil, fmt.Errorf("explicit skill target does not resolve to a root SKILL.md: %s", source.Raw)
}

// discoverSkills finds directories containing SKILL.md
// If includeRoot is true, root-level SKILL.md is also included (with Path=".")
func discoverSkills(repoPath string, includeRoot bool) []SkillInfo {
	var skills []SkillInfo

	filepath.Walk(repoPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}

		// Skip .git and known target dotdirs (.claude, .cursor, .skillshare, etc.)
		// to avoid counting target-synced copies as source skills.
		if info.IsDir() {
			name := info.Name()
			if name == ".git" || TargetDotDirs[name] {
				return filepath.SkipDir
			}
		}

		// Check if this is a SKILL.md file
		if !info.IsDir() && info.Name() == "SKILL.md" {
			skillDir := filepath.Dir(path)
			relPath, _ := filepath.Rel(repoPath, skillDir)
			fm := utils.ParseFrontmatterFields(path, []string{"description", "license"})

			// Handle root level SKILL.md
			if relPath == "." {
				if includeRoot {
					skills = append(skills, SkillInfo{
						Name:        filepath.Base(repoPath),
						Path:        ".",
						License:     fm["license"],
						Description: fm["description"],
					})
				}
			} else {
				skills = append(skills, SkillInfo{
					Name:        filepath.Base(skillDir),
					Path:        strings.ReplaceAll(relPath, "\\", "/"),
					License:     fm["license"],
					Description: fm["description"],
				})
			}
		}

		return nil
	})

	// Apply .skillignore filtering.
	// Skill paths are directories (they contain SKILL.md), so pass isDir=true.
	matcher := skillignore.ReadMatcher(repoPath)
	if matcher.HasRules() {
		filtered := skills[:0]
		for _, s := range skills {
			if !matcher.Match(s.Path, true) {
				filtered = append(filtered, s)
			}
		}
		skills = filtered
	}

	return skills
}

// discoverAgents finds .md files in an agents/ convention directory.
// Also detects "pure agent repos" — repos with no SKILL.md and no agents/ dir
// but with .md files at root (per D5 rule 4).
func discoverAgents(repoPath string, hasSkills bool) []AgentInfo {
	var agents []AgentInfo

	// Rule 2: Check agents/ convention directory
	agentsDir := filepath.Join(repoPath, "agents")
	if info, err := os.Stat(agentsDir); err == nil && info.IsDir() {
		agents = append(agents, scanAgentDir(repoPath, agentsDir)...)
		return agents
	}

	// Rule 4: Pure agent repo fallback — no skills, no agents/ dir, root has .md files
	if !hasSkills {
		agents = append(agents, scanAgentDir(repoPath, repoPath)...)
	}

	return agents
}

// scanAgentDir scans a directory for .md agent files, excluding conventional files.
func scanAgentDir(repoRoot, dir string) []AgentInfo {
	var agents []AgentInfo

	filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}

		if info.IsDir() {
			name := info.Name()
			if name == ".git" || TargetDotDirs[name] {
				return filepath.SkipDir
			}
			return nil
		}

		if !strings.HasSuffix(strings.ToLower(info.Name()), ".md") {
			return nil
		}

		// Skip conventional excludes
		if conventionalAgentExcludes[info.Name()] {
			return nil
		}

		// Skip hidden files
		if strings.HasPrefix(info.Name(), ".") {
			return nil
		}

		relPath, relErr := filepath.Rel(repoRoot, path)
		if relErr != nil {
			return nil
		}
		relPath = strings.ReplaceAll(relPath, "\\", "/")

		name := strings.TrimSuffix(info.Name(), ".md")

		agents = append(agents, AgentInfo{
			Name:     name,
			Path:     relPath,
			FileName: info.Name(),
		})

		return nil
	})

	return agents
}

var conventionalAgentExcludes = map[string]bool{
	"README.md":    true,
	"CHANGELOG.md": true,
	"LICENSE.md":   true,
	"HISTORY.md":   true,
	"SECURITY.md":  true,
	"SKILL.md":     true,
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

	// Fast path 1: sparse checkout (preferred for speed if git is modern)
	// Works for GitHub and non-GitHub hosts.
	if gitSupportsSparseCheckout() {
		if err := sparseCloneSubdir(source.CloneURL, source.Subdir, repoPath, source.Branch, authEnv(source.CloneURL), onProgress); err == nil {
			subdirPath = filepath.Join(repoPath, source.Subdir)
			if info, statErr := os.Stat(subdirPath); statErr == nil && info.IsDir() {
				if hash, hashErr := getGitCommit(repoPath); hashErr == nil {
					commitHash = hash
				}
				skills := discoverSkills(subdirPath, true)
				agents := discoverAgents(subdirPath, len(skills) > 0)
				skills, agents, err = constrainDiscoveryToExplicitSkill(source, skills, agents)
				if err != nil {
					_ = os.RemoveAll(tempDir)
					return nil, err
				}
				return &DiscoveryResult{
					RepoPath:   tempDir,
					Skills:     skills,
					Agents:     agents,
					Source:     source,
					CommitHash: commitHash,
					Warnings:   warnings,
				}, nil
			}
			warnings = append(warnings, "sparse checkout discovery fallback: subdirectory missing after checkout")
			_ = os.RemoveAll(repoPath)
			subdirPath = ""
		} else {
			warnings = append(warnings, fmt.Sprintf("sparse checkout discovery fallback: %v", err))
			_ = os.RemoveAll(repoPath)
			subdirPath = ""
		}
	}

	// Fast path 2: GitHub/GHE Contents API
	// Fallback for when sparse checkout is unavailable or fails.
	if subdirPath == "" && isGitHubAPISource(source) {
		owner, repo := source.GitHubOwner(), source.GitHubRepo()
		subdirPath = filepath.Join(repoPath, source.Subdir)
		hash, dlErr := downloadGitHubDir(owner, repo, source.Subdir, subdirPath, source, onProgress)
		if dlErr == nil {
			commitHash = hash
			skills := discoverSkills(subdirPath, true)
			agents := discoverAgents(subdirPath, len(skills) > 0)
			skills, agents, err = constrainDiscoveryToExplicitSkill(source, skills, agents)
			if err != nil {
				_ = os.RemoveAll(tempDir)
				return nil, err
			}
			return &DiscoveryResult{
				RepoPath:   tempDir,
				Skills:     skills,
				Agents:     agents,
				Source:     source,
				CommitHash: commitHash,
			}, nil
		}
		warnings = append(warnings, fmt.Sprintf("GitHub API discovery fallback: %v", dlErr))
		_ = os.RemoveAll(repoPath)
		subdirPath = ""
	}

	// Fallback: full clone + fuzzy subdir resolution
	_ = os.RemoveAll(repoPath)
	if onProgress != nil {
		onProgress("Cloning repository...")
	}
	if err := cloneRepo(source.CloneURL, repoPath, source.Branch, true, onProgress); err != nil {
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
	agents := discoverAgents(subdirPath, len(skills) > 0)
	skills, agents, err = constrainDiscoveryToExplicitSkill(source, skills, agents)
	if err != nil {
		_ = os.RemoveAll(tempDir)
		return nil, err
	}
	return &DiscoveryResult{
		RepoPath:   tempDir,
		Skills:     skills,
		Agents:     agents,
		Source:     source,
		CommitHash: commitHash,
		Warnings:   warnings,
	}, nil
}

// discoverFromGitSubdirImpl is the non-progress variant used by the public facade.
func discoverFromGitSubdirImpl(source *Source) (*DiscoveryResult, error) {
	return discoverFromGitSubdirWithProgressImpl(source, nil)
}

// CleanupDiscovery removes the temporary directory created for git-based
// discovery (clone under RepoPath). It only runs when Source is known to be
// a git remote; local paths, missing source metadata, and other types are
// skipped so RepoPath is never treated as disposable unless it came from a
// git clone.

func cleanupDiscoveryImpl(result *DiscoveryResult) {
	if result == nil || result.RepoPath == "" || result.Source == nil {
		return
	}
	if !result.Source.IsGit() {
		return
	}
	os.RemoveAll(result.RepoPath)
}

// InstallFromDiscovery installs a skill from a discovered repository
