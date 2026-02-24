package install

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestGetSubdirTreeHash_Valid(t *testing.T) {
	repo := initTestRepo(t)

	// Create a subdirectory with a file
	subdir := filepath.Join(repo, "skills", "my-skill")
	os.MkdirAll(subdir, 0755)
	os.WriteFile(filepath.Join(subdir, "SKILL.md"), []byte("# Test"), 0644)
	gitAdd(t, repo, ".")
	gitCommit(t, repo, "add skill")

	hash := getSubdirTreeHash(repo, "skills/my-skill")
	if hash == "" {
		t.Fatal("expected non-empty tree hash")
	}
	if len(hash) != 40 {
		t.Errorf("expected 40-char hash, got %d chars: %s", len(hash), hash)
	}
}

func TestGetSubdirTreeHash_Empty(t *testing.T) {
	repo := initTestRepo(t)
	os.WriteFile(filepath.Join(repo, "README.md"), []byte("# Repo"), 0644)
	gitAdd(t, repo, ".")
	gitCommit(t, repo, "init")

	// Empty subdir arg → always ""
	hash := getSubdirTreeHash(repo, "")
	if hash != "" {
		t.Errorf("expected empty string for empty subdir, got %q", hash)
	}
}

func TestGetSubdirTreeHash_Nonexistent(t *testing.T) {
	repo := initTestRepo(t)
	os.WriteFile(filepath.Join(repo, "README.md"), []byte("# Repo"), 0644)
	gitAdd(t, repo, ".")
	gitCommit(t, repo, "init")

	hash := getSubdirTreeHash(repo, "no/such/dir")
	if hash != "" {
		t.Errorf("expected empty string for nonexistent subdir, got %q", hash)
	}
}

func TestGetSubdirTreeHash_Stable(t *testing.T) {
	repo := initTestRepo(t)

	// Create skill subdir
	subdir := filepath.Join(repo, "skills", "stable")
	os.MkdirAll(subdir, 0755)
	os.WriteFile(filepath.Join(subdir, "SKILL.md"), []byte("# Stable"), 0644)
	gitAdd(t, repo, ".")
	gitCommit(t, repo, "add stable skill")

	hash1 := getSubdirTreeHash(repo, "skills/stable")

	// Add a different file elsewhere — tree hash for stable should NOT change
	os.WriteFile(filepath.Join(repo, "other.txt"), []byte("unrelated"), 0644)
	gitAdd(t, repo, ".")
	gitCommit(t, repo, "add unrelated file")

	hash2 := getSubdirTreeHash(repo, "skills/stable")

	if hash1 != hash2 {
		t.Errorf("tree hash changed despite no changes in subdir: %s → %s", hash1, hash2)
	}
}

func TestGetSubdirTreeHash_ChangesOnModify(t *testing.T) {
	repo := initTestRepo(t)

	subdir := filepath.Join(repo, "skills", "changing")
	os.MkdirAll(subdir, 0755)
	os.WriteFile(filepath.Join(subdir, "SKILL.md"), []byte("# V1"), 0644)
	gitAdd(t, repo, ".")
	gitCommit(t, repo, "v1")

	hash1 := getSubdirTreeHash(repo, "skills/changing")

	// Modify the skill file
	os.WriteFile(filepath.Join(subdir, "SKILL.md"), []byte("# V2"), 0644)
	gitAdd(t, repo, ".")
	gitCommit(t, repo, "v2")

	hash2 := getSubdirTreeHash(repo, "skills/changing")

	if hash1 == hash2 {
		t.Error("tree hash should change when subdir content changes")
	}
}

func TestGetSubdirTreeHash_LeadingSlash(t *testing.T) {
	repo := initTestRepo(t)

	subdir := filepath.Join(repo, "skills", "my-skill")
	os.MkdirAll(subdir, 0755)
	os.WriteFile(filepath.Join(subdir, "SKILL.md"), []byte("# Test"), 0644)
	gitAdd(t, repo, ".")
	gitCommit(t, repo, "add skill")

	// Leading slash (as stored by //-syntax sources) should work identically
	hashNoSlash := getSubdirTreeHash(repo, "skills/my-skill")
	hashWithSlash := getSubdirTreeHash(repo, "/skills/my-skill")

	if hashNoSlash == "" {
		t.Fatal("expected non-empty tree hash without slash")
	}
	if hashWithSlash != hashNoSlash {
		t.Errorf("leading slash gave different result: %q vs %q", hashWithSlash, hashNoSlash)
	}
}

// --- test helpers ---

func initTestRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	runGit(t, dir, "init")
	runGit(t, dir, "config", "user.email", "test@test.com")
	runGit(t, dir, "config", "user.name", "test")
	return dir
}

func gitAdd(t *testing.T, dir, path string) {
	t.Helper()
	runGit(t, dir, "add", path)
}

func gitCommit(t *testing.T, dir, msg string) {
	t.Helper()
	runGit(t, dir, "commit", "-m", msg)
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=test",
		"GIT_AUTHOR_EMAIL=test@test.com",
		"GIT_COMMITTER_NAME=test",
		"GIT_COMMITTER_EMAIL=test@test.com",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %s\n%s", args, err, out)
	}
}
