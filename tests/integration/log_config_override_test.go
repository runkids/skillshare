//go:build !online

package integration

import (
	"path/filepath"
	"testing"

	"skillshare/internal/testutil"
)

// Global commands share the selected config path and label its log as global.
func TestLog_GlobalConfigOverrideUsesStateDir(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.CreateSkill("test-skill", map[string]string{"SKILL.md": "# Test\n\nTest."})
	targetPath := sb.CreateTarget("claude")
	sb.ConfigPath = filepath.Join(sb.Home, "shared", "skillshare", "config.yaml")
	sb.WriteConfig("source: " + sb.SourcePath + "\nmode: merge\ntargets:\n  claude:\n    path: " + targetPath + "\n")

	sb.RunCLI("sync", "-g").AssertSuccess(t)

	list := sb.RunCLI("log", "-g", "--no-tui")
	list.AssertSuccess(t)
	list.AssertOutputContains(t, "sync")
	list.AssertOutputNotContains(t, "· project")
}
