//go:build !online

package integration

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"skillshare/internal/testutil"
)

func TestPush_NoConfig_ReturnsError(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	os.Remove(sb.ConfigPath)

	result := sb.RunCLI("push")

	result.AssertFailure(t)
	result.AssertAnyOutputContains(t, "config not found")
}

func TestPush_NoGitRepo_ShowsError(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)

	result := sb.RunCLI("push")

	result.AssertSuccess(t) // Command succeeds but shows error message
	result.AssertOutputContains(t, "not a git repository")
}

func TestPush_NoRemote_ShowsError(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)

	// Initialize git but no remote
	cmd := exec.Command("git", "init")
	cmd.Dir = sb.SourcePath
	if err := cmd.Run(); err != nil {
		t.Skip("git not available")
	}

	result := sb.RunCLI("push")

	result.AssertSuccess(t)
	result.AssertOutputContains(t, "No git remote")
}

func TestPush_DryRun_ShowsWhatWouldBePushed(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)

	// Initialize git with remote
	initGitWithRemote(t, sb)

	// Create a skill (uncommitted)
	sb.CreateSkill("new-skill", map[string]string{"SKILL.md": "# New"})

	result := sb.RunCLI("push", "--dry-run")

	result.AssertSuccess(t)
	result.AssertOutputContains(t, "dry-run")
	result.AssertOutputContains(t, "Would stage")
}

func TestPush_NoChanges_ShowsNoChanges(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)

	// Initialize git with remote and commit everything
	initGitWithRemote(t, sb)
	commitAll(t, sb.SourcePath, "initial")

	result := sb.RunCLI("push", "--dry-run")

	result.AssertSuccess(t)
	result.AssertOutputContains(t, "No changes")
}

func TestPush_CustomMessage(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)

	initGitWithRemote(t, sb)
	sb.CreateSkill("test-skill", map[string]string{"SKILL.md": "# Test"})

	result := sb.RunCLI("push", "-m", "Custom commit message", "--dry-run")

	result.AssertSuccess(t)
	result.AssertOutputContains(t, "Custom commit message")
}

func TestPush_ActualPush_ToLocalBareRepo(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)

	// Create bare repo as "remote"
	bareRepo := filepath.Join(sb.Home, "remote.git")
	cmd := exec.Command("git", "init", "--bare", bareRepo)
	if err := cmd.Run(); err != nil {
		t.Skip("git not available")
	}

	// Initialize git and add remote
	cmd = exec.Command("git", "init")
	cmd.Dir = sb.SourcePath
	cmd.Run()

	cmd = exec.Command("git", "remote", "add", "origin", bareRepo)
	cmd.Dir = sb.SourcePath
	cmd.Run()

	configGit(t, sb.SourcePath)

	// Initial commit and push to set up tracking
	cmd = exec.Command("git", "commit", "--allow-empty", "-m", "initial")
	cmd.Dir = sb.SourcePath
	cmd.Run()

	cmd = exec.Command("git", "push", "-u", "origin", "master")
	cmd.Dir = sb.SourcePath
	if err := cmd.Run(); err != nil {
		// Try main branch
		cmd = exec.Command("git", "push", "-u", "origin", "main")
		cmd.Dir = sb.SourcePath
		cmd.Run()
	}

	// Create a skill
	sb.CreateSkill("pushed-skill", map[string]string{"SKILL.md": "# Pushed"})

	result := sb.RunCLI("push", "-m", "Test push")

	result.AssertSuccess(t)
	result.AssertOutputContains(t, "Push complete")
}

// Helper functions

func initGitWithRemote(t *testing.T, sb *testutil.Sandbox) {
	cmd := exec.Command("git", "init")
	cmd.Dir = sb.SourcePath
	if err := cmd.Run(); err != nil {
		t.Skip("git not available")
	}

	// Add a fake remote (won't actually push but passes remote check)
	cmd = exec.Command("git", "remote", "add", "origin", "git@github.com:test/test.git")
	cmd.Dir = sb.SourcePath
	cmd.Run()

	configGit(t, sb.SourcePath)
}

func configGit(t *testing.T, dir string) {
	cmd := exec.Command("git", "config", "user.email", "test@test.com")
	cmd.Dir = dir
	cmd.Run()

	cmd = exec.Command("git", "config", "user.name", "Test")
	cmd.Dir = dir
	cmd.Run()
}

func commitAll(t *testing.T, dir, message string) {
	cmd := exec.Command("git", "add", "-A")
	cmd.Dir = dir
	cmd.Run()

	cmd = exec.Command("git", "commit", "-m", message, "--allow-empty")
	cmd.Dir = dir
	cmd.Run()
}
