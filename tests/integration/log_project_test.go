//go:build !online

package integration

import (
	"testing"

	"skillshare/internal/testutil"
)

func TestLogProject_ShowsEmpty(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	projectRoot := sb.SetupProjectDir("claude")

	result := sb.RunCLIInDir(projectRoot, "log", "-p")
	result.AssertSuccess(t)
	result.AssertOutputContains(t, "No operation")
}

func TestLogProject_AfterSync(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	projectRoot := sb.SetupProjectDir("claude")

	// Create a skill so sync has something to do
	sb.CreateProjectSkill(projectRoot, "log-test-skill", map[string]string{
		"SKILL.md": "# Log Test Skill",
	})

	// Run sync first to generate a log entry
	syncResult := sb.RunCLIInDir(projectRoot, "sync", "-p")
	syncResult.AssertSuccess(t)

	// Check log shows the sync entry
	result := sb.RunCLIInDir(projectRoot, "log", "-p")
	result.AssertSuccess(t)
	result.AssertOutputContains(t, "sync")
}
