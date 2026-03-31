package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
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
