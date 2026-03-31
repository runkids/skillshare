package install

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

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

// gitCommandTimeout is the maximum time for a git network operation.
// Remote tracked-repo clones can take longer in constrained CI/Docker networks.
const gitCommandTimeout = 180 * time.Second

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
	return runGitCommandEnv(args, dir, nil)
}

// runGitCommandEnv is like runGitCommand but accepts extra environment variables.
func runGitCommandEnv(args []string, dir string, extraEnv []string) error {
	return runGitCommandWithProgress(args, dir, extraEnv, nil)
}

// UsedTokenAuth reports whether extraEnv contains token-based auth overrides.
func UsedTokenAuth(extraEnv []string) bool {
	for _, env := range extraEnv {
		if strings.HasPrefix(env, "GIT_CONFIG_KEY_") && strings.Contains(env, ".insteadOf") {
			return true
		}
	}
	return false
}

// WrapGitError inspects stderr output to produce actionable error messages.
func WrapGitError(stderr string, err error, tokenAuthAttempted bool) error {
	s := sanitizeTokens(strings.TrimSpace(stderr))
	if IsSSLError(s) {
		return fmt.Errorf("SSL certificate verification failed — options:\n"+
			"       1. Custom CA bundle: GIT_SSL_CAINFO=/path/to/ca-bundle.crt skillshare install <url>\n"+
			"       2. Skip verification: GIT_SSL_NO_VERIFY=true skillshare install <url>\n"+
			"       3. Use SSH instead: git@<host>:<owner>/<repo>.git\n       %s", s)
	}
	if IsAuthError(s) {
		if tokenAuthAttempted {
			return fmt.Errorf("authentication failed — token rejected, check permissions and expiry\n       %s", s)
		}
		return fmt.Errorf("authentication required — options:\n"+
			"       1. SSH URL: git@<host>:<owner>/<repo>.git\n"+
			"       2. Token env var: GITHUB_TOKEN, GITLAB_TOKEN, BITBUCKET_TOKEN, AZURE_DEVOPS_TOKEN, or SKILLSHARE_GIT_TOKEN\n"+
			"       3. Git credential helper: gh auth login\n       %s", s)
	}
	if s != "" {
		// Extract the fatal/error line from verbose git output, strip hint lines
		return fmt.Errorf("%s", extractGitFatal(s))
	}
	// When stderr is empty, extract exit code for a more actionable message
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		code := exitErr.ExitCode()
		switch code {
		case 128:
			if tokenAuthAttempted {
				return fmt.Errorf("git failed (exit 128): authentication token was rejected — check permissions and expiry")
			}
			return fmt.Errorf("git failed (exit 128): repository not found or authentication required")
		case 1:
			return fmt.Errorf("git failed (exit 1): command error — check network connectivity and repository URL")
		default:
			return fmt.Errorf("git failed (exit %d)", code)
		}
	}
	return err
}

// extractGitFatal extracts the "fatal:" or "error:" line from git stderr,
// stripping verbose "hint:" lines. If no fatal/error line is found, returns
// the original text with hint lines removed.
func extractGitFatal(stderr string) string {
	var fatal string
	var nonHint []string
	for _, line := range strings.Split(stderr, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if strings.HasPrefix(trimmed, "hint:") {
			continue
		}
		if strings.HasPrefix(trimmed, "fatal: ") {
			fatal = strings.TrimPrefix(trimmed, "fatal: ")
		} else if strings.HasPrefix(trimmed, "error: ") {
			fatal = strings.TrimPrefix(trimmed, "error: ")
		}
		nonHint = append(nonHint, trimmed)
	}
	if fatal != "" {
		return fatal
	}
	if len(nonHint) > 0 {
		return strings.Join(nonHint, "; ")
	}
	return stderr
}

// cloneRepo performs a git clone (quiet mode for cleaner output).
// If a token is available in env vars, it injects authentication via
// GIT_CONFIG env vars without modifying the stored remote URL.
// branch specifies the branch to clone; empty string uses the remote default.
func cloneRepo(url, destPath, branch string, shallow bool, onProgress ProgressCallback) error {
	args := []string{"clone"}
	if onProgress != nil {
		args = append(args, "--progress")
	} else {
		args = append(args, "--quiet")
	}
	if shallow {
		args = append(args, "--depth", "1")
	}
	if branch != "" {
		args = append(args, "--branch", branch)
	}
	args = append(args, url, destPath)
	return runGitCommandWithProgress(args, "", authEnv(url), onProgress)
}

// gitPull performs a git pull (quiet mode).
// If the remote uses HTTPS and a token is available, it injects
// authentication via GIT_CONFIG env vars (same mechanism as cloneRepo).
func gitPull(repoPath string, onProgress ProgressCallback) error {
	remoteURL := getRemoteURL(repoPath)
	args := []string{"pull", "--quiet"}
	if onProgress != nil {
		args = []string{"pull", "--progress"}
	}
	return runGitCommandWithProgress(args, repoPath, authEnv(remoteURL), onProgress)
}

// getRemoteURL returns the fetch URL for the "origin" remote, or "".
func getRemoteURL(repoPath string) string {
	cmd := exec.Command("git", "remote", "get-url", "origin")
	cmd.Dir = repoPath
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
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

// getGitFullHash returns the full HEAD commit hash for reliable rollback.
// NOTE: duplicates git.GetCurrentFullHash — kept here because internal/git
// imports internal/install (for AuthEnvForURL), so the reverse import would
// create a cycle.
func getGitFullHash(repoPath string) (string, error) {
	cmd := exec.Command("git", "rev-parse", "HEAD")
	cmd.Dir = repoPath
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func shortHash(hash string) string {
	if len(hash) <= 12 {
		return hash
	}
	return hash[:12]
}

func validateTrackedRepoDirName(name string) error {
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("tracked repo name cannot be empty")
	}
	if !strings.HasPrefix(name, "_") || len(name) < 2 {
		return fmt.Errorf("tracked repo name must start with '_' and include at least one additional character")
	}
	if strings.Contains(name, "..") {
		return fmt.Errorf("tracked repo name cannot contain '..'")
	}
	if strings.Contains(name, "/") || strings.Contains(name, "\\") {
		return fmt.Errorf("tracked repo name cannot contain path separators")
	}
	if filepath.Clean(name) != name {
		return fmt.Errorf("tracked repo name contains invalid path segments")
	}
	return nil
}

// gitResetHard resets the working tree to the given revision.
// NOTE: duplicates git.ResetHard — kept here to avoid import cycle
// (internal/git imports internal/install).
func gitResetHard(repoPath, rev string) error {
	cmd := exec.Command("git", "reset", "--hard", rev)
	cmd.Dir = repoPath
	return cmd.Run()
}

// getSubdirTreeHash returns the git tree hash for a subdirectory within a repo.
// This is the SHA of the tree object for that directory — it changes only when
// files inside the subdir change, even if the repo HEAD moves.
// Returns "" on any error (no .git, subdir not found, etc.).
func getSubdirTreeHash(repoPath, subdir string) string {
	if subdir == "" {
		return ""
	}
	// Strip leading "/" from subdir — git rev-parse doesn't accept absolute paths
	subdir = strings.TrimPrefix(subdir, "/")
	cmd := exec.Command("git", "rev-parse", "HEAD:"+subdir)
	cmd.Dir = repoPath
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// copyDir copies a directory recursively
