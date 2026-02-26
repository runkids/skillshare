package install

import (
	"fmt"
	"os"
	"path/filepath"
)

// buildDiscoverySkillSource constructs metadata Source string for a skill
// selected from a discovery result.
func buildDiscoverySkillSource(source *Source, skillPath string) string {
	if skillPath == "." {
		return source.Raw
	}
	if source.HasSubdir() {
		return source.Raw + "/" + skillPath
	}
	// Whole-repo SSH sources encode subdir using //path.
	if source.Type == SourceTypeGitSSH {
		return source.Raw + "//" + skillPath
	}
	return source.Raw + "/" + skillPath
}

func installImpl(source *Source, destPath string, opts InstallOptions) (*InstallResult, error) {
	result := &InstallResult{
		SkillName: source.Name,
		Source:    source.Raw,
	}

	// Check if destination exists
	destInfo, destErr := os.Stat(destPath)
	destExists := destErr == nil

	if destExists {
		if opts.Update {
			return handleUpdate(source, destPath, result, opts)
		}
		if !opts.Force {
			return nil, fmt.Errorf("skill '%s' already exists. To overwrite:\n       skillshare install %s --force", source.Name, source.Raw)
		}
		// Force mode: remove existing
		if !opts.DryRun {
			if err := os.RemoveAll(destPath); err != nil {
				return nil, fmt.Errorf("failed to remove existing skill: %w", err)
			}
		}
	} else if destInfo != nil && !destInfo.IsDir() {
		return nil, fmt.Errorf("destination exists but is not a directory")
	}

	result.SkillPath = destPath

	// Execute installation based on source type
	switch source.Type {
	case SourceTypeLocalPath:
		return installFromLocal(source, destPath, result, opts)
	case SourceTypeGitHub, SourceTypeGitHTTPS, SourceTypeGitSSH:
		return installFromGit(source, destPath, result, opts)
	default:
		return nil, fmt.Errorf("unsupported source type: %s", source.Type)
	}
}

func installFromLocal(source *Source, destPath string, result *InstallResult, opts InstallOptions) (*InstallResult, error) {
	// Verify source exists
	srcInfo, err := os.Stat(source.Path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("source path does not exist: %s", source.Path)
		}
		return nil, fmt.Errorf("cannot access source path: %w", err)
	}
	if !srcInfo.IsDir() {
		return nil, fmt.Errorf("source path is not a directory: %s", source.Path)
	}

	if opts.DryRun {
		result.Action = "would copy"
		return result, nil
	}

	// Copy directory
	if err := copyDir(source.Path, destPath); err != nil {
		return nil, fmt.Errorf("failed to copy skill: %w", err)
	}

	// Security audit
	if err := auditInstalledSkill(destPath, result, opts); err != nil {
		return nil, err
	}

	// Write metadata with file hashes
	meta := NewMetaFromSource(source)
	if hashes, hashErr := ComputeFileHashes(destPath); hashErr == nil {
		meta.FileHashes = hashes
	}
	if err := WriteMeta(destPath, meta); err != nil {
		result.Warnings = append(result.Warnings, fmt.Sprintf("failed to write metadata: %v", err))
	}

	// Check for SKILL.md
	checkSkillFile(destPath, result)

	result.Action = "copied"
	return result, nil
}

func installFromGit(source *Source, destPath string, result *InstallResult, opts InstallOptions) (*InstallResult, error) {
	// Check if git is available
	if !isGitInstalled() {
		return nil, fmt.Errorf("git is not installed or not in PATH")
	}

	// If subdir is specified, install directly
	if source.HasSubdir() {
		return installFromGitSubdir(source, destPath, result, opts)
	}

	// No subdir specified - this should be handled by DiscoverFromGit first
	// If we get here, treat it as "install entire repo as one skill"
	if opts.DryRun {
		result.Action = "would clone"
		return result, nil
	}

	// Clone the repository
	if err := cloneRepo(source.CloneURL, destPath, true, opts.OnProgress); err != nil {
		return nil, fmt.Errorf("failed to clone repository: %w", err)
	}

	// Write metadata with file hashes
	meta := NewMetaFromSource(source)
	if hash, err := getGitCommit(destPath); err == nil {
		meta.Version = hash
	}
	if hashes, hashErr := ComputeFileHashes(destPath); hashErr == nil {
		meta.FileHashes = hashes
	}
	if err := WriteMeta(destPath, meta); err != nil {
		result.Warnings = append(result.Warnings, fmt.Sprintf("failed to write metadata: %v", err))
	}

	// Check for SKILL.md
	checkSkillFile(destPath, result)

	// Security audit
	if err := auditInstalledSkill(destPath, result, opts); err != nil {
		return nil, err
	}

	result.Action = "cloned"
	return result, nil
}

