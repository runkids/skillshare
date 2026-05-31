package install

import (
	"errors"
	"fmt"
	"os"
	"strings"
)

// InstallOptions configures the install behavior
type InstallOptions struct {
	Name             string // Override skill name
	Kind             string // "skill", "agent", or "" (auto-detect)
	Force            bool   // Overwrite existing
	DryRun           bool   // Preview only
	Update           bool   // Update existing installation
	Track            bool   // Install as tracked repository (preserves .git)
	OnProgress       ProgressCallback
	Skills           []string // Select specific skills from multi-skill repo (comma-separated)
	AgentNames       []string // Select specific agents from repo (comma-separated)
	Exclude          []string // Skills to exclude from installation (comma-separated)
	All              bool     // Install all discovered skills without prompting
	Yes              bool     // Auto-accept all prompts (equivalent to --all for multi-skill repos)
	Into             string   // Install into subdirectory (e.g. "frontend" or "frontend/react")
	SkipAudit        bool     // Skip security audit entirely
	AuditVerbose     bool     // Print full audit findings in CLI output (default is compact summary)
	AuditThreshold   string   // Block threshold: CRITICAL/HIGH/MEDIUM/LOW/INFO
	AuditProjectRoot string   // Project root for project-mode audit rule resolution
	Quiet            bool     // Suppress per-skill output in InstallFromConfig
	Branch           string   // Git branch to clone from (empty = remote default)
	SourceDir        string   // Skills root dir for centralized metadata (set by caller)
}

// IsAgentMode returns true if explicitly installing agents.
func (o InstallOptions) IsAgentMode() bool { return o.Kind == "agent" }

// HasAgentFilter returns true if specific agents were requested via -a flag.
func (o InstallOptions) HasAgentFilter() bool { return len(o.AgentNames) > 0 }

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

// AgentInfo represents a discovered agent (.md file) in a repository
type AgentInfo struct {
	Name     string // Agent name (filename without .md)
	Path     string // Relative path from repo root (e.g. "agents/tutor.md")
	FileName string // Filename (e.g. "tutor.md")
}

// DiscoveryResult contains discovered skills and agents from a repository
type DiscoveryResult struct {
	RepoPath   string      // Temp directory where repo was cloned
	Skills     []SkillInfo // Discovered skills
	Agents     []AgentInfo // Discovered agents
	Source     *Source     // Original source
	CommitHash string      // Source commit hash when available
	Warnings   []string    // Non-fatal warnings during discovery
}

// HasAgents reports whether the discovery found any agents.
func (d *DiscoveryResult) HasAgents() bool {
	return len(d.Agents) > 0
}

// HasSkills reports whether the discovery found any skills.
func (d *DiscoveryResult) HasSkills() bool {
	return len(d.Skills) > 0
}

// IsMixed reports whether the discovery found both skills and agents.
func (d *DiscoveryResult) IsMixed() bool {
	return d.HasSkills() && d.HasAgents()
}

// TrackedRepoResult reports the outcome of a tracked repo installation
type TrackedRepoResult struct {
	RepoName       string   // Name of the tracked repo (e.g., "_team-skills")
	RepoPath       string   // Full path to the repo
	SkillCount     int      // Number of skills discovered
	Skills         []string // Names of discovered skills
	AgentCount     int      // Number of agents discovered
	Agents         []string // Names of discovered agents
	Action         string   // "cloned", "updated", "skipped"
	Warnings       []string
	AuditThreshold string
	AuditRiskScore int
	AuditRiskLabel string
	AuditSkipped   bool
}

// ErrSkipSameRepo is returned when the destination skill already exists and
// was installed from the same repository. Callers should treat this as a
// friendly skip, not a hard failure.
var ErrSkipSameRepo = errors.New("already installed from same repo")

