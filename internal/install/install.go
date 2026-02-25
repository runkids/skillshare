package install

import "os"

// InstallOptions configures the install behavior
type InstallOptions struct {
	Name             string // Override skill name
	Force            bool   // Overwrite existing
	DryRun           bool   // Preview only
	Update           bool   // Update existing installation
	Track            bool   // Install as tracked repository (preserves .git)
	OnProgress       ProgressCallback
	Skills           []string // Select specific skills from multi-skill repo (comma-separated)
	Exclude          []string // Skills to exclude from installation (comma-separated)
	All              bool     // Install all discovered skills without prompting
	Yes              bool     // Auto-accept all prompts (equivalent to --all for multi-skill repos)
	Into             string   // Install into subdirectory (e.g. "frontend" or "frontend/react")
	SkipAudit        bool     // Skip security audit entirely
	AuditVerbose     bool     // Print full audit findings in CLI output (default is compact summary)
	AuditThreshold   string   // Block threshold: CRITICAL/HIGH/MEDIUM/LOW/INFO
	AuditProjectRoot string   // Project root for project-mode audit rule resolution
}

// ShouldInstallAll returns true if all discovered skills should be installed without prompting.
func (o InstallOptions) ShouldInstallAll() bool { return o.All || o.Yes }

// HasSkillFilter returns true if specific skills were requested via --skill flag.
func (o InstallOptions) HasSkillFilter() bool { return len(o.Skills) > 0 }

// InstallResult reports the outcome of an installation
type InstallResult struct {
	SkillName      string
	SkillPath      string
	Source         string
	Action         string // "cloned", "copied", "updated", "skipped"
	Warnings       []string
	AuditThreshold string
	AuditRiskScore int
	AuditRiskLabel string
	AuditSkipped   bool
}

// SkillInfo represents a discovered skill in a repository
type SkillInfo struct {
	Name        string // Skill name (directory name)
	Path        string // Relative path from repo root
	License     string // License from SKILL.md frontmatter (if any)
	Description string // Description from SKILL.md frontmatter (if any)
}

// DiscoveryResult contains discovered skills from a repository
type DiscoveryResult struct {
	RepoPath   string      // Temp directory where repo was cloned
	Skills     []SkillInfo // Discovered skills
	Source     *Source     // Original source
	CommitHash string      // Source commit hash when available
	Warnings   []string    // Non-fatal warnings during discovery
}

// TrackedRepoResult reports the outcome of a tracked repo installation
type TrackedRepoResult struct {
	RepoName       string   // Name of the tracked repo (e.g., "_team-skills")
	RepoPath       string   // Full path to the repo
	SkillCount     int      // Number of skills discovered
	Skills         []string // Names of discovered skills
	Action         string   // "cloned", "updated", "skipped"
	Warnings       []string
	AuditThreshold string
	AuditRiskScore int
	AuditRiskLabel string
	AuditSkipped   bool
}

// removeAll is a test hook used by audit/install paths.
var removeAll = os.RemoveAll

// Install executes the installation from source to destination.
// This file is intentionally a thin facade; implementation lives in split files.
func Install(source *Source, destPath string, opts InstallOptions) (*InstallResult, error) {
	return installImpl(source, destPath, opts)
}

// DiscoverFromGit clones a repository and discovers skills inside it.
func DiscoverFromGit(source *Source) (*DiscoveryResult, error) {
	return discoverFromGitImpl(source)
}

// DiscoverFromGitWithProgress clones a repository and discovers skills inside it
// while optionally streaming git progress output.
func DiscoverFromGitWithProgress(source *Source, onProgress ProgressCallback) (*DiscoveryResult, error) {
	return discoverFromGitWithProgressImpl(source, onProgress)
}

// DiscoverFromGitSubdir clones a repository and discovers skills in the source subdir.
func DiscoverFromGitSubdir(source *Source) (*DiscoveryResult, error) {
	return discoverFromGitSubdirImpl(source)
}

// DiscoverFromGitSubdirWithProgress clones a repository and discovers skills in
// the source subdir while optionally streaming git progress output.
func DiscoverFromGitSubdirWithProgress(source *Source, onProgress ProgressCallback) (*DiscoveryResult, error) {
	return discoverFromGitSubdirWithProgressImpl(source, onProgress)
}

// CleanupDiscovery removes temporary resources created by discovery.
func CleanupDiscovery(result *DiscoveryResult) {
	cleanupDiscoveryImpl(result)
}

// InstallFromDiscovery installs one selected skill from a discovery result.
func InstallFromDiscovery(discovery *DiscoveryResult, skill SkillInfo, destPath string, opts InstallOptions) (*InstallResult, error) {
	return installFromDiscoveryImpl(discovery, skill, destPath, opts)
}

// InstallTrackedRepo clones a git repository as a tracked repo.
func InstallTrackedRepo(source *Source, sourceDir string, opts InstallOptions) (*TrackedRepoResult, error) {
	return installTrackedRepoImpl(source, sourceDir, opts)
}

// GetUpdatableSkills returns skill names that have metadata with a remote source.
func GetUpdatableSkills(sourceDir string) ([]string, error) {
	return getUpdatableSkillsImpl(sourceDir)
}

// GetTrackedRepos returns tracked repositories in the source directory.
func GetTrackedRepos(sourceDir string) ([]string, error) {
	return getTrackedReposImpl(sourceDir)
}
