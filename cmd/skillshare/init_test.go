package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func envValue(env []string, key string) (string, bool) {
	prefix := key + "="
	for i := len(env) - 1; i >= 0; i-- {
		item := env[i]
		if strings.HasPrefix(item, prefix) {
			return strings.TrimPrefix(item, prefix), true
		}
	}
	return "", false
}

func runGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %v (%s)", args, err, strings.TrimSpace(string(out)))
	}
	return strings.TrimSpace(string(out))
}

func TestCommitSourceFiles_CommitFailureIsReturned(t *testing.T) {
	repo := t.TempDir()

	runGit(t, repo, "init")
	runGit(t, repo, "config", "user.email", "test@example.com")
	runGit(t, repo, "config", "user.name", "Test User")

	hookPath := filepath.Join(repo, ".git", "hooks", "pre-commit")
	if err := os.WriteFile(hookPath, []byte("#!/bin/sh\nexit 1\n"), 0o755); err != nil {
		t.Fatalf("write pre-commit hook: %v", err)
	}

	skillDir := filepath.Join(repo, "demo-skill")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatalf("mkdir skill dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("# Demo"), 0o644); err != nil {
		t.Fatalf("write SKILL.md: %v", err)
	}

	err := commitSourceFiles(repo)
	if err == nil {
		t.Fatal("expected commitSourceFiles to return error when git commit fails")
	}
	if !strings.Contains(err.Error(), "git commit failed") {
		t.Fatalf("expected git commit failure message, got: %v", err)
	}
}

func TestCommitSourceFiles_NoChangesReturnsNil(t *testing.T) {
	repo := t.TempDir()

	runGit(t, repo, "init")
	runGit(t, repo, "config", "user.email", "test@example.com")
	runGit(t, repo, "config", "user.name", "Test User")

	if err := os.WriteFile(filepath.Join(repo, ".gitignore"), []byte(".DS_Store\n"), 0o644); err != nil {
		t.Fatalf("write .gitignore: %v", err)
	}
	runGit(t, repo, "add", ".")
	runGit(t, repo, "commit", "-m", "initial")

	if err := commitSourceFiles(repo); err != nil {
		t.Fatalf("expected nil error when nothing to commit, got: %v", err)
	}
}

func TestRemoteFetchEnv_DisablesInteractivePrompts(t *testing.T) {
	t.Setenv("GIT_SSH_COMMAND", "")
	env := remoteFetchEnv("https://github.com/org/repo.git")

	if v, ok := envValue(env, "GIT_TERMINAL_PROMPT"); !ok || v != "0" {
		t.Fatalf("expected GIT_TERMINAL_PROMPT=0, got %q (present=%v)", v, ok)
	}
	if v, ok := envValue(env, "GIT_ASKPASS"); !ok || v != "" {
		t.Fatalf("expected empty GIT_ASKPASS, got %q (present=%v)", v, ok)
	}
	if v, ok := envValue(env, "SSH_ASKPASS"); !ok || v != "" {
		t.Fatalf("expected empty SSH_ASKPASS, got %q (present=%v)", v, ok)
	}
	if v, ok := envValue(env, "GIT_SSH_COMMAND"); !ok || !strings.Contains(v, "BatchMode=yes") {
		t.Fatalf("expected default GIT_SSH_COMMAND with BatchMode=yes, got %q (present=%v)", v, ok)
	}
}

func TestRemoteFetchEnv_RespectsExistingSSHCommand(t *testing.T) {
	custom := "ssh -i /tmp/custom-key"
	t.Setenv("GIT_SSH_COMMAND", custom)

	env := remoteFetchEnv("https://github.com/org/repo.git")
	if v, ok := envValue(env, "GIT_SSH_COMMAND"); !ok || v != custom {
		t.Fatalf("expected existing GIT_SSH_COMMAND to be preserved, got %q (present=%v)", v, ok)
	}
}
