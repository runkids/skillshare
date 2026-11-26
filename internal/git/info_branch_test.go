package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestGetRemoteRefHash(t *testing.T) {
	// Create bare repo
	bareRepo := filepath.Join(t.TempDir(), "test.git")
	run(t, "", "git", "init", "--bare", bareRepo)

	// Clone and set up
	workDir := filepath.Join(t.TempDir(), "work")
	run(t, "", "git", "clone", bareRepo, workDir)
	run(t, workDir, "git", "config", "user.email", "test@test.com")
	run(t, workDir, "git", "config", "user.name", "Test")

	// Commit on main
	os.WriteFile(filepath.Join(workDir, "main.txt"), []byte("main"), 0644)
	run(t, workDir, "git", "add", ".")
	run(t, workDir, "git", "commit", "-m", "main commit")
	run(t, workDir, "git", "push", "origin", "HEAD")

	// Create dev branch with different commit
	run(t, workDir, "git", "checkout", "-b", "dev")
	os.WriteFile(filepath.Join(workDir, "dev.txt"), []byte("dev"), 0644)
	run(t, workDir, "git", "add", ".")
	run(t, workDir, "git", "commit", "-m", "dev commit")
	run(t, workDir, "git", "push", "origin", "dev")

	t.Run("empty branch returns HEAD hash", func(t *testing.T) {
		hash, err := GetRemoteRefHash(bareRepo, "")
		if err != nil {
			t.Fatalf("GetRemoteRefHash: %v", err)
		}
		if hash == "" {
			t.Error("expected non-empty hash")
		}
	})

	t.Run("specific branch returns that branch hash", func(t *testing.T) {
		headHash, _ := GetRemoteRefHash(bareRepo, "")
		devHash, err := GetRemoteRefHash(bareRepo, "dev")
		if err != nil {
			t.Fatalf("GetRemoteRefHash dev: %v", err)
		}
		if devHash == headHash {
			t.Error("dev hash should differ from HEAD (main)")
		}
	})

	t.Run("nonexistent branch returns error", func(t *testing.T) {
		_, err := GetRemoteRefHash(bareRepo, "nonexistent")
		if err == nil {
			t.Error("expected error for nonexistent branch")
		}
	})
}

func TestGetRemoteHeadWithEnv_NamesDefaultBranch(t *testing.T) {
	bareRepo := filepath.Join(t.TempDir(), "test.git")
	run(t, "", "git", "init", "--bare", bareRepo)
	run(t, bareRepo, "git", "symbolic-ref", "HEAD", "refs/heads/trunk")
	workDir := filepath.Join(t.TempDir(), "work")
	run(t, "", "git", "clone", bareRepo, workDir)
	run(t, workDir, "git", "config", "user.email", "test@test.com")
	run(t, workDir, "git", "config", "user.name", "Test")
	os.WriteFile(filepath.Join(workDir, "a.txt"), []byte("a"), 0644)
	run(t, workDir, "git", "add", ".")
	run(t, workDir, "git", "commit", "-m", "trunk commit")
	run(t, workDir, "git", "push", "origin", "HEAD:trunk")
	out, err := exec.Command("git", "-C", workDir, "rev-parse", "HEAD").Output()
	if err != nil {
		t.Fatal(err)
	}
	tip := strings.TrimSpace(string(out))

	t.Run("symbolic HEAD names its branch", func(t *testing.T) {
		hash, branch, err := GetRemoteHeadWithEnv(bareRepo, nil)
		if err != nil {
			t.Fatalf("GetRemoteHeadWithEnv: %v", err)
		}
		if hash != tip[:7] || branch != "trunk" {
			t.Fatalf("got (%q, %q), want (%q, %q)", hash, branch, tip[:7], "trunk")
		}
	})

	t.Run("detached HEAD names no branch", func(t *testing.T) {
		run(t, bareRepo, "git", "update-ref", "--no-deref", "HEAD", tip)
		hash, branch, err := GetRemoteHeadWithEnv(bareRepo, nil)
		if err != nil {
			t.Fatalf("GetRemoteHeadWithEnv: %v", err)
		}
		if hash != tip[:7] || branch != "" {
			t.Fatalf("got (%q, %q), want (%q, %q)", hash, branch, tip[:7], "")
		}
	})
}

func TestGetRemoteHeadHashWithEnv_DisablesPrompts(t *testing.T) {
	binDir := t.TempDir()
	gitPath := filepath.Join(binDir, "git")
	script := `#!/bin/sh
if [ "$1" != "ls-remote" ]; then
  echo "unexpected command" >&2
  exit 2
fi
if [ "$GIT_TERMINAL_PROMPT" != "0" ] || [ -n "$GIT_ASKPASS" ] || [ -n "$SSH_ASKPASS" ]; then
  echo "interactive prompts were not disabled" >&2
  exit 2
fi
printf 'abcdef1234567890\tHEAD\n'
`
	if err := os.WriteFile(gitPath, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	hash, err := GetRemoteHeadHashWithEnv("https://example.com/repo.git", nil)
	if err != nil {
		t.Fatalf("GetRemoteHeadHashWithEnv: %v", err)
	}
	if hash != "abcdef1" {
		t.Fatalf("hash = %q, want abcdef1", hash)
	}
}

func TestGetRemoteHeadHashWithEnv_TimesOut(t *testing.T) {
	oldTimeout := remoteHashTimeout
	remoteHashTimeout = 10 * time.Millisecond
	t.Cleanup(func() { remoteHashTimeout = oldTimeout })

	binDir := t.TempDir()
	gitPath := filepath.Join(binDir, "git")
	script := `#!/bin/sh
sleep 2
`
	if err := os.WriteFile(gitPath, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	_, err := GetRemoteHeadHashWithEnv("https://example.com/repo.git", nil)
	if err == nil {
		t.Fatal("expected timeout error")
	}
	if !strings.Contains(err.Error(), "timed out") {
		t.Fatalf("error = %q, want timeout", err.Error())
	}
}

// run executes a command and fails the test on error.
func run(t *testing.T, dir string, name string, args ...string) {
	t.Helper()
	cmd := exec.Command(name, args...)
	if dir != "" {
		cmd.Dir = dir
	}
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("%s %v failed: %s %v", name, args, out, err)
	}
}
