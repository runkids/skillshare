package install

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestUpdateGitIgnore_NormalizesBackslashes(t *testing.T) {
	dir := t.TempDir()

	// Simulate Windows: filepath.Join("skills", "my-skill") → "skills\my-skill"
	entry := "skills\\my-skill"
	if err := UpdateGitIgnore(dir, entry); err != nil {
		t.Fatal(err)
	}

	content, err := os.ReadFile(filepath.Join(dir, ".gitignore"))
	if err != nil {
		t.Fatal(err)
	}

	got := string(content)
	if strings.Contains(got, `\`) {
		t.Errorf("gitignore contains backslash:\n%s", got)
	}
	if !strings.Contains(got, "skills/my-skill/") {
		t.Errorf("gitignore should contain skills/my-skill/, got:\n%s", got)
	}
}

func TestRemoveFromGitIgnore_NormalizesBackslashes(t *testing.T) {
	dir := t.TempDir()

	// First add with forward slashes
	if err := UpdateGitIgnore(dir, "skills/my-skill"); err != nil {
		t.Fatal(err)
	}

	// Remove with backslashes (simulating Windows)
	removed, err := RemoveFromGitIgnore(dir, "skills\\my-skill")
	if err != nil {
		t.Fatal(err)
	}
	if !removed {
		t.Error("expected entry to be removed")
	}

	content, err := os.ReadFile(filepath.Join(dir, ".gitignore"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(content), "skills/my-skill") {
		t.Errorf("entry should have been removed, got:\n%s", string(content))
	}
}
