//go:build !online

package integration

import (
	"os"
	"testing"

	"skillshare/internal/testutil"
)

func TestCommit_NoConfig_ReturnsError(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	os.Remove(sb.ConfigPath)

	result := sb.RunCLI("commit")

	result.AssertFailure(t)
	result.AssertAnyOutputContains(t, "config not found")
}

func TestCommit_NoGitRepo_ShowsError(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)

	result := sb.RunCLI("commit")

	result.AssertSuccess(t)
	result.AssertOutputContains(t, "not a git repository")
}

func TestCommit_NoRemote_CreatesLocalCommit(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)

	initGitOnly(t, sb)
	sb.CreateSkill("local-skill", map[string]string{"SKILL.md": "# Local"})

	result := sb.RunCLI("commit", "-m", "local checkpoint")

	result.AssertSuccess(t)
	result.AssertOutputContains(t, "Commit complete")

	message := testutil.RunGit(t, sb.SourcePath, "log", "-1", "--pretty=%s")
	if message != "local checkpoint" {
		t.Fatalf("commit message = %q, want %q", message, "local checkpoint")
	}

	status := testutil.RunGit(t, sb.SourcePath, "status", "--porcelain")
	if status != "" {
		t.Fatalf("expected clean working tree, got %q", status)
	}
}

func TestCommit_DryRun_DoesNotCreateCommit(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)

	initGitOnly(t, sb)
	sb.CreateSkill("dry-run-skill", map[string]string{"SKILL.md": "# Dry Run"})

	result := sb.RunCLI("commit", "--dry-run", "-m", "dry run checkpoint")

	result.AssertSuccess(t)
	result.AssertOutputContains(t, "dry-run")
	result.AssertOutputContains(t, "Would commit with message: dry run checkpoint")

	count := testutil.RunGit(t, sb.SourcePath, "rev-list", "--count", "HEAD")
	if count != "1" {
		t.Fatalf("commit count = %q, want 1", count)
	}
}

func TestCommit_NoChanges_ShowsNoChanges(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)

	initGitOnly(t, sb)

	result := sb.RunCLI("commit")

	result.AssertSuccess(t)
	result.AssertOutputContains(t, "No changes to commit")
}

func initGitOnly(t *testing.T, sb *testutil.Sandbox) {
	t.Helper()

	testutil.RunGit(t, sb.SourcePath, "init")
	testutil.ConfigureGitUser(t, sb.SourcePath)
	testutil.RunGit(t, sb.SourcePath, "commit", "--allow-empty", "-m", "initial")
}
