//go:build !online

package integration

import (
	"testing"

	"skillshare/internal/testutil"
)

func TestStatusProject_ShowsSyncedTargets(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	projectRoot := sb.SetupProjectDir("claude-code", "cursor")
	sb.CreateProjectSkill(projectRoot, "skill-a", map[string]string{"SKILL.md": "# A"})

	// Sync first
	sb.RunCLIInDir(projectRoot, "sync", "-p").AssertSuccess(t)

	result := sb.RunCLIInDir(projectRoot, "status", "-p")
	result.AssertSuccess(t)
	result.AssertOutputContains(t, "merged")
	result.AssertOutputContains(t, "1 shared")
}

func TestStatusProject_ShowsUnsynced(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	projectRoot := sb.SetupProjectDir("claude-code")
	sb.CreateProjectSkill(projectRoot, "unsynced", map[string]string{"SKILL.md": "# U"})

	// Don't sync — should show "has files" (target dir exists but no symlinks)
	result := sb.RunCLIInDir(projectRoot, "status", "-p")
	result.AssertSuccess(t)
	result.AssertOutputContains(t, "has files")
}

func TestStatusProject_ShowsSourceAndTargets(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	projectRoot := sb.SetupProjectDir("claude-code")

	result := sb.RunCLIInDir(projectRoot, "status", "-p")
	result.AssertSuccess(t)
	result.AssertOutputContains(t, "Source")
	result.AssertOutputContains(t, "Targets")
}
