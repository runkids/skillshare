//go:build !online

package integration

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"skillshare/internal/testutil"
)

// setupPushPullMachine makes sb.SourcePath a clone of a seeded bare remote
// with upstream tracking, as on a second machine after `init --remote`.
func setupPushPullMachine(t *testing.T, sb *testutil.Sandbox, targetPath string) string {
	t.Helper()
	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets:
  claude:
    path: ` + targetPath + `
`)
	bare := testutil.SetupBareRemoteRepo(t, sb.Home)
	testutil.SeedRemoteBranch(t, sb.Home, bare, "main", map[string]string{
		"shared/SKILL.md": "# Shared\n",
	})
	testutil.RunGit(t, sb.SourcePath, "init")
	testutil.RunGit(t, sb.SourcePath, "remote", "add", "origin", bare)
	testutil.ConfigureGitUser(t, sb.SourcePath)
	testutil.RunGit(t, sb.SourcePath, "fetch", "origin")
	testutil.RunGit(t, sb.SourcePath, "checkout", "-B", "main", "--track", "origin/main")
	return bare
}

// pushFromOtherMachine commits files to the remote from a separate clone.
func pushFromOtherMachine(t *testing.T, sb *testutil.Sandbox, bare string, files map[string]string) {
	t.Helper()
	dir := filepath.Join(sb.Home, "other-machine")
	testutil.RunGit(t, "", "clone", bare, dir)
	testutil.ConfigureGitUser(t, dir)
	for rel, content := range files {
		full := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	testutil.RunGit(t, dir, "add", "-A")
	testutil.RunGit(t, dir, "commit", "-m", "other machine")
	testutil.RunGit(t, dir, "push", "origin", "HEAD:main")
}

func TestPushPull_MergesRemoteChangesPushesAndSyncs(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	targetPath := sb.CreateTarget("claude")
	bare := setupPushPullMachine(t, sb, targetPath)
	pushFromOtherMachine(t, sb, bare, map[string]string{"remote-skill/SKILL.md": "# Remote\n"})
	sb.CreateSkill("local-skill", map[string]string{"SKILL.md": "# Local\n"})

	result := sb.RunCLI("push", "--pull", "-m", "Add local skill")

	result.AssertSuccess(t)
	remoteFiles := testutil.RunGit(t, bare, "ls-tree", "-r", "--name-only", "main")
	if !containsLine(remoteFiles, "local-skill/SKILL.md") || !containsLine(remoteFiles, "remote-skill/SKILL.md") {
		t.Fatalf("remote should hold both skills after push --pull, got:\n%s", remoteFiles)
	}
	if !sb.FileExists(filepath.Join(targetPath, "remote-skill", "SKILL.md")) {
		t.Error("pulled skill should be synced to the target")
	}
}

func TestPushPull_ConflictKeepsLocalCommitAndDoesNotPush(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	bare := setupPushPullMachine(t, sb, sb.CreateTarget("claude"))
	pushFromOtherMachine(t, sb, bare, map[string]string{"shared/SKILL.md": "# Shared (remote edit)\n"})
	remoteBefore := testutil.RunGit(t, bare, "rev-parse", "main")
	if err := os.WriteFile(filepath.Join(sb.SourcePath, "shared", "SKILL.md"), []byte("# Shared (local edit)\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	result := sb.RunCLI("push", "--pull", "-m", "Local edit")

	result.AssertFailure(t)
	if got := testutil.RunGit(t, bare, "rev-parse", "main"); got != remoteBefore {
		t.Errorf("remote moved on conflict: %s -> %s", remoteBefore, got)
	}
	if msg := testutil.RunGit(t, sb.SourcePath, "log", "-1", "--format=%s"); msg != "Local edit" {
		t.Errorf("local commit should be kept, HEAD subject = %q", msg)
	}
}

func TestPushPull_EmptyRemotePushesAndSetsUpstream(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)
	bare := testutil.SetupBareRemoteRepo(t, sb.Home)
	initLocalRepoWithRemotePull(t, sb.SourcePath, bare)
	sb.CreateSkill("local-skill", map[string]string{"SKILL.md": "# Local\n"})

	result := sb.RunCLI("push", "--pull")

	result.AssertSuccess(t)
	upstream := testutil.RunGit(t, sb.SourcePath, "rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{u}")
	if upstream == "" {
		t.Error("expected upstream to be set after the first push")
	}
}

func TestPushPull_DryRunChangesNothing(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	bare := setupPushPullMachine(t, sb, sb.CreateTarget("claude"))
	sb.CreateSkill("local-skill", map[string]string{"SKILL.md": "# Local\n"})
	headBefore := testutil.RunGit(t, sb.SourcePath, "rev-parse", "HEAD")
	remoteBefore := testutil.RunGit(t, bare, "rev-parse", "main")

	result := sb.RunCLI("push", "--pull", "--dry-run")

	result.AssertSuccess(t)
	result.AssertRowContains(t, "Pull", "would merge remote changes")
	if testutil.RunGit(t, sb.SourcePath, "rev-parse", "HEAD") != headBefore || testutil.RunGit(t, bare, "rev-parse", "main") != remoteBefore {
		t.Error("dry-run must not commit or push")
	}
}

func TestCommit_RejectsPullFlag(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	setupPushPullMachine(t, sb, sb.CreateTarget("claude"))

	result := sb.RunCLI("commit", "--pull")

	result.AssertFailure(t)
	result.AssertAnyOutputContains(t, "--pull")
}

func containsLine(text, line string) bool {
	for _, l := range strings.Split(text, "\n") {
		if l == line {
			return true
		}
	}
	return false
}
