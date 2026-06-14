package install

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode"
)

const maxURLDecodeRounds = 3

// validateRepoSubdir checks that a repo subdir is a safe relative path.
// It rejects: NUL, control chars, backslashes, absolute paths, traversal (.)
// and (..) segments. It iteratively URL-decodes up to maxURLDecodeRounds to
// catch encoded traversal like %2e%2e or ..%2F... Every observable form —
// before and after each decode — must pass checkSubdirSafety. If the string
// is still encodable after exhausting all rounds, it is rejected.
func validateRepoSubdir(subdir string) error {
	if subdir == "" {
		return nil
	}
	current := subdir
	for round := 0; round < maxURLDecodeRounds; round++ {
		if err := checkSubdirSafety(current); err != nil {
			return fmt.Errorf("unsafe subdir %q: %w", subdir, err)
		}
		decoded, err := url.PathUnescape(current)
		if err != nil {
			return fmt.Errorf("unsafe subdir %q: invalid URL encoding: %w", subdir, err)
		}
		if decoded == current {
			return nil // stable — no more encoding to strip
		}
		current = decoded
	}
	// Final round: validate the fully-decoded result.
	if err := checkSubdirSafety(current); err != nil {
		return fmt.Errorf("unsafe subdir %q: %w", subdir, err)
	}
	// Check if it can still be decoded (deeper than maxURLDecodeRounds).
	if decoded, err := url.PathUnescape(current); err == nil && decoded != current {
		return fmt.Errorf("unsafe subdir %q: too deeply URL-encoded", subdir)
	}
	return nil
}

func checkSubdirSafety(s string) error {
	for i, r := range s {
		if r == 0 {
			return fmt.Errorf("contains NUL at byte %d", i)
		}
		if r < 0x20 || r == 0x7f || (r >= 0x80 && r < 0xa0) {
			return fmt.Errorf("contains control character U+%04X at byte %d", r, i)
		}
		if r == '\\' {
			return fmt.Errorf("contains backslash at byte %d", i)
		}
	}
	if filepath.IsAbs(s) {
		return fmt.Errorf("is absolute path")
	}
	for _, seg := range strings.Split(s, "/") {
		if seg == "." || seg == ".." {
			return fmt.Errorf("contains traversal segment %q", seg)
		}
	}
	return nil
}

// validateSourceName checks that a derived source name is safe.
// It rejects: empty, ".", "..", NUL, control chars, slashes, and backslashes.
// Dots within the name (e.g. "team.json") are allowed.
func validateSourceName(name string) error {
	if name == "" || name == "." || name == ".." {
		return fmt.Errorf("name is empty, dot, or dot-dot")
	}
	for i, r := range name {
		if r == 0 {
			return fmt.Errorf("name contains NUL at byte %d", i)
		}
		if unicode.IsControl(r) {
			return fmt.Errorf("name contains control character U+%04X at byte %d", r, i)
		}
		if r == '/' || r == '\\' {
			return fmt.Errorf("name contains path separator at byte %d", i)
		}
	}
	return nil
}

// SourceType represents the type of installation source
type SourceType int

const (
	SourceTypeUnknown SourceType = iota
	SourceTypeLocalPath
	SourceTypeGitHub
	SourceTypeGitHTTPS
	SourceTypeGitSSH
)

func (t SourceType) String() string {
	switch t {
	case SourceTypeLocalPath:
		return "local"
	case SourceTypeGitHub:
		return "github"
	case SourceTypeGitHTTPS:
		return "git-https"
	case SourceTypeGitSSH:
		return "git-ssh"
	default:
		return "unknown"
	}
}

// Source represents a parsed installation source
type Source struct {
	Type     SourceType
	Raw      string // Original user input
	CloneURL string // Git clone URL (empty for local)
	Subdir   string // Subdirectory path for monorepo
	Path     string // Local path (empty for git)
	Name     string // Derived skill name
	Branch   string // Git branch to clone from (empty = remote default)
	// ExplicitSkill is true when the user pointed directly at a SKILL.md file.
	// That intent should resolve to exactly one skill, not a pack/discovery view.
	ExplicitSkill bool
}

