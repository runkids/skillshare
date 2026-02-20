//go:build !online

package integration

import (
	"os"
	"path/filepath"
	"testing"

	"skillshare/internal/testutil"
)

func TestDiff_InSync_ShowsNoChanges(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.CreateSkill("skill1", map[string]string{"SKILL.md": "# Skill 1"})
	targetPath := sb.CreateTarget("claude")

	// Create symlink to simulate synced state
	os.Symlink(filepath.Join(sb.SourcePath, "skill1"), filepath.Join(targetPath, "skill1"))

	sb.WriteConfig(`source: ` + sb.SourcePath + `
mode: merge
targets:
  claude:
    path: ` + targetPath + `
`)

	result := sb.RunCLI("diff")

	result.AssertSuccess(t)
	result.AssertOutputContains(t, "claude")
}

func TestDiff_SkillOnlyInSource_ShowsDifference(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.CreateSkill("new-skill", map[string]string{"SKILL.md": "# New Skill"})
	targetPath := sb.CreateTarget("claude")
	// Target is empty, skill not synced yet

	sb.WriteConfig(`source: ` + sb.SourcePath + `
mode: merge
targets:
  claude:
    path: ` + targetPath + `
`)

	result := sb.RunCLI("diff")

	result.AssertSuccess(t)
	result.AssertOutputContains(t, "claude")
}

func TestDiff_LocalOnlySkill_ShowsDifference(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	targetPath := sb.CreateTarget("claude")

	// Create local skill in target only
	localSkillPath := filepath.Join(targetPath, "local-skill")
	os.MkdirAll(localSkillPath, 0755)
	os.WriteFile(filepath.Join(localSkillPath, "SKILL.md"), []byte("# Local"), 0644)

	sb.WriteConfig(`source: ` + sb.SourcePath + `
mode: merge
targets:
  claude:
    path: ` + targetPath + `
`)

	result := sb.RunCLI("diff")

	result.AssertSuccess(t)
	result.AssertOutputContains(t, "claude")
}

func TestDiff_SpecificTarget_ShowsOnlyThat(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.CreateSkill("skill1", map[string]string{"SKILL.md": "# Skill 1"})
	claudePath := sb.CreateTarget("claude")
	codexPath := sb.CreateTarget("codex")

	sb.WriteConfig(`source: ` + sb.SourcePath + `
mode: merge
targets:
  claude:
    path: ` + claudePath + `
  codex:
    path: ` + codexPath + `
`)

	result := sb.RunCLI("diff", "--target", "claude")

	result.AssertSuccess(t)
	result.AssertOutputContains(t, "claude")
	result.AssertOutputNotContains(t, "codex")
}

func TestDiff_TargetNotFound_ReturnsError(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)

	result := sb.RunCLI("diff", "--target", "nonexistent")

	result.AssertFailure(t)
	result.AssertAnyOutputContains(t, "not found")
}

func TestDiff_NoConfig_ReturnsError(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	os.Remove(sb.ConfigPath)

	result := sb.RunCLI("diff")

	result.AssertFailure(t)
	result.AssertAnyOutputContains(t, "init")
}

func TestDiff_CopyMode_DetectsContentDrift(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	skillDir := sb.CreateSkill("skill-a", map[string]string{
		"SKILL.md": "# Original",
	})
	targetPath := sb.CreateTarget("claude")

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets:
  claude:
    path: ` + targetPath + `
    mode: copy
`)

	// Sync to establish manifest with checksum
	sb.RunCLI("sync").AssertSuccess(t)

	// Diff should show fully synced
	result := sb.RunCLI("diff")
	result.AssertSuccess(t)
	result.AssertOutputContains(t, "synced")

	// Modify source content
	os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("# Modified"), 0644)

	// Diff should now detect content drift
	result = sb.RunCLI("diff")
	result.AssertSuccess(t)
	result.AssertOutputContains(t, "content changed")
}

func TestDiff_CopyMode_EmptyManifest(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.CreateSkill("skill-a", map[string]string{
		"SKILL.md": "# Skill A",
	})
	targetPath := sb.CreateTarget("claude")

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets:
  claude:
    path: ` + targetPath + `
    mode: copy
`)

	// Run diff WITHOUT syncing first — empty manifest, but mode is copy
	result := sb.RunCLI("diff")
	result.AssertSuccess(t)
	result.AssertOutputContains(t, "missing")
}
