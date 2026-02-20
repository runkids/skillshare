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

func writeProjectMeta(t *testing.T, skillDir string) {
	t.Helper()
	meta := map[string]any{"source": "/tmp/fake-source", "type": "local"}
	data, _ := json.Marshal(meta)
	if err := os.WriteFile(filepath.Join(skillDir, ".skillshare-meta.json"), data, 0644); err != nil {
		t.Fatalf("failed to write meta: %v", err)
	}
}

func TestUpdateProject_MultiNames_DryRun(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	projectRoot := sb.SetupProjectDir("claude")

	d1 := sb.CreateProjectSkill(projectRoot, "skill-a", map[string]string{"SKILL.md": "# A"})
	writeProjectMeta(t, d1)
	d2 := sb.CreateProjectSkill(projectRoot, "skill-b", map[string]string{"SKILL.md": "# B"})
	writeProjectMeta(t, d2)

	result := sb.RunCLIInDir(projectRoot, "update", "skill-a", "skill-b", "--dry-run", "-p")
	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, "skill-a")
	result.AssertAnyOutputContains(t, "skill-b")
	result.AssertAnyOutputContains(t, "dry-run")
}

func TestUpdateProject_Group_DryRun(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	projectRoot := sb.SetupProjectDir("claude")

	// Create group in project skills
	skillsDir := filepath.Join(projectRoot, ".skillshare", "skills")
	groupDir := filepath.Join(skillsDir, "frontend")
	os.MkdirAll(filepath.Join(groupDir, "react"), 0755)
	os.MkdirAll(filepath.Join(groupDir, "vue"), 0755)
	os.WriteFile(filepath.Join(groupDir, "react", "SKILL.md"), []byte("# React"), 0644)
	os.WriteFile(filepath.Join(groupDir, "vue", "SKILL.md"), []byte("# Vue"), 0644)
	writeProjectMeta(t, filepath.Join(groupDir, "react"))
	writeProjectMeta(t, filepath.Join(groupDir, "vue"))

	result := sb.RunCLIInDir(projectRoot, "update", "--group", "frontend", "--dry-run", "-p")
	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, "react")
	result.AssertAnyOutputContains(t, "vue")
}