// DiscoverFromGit clones a repo and discovers available skills

func installFromDiscoveryImpl(discovery *DiscoveryResult, skill SkillInfo, destPath string, opts InstallOptions) (*InstallResult, error) {
	// Build full source path
	// For subdir discovery, skill.Path is relative to the subdir
	// For whole-repo discovery, skill.Path is relative to repo root
	var fullSource string
	var fullSubdir string

	if skill.Path == "." {
		// Root skill of a subdir discovery
		fullSource = buildDiscoverySkillSource(discovery.Source, skill.Path)
		fullSubdir = discovery.Source.Subdir
	} else if discovery.Source.HasSubdir() {
		// Nested skill within subdir discovery
		fullSource = buildDiscoverySkillSource(discovery.Source, skill.Path)
		fullSubdir = discovery.Source.Subdir + "/" + skill.Path
	} else {
		// Whole-repo discovery
		fullSource = buildDiscoverySkillSource(discovery.Source, skill.Path)
		fullSubdir = skill.Path
	}

	result := &InstallResult{
		SkillName: skill.Name,
		Source:    fullSource,
	}

	// Check if destination exists
	if _, err := os.Stat(destPath); err == nil {
		if !opts.Force {
			return nil, fmt.Errorf("already exists. To overwrite:\n       skillshare install %s --force", fullSource)
		}
		if !opts.DryRun {
			if err := os.RemoveAll(destPath); err != nil {
				return nil, fmt.Errorf("failed to remove existing skill: %w", err)
			}
		}
	}

	result.SkillPath = destPath

	if opts.DryRun {
		result.Action = "would install"
		return result, nil
	}

	// Determine source path in temp repo
	var srcPath string
	if discovery.Source.HasSubdir() {
		// Subdir discovery: paths are relative to the subdir
		if skill.Path == "." {
			srcPath = filepath.Join(discovery.RepoPath, "repo", discovery.Source.Subdir)
		} else {
			srcPath = filepath.Join(discovery.RepoPath, "repo", discovery.Source.Subdir, skill.Path)
		}
	} else {
		// Whole-repo discovery: paths are relative to repo root
		srcPath = filepath.Join(discovery.RepoPath, "repo", skill.Path)
	}

	if err := copyDir(srcPath, destPath); err != nil {
		return nil, fmt.Errorf("failed to copy skill: %w", err)
	}

	// Security audit
	if err := auditInstalledSkill(destPath, result, opts); err != nil {
		return nil, err
	}

	// Write metadata with file hashes
	source := &Source{
		Type:     discovery.Source.Type,
		Raw:      fullSource,
		CloneURL: discovery.Source.CloneURL,
		Subdir:   fullSubdir,
		Name:     skill.Name,
	}
	meta := NewMetaFromSource(source)
	if discovery.CommitHash != "" {
		meta.Version = discovery.CommitHash
	} else if hash, err := getGitCommit(filepath.Join(discovery.RepoPath, "repo")); err == nil {
		meta.Version = hash
	}
	if fullSubdir != "" {
		meta.TreeHash = getSubdirTreeHash(filepath.Join(discovery.RepoPath, "repo"), fullSubdir)
	}
	if hashes, hashErr := ComputeFileHashes(destPath); hashErr == nil {
		meta.FileHashes = hashes
	}
	if err := WriteMeta(destPath, meta); err != nil {
		result.Warnings = append(result.Warnings, fmt.Sprintf("failed to write metadata: %v", err))
	}

	result.Action = "installed"
	return result, nil
}

