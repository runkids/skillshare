package install

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

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
}

// GitHub URL pattern: github.com/owner/repo[/path/to/subdir]
var githubPattern = regexp.MustCompile(`^(?:https?://)?github\.com/([^/]+)/([^/]+)(?:/(.+))?$`)

// Git SSH pattern: git@host:owner/repo.git
var gitSSHPattern = regexp.MustCompile(`^git@([^:]+):([^/]+)/(.+?)(?:\.git)?$`)

// Git HTTPS pattern: https://host/owner/repo[.git]
var gitHTTPSPattern = regexp.MustCompile(`^https?://([^/]+)/([^/]+)/([^/]+?)(?:\.git)?(?:/(.+))?$`)

// File URL pattern: file:///path/to/repo
var fileURLPattern = regexp.MustCompile(`^file://(.+)$`)

// ParseSource analyzes the input string and returns a Source struct
func ParseSource(input string) (*Source, error) {
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

	// Try GitHub shorthand pattern
	if matches := githubPattern.FindStringSubmatch(input); matches != nil {
		return parseGitHub(matches, source)
	}

	// Try Git SSH pattern
	if matches := gitSSHPattern.FindStringSubmatch(input); matches != nil {
		return parseGitSSH(matches, source)
	}

	// Try Git HTTPS pattern (non-GitHub)
	if matches := gitHTTPSPattern.FindStringSubmatch(input); matches != nil {
		return parseGitHTTPS(matches, source)
	}

	return nil, fmt.Errorf("unrecognized source format: %s", input)
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
func expandGitHubShorthand(input string) string {
	// Skip if already has a known prefix
	if strings.HasPrefix(input, "github.com/") ||
		strings.HasPrefix(input, "http://") ||
		strings.HasPrefix(input, "https://") ||
		strings.HasPrefix(input, "git@") ||
		strings.HasPrefix(input, "file://") ||
		isLocalPath(input) {
		return input
	}

	// Check if it looks like owner/repo (at least one slash)
	if strings.Contains(input, "/") {
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
	repo := strings.TrimSuffix(matches[2], ".git")
	subdir := ""
	if len(matches) > 3 {
		subdir = matches[3]
	}

	// Handle GitHub web URL format: /tree/{branch}/path or /blob/{branch}/path
	// Strip the tree/branch or blob/branch prefix to get the actual subdir
	subdir = stripGitHubBranchPrefix(subdir)

	// Normalize "." subdir (explicit root) to empty string
	if subdir == "." {
		subdir = ""
	}

	source.Type = SourceTypeGitHub
	source.CloneURL = fmt.Sprintf("https://github.com/%s/%s.git", owner, repo)

	if subdir != "" {
		source.Subdir = subdir
		// Name is the last segment of subdir
		source.Name = filepath.Base(subdir)
	} else {
		source.Name = repo
	}

	return source, nil
}

// stripGitHubBranchPrefix removes tree/{branch}/ or blob/{branch}/ from GitHub web URLs
func stripGitHubBranchPrefix(subdir string) string {
	if subdir == "" {
		return ""
	}

	parts := strings.SplitN(subdir, "/", 3)
	// Check if starts with "tree" or "blob" (GitHub web URL format)
	if len(parts) >= 2 && (parts[0] == "tree" || parts[0] == "blob") {
		// parts[0] = "tree" or "blob"
		// parts[1] = branch name (e.g., "main", "master", "v1.0")
		// parts[2] = actual path (if exists)
		if len(parts) == 3 {
			return parts[2]
		}
		// Only tree/branch, no actual subdir
		return ""
	}

	return subdir
}

func parseGitSSH(matches []string, source *Source) (*Source, error) {
	// matches: [full, host, owner, repo]
	host := matches[1]
	owner := matches[2]
	repo := strings.TrimSuffix(matches[3], ".git")

	source.Type = SourceTypeGitSSH
	source.CloneURL = fmt.Sprintf("git@%s:%s/%s.git", host, owner, repo)
	source.Name = repo

	return source, nil
}

func parseFileURL(matches []string, source *Source) (*Source, error) {
	// matches: [full, path]
	path := filepath.Clean(matches[1])

	source.Type = SourceTypeGitHTTPS // Treat as git for cloning
	source.CloneURL = "file://" + path
	source.Name = filepath.Base(path)

	return source, nil
}

func parseGitHTTPS(matches []string, source *Source) (*Source, error) {
	// matches: [full, host, owner, repo, subdir]
	host := matches[1]
	owner := matches[2]
	repo := strings.TrimSuffix(matches[3], ".git")
	subdir := ""
	if len(matches) > 4 {
		subdir = matches[4]
	}

	// Strip platform-specific branch prefixes from web URLs
	subdir = stripGitBranchPrefix(host, subdir)

	// Normalize "." subdir (explicit root) to empty string
	if subdir == "." {
		subdir = ""
	}

	source.Type = SourceTypeGitHTTPS
	source.CloneURL = fmt.Sprintf("https://%s/%s/%s.git", host, owner, repo)

	if subdir != "" {
		source.Subdir = subdir
		source.Name = filepath.Base(subdir)
	} else {
		source.Name = repo
	}

	return source, nil
}

// stripGitBranchPrefix removes platform-specific branch path segments from web URLs.
// Bitbucket: src/{branch}/path → path
// GitLab:    -/tree/{branch}/path → path, -/blob/{branch}/path → path
func stripGitBranchPrefix(host, subdir string) string {
	if subdir == "" {
		return ""
	}

	subdir = strings.TrimRight(subdir, "/")
	parts := strings.SplitN(subdir, "/", 3)

	// Bitbucket: src/{branch}/path
	if strings.Contains(host, "bitbucket") && len(parts) >= 2 && parts[0] == "src" {
		if len(parts) == 3 {
			return parts[2]
		}
		return ""
	}

	// GitLab: -/tree/{branch}/path or -/blob/{branch}/path
	if parts[0] == "-" && len(parts) >= 2 {
		rest := strings.SplitN(parts[1], "/", 2)
		if rest[0] == "tree" || rest[0] == "blob" {
			// subdir is "-/tree/{branch}/path" or "-/blob/{branch}/path"
			// After SplitN(subdir, "/", 3): parts = ["-", "tree", "{branch}/path"]
			// Need to further split parts[2] to get past branch
			if len(parts) == 3 {
				inner := strings.SplitN(parts[2], "/", 2)
				// inner[0] = branch, inner[1] = actual path
				if len(inner) == 2 {
					return inner[1]
				}
			}
			return ""
		}
	}

	return subdir
}

// HasSubdir returns true if this source requires subdirectory extraction
func (s *Source) HasSubdir() bool {
	return s.Subdir != ""
}

// IsGit returns true if this source requires git clone
func (s *Source) IsGit() bool {
	return s.Type == SourceTypeGitHub ||
		s.Type == SourceTypeGitHTTPS ||
		s.Type == SourceTypeGitSSH
}

// MetaType returns the type string for metadata
func (s *Source) MetaType() string {
	if s.HasSubdir() {
		return s.Type.String() + "-subdir"
	}
	return s.Type.String()
}
