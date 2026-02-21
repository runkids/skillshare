//go:build !online

package integration

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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

	testutil.ConfigureGitUser(t, sb.SourcePath)

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

func TestPush_NoUpstream_AutoSetsUpstream(t *testing.T) {
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

	// Initialize git and add remote — but do NOT set upstream tracking
	cmd = exec.Command("git", "init")
	cmd.Dir = sb.SourcePath
	cmd.Run()

	cmd = exec.Command("git", "remote", "add", "origin", bareRepo)
	cmd.Dir = sb.SourcePath
	cmd.Run()

	testutil.ConfigureGitUser(t, sb.SourcePath)

	// Create initial commit (simulates what skillshare init does)
	cmd = exec.Command("git", "commit", "--allow-empty", "-m", "initial")
	cmd.Dir = sb.SourcePath
	cmd.Run()

	// Create a skill (uncommitted)
	sb.CreateSkill("first-push-skill", map[string]string{"SKILL.md": "# First Push"})

	// Push should succeed even without upstream tracking
	result := sb.RunCLI("push", "-m", "first push")

	result.AssertSuccess(t)
	result.AssertOutputContains(t, "Push complete")
}

func TestPush_NoUpstream_RemoteMain_LocalMaster_PushesToMain_NoMasterCreated(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)

	bareRepo := testutil.SetupBareRemoteRepo(t, sb.Home)
	testutil.SeedRemoteBranch(t, sb.Home, bareRepo, "main", map[string]string{
		"README.md": "# main",
	})

	testutil.RunGit(t, sb.SourcePath, "init")
	testutil.RunGit(t, sb.SourcePath, "remote", "add", "origin", bareRepo)
	testutil.ConfigureGitUser(t, sb.SourcePath)
	testutil.RunGit(t, sb.SourcePath, "fetch", "origin")
	testutil.RunGit(t, sb.SourcePath, "checkout", "-b", "master", "origin/main")
	testutil.RunGit(t, sb.SourcePath, "branch", "--unset-upstream")

	sb.CreateSkill("local-skill", map[string]string{"SKILL.md": "# Local"})

	result := sb.RunCLI("push", "-m", "sync to main")
	result.AssertSuccess(t)
	result.AssertOutputContains(t, "Push complete")

	refs := testutil.RunGit(t, bareRepo, "for-each-ref", "--format=%(refname:short)", "refs/heads")
	if !strings.Contains(refs, "main") {
		t.Fatalf("expected remote to contain main branch, got: %s", refs)
	}
	if strings.Contains(refs, "master") {
		t.Fatalf("expected no master branch to be created, got: %s", refs)
	}
}

func TestPush_NoUpstream_RemoteCustomDefault_PushesToRemoteDefault(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)

	bareRepo := testutil.SetupBareRemoteRepo(t, sb.Home)
	testutil.SeedRemoteBranch(t, sb.Home, bareRepo, "trunk", map[string]string{
		"README.md": "# trunk",
	})

	testutil.RunGit(t, sb.SourcePath, "init")
	testutil.RunGit(t, sb.SourcePath, "remote", "add", "origin", bareRepo)
	testutil.ConfigureGitUser(t, sb.SourcePath)
	testutil.RunGit(t, sb.SourcePath, "fetch", "origin")
	testutil.RunGit(t, sb.SourcePath, "checkout", "-b", "feature", "origin/trunk")
	testutil.RunGit(t, sb.SourcePath, "branch", "--unset-upstream")

	sb.CreateSkill("local-skill", map[string]string{"SKILL.md": "# Local"})

	result := sb.RunCLI("push", "-m", "sync to trunk")
	result.AssertSuccess(t)
	result.AssertOutputContains(t, "Push complete")

	upstream := testutil.RunGit(t, sb.SourcePath, "rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{u}")
	if upstream != "origin/trunk" {
		t.Fatalf("expected upstream origin/trunk, got %q", upstream)
	}

	refs := testutil.RunGit(t, bareRepo, "for-each-ref", "--format=%(refname:short)", "refs/heads")
	if !strings.Contains(refs, "trunk") {
		t.Fatalf("expected remote to contain trunk branch, got: %s", refs)
	}
	if strings.Contains(refs, "feature") {
		t.Fatalf("expected no feature branch to be created, got: %s", refs)
	}
}

func TestPush_NoUpstream_RemoteEmpty_UsesLocalBranch(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)

	bareRepo := testutil.SetupBareRemoteRepo(t, sb.Home)

	testutil.RunGit(t, sb.SourcePath, "init")
	testutil.RunGit(t, sb.SourcePath, "remote", "add", "origin", bareRepo)
	testutil.ConfigureGitUser(t, sb.SourcePath)
	testutil.RunGit(t, sb.SourcePath, "commit", "--allow-empty", "-m", "initial")

	localBranch := testutil.RunGit(t, sb.SourcePath, "branch", "--show-current")
	sb.CreateSkill("local-skill", map[string]string{"SKILL.md": "# Local"})

	result := sb.RunCLI("push", "-m", "first push")
	result.AssertSuccess(t)
	result.AssertOutputContains(t, "Push complete")

	refs := testutil.RunGit(t, bareRepo, "for-each-ref", "--format=%(refname:short)", "refs/heads")
	if !strings.Contains(refs, localBranch) {
		t.Fatalf("expected remote to contain local branch %s, got: %s", localBranch, refs)
	}
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

	testutil.ConfigureGitUser(t, sb.SourcePath)
}

func commitAll(t *testing.T, dir, message string) {
	testutil.RunGit(t, dir, "add", "-A")
	testutil.RunGit(t, dir, "commit", "-m", message, "--allow-empty")
}