// GitHub URL pattern: github.com/owner/repo[/path/to/subdir]
var githubPattern = regexp.MustCompile(`^(?:https?://)?github\.com/([^/]+)/([^/]+)(?:/(.+))?$`)

// Git SSH pattern: user@host:owner/repo[.git][//subdir]
var gitSSHPattern = regexp.MustCompile(`^([^@:\s]+)@([^:\s]+):([^/]+)/(.+?)(?:\.git)?(?://(.+))?$`)

// Git SSH URL with scheme: ssh://[user@]host[:port]/path[.git][//subdir]
var sshURLPattern = regexp.MustCompile(`^ssh://(?:([^@/]+)@)?([^/:]+)(?::(\d+))?/(.+?)(?:\.git)?(?://(.+))?$`)

// Git HTTPS pattern: https://host/path (flexible path for GitLab subgroups)
var gitHTTPSPattern = regexp.MustCompile(`^(https?)://([^/]+)/(.+)$`)

// File URL pattern: file:///path/to/repo[//subdir]
var fileURLPattern = regexp.MustCompile(`^file://(.+?)(?://(.+))?$`)

// Azure DevOps HTTPS: https://dev.azure.com/{org}/{project}/_git/{repo}[/subdir]
var azureDevOpsPattern = regexp.MustCompile(
	`^https?://dev\.azure\.com/([^/]+)/([^/]+)/_git/([^/?]+?)(?:\.git)?(?:/(.+))?$`)

// Azure DevOps legacy: https://{org}.visualstudio.com/{project}/_git/{repo}[/subdir]
var azureVSPattern = regexp.MustCompile(
	`^https?://([^.]+)\.visualstudio\.com/([^/]+)/_git/([^/?]+?)(?:\.git)?(?:/(.+))?$`)

// Azure DevOps SSH: git@ssh.dev.azure.com:v3/{org}/{project}/{repo}[//subdir]
var azureSSHPattern = regexp.MustCompile(
	`^git@ssh\.dev\.azure\.com:v3/([^/]+)/([^/]+)/(.+?)(?:\.git)?(?://(.+))?$`)

// Azure DevOps on-premises: https://{custom-host}/{org}/{project}/_git/{repo}[/subdir]
var azureOnPremPattern = regexp.MustCompile(
	`^https?://([^/]+)/([^/]+)/([^/]+)/_git/([^/?]+?)(?:\.git)?(?:/(.+))?$`)

// ParseOptions holds optional configuration that affects source parsing.
type ParseOptions struct {
	GitLabHosts []string // extra hostnames to treat as GitLab (nested subgroup support)
	AzureHosts  []string // extra hostnames to treat as Azure DevOps on-premises
}

// IsSSHURL reports whether input is an SSH URL — either scp-style
// (git@host:owner/repo.git) or scheme-style (ssh://[user@]host[:port]/path),
// optionally with a //subdir suffix. Such sources must be resolved by cloning
// rather than a direct HTTP fetch or local read.
func IsSSHURL(input string) bool {
	s := strings.TrimSpace(input)
	return strings.HasPrefix(s, "ssh://") || gitSSHPattern.MatchString(s)
}

// SSHIdentity extracts the SSH username and hostname from scp-style or
// scheme-style SSH sources. It returns ok=false when either part is absent.
func SSHIdentity(input string) (user, host string, ok bool) {
	s := strings.TrimSpace(input)
	if matches := gitSSHPattern.FindStringSubmatch(s); matches != nil {
		user = strings.TrimSpace(matches[1])
		host = strings.ToLower(strings.TrimSpace(matches[2]))
		return user, host, user != "" && host != ""
	}
	if matches := sshURLPattern.FindStringSubmatch(s); matches != nil {
		user = strings.TrimSpace(matches[1])
		host = strings.ToLower(strings.TrimSpace(matches[2]))
		return user, host, user != "" && host != ""
	}
	return "", "", false
}

// ParseSource analyzes the input string and returns a Source struct.
// Uses default (zero) ParseOptions.
func ParseSource(input string) (*Source, error) {
	return ParseSourceWithOptions(input, ParseOptions{})
}

