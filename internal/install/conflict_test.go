package install

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNormalizeCloneURL(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"https://github.com/owner/repo.git", "github.com/owner/repo"},
		{"https://github.com/owner/repo", "github.com/owner/repo"},
		{"git@github.com:owner/repo.git", "github.com/owner/repo"},
		{"git@github.com:owner/repo", "github.com/owner/repo"},
		{"https://github.com/Owner/Repo.git", "github.com/owner/repo"},
		{"https://github.com/owner/repo/", "github.com/owner/repo"},
		{"http://github.com/owner/repo.git", "github.com/owner/repo"},
	}
	for _, tt := range tests {
		got := normalizeCloneURL(tt.input)
		if got != tt.want {
			t.Errorf("normalizeCloneURL(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestRepoURLsMatch(t *testing.T) {
	tests := []struct {
		a, b string
		want bool
	}{
		{"https://github.com/owner/repo.git", "https://github.com/owner/repo.git", true},
		{"https://github.com/owner/repo.git", "git@github.com:owner/repo.git", true},
		{"https://github.com/owner/repo", "https://github.com/owner/repo.git", true},
		{"https://github.com/owner/repo.git", "https://github.com/other/repo.git", false},
		{"https://github.com/owner/repo.git", "", false},
	}
	for _, tt := range tests {
		got := repoURLsMatch(tt.a, tt.b)
		if got != tt.want {
			t.Errorf("repoURLsMatch(%q, %q) = %v, want %v", tt.a, tt.b, got, tt.want)
		}
	}
}

func TestCheckExistingConflict_SameRepo(t *testing.T) {
	dir := t.TempDir()
	skillDir := filepath.Join(dir, "my-skill")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}
	meta := &SkillMeta{
		Source:  "https://github.com/owner/repo",
		Type:    "github",
		RepoURL: "https://github.com/owner/repo.git",
	}
	if err := WriteMeta(skillDir, meta); err != nil {
		t.Fatal(err)
	}

	err := checkExistingConflict(skillDir, "https://github.com/owner/repo.git", "skillshare install ... --force")
	if !errors.Is(err, ErrSkipSameRepo) {
		t.Errorf("expected ErrSkipSameRepo, got: %v", err)
	}
}

func TestCheckExistingConflict_DifferentRepo(t *testing.T) {
	dir := t.TempDir()
	skillDir := filepath.Join(dir, "my-skill")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}
	meta := &SkillMeta{
		Source:  "https://github.com/other/repo",
		Type:    "github",
		RepoURL: "https://github.com/other/repo.git",
	}
	if err := WriteMeta(skillDir, meta); err != nil {
		t.Fatal(err)
	}

	err := checkExistingConflict(skillDir, "https://github.com/owner/repo.git", "skillshare install ... --force")
	if errors.Is(err, ErrSkipSameRepo) {
		t.Error("should not be ErrSkipSameRepo for different repo")
	}
	if err == nil {
		t.Error("expected error for different repo")
	}
	// Should mention the other repo URL
	if got := err.Error(); !strings.Contains(got, "other/repo") {
		t.Errorf("error should mention existing repo, got: %s", got)
	}
}

func TestCheckExistingConflict_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	skillDir := filepath.Join(dir, "my-skill")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// Empty directory — no meta, no SKILL.md

	err := checkExistingConflict(skillDir, "https://github.com/owner/repo.git", "skillshare install ... --force")
	if err != nil {
		t.Errorf("empty dir should return nil (safe to overwrite), got: %v", err)
	}
}

func TestCheckExistingConflict_HasSkillMdNoMeta(t *testing.T) {
	dir := t.TempDir()
	skillDir := filepath.Join(dir, "my-skill")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// Has SKILL.md but no meta — real skill, unknown origin
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("# test"), 0o644); err != nil {
		t.Fatal(err)
	}

	err := checkExistingConflict(skillDir, "https://github.com/owner/repo.git", "skillshare install ... --force")
	if errors.Is(err, ErrSkipSameRepo) {
		t.Error("should not be ErrSkipSameRepo when meta is absent")
	}
	if err == nil {
		t.Error("expected error when SKILL.md exists but no meta")
	}
}

func TestBuildForceHint(t *testing.T) {
	tests := []struct {
		raw, into, want string
	}{
		{"https://github.com/o/r", "", "skillshare install https://github.com/o/r --force"},
		{"https://github.com/o/r", "frontend/vue", "skillshare install https://github.com/o/r --into frontend/vue --force"},
	}
	for _, tt := range tests {
		got := buildForceHint(tt.raw, tt.into)
		if got != tt.want {
			t.Errorf("buildForceHint(%q, %q) = %q, want %q", tt.raw, tt.into, got, tt.want)
		}
	}
}
