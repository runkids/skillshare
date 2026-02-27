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

func TestIsSkillCurrentAtRepoState_SubdirMissingTreeHashCommitMatchSkips(t *testing.T) {
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

	if !isSkillCurrentAtRepoState(dest, "skills/my-skill", commit, repo, map[string]string{}) {
		t.Fatal("expected subdir skill without tree hash to be up-to-date when commit matches")
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

func TestIsSkillCurrentAtRepoState_SubdirTreeHashMatchSkipsOnCommitMismatch(t *testing.T) {
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

	originalCommit, err := getGitCommit(repo)
	if err != nil || originalCommit == "" {
		t.Fatalf("getGitCommit failed: %v, commit=%q", err, originalCommit)
	}
	tree := getSubdirTreeHash(repo, "skills/my-skill")
	if tree == "" {
		t.Fatal("expected non-empty tree hash")
	}

	// Commit changes outside the skill subdir.
	if err := os.WriteFile(filepath.Join(repo, "README.md"), []byte("updated"), 0644); err != nil {
		t.Fatal(err)
	}
	gitAdd(t, repo, ".")
	gitCommit(t, repo, "update unrelated file")

	newCommit, err := getGitCommit(repo)
	if err != nil || newCommit == "" {
		t.Fatalf("getGitCommit failed: %v, commit=%q", err, newCommit)
	}
	if newCommit == originalCommit {
		t.Fatal("expected commit to change after unrelated update")
	}

	dest := t.TempDir()
	if err := WriteMeta(dest, &SkillMeta{
		Source:      "https://example.com/repo/skills/my-skill",
		Type:        "github",
		InstalledAt: time.Now(),
		Version:     originalCommit,
		TreeHash:    tree,
		Subdir:      "skills/my-skill",
	}); err != nil {
		t.Fatalf("WriteMeta failed: %v", err)
	}

	if !isSkillCurrentAtRepoState(dest, "skills/my-skill", newCommit, repo, map[string]string{}) {
		t.Fatal("expected subdir skill to skip when tree hash matches despite commit mismatch")
	}
}

func TestRefreshSkillMetaVersionIfNeeded(t *testing.T) {
	dest := t.TempDir()
	meta := &SkillMeta{
		Source:      "https://example.com/repo/skills/my-skill",
		Type:        "github",
		InstalledAt: time.Now(),
		Version:     "old-commit",
		TreeHash:    "tree-hash",
		Subdir:      "skills/my-skill",
	}
	if err := WriteMeta(dest, meta); err != nil {
		t.Fatalf("WriteMeta failed: %v", err)
	}

	if err := refreshSkillMetaVersionIfNeeded(dest, "new-commit"); err != nil {
		t.Fatalf("refreshSkillMetaVersionIfNeeded failed: %v", err)
	}

	updated, err := ReadMeta(dest)
	if err != nil {
		t.Fatalf("ReadMeta failed: %v", err)
	}
	if updated == nil {
		t.Fatal("expected metadata to exist")
	}
	if updated.Version != "new-commit" {
		t.Fatalf("expected version to be refreshed, got %q", updated.Version)
	}
	if updated.TreeHash != "tree-hash" {
		t.Fatalf("expected tree hash to be preserved, got %q", updated.TreeHash)
	}
}
