package install

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"skillshare/internal/utils"
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

func discoverySourceRoot(discovery *DiscoveryResult) string {
	if discovery.Source != nil && discovery.Source.Type == SourceTypeLocalPath {
		return discovery.RepoPath
	}
	return filepath.Join(discovery.RepoPath, "repo")
}

// descendantSkillPaths returns the set of slash-normalized paths (relative to
// the source root of `skill`) of every other discovered skill that lives
// strictly inside `skill`. The returned map is suitable as the excludes
// argument to copyDirExcluding.
//
// For the root skill (Path="."), every other discovered skill counts as a
// descendant. For a non-root skill, only nested sub-skills count. Returns nil
// when there is nothing to exclude so callers hit the fast path in
// copyDirExcluding.
func descendantSkillPaths(discovery *DiscoveryResult, skill SkillInfo) map[string]bool {
	if discovery == nil || len(discovery.Skills) <= 1 {
		return nil
	}
	current := filepath.ToSlash(skill.Path)
	var prefix string
	if current != "." {
		prefix = current + "/"
	}
	excludes := make(map[string]bool)
	for _, other := range discovery.Skills {
		otherPath := filepath.ToSlash(other.Path)
		if otherPath == current {
			continue
		}
		if current == "." {
			// Root skill: every other skill path is a strict descendant.
			if otherPath != "." {
				excludes[otherPath] = true
			}
			continue
		}
		// Non-root: only paths strictly under current count.
		if rel, ok := strings.CutPrefix(otherPath, prefix); ok {
			// rel is the path relative to `skill` (which is how
			// copyDirExcluding walks `srcPath`).
			excludes[rel] = true
		}
	}
	if len(excludes) == 0 {
		return nil
	}
	return excludes
}

func installAgentRelativePath(agent AgentInfo) string {
	return strings.TrimPrefix(filepath.ToSlash(agent.Path), "agents/")
}

func resolveDiscoveredAgentSourcePath(discovery *DiscoveryResult, agent AgentInfo) string {
	sourceRoot := discoverySourceRoot(discovery)
	if discovery.Source.HasSubdir() {
		return filepath.Join(sourceRoot, discovery.Source.Subdir, agent.Path)
	}
	return filepath.Join(sourceRoot, agent.Path)
}

func buildDiscoveredAgentSource(discovery *DiscoveryResult, agent AgentInfo) *Source {
	return &Source{
		Type:     discovery.Source.Type,
		Raw:      buildDiscoverySkillSource(discovery.Source, agent.Path),
		CloneURL: discovery.Source.CloneURL,
		Subdir:   agent.Path,
		Name:     agent.Name,
		Branch:   discovery.Source.Branch,
	}
}

func writeDiscoveredAgentMetadata(discovery *DiscoveryResult, agent AgentInfo, destFile, sourceDir string) error {
	source := buildDiscoveredAgentSource(discovery, agent)
	meta := NewMetaFromSource(source)
	meta.Kind = "agent"
	if discovery.CommitHash != "" {
		meta.Version = discovery.CommitHash
	}
	if hash, hashErr := computeSingleFileHash(destFile); hashErr == nil {
		meta.FileHashes = map[string]string{filepath.Base(destFile): hash}
	}
	return WriteMetaToStore(sourceDir, destFile, meta)
}

func installAgentFromDiscoveryInternal(discovery *DiscoveryResult, agent AgentInfo, destFile string, opts InstallOptions, writeMeta bool) (*InstallResult, error) {
	result := &InstallResult{
		SkillName: agent.Name,
		Source:    buildDiscoverySkillSource(discovery.Source, agent.Path),
		SkillPath: destFile,
	}

	if opts.DryRun {
		result.Action = "would install"
		return result, nil
	}

	if err := os.MkdirAll(filepath.Dir(destFile), 0755); err != nil {
		return nil, fmt.Errorf("failed to create agents directory: %w", err)
	}

	if _, err := os.Stat(destFile); err == nil && !opts.Force {
		result.Action = "skipped"
		result.Warnings = append(result.Warnings, "agent already exists (use --force to overwrite)")
		return result, nil
	}

	data, err := os.ReadFile(resolveDiscoveredAgentSourcePath(discovery, agent))
	if err != nil {
		return nil, fmt.Errorf("failed to read agent %s: %w", agent.FileName, err)
	}

	if err := os.WriteFile(destFile, data, 0644); err != nil {
		return nil, fmt.Errorf("failed to write agent %s: %w", agent.FileName, err)
	}

	if err := auditInstalledAgent(destFile, result, opts); err != nil {
		return nil, err
	}

	if writeMeta {
		if err := writeDiscoveredAgentMetadata(discovery, agent, destFile, opts.SourceDir); err != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("failed to write metadata: %v", err))
		}
	}

	result.Action = "installed"
	return result, nil
}

