package testutil

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// SetIsolatedXDG points XDG dirs to a per-test root.
func SetIsolatedXDG(t *testing.T, root string) {
	t.Helper()

	xdgRoot := filepath.Join(root, "xdg")
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(xdgRoot, "config"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(xdgRoot, "data"))
	t.Setenv("XDG_STATE_HOME", filepath.Join(xdgRoot, "state"))
}

// ConfigureGitUser sets local git identity for test commits.
func ConfigureGitUser(t *testing.T, dir string) {
	t.Helper()
	RunGit(t, dir, "config", "user.email", "test@test.com")
	RunGit(t, dir, "config", "user.name", "Test")
}

// RunGit executes a git command and returns trimmed combined output.
func RunGit(t *testing.T, dir string, args ...string) string {
	t.Helper()

	cmd := exec.Command("git", args...)
	if dir != "" {
		cmd.Dir = dir
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %v (%s)", args, err, strings.TrimSpace(string(out)))
	}
	return strings.TrimSpace(string(out))
}

// SetupBareRemoteRepo initializes a bare remote repo at baseDir/remote.git.
func SetupBareRemoteRepo(t *testing.T, baseDir string) string {
	t.Helper()

	bareRepo := filepath.Join(baseDir, "remote.git")
	cmd := exec.Command("git", "init", "--bare", bareRepo)
	if err := cmd.Run(); err != nil {
		t.Skip("git not available")
	}
	return bareRepo
}

// SeedRemoteBranch creates one commit on the specified remote branch and sets remote HEAD.
func SeedRemoteBranch(t *testing.T, baseDir, bareRepo, branch string, files map[string]string) {
	t.Helper()

	seedDir := filepath.Join(baseDir, "seed-"+branch)
	RunGit(t, "", "clone", bareRepo, seedDir)
	ConfigureGitUser(t, seedDir)

	for rel, content := range files {
		full := filepath.Join(seedDir, rel)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatalf("failed to create dir for %s: %v", rel, err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatalf("failed to write %s: %v", rel, err)
		}
	}

	RunGit(t, seedDir, "add", "-A")
	RunGit(t, seedDir, "commit", "-m", "seed "+branch)
	RunGit(t, seedDir, "push", "origin", "HEAD:"+branch)
	RunGit(t, bareRepo, "symbolic-ref", "HEAD", "refs/heads/"+branch)
}