// ParseSourceWithOptions analyzes the input string with the given options
// and returns a Source struct.
func ParseSourceWithOptions(input string, opts ParseOptions) (*Source, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return nil, fmt.Errorf("source cannot be empty")
	}

	// Expand GitHub shorthand: owner/repo -> github.com/owner/repo
	input = expandGitHubShorthand(input)

	source := &Source{Raw: input}

	// Check for file:// URL (for testing with local git repos)
	if matches := fileURLPattern.FindStringSubmatch(input); matches != nil {
		return parseFileURL(matches, source)
	}

	// Check for local path first (starts with /, ~, or .)
	if isLocalPath(input) {
		return parseLocalPath(input, source)
	}

	// Try Azure DevOps patterns (before generic HTTPS to avoid misparse)
	if matches := azureDevOpsPattern.FindStringSubmatch(input); matches != nil {
		return parseAzureDevOps(matches[1], matches[2], matches[3], matches[4], source)
	}
	if matches := azureVSPattern.FindStringSubmatch(input); matches != nil {
		return parseAzureDevOps(matches[1], matches[2], matches[3], matches[4], source)
	}
	if matches := azureSSHPattern.FindStringSubmatch(input); matches != nil {
		return parseAzureSSH(matches[1], matches[2], matches[3], matches[4], source)
	}

	// Try Azure DevOps on-premises (custom host with /_git/ marker)
	if len(opts.AzureHosts) > 0 {
		if matches := azureOnPremPattern.FindStringSubmatch(input); matches != nil {
			if isAzureHost(matches[1], opts.AzureHosts) {
				return parseAzureOnPrem(matches[1], matches[2], matches[3], matches[4], matches[5], source)
			}
		}
	}

	// Try GitHub shorthand pattern
	if matches := githubPattern.FindStringSubmatch(input); matches != nil {
		return parseGitHub(matches, source)
	}

	// Try Git SSH URL with scheme (ssh://...)
	if matches := sshURLPattern.FindStringSubmatch(input); matches != nil {
		return parseSSHURL(matches, source)
	}

	// Try Git SSH pattern
	if matches := gitSSHPattern.FindStringSubmatch(input); matches != nil {
		return parseGitSSH(matches, source)
	}

	// Try Git HTTPS pattern (non-GitHub)
	if matches := gitHTTPSPattern.FindStringSubmatch(input); matches != nil {
		return parseGitHTTPS(matches, source, opts)
	}

	return nil, fmt.Errorf("unrecognized source format: %s", input)
}

// validateCloneURL checks that a CloneURL contains no NUL or control characters.
func validateCloneURL(cloneURL string) error {
	for i, r := range cloneURL {
		if r == 0 {
			return fmt.Errorf("clone URL contains NUL at byte %d", i)
		}
		if r < 0x20 || r == 0x7f || (r >= 0x80 && r < 0xa0) {
			return fmt.Errorf("clone URL contains control character at byte %d", i)
		}
	}
	return nil
}

func isLocalPath(input string) bool {
	return strings.HasPrefix(input, "/") ||
		strings.HasPrefix(input, "~") ||
		strings.HasPrefix(input, "./") ||
		strings.HasPrefix(input, "../")
}

// expandGitHubShorthand expands owner/repo to github.com/owner/repo
// Examples:
//   - anthropics/skills -> github.com/anthropics/skills
//   - anthropics/skills/skills/pdf -> github.com/anthropics/skills/skills/pdf
//   - ado:org/project/repo -> https://dev.azure.com/org/project/_git/repo
func expandGitHubShorthand(input string) string {
	// Azure DevOps shorthand: ado:org/project/repo[/subdir]
	if strings.HasPrefix(input, "ado:") {
		parts := strings.SplitN(input[4:], "/", 4) // org/project/repo[/subdir]
		if len(parts) >= 3 {
			base := fmt.Sprintf("https://dev.azure.com/%s/%s/_git/%s", parts[0], parts[1], parts[2])
			if len(parts) == 4 {
				return base + "/" + parts[3]
			}
			return base
		}
	}

	// Skip if already has a known prefix
	if strings.HasPrefix(input, "github.com/") ||
		strings.HasPrefix(input, "http://") ||
		strings.HasPrefix(input, "https://") ||
		strings.HasPrefix(input, "ssh://") ||
		gitSSHPattern.MatchString(input) ||
		strings.HasPrefix(input, "file://") ||
		isLocalPath(input) {
		return input
	}

	// Check if it looks like owner/repo (at least one slash)
	if strings.Contains(input, "/") {
		// If the first segment contains ".", it's a domain (e.g., gitlab.com/user/repo)
		// not a GitHub owner — prepend https:// so gitHTTPSPattern can match it
		firstSlash := strings.Index(input, "/")
		firstSegment := input[:firstSlash]
		if strings.Contains(firstSegment, ".") {
			return "https://" + input
		}
		return "github.com/" + input
	}

	return input
}

