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

func TestUpdateGitIgnore_CreatesFile(t *testing.T) {
	dir := t.TempDir()

	if err := UpdateGitIgnore(dir, "_team-skills"); err != nil {
		t.Fatal(err)
	}

	content, err := os.ReadFile(filepath.Join(dir, ".gitignore"))
	if err != nil {
		t.Fatal(err)
	}
	got := string(content)
	if !strings.Contains(got, gitignoreMarkerStart) {
		t.Error("expected marker start in .gitignore")
	}
	if !strings.Contains(got, gitignoreMarkerEnd) {
		t.Error("expected marker end in .gitignore")
	}
	if !strings.Contains(got, "_team-skills/") {
		t.Errorf("expected entry '_team-skills/' in .gitignore, got:\n%s", got)
	}
}

func TestUpdateGitIgnore_Idempotent(t *testing.T) {
	dir := t.TempDir()

	if err := UpdateGitIgnore(dir, "_team"); err != nil {
		t.Fatal(err)
	}
	if err := UpdateGitIgnore(dir, "_team"); err != nil {
		t.Fatal(err)
	}

	content, err := os.ReadFile(filepath.Join(dir, ".gitignore"))
	if err != nil {
		t.Fatal(err)
	}
	// Count occurrences of the entry
	count := strings.Count(string(content), "_team/")
	if count != 1 {
		t.Errorf("expected exactly 1 entry, got %d occurrences in:\n%s", count, string(content))
	}
}

func TestRemoveFromGitIgnore_NonExistent(t *testing.T) {
	dir := t.TempDir()
	// No .gitignore file exists
	removed, err := RemoveFromGitIgnore(dir, "something")
	if err != nil {
		t.Fatal(err)
	}
	if removed {
		t.Error("expected false when .gitignore doesn't exist")
	}
}

func TestRemoveFromGitIgnore_NotInBlock(t *testing.T) {
	dir := t.TempDir()
	// Create .gitignore with marker block but different entry
	if err := UpdateGitIgnore(dir, "_other-repo"); err != nil {
		t.Fatal(err)
	}

	removed, err := RemoveFromGitIgnore(dir, "_nonexistent")
	if err != nil {
		t.Fatal(err)
	}
	if removed {
		t.Error("expected false when entry is not in block")
	}
}

func TestGitignoreContains_Found(t *testing.T) {
	dir := t.TempDir()
	gi := filepath.Join(dir, ".gitignore")
	os.WriteFile(gi, []byte("node_modules/\n_team/\n"), 0644)

	found, err := gitignoreContains(gi, "_team/")
	if err != nil {
		t.Fatal(err)
	}
	if !found {
		t.Error("expected to find entry")
	}
}

func TestGitignoreContains_NotFound(t *testing.T) {
	dir := t.TempDir()
	gi := filepath.Join(dir, ".gitignore")
	os.WriteFile(gi, []byte("node_modules/\n"), 0644)

	found, err := gitignoreContains(gi, "_team/")
	if err != nil {
		t.Fatal(err)
	}
	if found {
		t.Error("expected not found")
	}
}

func TestGitignoreContains_NoFile(t *testing.T) {
	found, err := gitignoreContains(filepath.Join(t.TempDir(), ".gitignore"), "_team/")
	if err != nil {
		t.Fatal(err)
	}
	if found {
		t.Error("expected not found for non-existent file")
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
