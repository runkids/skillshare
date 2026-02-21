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

// Tests for pull command (git remote operations)

func TestPull_NoGitRepo_ShowsError(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)

	result := sb.RunCLI("pull")

	result.AssertSuccess(t)
	result.AssertOutputContains(t, "not a git repository")
}

func TestPull_NoRemote_ShowsError(t *testing.T) {
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

	result := sb.RunCLI("pull")

	result.AssertSuccess(t)
	result.AssertOutputContains(t, "No git remote")
}

func TestPull_UncommittedChanges_Refuses(t *testing.T) {
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

	// Initial commit and push
	cmd = exec.Command("git", "commit", "--allow-empty", "-m", "initial")
	cmd.Dir = sb.SourcePath
	cmd.Run()

	cmd = exec.Command("git", "push", "-u", "origin", "master")
	cmd.Dir = sb.SourcePath
	if err := cmd.Run(); err != nil {
		cmd = exec.Command("git", "push", "-u", "origin", "main")
		cmd.Dir = sb.SourcePath
		cmd.Run()
	}

	// Create uncommitted changes
	sb.CreateSkill("uncommitted-skill", map[string]string{"SKILL.md": "# Uncommitted"})

	result := sb.RunCLI("pull")

	result.AssertSuccess(t)
	result.AssertOutputContains(t, "Local changes detected")
}

func TestPull_DryRun_ShowsActions(t *testing.T) {
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

	// Initial commit and push
	cmd = exec.Command("git", "commit", "--allow-empty", "-m", "initial")
	cmd.Dir = sb.SourcePath
	cmd.Run()

	cmd = exec.Command("git", "push", "-u", "origin", "master")
	cmd.Dir = sb.SourcePath
	if err := cmd.Run(); err != nil {
		cmd = exec.Command("git", "push", "-u", "origin", "main")
		cmd.Dir = sb.SourcePath
		cmd.Run()
	}

	result := sb.RunCLI("pull", "--dry-run")

	result.AssertSuccess(t)
	result.AssertOutputContains(t, "dry-run")
}

func TestPull_ActualPull_AndSyncs(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	targetPath := sb.CreateTarget("claude")

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets:
  claude:
    path: ` + targetPath + `
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

	// Create skill, commit, and push
	sb.CreateSkill("remote-skill", map[string]string{"SKILL.md": "# Remote Skill"})

	cmd = exec.Command("git", "add", "-A")
	cmd.Dir = sb.SourcePath
	cmd.Run()

	cmd = exec.Command("git", "commit", "-m", "add skill")
	cmd.Dir = sb.SourcePath
	cmd.Run()

	cmd = exec.Command("git", "push", "-u", "origin", "master")
	cmd.Dir = sb.SourcePath
	if err := cmd.Run(); err != nil {
		cmd = exec.Command("git", "push", "-u", "origin", "main")
		cmd.Dir = sb.SourcePath
		cmd.Run()
	}

	// Now run pull (already up to date, but should sync)
	result := sb.RunCLI("pull")

	result.AssertSuccess(t)
	// Should sync to target
	if !sb.FileExists(filepath.Join(targetPath, "remote-skill", "SKILL.md")) {
		t.Error("skill should be synced to target after pull")
	}
}

