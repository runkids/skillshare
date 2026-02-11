package install

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"skillshare/internal/audit"
)

// InstallOptions configures the install behavior
type InstallOptions struct {
	Name   string   // Override skill name
	Force  bool     // Overwrite existing
	DryRun bool     // Preview only
	Update bool     // Update existing installation
	Track  bool     // Install as tracked repository (preserves .git)
	Skills []string // Select specific skills from multi-skill repo (comma-separated)
	All    bool     // Install all discovered skills without prompting
	Yes    bool     // Auto-accept all prompts (equivalent to --all for multi-skill repos)
}

// ShouldInstallAll returns true if all discovered skills should be installed without prompting.
func (o InstallOptions) ShouldInstallAll() bool { return o.All || o.Yes }

// HasSkillFilter returns true if specific skills were requested via --skill flag.
func (o InstallOptions) HasSkillFilter() bool { return len(o.Skills) > 0 }

// InstallResult reports the outcome of an installation
type InstallResult struct {
	SkillName string
	SkillPath string
	Source    string
	Action    string // "cloned", "copied", "updated", "skipped"
	Warnings  []string
}

// SkillInfo represents a discovered skill in a repository
type SkillInfo struct {
	Name string // Skill name (directory name)
	Path string // Relative path from repo root
}

// DiscoveryResult contains discovered skills from a repository
type DiscoveryResult struct {
	RepoPath string      // Temp directory where repo was cloned
	Skills   []SkillInfo // Discovered skills
	Source   *Source     // Original source
}

// Install executes the installation from source to destination
func Install(source *Source, destPath string, opts InstallOptions) (*InstallResult, error) {
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
	if err := auditInstalledSkill(destPath, result, opts.Force); err != nil {
		return nil, err
	}

	// Write metadata
	meta := NewMetaFromSource(source)
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
	if err := cloneRepo(source.CloneURL, destPath, true); err != nil {
		return nil, fmt.Errorf("failed to clone repository: %w", err)
	}

	// Write metadata
	meta := NewMetaFromSource(source)
	// Try to get the commit hash
	if hash, err := getGitCommit(destPath); err == nil {
		meta.Version = hash
	}
	if err := WriteMeta(destPath, meta); err != nil {
		result.Warnings = append(result.Warnings, fmt.Sprintf("failed to write metadata: %v", err))
	}

	// Check for SKILL.md
	checkSkillFile(destPath, result)

	result.Action = "cloned"
	return result, nil
}

// DiscoverFromGit clones a repo and discovers available skills
func DiscoverFromGit(source *Source) (*DiscoveryResult, error) {
	if !isGitInstalled() {
		return nil, fmt.Errorf("git is not installed or not in PATH")
	}

	// Clone to temp directory
	tempDir, err := os.MkdirTemp("", "skillshare-discover-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp directory: %w", err)
	}

	repoPath := filepath.Join(tempDir, "repo")
	if err := cloneRepo(source.CloneURL, repoPath, true); err != nil {
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

	return &DiscoveryResult{
		RepoPath: tempDir,
		Skills:   skills,
		Source:   source,
	}, nil
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

			// Handle root level SKILL.md
			if relPath == "." {
				if includeRoot {
					skills = append(skills, SkillInfo{
						Name: filepath.Base(repoPath),
						Path: ".",
					})
				}
			} else {
				skills = append(skills, SkillInfo{
					Name: filepath.Base(skillDir),
					Path: strings.ReplaceAll(relPath, "\\", "/"),
				})
			}
		}

		return nil
	})

	return skills
}

// DiscoverFromGitSubdir clones a repo and discovers skills within a subdirectory
// Unlike DiscoverFromGit, this includes root-level SKILL.md of the subdir
func DiscoverFromGitSubdir(source *Source) (*DiscoveryResult, error) {
	if !isGitInstalled() {
		return nil, fmt.Errorf("git is not installed or not in PATH")
	}

	if !source.HasSubdir() {
		return nil, fmt.Errorf("source has no subdirectory specified")
	}

	// Clone to temp directory
	tempDir, err := os.MkdirTemp("", "skillshare-discover-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp directory: %w", err)
	}

	repoPath := filepath.Join(tempDir, "repo")
	if err := cloneRepo(source.CloneURL, repoPath, true); err != nil {
		os.RemoveAll(tempDir)
		return nil, fmt.Errorf("failed to clone repository: %w", err)
	}

	// Verify subdirectory exists
	subdirPath := filepath.Join(repoPath, source.Subdir)
	info, err := os.Stat(subdirPath)
	if err != nil {
		os.RemoveAll(tempDir)
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("subdirectory '%s' does not exist in repository", source.Subdir)
		}
		return nil, fmt.Errorf("cannot access subdirectory: %w", err)
	}
	if !info.IsDir() {
		os.RemoveAll(tempDir)
		return nil, fmt.Errorf("'%s' is not a directory", source.Subdir)
	}

	// Discover skills within the subdirectory (include root)
	skills := discoverSkills(subdirPath, true)

	return &DiscoveryResult{
		RepoPath: tempDir,
		Skills:   skills,
		Source:   source,
	}, nil
}

