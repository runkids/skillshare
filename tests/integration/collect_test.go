//go:build !online

package integration

import (
	"os"
	"path/filepath"
	"testing"

	"skillshare/internal/testutil"
)

func TestCollect_FindsLocalSkills(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	targetPath := sb.CreateTarget("claude")

	// Create local skill in target (not a symlink)
	localSkillPath := filepath.Join(targetPath, "local-skill")
	os.MkdirAll(localSkillPath, 0755)
	os.WriteFile(filepath.Join(localSkillPath, "SKILL.md"), []byte("# Local Skill"), 0644)

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets:
  claude:
    path: ` + targetPath + `
`)

	// Run with --dry-run to just see what would be collected
	result := sb.RunCLI("collect", "--dry-run")

	result.AssertSuccess(t)
	result.AssertOutputContains(t, "local-skill")
}

func TestCollect_SpecificTarget_OnlyThat(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	claudePath := sb.CreateTarget("claude")
	codexPath := sb.CreateTarget("codex")

	// Create local skills in both targets
	os.MkdirAll(filepath.Join(claudePath, "claude-skill"), 0755)
	os.WriteFile(filepath.Join(claudePath, "claude-skill", "SKILL.md"), []byte("# Claude"), 0644)
	os.MkdirAll(filepath.Join(codexPath, "codex-skill"), 0755)
	os.WriteFile(filepath.Join(codexPath, "codex-skill", "SKILL.md"), []byte("# Codex"), 0644)

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets:
  claude:
    path: ` + claudePath + `
  codex:
    path: ` + codexPath + `
`)

	result := sb.RunCLI("collect", "claude", "--dry-run")

	result.AssertSuccess(t)
	result.AssertOutputContains(t, "claude-skill")
	result.AssertOutputNotContains(t, "codex-skill")
}

func TestCollect_All_FromAllTargets(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	claudePath := sb.CreateTarget("claude")
	codexPath := sb.CreateTarget("codex")

	// Create local skills in both targets
	os.MkdirAll(filepath.Join(claudePath, "claude-skill"), 0755)
	os.WriteFile(filepath.Join(claudePath, "claude-skill", "SKILL.md"), []byte("# Claude"), 0644)
	os.MkdirAll(filepath.Join(codexPath, "codex-skill"), 0755)
	os.WriteFile(filepath.Join(codexPath, "codex-skill", "SKILL.md"), []byte("# Codex"), 0644)

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets:
  claude:
    path: ` + claudePath + `
  codex:
    path: ` + codexPath + `
`)

	result := sb.RunCLI("collect", "--all", "--dry-run")

	result.AssertSuccess(t)
	result.AssertOutputContains(t, "claude-skill")
	result.AssertOutputContains(t, "codex-skill")
}

func TestCollect_NoLocalSkills_ShowsMessage(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.CreateSkill("shared-skill", map[string]string{"SKILL.md": "# Shared"})
	targetPath := sb.CreateTarget("claude")

	// Create only symlinked skill (not local)
	os.Symlink(filepath.Join(sb.SourcePath, "shared-skill"), filepath.Join(targetPath, "shared-skill"))

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets:
  claude:
    path: ` + targetPath + `
`)

	result := sb.RunCLI("collect")

	result.AssertSuccess(t)
	result.AssertOutputContains(t, "No local skills")
}

func TestCollect_TargetNotFound_ReturnsError(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)

	result := sb.RunCLI("collect", "nonexistent")

	result.AssertFailure(t)
	result.AssertAnyOutputContains(t, "not found")
}

func TestCollect_CopiesToSource(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	targetPath := sb.CreateTarget("claude")

	// Create local skill
	localSkillPath := filepath.Join(targetPath, "new-skill")
	os.MkdirAll(localSkillPath, 0755)
	os.WriteFile(filepath.Join(localSkillPath, "SKILL.md"), []byte("# New Skill Content"), 0644)

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets:
  claude:
    path: ` + targetPath + `
`)

	// Run with force to skip confirmation
	result := sb.RunCLIWithInput("y\n", "collect", "--force")

	result.AssertSuccess(t)
	result.AssertOutputContains(t, "copied to source")

	// Verify skill was copied to source
	copiedSkillPath := filepath.Join(sb.SourcePath, "new-skill", "SKILL.md")
	if !sb.FileExists(copiedSkillPath) {
		t.Error("skill should be copied to source")
	}
}

func TestCollect_ExistsInSource_Skips(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	// Create skill in source first
	sb.CreateSkill("existing-skill", map[string]string{"SKILL.md": "# Source Version"})

	targetPath := sb.CreateTarget("claude")

	// Create same skill in target (local copy)
	localSkillPath := filepath.Join(targetPath, "existing-skill")
	os.MkdirAll(localSkillPath, 0755)
	os.WriteFile(filepath.Join(localSkillPath, "SKILL.md"), []byte("# Target Version"), 0644)

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets:
  claude:
    path: ` + targetPath + `
`)

	result := sb.RunCLIWithInput("y\n", "collect")

	result.AssertSuccess(t)
	result.AssertOutputContains(t, "skipped")
}

func TestCollect_MultipleTargets_RequiresAllOrName(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	claudePath := sb.CreateTarget("claude")
	codexPath := sb.CreateTarget("codex")

	// Create local skills in both targets
	os.MkdirAll(filepath.Join(claudePath, "skill1"), 0755)
	os.WriteFile(filepath.Join(claudePath, "skill1", "SKILL.md"), []byte("# 1"), 0644)
	os.MkdirAll(filepath.Join(codexPath, "skill2"), 0755)
	os.WriteFile(filepath.Join(codexPath, "skill2", "SKILL.md"), []byte("# 2"), 0644)

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets:
  claude:
    path: ` + claudePath + `
  codex:
    path: ` + codexPath + `
`)

	// Without specifying target or --all
	result := sb.RunCLI("collect")

	// Should ask to specify target
	result.AssertSuccess(t)
	result.AssertOutputContains(t, "Specify a target")
}