func TestPull_NoUpstream_AutoSetsUpstream(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	targetPath := sb.CreateTarget("claude")

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets:
  claude:
    path: ` + targetPath + `
`)

	// Create bare repo as "remote"
	bareRepo := filepath.Join(sb.Home, "remote.git")
	cmd := exec.Command("git", "init", "--bare", bareRepo)
	if err := cmd.Run(); err != nil {
		t.Skip("git not available")
	}

	// Initialize source git, commit, and push to establish shared history
	cmd = exec.Command("git", "init")
	cmd.Dir = sb.SourcePath
	cmd.Run()

	cmd = exec.Command("git", "remote", "add", "origin", bareRepo)
	cmd.Dir = sb.SourcePath
	cmd.Run()

	testutil.ConfigureGitUser(t, sb.SourcePath)

	cmd = exec.Command("git", "commit", "--allow-empty", "-m", "initial")
	cmd.Dir = sb.SourcePath
	cmd.Run()

	// Detect local branch name and push with -u to establish initial history
	branchCmd := exec.Command("git", "branch", "--show-current")
	branchCmd.Dir = sb.SourcePath
	branchOut, _ := branchCmd.Output()
	localBranch := strings.TrimSpace(string(branchOut))

	cmd = exec.Command("git", "push", "-u", "origin", localBranch)
	cmd.Dir = sb.SourcePath
	cmd.Run()

	// Contributor clones, adds a skill, and pushes
	contributorDir := filepath.Join(sb.Home, "contributor")
	cmd = exec.Command("git", "clone", bareRepo, contributorDir)
	cmd.Run()

	testutil.ConfigureGitUser(t, contributorDir)

	skillDir := filepath.Join(contributorDir, "remote-skill")
	os.MkdirAll(skillDir, 0o755)
	os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("# Remote Skill"), 0o644)

	cmd = exec.Command("git", "add", "-A")
	cmd.Dir = contributorDir
	cmd.Run()

	cmd = exec.Command("git", "commit", "-m", "add remote skill")
	cmd.Dir = contributorDir
	cmd.Run()

	cmd = exec.Command("git", "push")
	cmd.Dir = contributorDir
	cmd.Run()

	// Remove upstream tracking to simulate the bug scenario (init with empty remote)
	cmd = exec.Command("git", "branch", "--unset-upstream")
	cmd.Dir = sb.SourcePath
	cmd.Run()

	// Pull should succeed even without upstream tracking
	result := sb.RunCLI("pull")

	result.AssertSuccess(t)
	result.AssertOutputContains(t, "Pull complete")
}

func TestPull_FirstPull_BothHaveSkills_NoForce_Fails_NoSync(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	targetPath := sb.CreateTarget("claude")
	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets:
  claude:
    path: ` + targetPath + `
`)

	bareRepo := testutil.SetupBareRemoteRepo(t, sb.Home)
	testutil.SeedRemoteBranch(t, sb.Home, bareRepo, "main", map[string]string{
		"remote-skill/SKILL.md": "# Remote Skill",
	})

	initLocalRepoWithRemotePull(t, sb.SourcePath, bareRepo)
	sb.CreateSkill("local-skill", map[string]string{"SKILL.md": "# Local Skill"})
	testutil.RunGit(t, sb.SourcePath, "add", "-A")
	testutil.RunGit(t, sb.SourcePath, "commit", "-m", "local skill")

	result := sb.RunCLI("pull")
	result.AssertFailure(t)
	result.AssertOutputContains(t, "Remote has skills, but local skills also exist")
	result.AssertOutputNotContains(t, "Pull complete")

	if sb.FileExists(filepath.Join(targetPath, "local-skill", "SKILL.md")) {
		t.Error("sync should not run when pull is blocked")
	}
}

func TestPull_BlockedPath_DoesNotPrintPullComplete(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)

	bareRepo := testutil.SetupBareRemoteRepo(t, sb.Home)
	testutil.SeedRemoteBranch(t, sb.Home, bareRepo, "main", map[string]string{
		"remote-skill/SKILL.md": "# Remote Skill",
	})

	initLocalRepoWithRemotePull(t, sb.SourcePath, bareRepo)
	sb.CreateSkill("local-skill", map[string]string{"SKILL.md": "# Local Skill"})
	testutil.RunGit(t, sb.SourcePath, "add", "-A")
	testutil.RunGit(t, sb.SourcePath, "commit", "-m", "local skill")

	result := sb.RunCLI("pull")
	result.AssertFailure(t)
	result.AssertOutputNotContains(t, "Pull complete")
}