// checkExistingConflict reads the meta of an existing skill and compares
// its repo_url with the incoming source. Returns:
//   - nil when the directory is empty/invalid (caller should overwrite)
//   - ErrSkipSameRepo when the repos match (caller should skip gracefully)
//   - a descriptive error when they differ or meta is absent (real conflict)
func checkExistingConflict(destPath string, incomingCloneURL string, forceHint string) error {
	meta, err := ReadMeta(destPath)
	if err != nil || meta == nil {
		// No meta — check if directory is truly empty.
		if dirIsEmpty(destPath) {
			return nil
		}
		// Non-empty directory without meta — unknown origin.
		return fmt.Errorf("already exists. To overwrite: %s", forceHint)
	}

	if repoURLsMatch(meta.RepoURL, incomingCloneURL) {
		return fmt.Errorf("%w: use 'skillshare update' or --force to overwrite", ErrSkipSameRepo)
	}

	// Different repo — real conflict with provenance info.
	return fmt.Errorf("already exists (installed from %s). To overwrite: %s", meta.RepoURL, forceHint)
}

// dirIsEmpty returns true if the directory has no entries (or only OS junk
// like .DS_Store). Returns false for non-directories or read errors.
func dirIsEmpty(path string) bool {
	entries, err := os.ReadDir(path)
	if err != nil {
		return false
	}
	for _, e := range entries {
		if e.Name() == ".DS_Store" {
			continue
		}
		return false
	}
	return true
}

// repoURLsMatch compares two clone URLs, normalizing trailing ".git" and
// protocol differences (https vs git@). Returns false when either URL is empty.
func repoURLsMatch(a, b string) bool {
	na, nb := normalizeCloneURL(a), normalizeCloneURL(b)
	return na != "" && nb != "" && na == nb
}

func normalizeCloneURL(u string) string {
	u = strings.TrimSpace(u)
	u = strings.TrimSuffix(u, ".git")
	u = strings.TrimSuffix(u, "/")
	// user@github.com:owner/repo → github.com/owner/repo
	if at := strings.Index(u, "@"); at > 0 && !strings.Contains(u, "://") {
		u = u[at+1:]
		u = strings.Replace(u, ":", "/", 1)
	}
	// https://github.com/owner/repo → github.com/owner/repo
	u = strings.TrimPrefix(u, "https://")
	u = strings.TrimPrefix(u, "http://")
	return strings.ToLower(u)
}

// buildForceHint constructs a user-facing hint like
// "skillshare install <source> --into <dir> --force".
func buildForceHint(rawSource, into string) string {
	cmd := "skillshare install " + rawSource
	if into != "" {
		cmd += " --into " + into
	}
	cmd += " --force"
	return cmd
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

// TrackKindAmbiguousError is returned by InferTrackedKind when a tracked
// install targets a repository that contains both skills and agents and the
// caller did not pass an explicit kind. It carries the discovered counts so
// the UI can present a kind picker without re-cloning.
type TrackKindAmbiguousError struct {
	Skills int
	Agents int
}

func (e *TrackKindAmbiguousError) Error() string {
	return "tracked install is ambiguous for mixed repositories; pass --kind skill or --kind agent"
}

// InferTrackedKind determines which resource kind a tracked install should use.
// Pure-agent repositories resolve to "agent". Mixed repositories must specify
// the kind explicitly to avoid ambiguous install roots; in that case the
// returned error is a *TrackKindAmbiguousError carrying the discovered counts.
func InferTrackedKind(source *Source, explicitKind string) (string, error) {
	if !source.IsGit() {
		return "", fmt.Errorf("--track requires a git repository source")
	}

	if explicitKind == "skill" || explicitKind == "agent" {
		return explicitKind, nil
	}

	var (
		discovery *DiscoveryResult
		err       error
	)
	if source.HasSubdir() {
		discovery, err = DiscoverFromGitSubdir(source)
	} else {
		discovery, err = DiscoverFromGit(source)
	}
	if err != nil {
		return "", err
	}
	defer CleanupDiscovery(discovery)

	switch {
	case discovery.HasSkills() && discovery.HasAgents():
		return "", &TrackKindAmbiguousError{
			Skills: len(discovery.Skills),
			Agents: len(discovery.Agents),
		}
	case discovery.HasAgents() && !discovery.HasSkills():
		return "agent", nil
	default:
		return "skill", nil
	}
}

// DiscoverLocal inspects a local directory and discovers skills and agents.
func DiscoverLocal(source *Source) (*DiscoveryResult, error) {
	return discoverLocalImpl(source)
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
