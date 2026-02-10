//go:build !online

package integration

import (
	"os"
	"path/filepath"
	"testing"

	"skillshare/internal/testutil"
)

func TestStatus_ShowsSourceInfo(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.CreateSkill("skill1", map[string]string{"SKILL.md": "# Skill 1"})
	sb.CreateSkill("skill2", map[string]string{"SKILL.md": "# Skill 2"})

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)

	result := sb.RunCLI("status")

	result.AssertSuccess(t)
	result.AssertOutputContains(t, "Source")
	result.AssertOutputContains(t, sb.SourcePath)
	result.AssertOutputContains(t, "2 skills")
}

func TestStatus_ShowsTargetStatus(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.CreateSkill("skill1", map[string]string{"SKILL.md": "# Skill 1"})
	targetPath := sb.CreateTarget("claude")

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets:
  claude:
    path: ` + targetPath + `
`)

	result := sb.RunCLI("status")

	result.AssertSuccess(t)
	result.AssertOutputContains(t, "Targets")
	result.AssertOutputContains(t, "claude")
}

func TestStatus_LinkedTarget_ShowsLinked(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.CreateSkill("skill1", map[string]string{"SKILL.md": "# Skill 1"})

	targetPath := filepath.Join(sb.Home, ".claude", "skills")
	os.MkdirAll(filepath.Dir(targetPath), 0755)
	os.Symlink(sb.SourcePath, targetPath)

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets:
  claude:
    path: ` + targetPath + `
    mode: symlink
`)

	result := sb.RunCLI("status")

	result.AssertSuccess(t)
	result.AssertOutputContains(t, "linked")
}

func TestStatus_MergedTarget_ShowsMerged(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.CreateSkill("skill1", map[string]string{"SKILL.md": "# Skill 1"})
	targetPath := sb.CreateTarget("claude")

	// Create symlink to skill (merge mode)
	skillLink := filepath.Join(targetPath, "skill1")
	os.Symlink(filepath.Join(sb.SourcePath, "skill1"), skillLink)

	sb.WriteConfig(`source: ` + sb.SourcePath + `
mode: merge
targets:
  claude:
    path: ` + targetPath + `
`)

	result := sb.RunCLI("status")

	result.AssertSuccess(t)
	result.AssertOutputContains(t, "merged")
}

func TestStatus_NoConfig_ReturnsError(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	os.Remove(sb.ConfigPath)

	result := sb.RunCLI("status")

	result.AssertFailure(t)
	result.AssertAnyOutputContains(t, "init")
}

func TestStatus_EmptySource_ShowsZeroSkills(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)

	result := sb.RunCLI("status")

	result.AssertSuccess(t)
	result.AssertOutputContains(t, "0 skills")
}