func TestPull_FirstPull_RemoteNoSkills_LocalHasSkills_AutoMergeHistories(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)

	bareRepo := testutil.SetupBareRemoteRepo(t, sb.Home)
	testutil.SeedRemoteBranch(t, sb.Home, bareRepo, "main", map[string]string{
		"README.md": "# Remote Readme",
	})

	initLocalRepoWithRemotePull(t, sb.SourcePath, bareRepo)
	sb.CreateSkill("local-skill", map[string]string{"SKILL.md": "# Local Skill"})
	testutil.RunGit(t, sb.SourcePath, "add", "-A")
	testutil.RunGit(t, sb.SourcePath, "commit", "-m", "local skill")

	result := sb.RunCLI("pull")
	result.AssertSuccess(t)
	result.AssertOutputContains(t, "Pull complete")

	if !sb.FileExists(filepath.Join(sb.SourcePath, "README.md")) {
		t.Error("README from remote should exist after auto-merge")
	}
	if !sb.FileExists(filepath.Join(sb.SourcePath, "local-skill", "SKILL.md")) {
		t.Error("local skill should be preserved after auto-merge")
	}

	pushResult := sb.RunCLI("push", "-m", "after auto-merge")
	pushResult.AssertSuccess(t)
	pushResult.AssertOutputContains(t, "Push complete")
}

func TestPull_FirstPull_RemoteNoSkills_LocalNoSkills_ResetAndTrack(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)

	bareRepo := testutil.SetupBareRemoteRepo(t, sb.Home)
	testutil.SeedRemoteBranch(t, sb.Home, bareRepo, "main", map[string]string{
		"README.md": "# Remote Readme",
	})

	initLocalRepoWithRemotePull(t, sb.SourcePath, bareRepo)
	testutil.RunGit(t, sb.SourcePath, "commit", "--allow-empty", "-m", "initial")

	result := sb.RunCLI("pull")
	result.AssertSuccess(t)
	result.AssertOutputContains(t, "Pull complete")

	if !sb.FileExists(filepath.Join(sb.SourcePath, "README.md")) {
		t.Error("README from remote should exist after reset")
	}

	upstream := testutil.RunGit(t, sb.SourcePath, "rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{u}")
	if upstream != "origin/main" {
		t.Fatalf("expected upstream origin/main, got %q", upstream)
	}
}

func TestPull_FirstPull_Force_OverwritesLocal(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)

	bareRepo := testutil.SetupBareRemoteRepo(t, sb.Home)
	testutil.SeedRemoteBranch(t, sb.Home, bareRepo, "main", map[string]string{
		"remote-skill/SKILL.md": "# Remote Skill",
	})

	initLocalRepoWithRemotePull(t, sb.SourcePath, bareRepo)
	sb.CreateSkill("local-skill", map[string]string{"SKILL.md": "# Local Skill"})
	testutil.RunGit(t, sb.SourcePath, "add", "-A")
	testutil.RunGit(t, sb.SourcePath, "commit", "-m", "local skill")

	result := sb.RunCLI("pull", "--force")
	result.AssertSuccess(t)
	result.AssertOutputContains(t, "Pull complete")

	if !sb.FileExists(filepath.Join(sb.SourcePath, "remote-skill", "SKILL.md")) {
		t.Error("remote skill should exist after force pull")
	}
	if sb.FileExists(filepath.Join(sb.SourcePath, "local-skill", "SKILL.md")) {
		t.Error("local skill should be overwritten by force pull")
	}
}

func TestPull_NoUpstream_RemoteDefaultBranch_CustomName(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)

	bareRepo := testutil.SetupBareRemoteRepo(t, sb.Home)
	testutil.SeedRemoteBranch(t, sb.Home, bareRepo, "trunk", map[string]string{
		"remote-skill/SKILL.md": "# Remote Skill",
	})

	initLocalRepoWithRemotePull(t, sb.SourcePath, bareRepo)
	testutil.RunGit(t, sb.SourcePath, "commit", "--allow-empty", "-m", "initial")

	result := sb.RunCLI("pull")
	result.AssertSuccess(t)
	result.AssertOutputContains(t, "Pull complete")

	if !sb.FileExists(filepath.Join(sb.SourcePath, "remote-skill", "SKILL.md")) {
		t.Error("remote skill should be pulled from custom default branch")
	}

	upstream := testutil.RunGit(t, sb.SourcePath, "rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{u}")
	if upstream != "origin/trunk" {
		t.Fatalf("expected upstream origin/trunk, got %q", upstream)
	}
}

// Helper function for pull tests
func initLocalRepoWithRemotePull(t *testing.T, sourcePath, remote string) {
	t.Helper()
	testutil.RunGit(t, sourcePath, "init")
	testutil.RunGit(t, sourcePath, "remote", "add", "origin", remote)
	testutil.ConfigureGitUser(t, sourcePath)
}
