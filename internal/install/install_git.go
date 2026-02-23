package install

import (
	"context"
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

// isGitRepo is an alias for IsGitRepo (for internal use)
func isGitRepo(path string) bool {
	return IsGitRepo(path)
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

func usedTokenAuth(extraEnv []string) bool {
	for _, env := range extraEnv {
		if strings.HasPrefix(env, "GIT_CONFIG_KEY_") && strings.Contains(env, ".insteadOf") {
			return true
		}
	}
	return false
}

// wrapGitError inspects stderr output to produce actionable error messages.
func wrapGitError(stderr string, err error, tokenAuthAttempted bool) error {
	s := sanitizeTokens(strings.TrimSpace(stderr))
	if strings.Contains(s, "Authentication failed") ||
		strings.Contains(s, "could not read Username") ||
		strings.Contains(s, "terminal prompts disabled") {
		if tokenAuthAttempted {
			return fmt.Errorf("authentication failed — token rejected, check permissions and expiry\n       %s", s)
		}
		return fmt.Errorf("authentication required — options:\n"+
			"       1. SSH URL: git@<host>:<owner>/<repo>.git\n"+
			"       2. Token env var: GITHUB_TOKEN, GITLAB_TOKEN, BITBUCKET_TOKEN, or SKILLSHARE_GIT_TOKEN\n"+
			"       3. Git credential helper: gh auth login\n       %s", s)
	}
	if s != "" {
		return fmt.Errorf("%s", s)
	}
	return err
}

// cloneRepo performs a git clone (quiet mode for cleaner output).
// If a token is available in env vars, it injects authentication via
// GIT_CONFIG env vars without modifying the stored remote URL.
func cloneRepo(url, destPath string, shallow bool, onProgress ProgressCallback) error {
	args := []string{"clone"}
	if onProgress != nil {
		args = append(args, "--progress")
	} else {
		args = append(args, "--quiet")
	}
	if shallow {
		args = append(args, "--depth", "1")
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

// copyDir copies a directory recursively