func parseLocalPath(input string, source *Source) (*Source, error) {
	source.Type = SourceTypeLocalPath

	// Expand ~ to home directory
	path := input
	if strings.HasPrefix(path, "~") {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("cannot expand home directory: %w", err)
		}
		path = filepath.Join(home, path[1:])
	}

	// Convert to absolute path
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("invalid path: %w", err)
	}

	source.Path = absPath
	source.Name = filepath.Base(absPath)
	return source, nil
}

func parseGitHub(matches []string, source *Source) (*Source, error) {
	// matches: [full, owner, repo, subdir]
	owner := matches[1]
	if err := validateSourceName(owner); err != nil {
		return nil, fmt.Errorf("invalid owner name: %w", err)
	}
	repo := strings.TrimSuffix(matches[2], ".git")
	if err := validateSourceName(repo); err != nil {
		return nil, fmt.Errorf("invalid repo name: %w", err)
	}
	subdir := ""
	if len(matches) > 3 {
		subdir = matches[3]
	}

	// Handle GitHub web URL format: /tree/{branch}/path or /blob/{branch}/path
	// Strip the tree/branch or blob/branch prefix to get the actual subdir
	subdir, source.ExplicitSkill = stripGitHubBranchPrefix(subdir)

	// Normalize "." subdir (explicit root) to empty string
	if subdir == "." {
		subdir = ""
	}

	source.Type = SourceTypeGitHub
	source.CloneURL = fmt.Sprintf("https://github.com/%s/%s.git", owner, repo)

	if err := validateCloneURL(source.CloneURL); err != nil {
		return nil, err
	}

	if subdir != "" {
		if err := validateRepoSubdir(subdir); err != nil {
			return nil, err
		}
		source.Subdir = subdir
		name := filepath.Base(subdir)
		if err := validateSourceName(name); err != nil {
			return nil, err
		}
		source.Name = name
	} else {
		source.Name = repo
	}

	return source, nil
}

// stripGitHubBranchPrefix removes tree/{branch}/ or blob/{branch}/ from GitHub web URLs.
// When a blob/ URL points directly at a SKILL.md file, the containing directory is
// used instead so the resulting subdir represents a skill (not a literal file name).
func stripGitHubBranchPrefix(subdir string) (string, bool) {
	if subdir == "" {
		return "", false
	}

	parts := strings.SplitN(subdir, "/", 3)
	// Check if starts with "tree" or "blob" (GitHub web URL format)
	if len(parts) >= 2 && (parts[0] == "tree" || parts[0] == "blob") {
		// parts[0] = "tree" or "blob"
		// parts[1] = branch name (e.g., "main", "master", "v1.0")
		// parts[2] = actual path (if exists)
		isBlob := parts[0] == "blob"
		if len(parts) == 3 {
			return trimSkillFileSuffix(parts[2], isBlob)
		}
		// Only tree/branch, no actual subdir
		return "", false
	}

	return subdir, false
}

// trimSkillFileSuffix strips a trailing SKILL.md segment from a blob URL path so
// the resulting subdir is the containing directory of the skill. Non-blob URLs
// and paths that do not end in SKILL.md are returned unchanged.
func trimSkillFileSuffix(path string, isBlob bool) (string, bool) {
	if !isBlob {
		return path, false
	}
	if !strings.EqualFold(filepath.Base(path), "SKILL.md") {
		return path, false
	}
	parent := filepath.ToSlash(filepath.Dir(path))
	if parent == "." {
		return "", true
	}
	return parent, true
}

