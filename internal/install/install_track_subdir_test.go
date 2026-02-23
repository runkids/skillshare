package install

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestInstallTrackedRepo_SubdirSparseCheckout(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	if !gitSupportsSparseCheckout() {
		t.Skip("git does not support sparse-checkout")
	}

	tmp := t.TempDir()
	work := filepath.Join(tmp, "work")
	remote := filepath.Join(tmp, "remote.git")
	sourceDir := filepath.Join(tmp, "source")

	mustRunGit(t, "", "init", work)
	mustRunGit(t, work, "config", "user.email", "test@test.com")
	mustRunGit(t, work, "config", "user.name", "Test")

	if err := os.MkdirAll(filepath.Join(work, "skills", "one"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(work, "skills", "two"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(work, "skills", "one", "SKILL.md"), []byte("# one"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(work, "skills", "two", "SKILL.md"), []byte("# two"), 0644); err != nil {
		t.Fatal(err)
	}

	mustRunGit(t, work, "add", ".")
	mustRunGit(t, work, "commit", "-m", "init")
	mustRunGit(t, "", "clone", "--bare", work, remote)

	if err := os.MkdirAll(sourceDir, 0755); err != nil {
		t.Fatal(err)
	}

	source := &Source{
		Type:     SourceTypeGitHTTPS,
		Raw:      "file://" + remote + "/skills/one",
		CloneURL: "file://" + remote,
		Subdir:   "skills/one",
		Name:     "one",
	}

	result, err := InstallTrackedRepo(source, sourceDir, InstallOptions{Name: "track-subdir", Force: true})
	if err != nil {
		t.Fatalf("InstallTrackedRepo() error = %v", err)
	}
	if result.Action != "cloned" {
		t.Fatalf("Action = %q, want %q", result.Action, "cloned")
	}
	if result.SkillCount != 1 {
		t.Fatalf("SkillCount = %d, want 1", result.SkillCount)
	}

	repoPath := filepath.Join(sourceDir, "_track-subdir")
	if _, err := os.Stat(filepath.Join(repoPath, ".git")); err != nil {
		t.Fatalf("expected tracked repo to preserve .git: %v", err)
	}
	if _, err := os.Stat(filepath.Join(repoPath, "skills", "one", "SKILL.md")); err != nil {
		t.Fatalf("expected sparse subdir skill to exist: %v", err)
	}
	if _, err := os.Stat(filepath.Join(repoPath, "skills", "two", "SKILL.md")); err == nil {
		t.Fatalf("expected non-sparse path to be absent")
	}
}

func TestInstallTrackedRepo_SubdirSparseFallback(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	if !gitSupportsSparseCheckout() {
		t.Skip("git does not support sparse-checkout")
	}

	tmp := t.TempDir()
	work := filepath.Join(tmp, "work")
	remote := filepath.Join(tmp, "remote.git")
	sourceDir := filepath.Join(tmp, "source")

	mustRunGit(t, "", "init", work)
	mustRunGit(t, work, "config", "user.email", "test@test.com")
	mustRunGit(t, work, "config", "user.name", "Test")

	if err := os.MkdirAll(filepath.Join(work, "skills", "one"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(work, "skills", "two"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(work, "skills", "one", "SKILL.md"), []byte("# one"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(work, "skills", "two", "SKILL.md"), []byte("# two"), 0644); err != nil {
		t.Fatal(err)
	}

	mustRunGit(t, work, "add", ".")
	mustRunGit(t, work, "commit", "-m", "init")
	mustRunGit(t, "", "clone", "--bare", work, remote)

	if err := os.MkdirAll(sourceDir, 0755); err != nil {
		t.Fatal(err)
	}

	source := &Source{
		Type:     SourceTypeGitHTTPS,
		Raw:      "file://" + remote + "//invalid",
		CloneURL: "file://" + remote,
		Subdir:   "/invalid",
		Name:     "invalid",
	}

	result, err := InstallTrackedRepo(source, sourceDir, InstallOptions{Name: "track-subdir-fallback", Force: true})
	if err != nil {
		t.Fatalf("InstallTrackedRepo() fallback error = %v", err)
	}
	if result.Action != "cloned" {
		t.Fatalf("Action = %q, want %q", result.Action, "cloned")
	}
	if result.SkillCount != 2 {
		t.Fatalf("SkillCount = %d, want 2 (full-clone fallback)", result.SkillCount)
	}

	repoPath := filepath.Join(sourceDir, "_track-subdir-fallback")
	if _, err := os.Stat(filepath.Join(repoPath, ".git")); err != nil {
		t.Fatalf("expected tracked repo to preserve .git: %v", err)
	}
	if _, err := os.Stat(filepath.Join(repoPath, "skills", "one", "SKILL.md")); err != nil {
		t.Fatalf("expected fallback full clone file to exist: %v", err)
	}
	if _, err := os.Stat(filepath.Join(repoPath, "skills", "two", "SKILL.md")); err != nil {
		t.Fatalf("expected fallback full clone file to exist: %v", err)
	}
}
