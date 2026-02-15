//go:build !online

package integration

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"skillshare/internal/testutil"
)

func TestUpdateProject_LocalSkill_Error(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	projectRoot := sb.SetupProjectDir("claude")
	sb.CreateProjectSkill(projectRoot, "local", map[string]string{
		"SKILL.md": "# Local",
	})

	result := sb.RunCLIInDir(projectRoot, "update", "local", "-p")
	result.AssertFailure(t)
	result.AssertAnyOutputContains(t, "local skill")
}

func TestUpdateProject_NotFound_Error(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	projectRoot := sb.SetupProjectDir("claude")

	result := sb.RunCLIInDir(projectRoot, "update", "ghost", "-p")
	result.AssertFailure(t)
	result.AssertAnyOutputContains(t, "not found")
}

func TestUpdateProject_DryRun(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	projectRoot := sb.SetupProjectDir("claude")

	skillDir := sb.CreateProjectSkill(projectRoot, "remote", map[string]string{
		"SKILL.md": "# Remote",
	})
	meta := map[string]interface{}{"source": "/tmp/fake-source", "type": "local"}
	metaJSON, _ := json.Marshal(meta)
	os.WriteFile(filepath.Join(skillDir, ".skillshare-meta.json"), metaJSON, 0644)

	result := sb.RunCLIInDir(projectRoot, "update", "remote", "--dry-run", "-p")
	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, "dry-run")
}

func TestUpdateProject_AllDryRun_SkipsLocal(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	projectRoot := sb.SetupProjectDir("claude")

	// Local (no meta) - should be skipped
	sb.CreateProjectSkill(projectRoot, "local-only", map[string]string{
		"SKILL.md": "# Local Only",
	})

	result := sb.RunCLIInDir(projectRoot, "update", "--all", "--dry-run", "-p")
	result.AssertSuccess(t)
	// Should not contain "local-only" in dry-run output since it has no meta
	result.AssertOutputNotContains(t, "local-only")
}
