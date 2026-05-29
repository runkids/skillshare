package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func readGitignore(t *testing.T, dir string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(dir, ".gitignore"))
	if err != nil {
		t.Fatalf("read .gitignore: %v", err)
	}
	return string(b)
}

func TestWriteScopeGitignore_FreshRootExcludesConfig(t *testing.T) {
	dir := t.TempDir()
	if err := WriteScopeGitignore(dir, "root"); err != nil {
		t.Fatal(err)
	}
	got := readGitignore(t, dir)
	if !strings.Contains(got, "config.yaml") {
		t.Errorf("root .gitignore must exclude config.yaml, got:\n%s", got)
	}
}

func TestWriteScopeGitignore_FreshNonRootOmitsConfig(t *testing.T) {
	dir := t.TempDir()
	if err := WriteScopeGitignore(dir, "skills"); err != nil {
		t.Fatal(err)
	}
	got := readGitignore(t, dir)
	if strings.Contains(got, "config.yaml") {
		t.Errorf("non-root .gitignore must not mention config.yaml, got:\n%s", got)
	}
}

func TestWriteScopeGitignore_AppendsToExistingAtRoot(t *testing.T) {
	dir := t.TempDir()
	gi := filepath.Join(dir, ".gitignore")
	if err := os.WriteFile(gi, []byte("node_modules/\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := WriteScopeGitignore(dir, "root"); err != nil {
		t.Fatal(err)
	}
	got := readGitignore(t, dir)
	if !strings.Contains(got, "config.yaml") {
		t.Errorf("existing root .gitignore must gain config.yaml, got:\n%s", got)
	}
	if !strings.Contains(got, "node_modules/") {
		t.Errorf("existing entries must be preserved, got:\n%s", got)
	}
}

func TestWriteScopeGitignore_IdempotentAtRoot(t *testing.T) {
	dir := t.TempDir()
	if err := WriteScopeGitignore(dir, "root"); err != nil {
		t.Fatal(err)
	}
	if err := WriteScopeGitignore(dir, "root"); err != nil {
		t.Fatal(err)
	}
	got := readGitignore(t, dir)
	if n := strings.Count(got, "config.yaml"); n != 1 {
		t.Errorf("config.yaml must appear exactly once, got %d:\n%s", n, got)
	}
}

func gitExec(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@test.com",
		"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@test.com",
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func TestNestedRepos_DetectsSubdirGitOnly(t *testing.T) {
	dir := t.TempDir()
	mustMkdir := func(p string) {
		if err := os.MkdirAll(filepath.Join(dir, p), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	mustMkdir(".git")              // the scope repo's own .git — must be ignored
	mustMkdir("skills/.git")       // a nested repo — must be detected
	mustMkdir("old/.git.disabled") // already disabled — must be ignored
	mustMkdir("plain/references")  // a plain skill dir — must be ignored

	got, err := NestedRepos(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0] != "skills" {
		t.Errorf("want [skills], got %v", got)
	}
}

func TestEnsureConfigUntracked_RemovesTrackedConfig(t *testing.T) {
	dir := t.TempDir()
	gitExec(t, dir, "init")
	gitExec(t, dir, "config", "user.email", "t@t.com")
	gitExec(t, dir, "config", "user.name", "t")
	cfgPath := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(cfgPath, []byte("k: v\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitExec(t, dir, "add", "config.yaml")
	gitExec(t, dir, "commit", "-m", "leak config")

	removed, err := EnsureConfigUntracked(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !removed {
		t.Error("expected removed=true for a tracked config.yaml")
	}
	if isTracked(dir, "config.yaml") {
		t.Error("config.yaml must no longer be tracked")
	}
	if _, err := os.Stat(cfgPath); err != nil {
		t.Error("config.yaml file must remain on disk after untracking")
	}
	if gi := readGitignore(t, dir); !strings.Contains(gi, "config.yaml") {
		t.Errorf(".gitignore must contain config.yaml, got:\n%s", gi)
	}
	if removed, _ := EnsureConfigUntracked(dir); removed {
		t.Error("second call must be a no-op (removed=false)")
	}
}

func TestDisableNestedRepo_RenamesAndRefusesClobber(t *testing.T) {
	dir := t.TempDir()
	subGit := filepath.Join(dir, "skills", ".git")
	if err := os.MkdirAll(subGit, 0o755); err != nil {
		t.Fatal(err)
	}

	if err := DisableNestedRepo(dir, "skills"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(subGit); !os.IsNotExist(err) {
		t.Error("skills/.git must be renamed away")
	}
	if _, err := os.Stat(filepath.Join(dir, "skills", ".git.disabled")); err != nil {
		t.Error("skills/.git.disabled must exist after disabling")
	}
	if gi := readGitignore(t, dir); !strings.Contains(gi, ".git.disabled") {
		t.Errorf(".gitignore must contain .git.disabled, got:\n%s", gi)
	}

	// A re-created .git plus an existing .git.disabled must not be clobbered.
	if err := os.MkdirAll(subGit, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := DisableNestedRepo(dir, "skills"); err == nil {
		t.Error("expected an error when .git.disabled already exists")
	}
}

// TestDisableNestedRepo_DropsStaleGitlink covers the already-broken repair case:
// a parent repo that already committed the nested directory as a gitlink (the
// real-world symptom). Disabling must drop the stale gitlink so a subsequent
// `git add -A` re-tracks the directory's files as blobs instead of leaving an
// empty submodule.
func TestDisableNestedRepo_DropsStaleGitlink(t *testing.T) {
	dir := t.TempDir()
	gitExec(t, dir, "init")
	gitExec(t, dir, "config", "user.email", "t@t.com")
	gitExec(t, dir, "config", "user.name", "t")

	// A nested repo with a file, then commit it from the parent — this records
	// "skills" as a gitlink (160000) in the parent index.
	sub := filepath.Join(dir, "skills")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	gitExec(t, sub, "init")
	gitExec(t, sub, "config", "user.email", "t@t.com")
	gitExec(t, sub, "config", "user.name", "t")
	if err := os.WriteFile(filepath.Join(sub, "SKILL.md"), []byte("# skill\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitExec(t, sub, "add", "-A")
	gitExec(t, sub, "commit", "-m", "nested")
	gitExec(t, dir, "add", "skills")
	gitExec(t, dir, "commit", "-m", "record gitlink")

	if !isGitlink(dir, "skills") {
		t.Fatal("setup failed: skills should be a gitlink before disabling")
	}

	if err := DisableNestedRepo(dir, "skills"); err != nil {
		t.Fatal(err)
	}
	if isGitlink(dir, "skills") {
		t.Error("stale gitlink must be dropped from the index after disabling")
	}

	// After re-staging, the directory's files are tracked as blobs.
	gitExec(t, dir, "add", "-A")
	out, err := exec.Command("git", "-C", dir, "ls-files", "skills/SKILL.md").Output()
	if err != nil || strings.TrimSpace(string(out)) == "" {
		t.Errorf("skills/SKILL.md must be tracked as a file after disabling, got %q (err %v)", out, err)
	}
}