func parseGitSSH(matches []string, source *Source) (*Source, error) {
	// matches: [full, user, host, owner, repo, subdir]
	user := matches[1]
	host := matches[2]
	owner := matches[3]
	if err := validateSourceName(owner); err != nil {
		return nil, fmt.Errorf("invalid owner name: %w", err)
	}
	repo := strings.TrimSuffix(matches[4], ".git")
	subdir := ""
	if len(matches) > 5 {
		subdir = matches[5]
	}

	source.Type = SourceTypeGitSSH
	source.CloneURL = fmt.Sprintf("%s@%s:%s/%s.git", user, host, owner, repo)

	if err := validateCloneURL(source.CloneURL); err != nil {
		return nil, err
	}

	if subdir != "" {
		if err := validateRepoSubdir(subdir); err != nil {
			return nil, err
		}
		source.Subdir = subdir
		name := filepath.Base(subdir)
		if err := validateSourceName(name); err != nil {
			return nil, err
		}
		source.Name = name
	} else {
		// Validate the final segment (actual repo name), not the full path
		// which may contain subgroup separators (e.g. org/subgroup/repo).
		name := filepath.Base(repo)
		if err := validateSourceName(name); err != nil {
			return nil, err
		}
		source.Name = name
	}

	return source, nil
}

func parseSSHURL(matches []string, source *Source) (*Source, error) {
	// matches: [full, user, host, port, repoPath, subdir]
	user := matches[1]
	host := matches[2]
	port := matches[3]
	repoPath := strings.TrimSuffix(matches[4], ".git")
	subdir := ""
	if len(matches) > 5 {
		subdir = matches[5]
	}

	source.Type = SourceTypeGitSSH

	hostPart := host
	if port != "" {
		hostPart = host + ":" + port
	}
	userPart := ""
	if user != "" {
		userPart = user + "@"
	}
	source.CloneURL = fmt.Sprintf("ssh://%s%s/%s.git", userPart, hostPart, repoPath)

	if err := validateCloneURL(source.CloneURL); err != nil {
		return nil, err
	}

	if subdir != "" {
		if err := validateRepoSubdir(subdir); err != nil {
			return nil, err
		}
		source.Subdir = subdir
		name := filepath.Base(subdir)
		if err := validateSourceName(name); err != nil {
			return nil, err
		}
		source.Name = name
	} else {
		// Validate the final segment (actual repo name), not the full path.
		name := filepath.Base(repoPath)
		if err := validateSourceName(name); err != nil {
			return nil, err
		}
		source.Name = name
	}

	return source, nil
}

func parseFileURL(matches []string, source *Source) (*Source, error) {
	// matches: [full, path, subdir]
	path := filepath.Clean(matches[1])
	subdir := ""
	if len(matches) > 2 && matches[2] != "" {
		subdir = strings.TrimRight(matches[2], "/")
	}

	source.Type = SourceTypeGitHTTPS // Treat as git for cloning
	source.CloneURL = "file://" + path

	if err := validateCloneURL(source.CloneURL); err != nil {
		return nil, err
	}

	if subdir != "" {
		if err := validateRepoSubdir(subdir); err != nil {
			return nil, err
		}
		source.Subdir = subdir
		name := filepath.Base(subdir)
		if err := validateSourceName(name); err != nil {
			return nil, err
		}
		source.Name = name
	} else {
		source.Name = filepath.Base(path)
	}

	return source, nil
}

func parseAzureDevOps(org, project, repo, subdir string, source *Source) (*Source, error) {
	return parseAzureOnPrem("dev.azure.com", org, project, repo, subdir, source)
}

func parseAzureOnPrem(host, org, project, repo, subdir string, source *Source) (*Source, error) {
	repo = strings.TrimSuffix(repo, ".git")
	source.Type = SourceTypeGitHTTPS
	source.CloneURL = fmt.Sprintf("https://%s/%s/%s/_git/%s", host, org, project, repo)
	if err := validateCloneURL(source.CloneURL); err != nil {
		return nil, err
	}
	if subdir != "" {
		if err := validateRepoSubdir(subdir); err != nil {
			return nil, err
		}
		source.Subdir = subdir
		name := filepath.Base(subdir)
		if err := validateSourceName(name); err != nil {
			return nil, err
		}
		source.Name = name
	} else {
		name := filepath.Base(repo)
		if err := validateSourceName(name); err != nil {
			return nil, err
		}
		source.Name = name
	}
	return source, nil
}