func installFromGitSubdir(source *Source, destPath string, result *InstallResult, opts InstallOptions) (*InstallResult, error) {
	if opts.DryRun {
		result.Action = "would clone and extract"
		return result, nil
	}

	// Clone to temp directory
	tempDir, err := os.MkdirTemp("", "skillshare-install-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(tempDir)

	tempRepoPath := filepath.Join(tempDir, "repo")
	var subdirPath string
	var resolved string
	var commitHash string

	// Fast path 1: sparse checkout (preferred for speed if git is modern)
	// Works for GitHub and non-GitHub hosts.
	if gitSupportsSparseCheckout() {
		resolved = source.Subdir
		if err := sparseCloneSubdir(source.CloneURL, resolved, tempRepoPath, authEnv(source.CloneURL), opts.OnProgress); err == nil {
			subdirPath = filepath.Join(tempRepoPath, resolved)
			if info, statErr := os.Stat(subdirPath); statErr != nil || !info.IsDir() {
				subdirPath = ""
				result.Warnings = append(result.Warnings, "sparse checkout install fallback: subdirectory missing after checkout")
				_ = os.RemoveAll(tempRepoPath)
			} else if hash, hashErr := getGitCommit(tempRepoPath); hashErr == nil {
				commitHash = hash
			}
		} else {
			result.Warnings = append(result.Warnings, fmt.Sprintf("sparse checkout install fallback: %v", err))
			_ = os.RemoveAll(tempRepoPath)
			subdirPath = ""
		}
	}

	// Fast path 2: GitHub/GHE Contents API
	// Fallback for when sparse checkout is unavailable or fails.
	if subdirPath == "" && isGitHubAPISource(source) {
		owner, repo := source.GitHubOwner(), source.GitHubRepo()
		resolved = source.Subdir
		subdirPath = filepath.Join(tempRepoPath, resolved)
		hash, dlErr := downloadGitHubDir(owner, repo, source.Subdir, subdirPath, source, opts.OnProgress)
		if dlErr == nil {
			commitHash = hash
		} else {
			result.Warnings = append(result.Warnings, fmt.Sprintf("GitHub API install fallback: %v", dlErr))
			subdirPath = ""
			_ = os.RemoveAll(tempRepoPath)
		}
	}

	// Fallback: full clone + fuzzy subdir resolution
	if subdirPath == "" {
		_ = os.RemoveAll(tempRepoPath)
		if opts.OnProgress != nil {
			opts.OnProgress("Cloning repository...")
		}
		if err := cloneRepo(source.CloneURL, tempRepoPath, true, opts.OnProgress); err != nil {
			return nil, fmt.Errorf("failed to clone repository: %w", err)
		}

		var err error
		resolved, err = resolveSubdir(tempRepoPath, source.Subdir)
		if err != nil {
			return nil, err
		}
		if resolved != source.Subdir {
			source.Subdir = resolved
			source.Name = filepath.Base(resolved)
			result.SkillName = source.Name
		}
		subdirPath = filepath.Join(tempRepoPath, resolved)
		if hash, hashErr := getGitCommit(tempRepoPath); hashErr == nil {
			commitHash = hash
		}
	}

	// Copy subdirectory to destination
	if err := copyDir(subdirPath, destPath); err != nil {
		return nil, fmt.Errorf("failed to copy skill: %w", err)
	}

	// Security audit
	if err := auditInstalledSkill(destPath, result, opts); err != nil {
		return nil, err
	}

	// Write metadata with file hashes
	meta := NewMetaFromSource(source)
	if commitHash != "" {
		meta.Version = commitHash
	}
	if resolved != "" {
		meta.TreeHash = getSubdirTreeHash(tempRepoPath, resolved)
	}
	if hashes, hashErr := ComputeFileHashes(destPath); hashErr == nil {
		meta.FileHashes = hashes
	}
	if err := WriteMeta(destPath, meta); err != nil {
		result.Warnings = append(result.Warnings, fmt.Sprintf("failed to write metadata: %v", err))
	}

	// Check for SKILL.md
	checkSkillFile(destPath, result)

	result.Action = "cloned and extracted"
	return result, nil
}

func checkSkillFile(skillPath string, result *InstallResult) {
	skillFile := filepath.Join(skillPath, "SKILL.md")
	if _, err := os.Stat(skillFile); os.IsNotExist(err) {
		result.Warnings = append(result.Warnings, "no SKILL.md found in skill directory")
	}
}

// auditInstalledSkill scans the installed skill for security threats.
// It blocks installation when findings are at or above configured threshold
// unless force is enabled.
