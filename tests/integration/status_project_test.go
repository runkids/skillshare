//go:build !online

package integration

import (
	"path/filepath"
	"testing"

	"skillshare/internal/testutil"
)

func TestStatusProject_ShowsSyncedTargets(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	projectRoot := sb.SetupProjectDir("claude", "cursor")
	sb.CreateProjectSkill(projectRoot, "skill-a", map[string]string{"SKILL.md": "# A"})

	// Sync first
	sb.RunCLIInDir(projectRoot, "sync", "-p").AssertSuccess(t)

	result := sb.RunCLIInDir(projectRoot, "status", "-p")
	result.AssertSuccess(t)
	result.AssertOutputContains(t, "✓ 1 linked")
}

func TestStatusProject_ShowsUnsynced(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	projectRoot := sb.SetupProjectDir("claude")
	sb.CreateProjectSkill(projectRoot, "unsynced", map[string]string{"SKILL.md": "# U"})

	// Don't sync — should show "has files" (target dir exists but no symlinks)
	result := sb.RunCLIInDir(projectRoot, "status", "-p")
	result.AssertSuccess(t)
	result.AssertOutputContains(t, "has files")
}

func TestStatusProject_ShowsSourceAndTargets(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	projectRoot := sb.SetupProjectDir("claude")

	result := sb.RunCLIInDir(projectRoot, "status", "-p")
	result.AssertSuccess(t)
	result.AssertOutputContains(t, "Source")
	result.AssertOutputContains(t, "Targets")
}

func TestStatusProject_TrackedRepoGitStatusError_ShowsUnknown(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	projectRoot := sb.SetupProjectDir("claude")
	makeTrackedRepoWithBrokenStatus(t, filepath.Join(projectRoot, ".skillshare", "skills"), "_broken-repo")

	result := sb.RunCLIInDir(projectRoot, "status", "-p")
	result.AssertSuccess(t)
	result.AssertRowContains(t, "_broken-repo", "1 skill · failed to check git status")
}
