package install

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestIsSkillCurrentAtRepoState_RootSkillCommitMatch(t *testing.T) {
	repo := initTestRepo(t)
	if err := os.WriteFile(filepath.Join(repo, "SKILL.md"), []byte("# Root"), 0644); err != nil {
		t.Fatal(err)
	}
	gitAdd(t, repo, ".")
	gitCommit(t, repo, "add root skill")

	commit, err := getGitCommit(repo)
	if err != nil || commit == "" {
		t.Fatalf("getGitCommit failed: %v, commit=%q", err, commit)
	}

	dest := t.TempDir()
	if err := WriteMeta(dest, &SkillMeta{
		Source:      "https://example.com/repo",
		Type:        "github",
		InstalledAt: time.Now(),
		Version:     commit,
	}); err != nil {
		t.Fatalf("WriteMeta failed: %v", err)
	}

	if !isSkillCurrentAtRepoState(dest, ".", commit, repo, map[string]string{}) {
		t.Fatal("expected root skill to be up-to-date when commit matches")
	}
}

func TestIsSkillCurrentAtRepoState_SubdirTreeHashMatch(t *testing.T) {
	repo := initTestRepo(t)
	subdir := filepath.Join(repo, "skills", "my-skill")
	if err := os.MkdirAll(subdir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(subdir, "SKILL.md"), []byte("# Skill"), 0644); err != nil {
		t.Fatal(err)
	}
	gitAdd(t, repo, ".")
	gitCommit(t, repo, "add subdir skill")

	commit, err := getGitCommit(repo)
	if err != nil || commit == "" {
		t.Fatalf("getGitCommit failed: %v, commit=%q", err, commit)
	}
	tree := getSubdirTreeHash(repo, "skills/my-skill")
	if tree == "" {
		t.Fatal("expected non-empty tree hash")
	}

	dest := t.TempDir()
	if err := WriteMeta(dest, &SkillMeta{
		Source:      "https://example.com/repo/skills/my-skill",
		Type:        "github",
		InstalledAt: time.Now(),
		Version:     commit,
		TreeHash:    tree,
		Subdir:      "skills/my-skill",
	}); err != nil {
		t.Fatalf("WriteMeta failed: %v", err)
	}

	if !isSkillCurrentAtRepoState(dest, "skills/my-skill", commit, repo, map[string]string{}) {
		t.Fatal("expected subdir skill to be up-to-date when commit/tree hash match")
	}
}

func TestIsSkillCurrentAtRepoState_SubdirMissingTreeHashDoesNotSkip(t *testing.T) {
	repo := initTestRepo(t)
	subdir := filepath.Join(repo, "skills", "my-skill")
	if err := os.MkdirAll(subdir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(subdir, "SKILL.md"), []byte("# Skill"), 0644); err != nil {
		t.Fatal(err)
	}
	gitAdd(t, repo, ".")
	gitCommit(t, repo, "add subdir skill")

	commit, err := getGitCommit(repo)
	if err != nil || commit == "" {
		t.Fatalf("getGitCommit failed: %v, commit=%q", err, commit)
	}

	dest := t.TempDir()
	if err := WriteMeta(dest, &SkillMeta{
		Source:      "https://example.com/repo/skills/my-skill",
		Type:        "github",
		InstalledAt: time.Now(),
		Version:     commit,
		Subdir:      "skills/my-skill",
	}); err != nil {
		t.Fatalf("WriteMeta failed: %v", err)
	}

	if isSkillCurrentAtRepoState(dest, "skills/my-skill", commit, repo, map[string]string{}) {
		t.Fatal("expected subdir skill without tree hash to require reinstall")
	}
}

func TestIsSkillCurrentAtRepoState_CommitMismatchDoesNotSkip(t *testing.T) {
	repo := initTestRepo(t)
	if err := os.WriteFile(filepath.Join(repo, "SKILL.md"), []byte("# Root"), 0644); err != nil {
		t.Fatal(err)
	}
	gitAdd(t, repo, ".")
	gitCommit(t, repo, "add root skill")

	commit, err := getGitCommit(repo)
	if err != nil || commit == "" {
		t.Fatalf("getGitCommit failed: %v, commit=%q", err, commit)
	}

	dest := t.TempDir()
	if err := WriteMeta(dest, &SkillMeta{
		Source:      "https://example.com/repo",
		Type:        "github",
		InstalledAt: time.Now(),
		Version:     "deadbeef",
	}); err != nil {
		t.Fatalf("WriteMeta failed: %v", err)
	}

	if isSkillCurrentAtRepoState(dest, ".", commit, repo, map[string]string{}) {
		t.Fatal("expected commit mismatch to require reinstall")
	}
}
