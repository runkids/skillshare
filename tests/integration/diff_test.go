//go:build !online

package integration

import (
	"os"
	"path/filepath"
	"strings"
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
	result.AssertOutputContains(t, "Modified")
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
	result.AssertOutputContains(t, "New")
}

func TestDiff_CopyMode_DetectsDeletedTargetDir(t *testing.T) {
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

	// Sync to establish manifest
	sb.RunCLI("sync").AssertSuccess(t)

	// Manually delete the target skill directory
	os.RemoveAll(filepath.Join(targetPath, "skill-a"))

	// Diff should detect the deleted directory, NOT report "Fully synced"
	result := sb.RunCLI("diff")
	result.AssertSuccess(t)
	result.AssertOutputContains(t, "Restore")
	result.AssertOutputNotContains(t, "Fully synced")
}

func TestDiff_MultiTarget_SameResult_Grouped(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.CreateSkill("skill-a", map[string]string{"SKILL.md": "# A"})
	sb.CreateSkill("skill-b", map[string]string{"SKILL.md": "# B"})
	claudePath := sb.CreateTarget("claude")
	agentsPath := sb.CreateTarget("agents")

	sb.WriteConfig(`source: ` + sb.SourcePath + `
mode: merge
targets:
  agents:
    path: ` + agentsPath + `
  claude:
    path: ` + claudePath + `
`)

	result := sb.RunCLI("diff")
	result.AssertSuccess(t)

	// Both targets should be merged into one header
	result.AssertOutputContains(t, "agents, claude")

	// Items should only appear once (not duplicated)
	out := result.Stdout
	countA := strings.Count(out, "skill-a")
	countB := strings.Count(out, "skill-b")
	if countA != 1 {
		t.Errorf("expected skill-a to appear once, got %d times in:\n%s", countA, out)
	}
	if countB != 1 {
		t.Errorf("expected skill-b to appear once, got %d times in:\n%s", countB, out)
	}
}

func TestDiff_MultiTarget_DifferentResult_Separate(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.CreateSkill("skill-a", map[string]string{"SKILL.md": "# A"})
	sb.CreateSkill("skill-b", map[string]string{"SKILL.md": "# B"})
	claudePath := sb.CreateTarget("claude")
	cursorPath := sb.CreateTarget("cursor")

	// Sync skill-a to cursor only (via symlink) so the diff results differ
	os.Symlink(filepath.Join(sb.SourcePath, "skill-a"), filepath.Join(cursorPath, "skill-a"))

	sb.WriteConfig(`source: ` + sb.SourcePath + `
mode: merge
targets:
  claude:
    path: ` + claudePath + `
  cursor:
    path: ` + cursorPath + `
`)

	result := sb.RunCLI("diff")
	result.AssertSuccess(t)

	// Each target should have its own header (not merged)
	result.AssertOutputContains(t, "claude")
	result.AssertOutputContains(t, "cursor")
	// They should NOT be merged into one line
	result.AssertOutputNotContains(t, "claude, cursor")
	result.AssertOutputNotContains(t, "cursor, claude")
}

func TestDiff_MultiTarget_AllSynced_Merged(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.CreateSkill("skill-a", map[string]string{"SKILL.md": "# A"})
	claudePath := sb.CreateTarget("claude")
	agentsPath := sb.CreateTarget("agents")

	// Sync both targets
	os.Symlink(filepath.Join(sb.SourcePath, "skill-a"), filepath.Join(claudePath, "skill-a"))
	os.Symlink(filepath.Join(sb.SourcePath, "skill-a"), filepath.Join(agentsPath, "skill-a"))

	sb.WriteConfig(`source: ` + sb.SourcePath + `
mode: merge
targets:
  agents:
    path: ` + agentsPath + `
  claude:
    path: ` + claudePath + `
`)

	result := sb.RunCLI("diff")
	result.AssertSuccess(t)

	// Both fully synced targets should be merged into one line
	result.AssertOutputContains(t, "agents, claude: fully synced")
}

func TestDiff_CopyMode_DetectsNonDirectoryTargetEntry(t *testing.T) {
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

	// Sync to establish manifest
	sb.RunCLI("sync").AssertSuccess(t)

	// Replace managed directory with a regular file
	os.RemoveAll(filepath.Join(targetPath, "skill-a"))
	os.WriteFile(filepath.Join(targetPath, "skill-a"), []byte("not-a-dir"), 0644)

	result := sb.RunCLI("diff")
	result.AssertSuccess(t)
	result.AssertOutputContains(t, "not a directory")
	result.AssertOutputNotContains(t, "synced")
}

func TestDiff_CopyMode_ShowsFileStat(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.CreateSkill("skill-a", map[string]string{
		"SKILL.md":  "# Original",
		"prompt.md": "original prompt",
	})
	targetPath := sb.CreateTarget("claude")

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets:
  claude:
    path: ` + targetPath + `
    mode: copy
`)
	sb.RunCLI("sync").AssertSuccess(t)

	// Modify source
	os.WriteFile(filepath.Join(sb.SourcePath, "skill-a", "SKILL.md"), []byte("# Changed"), 0644)

	result := sb.RunCLI("diff", "--stat")
	result.AssertSuccess(t)
	result.AssertOutputContains(t, "SKILL.md")
}

func TestDiff_PatchFlag_ShowsUnifiedDiff(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.CreateSkill("skill-a", map[string]string{
		"SKILL.md": "# Original\nline2\nline3",
	})
	targetPath := sb.CreateTarget("claude")

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets:
  claude:
    path: ` + targetPath + `
    mode: copy
`)
	sb.RunCLI("sync").AssertSuccess(t)

	// Modify source
	os.WriteFile(filepath.Join(sb.SourcePath, "skill-a", "SKILL.md"), []byte("# Modified\nline2\nnew line"), 0644)

	result := sb.RunCLI("diff", "--patch")
	result.AssertSuccess(t)
	result.AssertOutputContains(t, "Modified")
	result.AssertOutputContains(t, "SKILL.md")
}

func TestDiff_NewLabels_ShowCorrectSymbols(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.CreateSkill("new-skill", map[string]string{"SKILL.md": "# New"})
	targetPath := sb.CreateTarget("claude")

	sb.WriteConfig(`source: ` + sb.SourcePath + `
mode: merge
targets:
  claude:
    path: ` + targetPath + `
`)

	result := sb.RunCLI("diff")
	result.AssertSuccess(t)
	result.AssertOutputContains(t, "New")
}

func TestDiff_ShowsSummary(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.CreateSkill("skill-a", map[string]string{"SKILL.md": "# A"})
	claudePath := sb.CreateTarget("claude")
	agentsPath := sb.CreateTarget("agents")

	// Sync agents only
	os.Symlink(filepath.Join(sb.SourcePath, "skill-a"), filepath.Join(agentsPath, "skill-a"))

	sb.WriteConfig(`source: ` + sb.SourcePath + `
mode: merge
targets:
  agents:
    path: ` + agentsPath + `
  claude:
    path: ` + claudePath + `
`)

	result := sb.RunCLI("diff")
	result.AssertSuccess(t)
	result.AssertOutputContains(t, "Summary")
}