func parseAzureSSH(org, project, repo, subdir string, source *Source) (*Source, error) {
	repo = strings.TrimSuffix(repo, ".git")
	source.Type = SourceTypeGitSSH
	source.CloneURL = fmt.Sprintf("git@ssh.dev.azure.com:v3/%s/%s/%s", org, project, repo)
	if err := validateCloneURL(source.CloneURL); err != nil {
		return nil, err
	}
	if subdir != "" {
		if err := validateRepoSubdir(subdir); err != nil {
			return nil, err
		}
		source.Subdir = subdir
		name := filepath.Base(subdir)
		if err := validateSourceName(name); err != nil {
			return nil, err
		}
		source.Name = name
	} else {
		name := filepath.Base(repo)
		if err := validateSourceName(name); err != nil {
			return nil, err
		}
		source.Name = name
	}
	return source, nil
}

func parseGitHTTPS(matches []string, source *Source, opts ParseOptions) (*Source, error) {
	// matches: [full, schema, host, path]
	schema := matches[1]
	host := matches[2]
	// Trim trailing slashes first, then /. — order matters:
	// "foo/.//" → "foo/." → "foo"
	path := strings.TrimRight(matches[3], "/")
	path = strings.TrimSuffix(path, "/.")

	var repoPath, subdir string

	if strings.HasSuffix(path, ".git") {
		// Explicit .git suffix marks end of repo path, no subdir
		repoPath = strings.TrimSuffix(path, ".git")
	} else if idx := strings.Index(path, ".git/"); idx >= 0 {
		// .git/ in the middle splits repo from subdir
		repoPath = path[:idx]
		subdir = path[idx+len(".git/"):]
	} else if idx := strings.Index(path, "/-/"); idx >= 0 {
		// GitLab web URL marker: /-/tree/branch/path or /-/blob/branch/path
		repoPath = path[:idx]
		subdir = "-/" + path[idx+len("/-/"):]
	} else if strings.Contains(host, "bitbucket") {
		if idx := strings.Index(path, "/src/"); idx >= 0 {
			// Bitbucket web URL: owner/repo/src/branch/path
			repoPath = path[:idx]
			subdir = path[idx+1:] // keep "src/..." for stripGitBranchPrefix
		} else {
			repoPath = path
		}
	} else if isGitLabHost(host, opts.GitLabHosts) {
		// GitLab hosts (including on-prem like onprem.gitlab.internal)
		// may have nested subgroups up to 20 levels deep.
		// Without .git, treat entire path as repo.
		repoPath = path
	} else {
		// Default for GHE, Gitea, Gogs, and other platforms:
		// assume owner/repo (2 segments), remainder is subdir.
		parts := strings.SplitN(path, "/", 3)
		if len(parts) >= 2 {
			repoPath = parts[0] + "/" + parts[1]
			if len(parts) == 3 {
				subdir = parts[2]
			}
		} else {
			repoPath = path
		}
	}

	// Strip platform-specific branch prefixes from web URLs
	subdir, source.ExplicitSkill = stripGitBranchPrefix(host, subdir)

	// Normalize "." subdir (explicit root) to empty string
	if subdir == "." {
		subdir = ""
	}

	repoName := filepath.Base(repoPath)
	if err := validateSourceName(repoName); err != nil {
		return nil, fmt.Errorf("invalid repo name: %w", err)
	}

	source.Type = SourceTypeGitHTTPS
	source.CloneURL = fmt.Sprintf("%s://%s/%s.git", schema, host, repoPath)

	if err := validateCloneURL(source.CloneURL); err != nil {
		return nil, err
	}

	if subdir != "" {
		if err := validateRepoSubdir(subdir); err != nil {
			return nil, err
		}
		source.Subdir = subdir
		name := filepath.Base(subdir)
		if err := validateSourceName(name); err != nil {
			return nil, err
		}
		source.Name = name
	} else {
		source.Name = repoName
	}

	return source, nil
}

func hostMatchesAny(host string, list []string) bool {
	for _, h := range list {
		if strings.EqualFold(h, host) {
			return true
		}
	}
	return false
}

