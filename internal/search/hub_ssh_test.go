package search

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

const sampleHubIndex = `{"schemaVersion":1,"skills":[{"name":"alpha","description":"A skill","source":"o/r"}]}`

// initGitRepoWithFile creates a git repo containing relPath with the given
// content and returns the repo path. The path is usable as a local git clone
// URL (git clone accepts a filesystem path).
func initGitRepoWithFile(t *testing.T, relPath, content string) string {
	t.Helper()
	dir := t.TempDir()
	runGitT(t, dir, "init")
	runGitT(t, dir, "config", "user.email", "test@test.com")
	runGitT(t, dir, "config", "user.name", "test")

	full := filepath.Join(dir, relPath)
	if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(full, []byte(content), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}
	runGitT(t, dir, "add", ".")
	runGitT(t, dir, "commit", "-m", "init")
	return dir
}

func runGitT(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=test",
		"GIT_AUTHOR_EMAIL=test@test.com",
		"GIT_COMMITTER_NAME=test",
		"GIT_COMMITTER_EMAIL=test@test.com",
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v failed: %s\n%s", args, err, out)
	}
}

func TestReadIndexFromGitClone_DefaultFile(t *testing.T) {
	repo := initGitRepoWithFile(t, defaultHubIndexFile, sampleHubIndex)

	doc, err := readIndexFromGitClone(repo, defaultHubIndexFile)
	if err != nil {
		t.Fatalf("readIndexFromGitClone: %v", err)
	}
	if len(doc.Skills) != 1 || doc.Skills[0].Name != "alpha" {
		t.Fatalf("unexpected doc: %+v", doc)
	}
}

func TestReadIndexFromGitClone_CustomSubdir(t *testing.T) {
	repo := initGitRepoWithFile(t, "hubs/team.json", sampleHubIndex)

	doc, err := readIndexFromGitClone(repo, "hubs/team.json")
	if err != nil {
		t.Fatalf("readIndexFromGitClone: %v", err)
	}
	if len(doc.Skills) != 1 {
		t.Fatalf("want 1 skill, got %d", len(doc.Skills))
	}
}

func TestReadIndexFromGitClone_MissingFile(t *testing.T) {
	repo := initGitRepoWithFile(t, defaultHubIndexFile, sampleHubIndex)

	if _, err := readIndexFromGitClone(repo, "nope.json"); err == nil {
		t.Fatal("expected error for missing index file")
	}
}

func TestReadIndexFromGitClone_PathTraversalRejected(t *testing.T) {
	repo := initGitRepoWithFile(t, defaultHubIndexFile, sampleHubIndex)

	for _, rel := range []string{"../escape.json", "../../etc/passwd", "/etc/passwd"} {
		if _, err := readIndexFromGitClone(repo, rel); err == nil {
			t.Errorf("expected error for traversal path %q", rel)
		}
	}
}
