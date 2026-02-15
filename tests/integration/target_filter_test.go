//go:build !online

package integration

import (
	"path/filepath"
	"strings"
	"testing"

	"skillshare/internal/testutil"
)

func TestTargetFilter_AddInclude(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	targetPath := sb.CreateTarget("claude")
	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets:
  claude:
    path: ` + targetPath + `
`)

	result := sb.RunCLI("target", "claude", "--add-include", "team-*")
	result.AssertSuccess(t)
	result.AssertOutputContains(t, "added include: team-*")
	result.AssertOutputContains(t, "Run 'skillshare sync'")

	// Verify config was updated
	configContent := sb.ReadFile(sb.ConfigPath)
	if !strings.Contains(configContent, "team-*") {
		t.Error("include pattern should be in config")
	}
}

func TestTargetFilter_AddExclude(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	targetPath := sb.CreateTarget("claude")
	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets:
  claude:
    path: ` + targetPath + `
`)

	result := sb.RunCLI("target", "claude", "--add-exclude", "_legacy*")
	result.AssertSuccess(t)
	result.AssertOutputContains(t, "added exclude: _legacy*")

	configContent := sb.ReadFile(sb.ConfigPath)
	if !strings.Contains(configContent, "_legacy*") {
		t.Error("exclude pattern should be in config")
	}
}

func TestTargetFilter_RemoveInclude(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	targetPath := sb.CreateTarget("claude")
	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets:
  claude:
    path: ` + targetPath + `
    include: [team-*, org-*]
`)

	result := sb.RunCLI("target", "claude", "--remove-include", "team-*")
	result.AssertSuccess(t)
	result.AssertOutputContains(t, "removed include: team-*")

	configContent := sb.ReadFile(sb.ConfigPath)
	if strings.Contains(configContent, "team-*") {
		t.Error("team-* should be removed from config")
	}
	if !strings.Contains(configContent, "org-*") {
		t.Error("org-* should still be in config")
	}
}

func TestTargetFilter_RemoveExclude(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	targetPath := sb.CreateTarget("claude")
	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets:
  claude:
    path: ` + targetPath + `
    exclude: [_legacy*, test-*]
`)

	result := sb.RunCLI("target", "claude", "--remove-exclude", "_legacy*")
	result.AssertSuccess(t)
	result.AssertOutputContains(t, "removed exclude: _legacy*")

	configContent := sb.ReadFile(sb.ConfigPath)
	if strings.Contains(configContent, "_legacy*") {
		t.Error("_legacy* should be removed from config")
	}
	if !strings.Contains(configContent, "test-*") {
		t.Error("test-* should still be in config")
	}
}

func TestTargetFilter_InvalidPattern(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	targetPath := sb.CreateTarget("claude")
	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets:
  claude:
    path: ` + targetPath + `
`)

	result := sb.RunCLI("target", "claude", "--add-include", "[invalid")
	result.AssertFailure(t)
	result.AssertAnyOutputContains(t, "invalid")
}

func TestTargetFilter_ShowsInInfo(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	targetPath := sb.CreateTarget("claude")
	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets:
  claude:
    path: ` + targetPath + `
    include: [team-*]
    exclude: [_legacy*]
`)

	result := sb.RunCLI("target", "claude")
	result.AssertSuccess(t)
	result.AssertOutputContains(t, "Include: team-*")
	result.AssertOutputContains(t, "Exclude: _legacy*")
}

func TestTargetFilter_SyncAfterFilter(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.CreateSkill("team-skill", map[string]string{"SKILL.md": "# Team"})
	sb.CreateSkill("other-skill", map[string]string{"SKILL.md": "# Other"})
	targetPath := sb.CreateTarget("claude")

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets:
  claude:
    path: ` + targetPath + `
`)

	// Sync without filter -> both skills
	sb.RunCLI("sync").AssertSuccess(t)
	if !sb.IsSymlink(filepath.Join(targetPath, "team-skill")) {
		t.Error("team-skill should be linked")
	}
	if !sb.IsSymlink(filepath.Join(targetPath, "other-skill")) {
		t.Error("other-skill should be linked")
	}

	// Add include filter via CLI
	sb.RunCLI("target", "claude", "--add-include", "team-*").AssertSuccess(t)

	// Sync again -> only team-skill
	sb.RunCLI("sync").AssertSuccess(t)
	if !sb.IsSymlink(filepath.Join(targetPath, "team-skill")) {
		t.Error("team-skill should still be linked")
	}
	if sb.FileExists(filepath.Join(targetPath, "other-skill")) {
		t.Error("other-skill should be pruned after include filter")
	}
}

func TestTargetFilter_Deduplicate(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	targetPath := sb.CreateTarget("claude")
	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets:
  claude:
    path: ` + targetPath + `
    include: [team-*]
`)

	// Adding same pattern again should be a no-op
	result := sb.RunCLI("target", "claude", "--add-include", "team-*")
	result.AssertSuccess(t)
	// No "added" message since it's a duplicate
	if strings.Contains(result.Stdout, "added") {
		t.Error("should not report adding a duplicate pattern")
	}
}

func TestTargetFilter_HelpShowsFilterFlags(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	result := sb.RunCLI("target", "help")
	result.AssertSuccess(t)
	result.AssertOutputContains(t, "--add-include")
	result.AssertOutputContains(t, "--add-exclude")
	result.AssertOutputContains(t, "--remove-include")
	result.AssertOutputContains(t, "--remove-exclude")
	result.AssertOutputContains(t, "Project mode")
}

func TestTargetFilter_Project_AddAndShow(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	projectRoot := sb.SetupProjectDir("claude")

	result := sb.RunCLIInDir(projectRoot, "target", "claude", "--add-include", "team-*", "-p")
	result.AssertSuccess(t)
	result.AssertOutputContains(t, "added include: team-*")

	// Verify the include shows in info
	info := sb.RunCLIInDir(projectRoot, "target", "claude", "-p")
	info.AssertSuccess(t)
	info.AssertOutputContains(t, "Include: team-*")
}