// isAzureHost returns true if the host is an Azure DevOps on-premises instance.
func isAzureHost(host string, extraHosts []string) bool {
	return hostMatchesAny(host, extraHosts)
}

// isGitLabHost returns true if the host should be treated as a GitLab instance.
func isGitLabHost(host string, extraHosts []string) bool {
	return strings.Contains(host, "gitlab") || strings.Contains(host, "jihulab") ||
		hostMatchesAny(host, extraHosts)
}

// stripGitBranchPrefix removes platform-specific branch path segments from web URLs.
// Bitbucket: src/{branch}/path → path
// GitLab:    -/tree/{branch}/path → path, -/blob/{branch}/path → path
func stripGitBranchPrefix(host, subdir string) (string, bool) {
	if subdir == "" {
		return "", false
	}

	subdir = strings.TrimRight(subdir, "/")
	parts := strings.SplitN(subdir, "/", 3)

	// Bitbucket: src/{branch}/path — there is no separate blob marker, so we
	// best-effort treat a trailing SKILL.md the same as a blob URL.
	if strings.Contains(host, "bitbucket") && len(parts) >= 2 && parts[0] == "src" {
		if len(parts) == 3 {
			return trimSkillFileSuffix(parts[2], true)
		}
		return "", false
	}

	// GitLab: -/tree/{branch}/path or -/blob/{branch}/path
	if parts[0] == "-" && len(parts) >= 2 {
		rest := strings.SplitN(parts[1], "/", 2)
		if rest[0] == "tree" || rest[0] == "blob" {
			isBlob := rest[0] == "blob"
			// subdir is "-/tree/{branch}/path" or "-/blob/{branch}/path"
			// After SplitN(subdir, "/", 3): parts = ["-", "tree", "{branch}/path"]
			// Need to further split parts[2] to get past branch
			if len(parts) == 3 {
				inner := strings.SplitN(parts[2], "/", 2)
				// inner[0] = branch, inner[1] = actual path
				if len(inner) == 2 {
					return trimSkillFileSuffix(inner[1], isBlob)
				}
			}
			return "", false
		}
	}

	return subdir, false
}

// HasSubdir returns true if this source requires subdirectory extraction
func (s *Source) HasSubdir() bool {
	return s.Subdir != ""
}

// TargetsExplicitSkill reports whether the user pointed directly at a SKILL.md file.
func (s *Source) TargetsExplicitSkill() bool {
	return s != nil && s.ExplicitSkill
}

// IsGit returns true if this source requires git clone
func (s *Source) IsGit() bool {
	return s.Type == SourceTypeGitHub ||
		s.Type == SourceTypeGitHTTPS ||
		s.Type == SourceTypeGitSSH
}

// GitHubOwner returns the repository owner for GitHub/GHE sources.
// Returns empty string for non-GitHub hosts or unparsable URLs.
func (s *Source) GitHubOwner() string {
	owner, _ := s.gitHubOwnerRepo()
	return owner
}

// GitHubRepo returns the repository name for GitHub/GHE sources.
// Returns empty string for non-GitHub hosts or unparsable URLs.
func (s *Source) GitHubRepo() string {
	_, repo := s.gitHubOwnerRepo()
	return repo
}

func (s *Source) gitHubOwnerRepo() (owner, repo string) {
	_, owner, repo = s.gitHubHostOwnerRepo()
	return owner, repo
}

// GitHubAPIBase returns the REST API base URL for GitHub-family sources
// (https://api.github.com for github.com, https://api.<host>.ghe.com for
// GHE Cloud/Data Residency, or https://<host>/api/v3 for GHE Server).
// Returns "" for non-GitHub hosts or unparsable URLs.
func (s *Source) GitHubAPIBase() string {
	host, _, _ := s.gitHubHostOwnerRepo()
	if host == "" {
		return ""
	}
	return gitHubAPIBaseForHost(host)
}