// CleanupDiscovery removes the temporary directory from discovery
func CleanupDiscovery(result *DiscoveryResult) {
	if result != nil && result.RepoPath != "" {
		os.RemoveAll(result.RepoPath)
	}
}

// InstallFromDiscovery installs a skill from a discovered repository
func InstallFromDiscovery(discovery *DiscoveryResult, skill SkillInfo, destPath string, opts InstallOptions) (*InstallResult, error) {
	// Build full source path
	// For subdir discovery, skill.Path is relative to the subdir
	// For whole-repo discovery, skill.Path is relative to repo root
	var fullSource string
	var fullSubdir string

	if skill.Path == "." {
		// Root skill of a subdir discovery
		fullSource = discovery.Source.Raw
		fullSubdir = discovery.Source.Subdir
	} else if discovery.Source.HasSubdir() {
		// Nested skill within subdir discovery
		fullSource = discovery.Source.Raw + "/" + skill.Path
		fullSubdir = discovery.Source.Subdir + "/" + skill.Path
	} else {
		// Whole-repo discovery
		fullSource = discovery.Source.Raw + "/" + skill.Path
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
	if err := auditInstalledSkill(destPath, result, opts.Force); err != nil {
		return nil, err
	}

	// Write metadata
	source := &Source{
		Type:     discovery.Source.Type,
		Raw:      fullSource,
		CloneURL: discovery.Source.CloneURL,
		Subdir:   fullSubdir,
		Name:     skill.Name,
	}
	meta := NewMetaFromSource(source)
	if hash, err := getGitCommit(filepath.Join(discovery.RepoPath, "repo")); err == nil {
		meta.Version = hash
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
	if err := cloneRepo(source.CloneURL, tempRepoPath, true); err != nil {
		return nil, fmt.Errorf("failed to clone repository: %w", err)
	}

	// Verify subdirectory exists
	subdirPath := filepath.Join(tempRepoPath, source.Subdir)
	info, err := os.Stat(subdirPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("subdirectory '%s' does not exist in repository", source.Subdir)
		}
		return nil, fmt.Errorf("cannot access subdirectory: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("'%s' is not a directory", source.Subdir)
	}

	// Copy subdirectory to destination
	if err := copyDir(subdirPath, destPath); err != nil {
		return nil, fmt.Errorf("failed to copy skill: %w", err)
	}

	// Security audit
	if err := auditInstalledSkill(destPath, result, opts.Force); err != nil {
		return nil, err
	}

	// Write metadata
	meta := NewMetaFromSource(source)
	// Try to get the commit hash from temp repo
	if hash, err := getGitCommit(tempRepoPath); err == nil {
		meta.Version = hash
	}
	if err := WriteMeta(destPath, meta); err != nil {
		result.Warnings = append(result.Warnings, fmt.Sprintf("failed to write metadata: %v", err))
	}

	// Check for SKILL.md
	checkSkillFile(destPath, result)

	result.Action = "cloned and extracted"
	return result, nil
}

func handleUpdate(source *Source, destPath string, result *InstallResult, opts InstallOptions) (*InstallResult, error) {
	result.SkillPath = destPath

	// For git repos without subdir, try git pull
	if source.IsGit() && !source.HasSubdir() && isGitRepo(destPath) {
		if opts.DryRun {
			result.Action = "would update (git pull)"
			return result, nil
		}

		if err := gitPull(destPath); err != nil {
			return nil, fmt.Errorf("failed to update: %w", err)
		}

		// Update metadata timestamp
		meta, _ := ReadMeta(destPath)
		if meta != nil {
			if hash, err := getGitCommit(destPath); err == nil {
				meta.Version = hash
			}
			WriteMeta(destPath, meta)
		}

		result.Action = "updated"
		return result, nil
	}

	// For other cases (e.g., git with subdir), reinstall automatically
	// --update implies willingness to reinstall when git pull is not possible

	if opts.DryRun {
		result.Action = "would reinstall from source"
		return result, nil
	}

	// Safe update: install to temp first, then swap
	tempDir, err := os.MkdirTemp("", "skillshare-update-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(tempDir)

	tempDest := filepath.Join(tempDir, "skill")

	// Install to temp location first
	_, err = Install(source, tempDest, InstallOptions{
		Name:   opts.Name,
		Force:  true,
		DryRun: false,
		Update: false,
	})
	if err != nil {
		// Installation failed - original skill is preserved
		return nil, err
	}

	// Installation succeeded - now safe to remove original and move new
	if err := os.RemoveAll(destPath); err != nil {
		return nil, fmt.Errorf("failed to remove existing skill: %w", err)
	}

	if err := os.Rename(tempDest, destPath); err != nil {
		// Rename failed (possibly cross-device), try copy instead
		if err := copyDir(tempDest, destPath); err != nil {
			return nil, fmt.Errorf("failed to move updated skill: %w", err)
		}
	}

	result.Action = "reinstalled"
	return result, nil
}

// checkSkillFile adds a warning if SKILL.md is not found
func checkSkillFile(skillPath string, result *InstallResult) {
	skillFile := filepath.Join(skillPath, "SKILL.md")
	if _, err := os.Stat(skillFile); os.IsNotExist(err) {
		result.Warnings = append(result.Warnings, "no SKILL.md found in skill directory")
	}
}

// auditInstalledSkill scans the installed skill for security threats.
// If CRITICAL findings are detected and force is false, it removes the skill
// directory and returns an error. HIGH/MEDIUM findings are appended as warnings.
func auditInstalledSkill(destPath string, result *InstallResult, force bool) error {
	scanResult, err := audit.ScanSkill(destPath)
	if err != nil {
		// Non-fatal: warn but don't block
		result.Warnings = append(result.Warnings, fmt.Sprintf("audit scan error: %v", err))
		return nil
	}

	if len(scanResult.Findings) == 0 {
		return nil
	}

	// Build warning messages for all findings (include snippet for context)
	for _, f := range scanResult.Findings {
		msg := fmt.Sprintf("audit %s: %s (%s:%d)", f.Severity, f.Message, f.File, f.Line)
		if f.Snippet != "" {
			msg += fmt.Sprintf("\n       %q", f.Snippet)
		}
		result.Warnings = append(result.Warnings, msg)
	}

	// CRITICAL findings block installation unless --force
	if scanResult.HasCritical() && !force {
		os.RemoveAll(destPath)
		var details []string
		for _, f := range scanResult.Findings {
			if f.Severity == audit.SeverityCritical {
				detail := fmt.Sprintf("  %s: %s (%s:%d)", f.Severity, f.Message, f.File, f.Line)
				if f.Snippet != "" {
					detail += fmt.Sprintf("\n    %q", f.Snippet)
				}
				details = append(details, detail)
			}
		}
		return fmt.Errorf("security audit failed — critical threats detected:\n%s\n\nUse --force to override", strings.Join(details, "\n"))
	}

	return nil
}

// isGitInstalled checks if git command is available
func isGitInstalled() bool {
	_, err := exec.LookPath("git")
	return err == nil
}

// IsGitRepo checks if path is a git repository
func IsGitRepo(path string) bool {
	gitDir := filepath.Join(path, ".git")
	info, err := os.Stat(gitDir)
	return err == nil && info.IsDir()
}

// isGitRepo is an alias for IsGitRepo (for internal use)
func isGitRepo(path string) bool {
	return IsGitRepo(path)
}

// gitCommandTimeout is the maximum time for a git network operation.
const gitCommandTimeout = 60 * time.Second

// gitCommand creates an exec.Cmd for git with GIT_TERMINAL_PROMPT=0
// to prevent interactive credential prompts that hang CLI spinners and web UI.
func gitCommand(ctx context.Context, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Env = append(os.Environ(),
		"GIT_TERMINAL_PROMPT=0",
		"GIT_ASKPASS=",
		"SSH_ASKPASS=",
	)
	return cmd
}

// runGitCommand runs a git command with timeout, captures stderr for error messages.
func runGitCommand(args []string, dir string) error {
	ctx, cancel := context.WithTimeout(context.Background(), gitCommandTimeout)
	defer cancel()

	cmd := gitCommand(ctx, args...)
	if dir != "" {
		cmd.Dir = dir
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		return wrapGitError(stderr.String(), err)
	}
	return nil
}

// wrapGitError inspects stderr output to produce actionable error messages.
func wrapGitError(stderr string, err error) error {
	s := strings.TrimSpace(stderr)
	if strings.Contains(s, "Authentication failed") ||
		strings.Contains(s, "could not read Username") ||
		strings.Contains(s, "terminal prompts disabled") {
		return fmt.Errorf("authentication required — for private repos use SSH URL:\n       git@<host>:<owner>/<repo>.git\n       %s", s)
	}
	if s != "" {
		return fmt.Errorf("%s", s)
	}
	return err
}

// cloneRepo performs a git clone (quiet mode for cleaner output)
func cloneRepo(url, destPath string, shallow bool) error {
	args := []string{"clone", "--quiet"}
	if shallow {
		args = append(args, "--depth", "1")
	}
	args = append(args, url, destPath)
	return runGitCommand(args, "")
}

// gitPull performs a git pull (quiet mode)
func gitPull(repoPath string) error {
	return runGitCommand([]string{"pull", "--quiet"}, repoPath)
}

// getGitCommit returns the current HEAD commit hash
func getGitCommit(repoPath string) (string, error) {
	cmd := exec.Command("git", "rev-parse", "--short", "HEAD")
	cmd.Dir = repoPath
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return string(output[:len(output)-1]), nil // Remove trailing newline
}

// copyDir copies a directory recursively
func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, _ := filepath.Rel(src, path)
		dstPath := filepath.Join(dst, relPath)

		// Skip .git directory
		if info.IsDir() && info.Name() == ".git" {
			return filepath.SkipDir
		}

		if info.IsDir() {
			return os.MkdirAll(dstPath, info.Mode())
		}

		return copyFile(path, dstPath)
	})
}