func installImpl(source *Source, destPath string, opts InstallOptions) (*InstallResult, error) {
	// Derive SourceDir from destPath if not set by caller.
	// destPath = sourceDir[/into]/skillName, so strip Into + skillName.
	if opts.SourceDir == "" {
		dir := filepath.Dir(destPath)
		if opts.Into != "" {
			// Strip the --into prefix from the parent
			dir = filepath.Dir(dir)
			for i := strings.Count(opts.Into, "/"); i > 0; i-- {
				dir = filepath.Dir(dir)
			}
		}
		opts.SourceDir = dir
	}

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
			hint := buildForceHint(source.Raw, opts.Into)
			if err := checkExistingConflict(destPath, source.CloneURL, hint); err != nil {
				return nil, err
			}
			// nil means empty/invalid dir — safe to overwrite, fall through.
		}
		// Force mode (or empty dir): remove existing
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
	if err := WriteMetaToStore(opts.SourceDir, destPath, meta); err != nil {
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
	if err := cloneRepo(source.CloneURL, destPath, source.Branch, true, opts.OnProgress); err != nil {
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
	if err := WriteMetaToStore(opts.SourceDir, destPath, meta); err != nil {
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
	return installFromDiscoveryInternal(discovery, skill, destPath, opts, true)
}

func discoveredSkillSourceParts(discovery *DiscoveryResult, skill SkillInfo) (string, string) {
	// For subdir discovery, skill.Path is relative to the subdir.
	// For whole-repo discovery, skill.Path is relative to repo root.
	if skill.Path == "." {
		// Root skill of a subdir discovery.
		return buildDiscoverySkillSource(discovery.Source, skill.Path), discovery.Source.Subdir
	}
	if discovery.Source.HasSubdir() {
		// Nested skill within subdir discovery.
		return buildDiscoverySkillSource(discovery.Source, skill.Path), discovery.Source.Subdir + "/" + skill.Path
	}
	// Whole-repo discovery.
	return buildDiscoverySkillSource(discovery.Source, skill.Path), skill.Path
}

func installFromDiscoveryInternal(discovery *DiscoveryResult, skill SkillInfo, destPath string, opts InstallOptions, writeMeta bool) (*InstallResult, error) {
	fullSource, fullSubdir := discoveredSkillSourceParts(discovery, skill)

	result := &InstallResult{
		SkillName: skill.Name,
		Source:    fullSource,
	}

	// Check if destination exists
	if _, err := os.Stat(destPath); err == nil {
		if !opts.Force {
			// Use the original repo URL for force hints, not the per-skill
			// fullSource URL (which isn't a valid install target).
			hint := buildForceHint(discovery.Source.Raw, opts.Into)
			if err := checkExistingConflict(destPath, discovery.Source.CloneURL, hint); err != nil {
				return nil, err
			}
			// nil means empty/invalid dir — safe to overwrite, fall through.
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
	sourceRoot := discoverySourceRoot(discovery)
	var srcPath string
	if discovery.Source.HasSubdir() {
		// Subdir discovery: paths are relative to the subdir
		if skill.Path == "." {
			srcPath = filepath.Join(sourceRoot, discovery.Source.Subdir)
		} else {
			srcPath = filepath.Join(sourceRoot, discovery.Source.Subdir, skill.Path)
		}
	} else {
		// Whole-repo discovery: paths are relative to repo root
		srcPath = filepath.Join(sourceRoot, skill.Path)
	}

	// In a repo-root orchestrator (root SKILL.md + children), the root skill
	// has no directory boundary — the entire repo root is its "directory",
	// which includes project infrastructure (src/, assets/, CI, etc.) that
	// is not skill content. Copy only the SKILL.md file itself.
	//
	// Do not apply this to scoped subdir discovery: there the selected subdir
	// is the boundary, so sibling files in that subdir are legitimate skill
	// content and must still be copied.
	excludes := descendantSkillPaths(discovery, skill)
	rootIsRepoRoot := skill.Path == "." && discovery.Source != nil && !discovery.Source.HasSubdir()

	if rootIsRepoRoot && excludes != nil {
		// Repo-root orchestrator: copy only SKILL.md (no directory boundary).
		if err := os.MkdirAll(destPath, 0755); err != nil {
			return nil, fmt.Errorf("failed to create destination: %w", err)
		}
		if err := copyFile(filepath.Join(srcPath, "SKILL.md"), filepath.Join(destPath, "SKILL.md")); err != nil {
			return nil, fmt.Errorf("failed to copy SKILL.md: %w", err)
		}
	} else if err := copyDirExcluding(srcPath, destPath, excludes); err != nil {
		return nil, fmt.Errorf("failed to copy skill: %w", err)
	}

	// Security audit
	if err := auditInstalledSkill(destPath, result, opts); err != nil {
		return nil, err
	}

	if writeMeta {
		if err := writeDiscoveredSkillMetadata(discovery, skill, destPath, opts.SourceDir, fullSource, fullSubdir); err != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("failed to write metadata: %v", err))
		}
	}

	result.Action = "installed"
	return result, nil
}

func writeDiscoveredSkillMetadata(discovery *DiscoveryResult, skill SkillInfo, destPath, sourceDir, fullSource, fullSubdir string) error {
	source := &Source{
		Type:     discovery.Source.Type,
		Raw:      fullSource,
		CloneURL: discovery.Source.CloneURL,
		Subdir:   fullSubdir,
		Name:     skill.Name,
		Branch:   discovery.Source.Branch,
	}
	meta := NewMetaFromSource(source)
	sourceRoot := discoverySourceRoot(discovery)
	if discovery.CommitHash != "" {
		meta.Version = discovery.CommitHash
	} else if hash, err := getGitCommit(sourceRoot); err == nil {
		meta.Version = hash
	}
	if fullSubdir != "" {
		meta.TreeHash = getSubdirTreeHash(sourceRoot, fullSubdir)
	}
	if hashes, hashErr := ComputeFileHashes(destPath); hashErr == nil {
		meta.FileHashes = hashes
	}
	return WriteMetaToStore(sourceDir, destPath, meta)
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
		if err := sparseCloneSubdir(source.CloneURL, resolved, tempRepoPath, source.Branch, authEnv(source.CloneURL), opts.OnProgress); err == nil {
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
		if err := cloneRepo(source.CloneURL, tempRepoPath, source.Branch, true, opts.OnProgress); err != nil {
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
	if err := WriteMetaToStore(opts.SourceDir, destPath, meta); err != nil {
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

// InstallAgentFromDiscovery installs a single agent (.md file) from a discovery result.
// Unlike skill install (directory copy), agent install copies a single file.
func InstallAgentFromDiscovery(discovery *DiscoveryResult, agent AgentInfo, destDir string, opts InstallOptions) (*InstallResult, error) {
	destFile := filepath.Join(destDir, filepath.FromSlash(installAgentRelativePath(agent)))
	return installAgentFromDiscoveryInternal(discovery, agent, destFile, opts, true)
}

func UpdateAgentFromDiscovery(discovery *DiscoveryResult, agent AgentInfo, destDir string, opts InstallOptions) (*InstallResult, error) {
	relPath := installAgentRelativePath(agent)
	destFile := filepath.Join(destDir, filepath.FromSlash(relPath))
	result := &InstallResult{
		SkillName: agent.Name,
		Source:    buildDiscoverySkillSource(discovery.Source, agent.Path),
		SkillPath: destFile,
	}

	if opts.DryRun {
		result.Action = "would reinstall from source"
		return result, nil
	}

	tempDir, err := os.MkdirTemp("", "skillshare-agent-update-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(tempDir)

	tempFile := filepath.Join(tempDir, filepath.FromSlash(relPath))
	innerOpts := opts
	innerOpts.DryRun = false
	innerOpts.Update = false
	innerOpts.SourceDir = tempDir

	innerResult, err := installAgentFromDiscoveryInternal(discovery, agent, tempFile, innerOpts, false)
	if err != nil {
		return nil, err
	}

	if err := os.MkdirAll(filepath.Dir(destFile), 0755); err != nil {
		return nil, fmt.Errorf("failed to create agents directory: %w", err)
	}
	// os.Rename is atomic on POSIX and overwrites the destination. If it
	// fails (e.g. cross-device EXDEV on exotic setups), fall back to copy.
	if err := os.Rename(tempFile, destFile); err != nil {
		data, readErr := os.ReadFile(tempFile)
		if readErr != nil {
			return nil, fmt.Errorf("failed to read staged agent: %w", readErr)
		}
		if writeErr := os.WriteFile(destFile, data, 0644); writeErr != nil {
			return nil, fmt.Errorf("failed to move updated agent: %w", writeErr)
		}
	}

	if err := writeDiscoveredAgentMetadata(discovery, agent, destFile, opts.SourceDir); err != nil {
		innerResult.Warnings = append(innerResult.Warnings, fmt.Sprintf("failed to write metadata: %v", err))
	}

	innerResult.SkillPath = destFile
	innerResult.Action = "updated"
	return innerResult, nil
}

// computeSingleFileHash computes the sha256 hash for a single file.
func computeSingleFileHash(filePath string) (string, error) {
	return utils.FileHashFormatted(filePath)
}

// auditInstalledSkill scans the installed skill for security threats.
// It blocks installation when findings are at or above configured threshold
// unless force is enabled.
