//go:build !online

package integration

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"skillshare/internal/testutil"
)

func initGitRepo(t *testing.T, repoPath string) {
	t.Helper()

	cmd := exec.Command("git", "init")
	cmd.Dir = repoPath
	if err := cmd.Run(); err != nil {
		t.Skip("git not available, skipping git test")
	}

	for _, c := range [][]string{
		{"config", "user.email", "test@test.com"},
		{"config", "user.name", "Test"},
		{"add", "."},
		{"commit", "-m", "Initial commit"},
	} {
		cmd = exec.Command("git", c...)
		cmd.Dir = repoPath
		cmd.Run()
	}
}

func createMultiSkillGitRepo(t *testing.T, sb *testutil.Sandbox, name string, skills []string) string {
	t.Helper()
	gitRepoPath := filepath.Join(sb.Root, name)
	for _, skill := range skills {
		skillPath := filepath.Join(gitRepoPath, skill)
		os.MkdirAll(skillPath, 0755)
		os.WriteFile(filepath.Join(skillPath, "SKILL.md"), []byte("# "+skill), 0644)
	}

	initGitRepo(t, gitRepoPath)
	return gitRepoPath
}