// copyFile copies a single file
func copyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	srcInfo, err := srcFile.Stat()
	if err != nil {
		return err
	}

	dstFile, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, srcInfo.Mode())
	if err != nil {
		return err
	}
	defer dstFile.Close()

	_, err = io.Copy(dstFile, srcFile)
	return err
}

// TrackedRepoResult reports the outcome of a tracked repo installation
type TrackedRepoResult struct {
	RepoName   string   // Name of the tracked repo (e.g., "_team-skills")
	RepoPath   string   // Full path to the repo
	SkillCount int      // Number of skills discovered
	Skills     []string // Names of discovered skills
	Action     string   // "cloned", "updated", "skipped"
	Warnings   []string
}

// InstallTrackedRepo clones a git repository as a tracked repo.
// The repo is cloned to @<repo-name>/ and preserves .git for updates.
func InstallTrackedRepo(source *Source, sourceDir string, opts InstallOptions) (*TrackedRepoResult, error) {
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
	destPath := filepath.Join(sourceDir, trackedName)

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

	// Clone the repository (full clone, not shallow, to support updates)
	if err := cloneRepoFull(source.CloneURL, destPath); err != nil {
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

	// Auto-add to .gitignore to prevent committing tracked repo contents
	if err := UpdateGitIgnore(sourceDir, trackedName); err != nil {
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

	if err := gitPull(repoPath); err != nil {
		return nil, fmt.Errorf("failed to update: %w", err)
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
func cloneRepoFull(url, destPath string) error {
	return runGitCommand([]string{"clone", "--quiet", url, destPath}, "")
}

// GetUpdatableSkills returns skill names that have metadata with a remote source.
// It walks subdirectories recursively so nested skills are found.
func GetUpdatableSkills(sourceDir string) ([]string, error) {
	var skills []string

	err := filepath.Walk(sourceDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if path == sourceDir {
			return nil
		}
		// Skip .git directories
		if info.IsDir() && info.Name() == ".git" {
			return filepath.SkipDir
		}
		// Skip tracked repo directories (start with _)
		if info.IsDir() && len(info.Name()) > 0 && info.Name()[0] == '_' {
			return filepath.SkipDir
		}
		// Look for metadata files
		if !info.IsDir() && info.Name() == metaFileName {
			skillDir := filepath.Dir(path)
			relPath, relErr := filepath.Rel(sourceDir, skillDir)
			if relErr != nil || relPath == "." {
				return nil
			}
			meta, metaErr := ReadMeta(skillDir)
			if metaErr != nil || meta == nil || meta.Source == "" {
				return nil
			}
			skills = append(skills, relPath)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return skills, nil
}

// GetTrackedRepos returns a list of tracked repositories in the source directory.
// It walks subdirectories recursively so repos nested in organizational
// directories (e.g. category/_team-repo/) are found.
func GetTrackedRepos(sourceDir string) ([]string, error) {
	var repos []string

	err := filepath.Walk(sourceDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if path == sourceDir {
			return nil
		}
		// Skip .git directories
		if info.IsDir() && info.Name() == ".git" {
			return filepath.SkipDir
		}
		// Look for _-prefixed directories that are git repos
		if info.IsDir() && len(info.Name()) > 0 && info.Name()[0] == '_' {
			gitDir := filepath.Join(path, ".git")
			if _, statErr := os.Stat(gitDir); statErr == nil {
				relPath, relErr := filepath.Rel(sourceDir, path)
				if relErr == nil {
					repos = append(repos, relPath)
				}
				return filepath.SkipDir // Don't recurse into tracked repos
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return repos, nil
}