// gitHubHostOwnerRepo extracts the host, owner, and repo from a GitHub-family
// clone URL (HTTPS or SSH). Returns empty strings for non-GitHub hosts.
func (s *Source) gitHubHostOwnerRepo() (host, owner, repo string) {
	cloneURL := strings.TrimSpace(s.CloneURL)
	if cloneURL == "" {
		return "", "", ""
	}

	// SSH clone URL: user@host:owner/repo.git
	if sshMatches := gitSSHPattern.FindStringSubmatch(cloneURL); sshMatches != nil {
		host = strings.ToLower(strings.TrimSpace(sshMatches[2]))
		if !isGitHubLikeHost(host) {
			return "", "", ""
		}
		return host, sshMatches[3], strings.TrimSuffix(sshMatches[4], ".git")
	}

	u, err := url.Parse(cloneURL)
	if err != nil {
		return "", "", ""
	}
	host = strings.ToLower(u.Hostname())
	if !isGitHubLikeHost(host) {
		return "", "", ""
	}

	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(parts) < 2 {
		return "", "", ""
	}

	owner = parts[0]
	repo = strings.TrimSuffix(parts[1], ".git")
	if owner == "" || repo == "" {
		return "", "", ""
	}
	return host, owner, repo
}

// TrackName returns a unique name for --track mode by joining path segments with "-".
// For GitHub:    https://github.com/openai/skills.git                      → "openai-skills"
// For SSH:       git@github.com:openai/skills.git                          → "openai-skills"
// For HTTPS:     https://gitlab.com/team/repo.git                          → "team-repo"
// For subgroups: https://gitlab.com/group/subgroup/project.git             → "group-subgroup-project"
// For Azure SSH: git@ssh.dev.azure.com:v3/org/proj/repo                    → "org-proj-repo"
// For Azure:     https://dev.azure.com/org/proj/_git/repo                  → "org-proj-repo"
// Falls back to source.Name if path cannot be extracted.
func (s *Source) TrackName() string {
	cloneURL := s.CloneURL
	if cloneURL == "" {
		return s.Name
	}

	// Azure DevOps SSH: git@ssh.dev.azure.com:v3/org/project/repo
	if sshMatches := azureSSHPattern.FindStringSubmatch(s.Raw); sshMatches != nil {
		return sshMatches[1] + "-" + sshMatches[2] + "-" + strings.TrimSuffix(sshMatches[3], ".git")
	}

	// Azure DevOps HTTPS: dev.azure.com/org/project/_git/repo
	if strings.Contains(cloneURL, "dev.azure.com") || strings.Contains(cloneURL, "visualstudio.com") {
		u, err := url.Parse(cloneURL)
		if err == nil {
			parts := strings.Split(strings.Trim(u.Path, "/"), "/")
			// parts: [org, project, _git, repo]
			if len(parts) >= 4 && parts[2] == "_git" {
				return parts[0] + "-" + parts[1] + "-" + parts[3]
			}
		}
	}

	// Azure DevOps on-premises: https://custom-host/org/project/_git/repo
	if strings.Contains(cloneURL, "/_git/") {
		u, err := url.Parse(cloneURL)
		if err == nil {
			parts := strings.Split(strings.Trim(u.Path, "/"), "/")
			if len(parts) >= 4 && parts[len(parts)-2] == "_git" {
				return parts[len(parts)-4] + "-" + parts[len(parts)-3] + "-" + parts[len(parts)-1]
			}
		}
	}

	// Try SSH format: user@host:owner/repo.git
	if sshMatches := gitSSHPattern.FindStringSubmatch(s.Raw); sshMatches != nil {
		owner := sshMatches[3]
		repo := strings.TrimSuffix(sshMatches[4], ".git")
		// Replace / with - to handle subgroup paths (e.g., group/subgroup/repo)
		return owner + "-" + strings.ReplaceAll(repo, "/", "-")
	}

	// Try extracting full path from HTTPS clone URL
	cloneURL = strings.TrimSuffix(cloneURL, ".git")
	if u, err := url.Parse(cloneURL); err == nil {
		pathStr := strings.Trim(u.Path, "/")
		if pathStr != "" {
			return strings.ReplaceAll(pathStr, "/", "-")
		}
	}

	return s.Name
}

// MetaType returns the type string for metadata
func (s *Source) MetaType() string {
	if s.HasSubdir() {
		return s.Type.String() + "-subdir"
	}
	return s.Type.String()
}
