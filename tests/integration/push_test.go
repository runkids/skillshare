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

	configGit(t, sb.SourcePath)

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

	bareRepo := setupBareRemotePush(t, sb)
	seedRemoteBranchPush(t, sb, bareRepo, "main", map[string]string{
		"README.md": "# main",
	})

	runGitPush(t, sb.SourcePath, "init")
	runGitPush(t, sb.SourcePath, "remote", "add", "origin", bareRepo)
	configGit(t, sb.SourcePath)
	runGitPush(t, sb.SourcePath, "fetch", "origin")
	runGitPush(t, sb.SourcePath, "checkout", "-b", "master", "origin/main")
	runGitPush(t, sb.SourcePath, "branch", "--unset-upstream")

	sb.CreateSkill("local-skill", map[string]string{"SKILL.md": "# Local"})

	result := sb.RunCLI("push", "-m", "sync to main")
	result.AssertSuccess(t)
	result.AssertOutputContains(t, "Push complete")

	refs := runGitPush(t, bareRepo, "for-each-ref", "--format=%(refname:short)", "refs/heads")
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

	bareRepo := setupBareRemotePush(t, sb)
	seedRemoteBranchPush(t, sb, bareRepo, "trunk", map[string]string{
		"README.md": "# trunk",
	})

	runGitPush(t, sb.SourcePath, "init")
	runGitPush(t, sb.SourcePath, "remote", "add", "origin", bareRepo)
	configGit(t, sb.SourcePath)
	runGitPush(t, sb.SourcePath, "fetch", "origin")
	runGitPush(t, sb.SourcePath, "checkout", "-b", "feature", "origin/trunk")
	runGitPush(t, sb.SourcePath, "branch", "--unset-upstream")

	sb.CreateSkill("local-skill", map[string]string{"SKILL.md": "# Local"})

	result := sb.RunCLI("push", "-m", "sync to trunk")
	result.AssertSuccess(t)
	result.AssertOutputContains(t, "Push complete")

	upstream := runGitPush(t, sb.SourcePath, "rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{u}")
	if upstream != "origin/trunk" {
		t.Fatalf("expected upstream origin/trunk, got %q", upstream)
	}

	refs := runGitPush(t, bareRepo, "for-each-ref", "--format=%(refname:short)", "refs/heads")
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

	bareRepo := setupBareRemotePush(t, sb)

	runGitPush(t, sb.SourcePath, "init")
	runGitPush(t, sb.SourcePath, "remote", "add", "origin", bareRepo)
	configGit(t, sb.SourcePath)
	runGitPush(t, sb.SourcePath, "commit", "--allow-empty", "-m", "initial")

	localBranch := runGitPush(t, sb.SourcePath, "branch", "--show-current")
	sb.CreateSkill("local-skill", map[string]string{"SKILL.md": "# Local"})

	result := sb.RunCLI("push", "-m", "first push")
	result.AssertSuccess(t)
	result.AssertOutputContains(t, "Push complete")

	refs := runGitPush(t, bareRepo, "for-each-ref", "--format=%(refname:short)", "refs/heads")
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

func runGitPush(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	if dir != "" {
		cmd.Dir = dir
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %v (%s)", args, err, strings.TrimSpace(string(out)))
	}
	return strings.TrimSpace(string(out))
}

func setupBareRemotePush(t *testing.T, sb *testutil.Sandbox) string {
	t.Helper()
	bareRepo := filepath.Join(sb.Home, "remote.git")
	cmd := exec.Command("git", "init", "--bare", bareRepo)
	if err := cmd.Run(); err != nil {
		t.Skip("git not available")
	}
	return bareRepo
}

func seedRemoteBranchPush(t *testing.T, sb *testutil.Sandbox, bareRepo, branch string, files map[string]string) {
	t.Helper()
	seedDir := filepath.Join(sb.Home, "seed-"+branch)
	runGitPush(t, "", "clone", bareRepo, seedDir)
	configGit(t, seedDir)

	for rel, content := range files {
		full := filepath.Join(seedDir, rel)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatalf("failed to create dir for %s: %v", rel, err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatalf("failed to write %s: %v", rel, err)
		}
	}

	runGitPush(t, seedDir, "add", "-A")
	runGitPush(t, seedDir, "commit", "-m", "seed "+branch)
	runGitPush(t, seedDir, "push", "origin", "HEAD:"+branch)
	runGitPush(t, bareRepo, "symbolic-ref", "HEAD", "refs/heads/"+branch)
}
