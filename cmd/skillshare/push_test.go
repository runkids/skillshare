package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	gitops "skillshare/internal/git"
)

func TestCommitConfigSafety_LeavesOtherEditsUncommitted(t *testing.T) {
	repo := t.TempDir()
	runGit(t, repo, "init", "-q")
	runGit(t, repo, "config", "user.email", "t@t.com")
	runGit(t, repo, "config", "user.name", "t")
	for name, body := range map[string]string{"config.yaml": "k: v\n", "skill.md": "# a\n"} {
		if err := os.WriteFile(filepath.Join(repo, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	runGit(t, repo, "add", "-A")
	runGit(t, repo, "commit", "-q", "-m", "track config")
	if _, err := gitops.EnsureConfigUntracked(repo); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "skill.md"), []byte("# edited during pull\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := commitConfigSafety(repo); err != nil {
		t.Fatal(err)
	}
	if tree := runGit(t, repo, "ls-tree", "--name-only", "HEAD"); strings.Contains(tree, "config.yaml") || !strings.Contains(tree, ".gitignore") {
		t.Fatalf("HEAD tree = %q; want config.yaml removed and .gitignore committed", tree)
	}
	if status := runGit(t, repo, "status", "--porcelain"); status != "M skill.md" {
		t.Fatalf("status = %q; want only the unrelated edit left uncommitted", status)
	}
}

func TestCommitConfigSafety_RefusesOtherGitignoreEdits(t *testing.T) {
	repo := t.TempDir()
	runGit(t, repo, "init", "-q")
	runGit(t, repo, "config", "user.email", "t@t.com")
	runGit(t, repo, "config", "user.name", "t")
	if err := os.WriteFile(filepath.Join(repo, "config.yaml"), []byte("k: v\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, repo, "add", "-A")
	runGit(t, repo, "commit", "-q", "-m", "track config")
	if _, err := gitops.EnsureConfigUntracked(repo); err != nil {
		t.Fatal(err)
	}
	f, err := os.OpenFile(filepath.Join(repo, ".gitignore"), os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString("node_modules/\n"); err != nil {
		t.Fatal(err)
	}
	f.Close()
	head := runGit(t, repo, "rev-parse", "HEAD")

	if err := commitConfigSafety(repo); err == nil {
		t.Fatal("commitConfigSafety() = nil; want an error for a concurrent .gitignore edit")
	}
	if got := runGit(t, repo, "rev-parse", "HEAD"); got != head {
		t.Fatal("commitConfigSafety() committed despite the concurrent .gitignore edit")
	}
}
