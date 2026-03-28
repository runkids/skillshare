//go:build !online

package integration

import (
	"encoding/json"
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

func TestCollect_CopyToMergeSwitch_FindsOrphanedCopies(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	targetPath := sb.CreateTarget("claude")

	// Simulate: skill was synced in copy mode (physical dir + manifest entry)
	copiedSkill := filepath.Join(targetPath, "copied-skill")
	os.MkdirAll(copiedSkill, 0755)
	os.WriteFile(filepath.Join(copiedSkill, "SKILL.md"), []byte("# Copied"), 0644)

	writeManifest(t, targetPath, map[string]string{"copied-skill": "abc123"})

	// Config now uses merge mode — orphaned copy should be detected as local
	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets:
  claude:
    path: ` + targetPath + `
    mode: merge
`)

	result := sb.RunCLI("collect", "--dry-run")

	result.AssertSuccess(t)
	result.AssertOutputContains(t, "copied-skill")
}

func TestCollect_GlobalCopyMode_InheritedTarget_SkipsManaged(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	targetPath := sb.CreateTarget("claude")

	// Copy-mode managed skill
	managedSkill := filepath.Join(targetPath, "managed-skill")
	os.MkdirAll(managedSkill, 0755)
	os.WriteFile(filepath.Join(managedSkill, "SKILL.md"), []byte("# Managed"), 0644)

	// Truly local skill
	localSkill := filepath.Join(targetPath, "local-skill")
	os.MkdirAll(localSkill, 0755)
	os.WriteFile(filepath.Join(localSkill, "SKILL.md"), []byte("# Local"), 0644)

	writeManifest(t, targetPath, map[string]string{"managed-skill": "abc123"})

	// Global mode: copy, target omits mode (inherits copy)
	sb.WriteConfig(`source: ` + sb.SourcePath + `
mode: copy
targets:
  claude:
    path: ` + targetPath + `
`)

	result := sb.RunCLI("collect", "--dry-run")

	result.AssertSuccess(t)
	result.AssertOutputContains(t, "local-skill")
	result.AssertOutputNotContains(t, "managed-skill")
}

// writeManifest writes a .skillshare-manifest.json to the target directory.
func writeManifest(t *testing.T, targetPath string, managed map[string]string) {
	t.Helper()
	m := map[string]any{
		"managed":    managed,
		"updated_at": "2026-01-01T00:00:00Z",
	}
	data, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(targetPath, ".skillshare-manifest.json"), data, 0644); err != nil {
		t.Fatal(err)
	}
}
