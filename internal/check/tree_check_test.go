package check

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestParseLsTreeOutput(t *testing.T) {
	input := `040000 tree abc123def456789012345678901234567890abcd	skills/my-skill
040000 tree 1234567890abcdef1234567890abcdef12345678	data/other
`
	result := parseLsTreeOutput(input)

	if len(result) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(result))
	}
	if result["skills/my-skill"] != "abc123def456789012345678901234567890abcd" {
		t.Errorf("unexpected hash for skills/my-skill: %s", result["skills/my-skill"])
	}
	if result["data/other"] != "1234567890abcdef1234567890abcdef12345678" {
		t.Errorf("unexpected hash for data/other: %s", result["data/other"])
	}
}

func TestParseLsTreeOutput_Empty(t *testing.T) {
	result := parseLsTreeOutput("")
	if len(result) != 0 {
		t.Errorf("expected empty map, got %d entries", len(result))
	}
}

func TestParseLsTreeOutput_MalformedLines(t *testing.T) {
	input := "not a valid line\n\n040000 tree abc123\tvalid/path\n"
	result := parseLsTreeOutput(input)
	if len(result) != 1 {
		t.Fatalf("expected 1 valid entry, got %d", len(result))
	}
	if result["valid/path"] != "abc123" {
		t.Errorf("unexpected hash: %s", result["valid/path"])
	}
}

func TestFetchRemoteTreeHashes_Valid(t *testing.T) {
	// Set up a bare remote repo with a subdir
	remote := setupBareRepoWithSubdir(t, "skills/my-skill", "SKILL.md", "# Test Skill")

	result := FetchRemoteTreeHashes("file://" + remote)
	if result == nil {
		t.Fatal("expected non-nil result")
	}

	hash, ok := result["skills/my-skill"]
	if !ok {
		t.Fatal("expected skills/my-skill in result")
	}
	if len(hash) != 40 {
		t.Errorf("expected 40-char hash, got %d: %s", len(hash), hash)
	}

	// Also check parent directory is listed
	if _, ok := result["skills"]; !ok {
		t.Error("expected skills/ parent directory in result")
	}
}

func TestFetchRemoteTreeHashes_InvalidURL(t *testing.T) {
	result := FetchRemoteTreeHashes("file:///nonexistent/repo")
	if result != nil {
		t.Errorf("expected nil for invalid URL, got %v", result)
	}
}

func TestFetchRemoteTreeHashes_SubdirNotFound(t *testing.T) {
	remote := setupBareRepoWithSubdir(t, "skills/exists", "SKILL.md", "# Exists")

	result := FetchRemoteTreeHashes("file://" + remote)
	if result == nil {
		t.Fatal("expected non-nil result")
	}

	if _, ok := result["skills/not-here"]; ok {
		t.Error("did not expect skills/not-here in result")
	}
}

func TestFetchRemoteTreeHashes_StableAfterUnrelatedChange(t *testing.T) {
	// Create a bare remote with one skill
	dir := t.TempDir()
	work := filepath.Join(dir, "work")
	bare := filepath.Join(dir, "bare.git")

	gitRun(t, "", "init", "--bare", bare)
	gitRun(t, "", "clone", bare, work)
	gitConfig(t, work)

	// Add a skill subdir
	skillDir := filepath.Join(work, "skills", "stable")
	os.MkdirAll(skillDir, 0755)
	os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("# Stable"), 0644)
	gitRun(t, work, "add", "-A")
	gitRun(t, work, "commit", "-m", "add stable skill")
	gitRun(t, work, "push", "origin", "HEAD")

	hash1 := FetchRemoteTreeHashes("file://" + bare)
	if hash1 == nil {
		t.Fatal("expected non-nil result for first fetch")
	}
	stableHash1 := hash1["skills/stable"]

	// Add an unrelated file and push
	os.WriteFile(filepath.Join(work, "README.md"), []byte("# Readme"), 0644)
	gitRun(t, work, "add", "-A")
	gitRun(t, work, "commit", "-m", "unrelated change")
	gitRun(t, work, "push", "origin", "HEAD")

	hash2 := FetchRemoteTreeHashes("file://" + bare)
	if hash2 == nil {
		t.Fatal("expected non-nil result for second fetch")
	}
	stableHash2 := hash2["skills/stable"]

	if stableHash1 != stableHash2 {
		t.Errorf("tree hash changed despite no changes in skills/stable: %s → %s", stableHash1, stableHash2)
	}
}

// --- test helpers ---

func setupBareRepoWithSubdir(t *testing.T, subdir, filename, content string) string {
	t.Helper()
	dir := t.TempDir()
	work := filepath.Join(dir, "work")
	bare := filepath.Join(dir, "bare.git")

	gitRun(t, "", "init", "--bare", bare)
	gitRun(t, "", "clone", bare, work)
	gitConfig(t, work)

	fullDir := filepath.Join(work, subdir)
	os.MkdirAll(fullDir, 0755)
	os.WriteFile(filepath.Join(fullDir, filename), []byte(content), 0644)
	gitRun(t, work, "add", "-A")
	gitRun(t, work, "commit", "-m", "initial")
	gitRun(t, work, "push", "origin", "HEAD")

	return bare
}

func gitConfig(t *testing.T, dir string) {
	t.Helper()
	gitRun(t, dir, "config", "user.email", "test@test.com")
	gitRun(t, dir, "config", "user.name", "test")
}

func gitRun(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	if dir != "" {
		cmd.Dir = dir
	}
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
