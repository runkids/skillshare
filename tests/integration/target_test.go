//go:build !online

package integration

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"skillshare/internal/testutil"
)

func TestTargetAdd_ValidName_AddsTarget(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)

	targetPath := filepath.Join(sb.Home, ".myapp", "skills")
	os.MkdirAll(targetPath, 0755)

	result := sb.RunCLIWithInput("y\n", "target", "add", "myapp", targetPath)

	result.AssertSuccess(t)
	result.AssertOutputContains(t, "Added target")
	result.AssertOutputContains(t, "Run 'skillshare sync'")

	// Verify config was updated
	configContent := sb.ReadFile(sb.ConfigPath)
	if !strings.Contains(configContent, "myapp") {
		t.Error("target should be added to config")
	}
}

func TestTargetAdd_InvalidName_ReturnsError(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)

	targetPath := filepath.Join(sb.Home, ".test", "skills")

	// "add" is a reserved word
	result := sb.RunCLI("target", "add", "add", targetPath)

	result.AssertFailure(t)
	result.AssertAnyOutputContains(t, "invalid")
}

func TestTargetAdd_AlreadyExists_ReturnsError(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	targetPath := sb.CreateTarget("claude")
	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets:
  claude:
    path: ` + targetPath + `
`)

	result := sb.RunCLI("target", "add", "claude", targetPath)

	result.AssertFailure(t)
	result.AssertAnyOutputContains(t, "already exists")
}

func TestTargetAdd_PathNotExists_ReturnsError(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)

	// Path that doesn't exist
	nonExistentPath := filepath.Join(sb.Home, ".nonexistent", "skills")

	result := sb.RunCLI("target", "add", "myapp", nonExistentPath)

	result.AssertFailure(t)
	result.AssertAnyOutputContains(t, "does not exist")
}

func TestTargetRemove_ExistingTarget_Removes(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.CreateSkill("skill1", map[string]string{"SKILL.md": "# Skill 1"})
	targetPath := sb.CreateTarget("claude")

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets:
  claude:
    path: ` + targetPath + `
`)

	result := sb.RunCLI("target", "remove", "claude")

	result.AssertSuccess(t)

	// Verify config was updated
	configContent := sb.ReadFile(sb.ConfigPath)
	if strings.Contains(configContent, "claude") {
		t.Error("target should be removed from config")
	}
}

func TestTargetRemove_DryRun_DoesNotRemove(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	targetPath := sb.CreateTarget("claude")
	localSkillPath := filepath.Join(targetPath, "local-skill")
	os.MkdirAll(localSkillPath, 0755)
	os.WriteFile(filepath.Join(localSkillPath, "SKILL.md"), []byte("# Local"), 0644)

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets:
  claude:
    path: ` + targetPath + `
`)

	result := sb.RunCLI("target", "remove", "claude", "--dry-run")

	result.AssertSuccess(t)
	result.AssertOutputContains(t, "Dry run")

	configContent := sb.ReadFile(sb.ConfigPath)
	if !strings.Contains(configContent, "claude") {
		t.Error("dry-run should not remove target from config")
	}

	if !sb.FileExists(filepath.Join(localSkillPath, "SKILL.md")) {
		t.Error("dry-run should not unlink target contents")
	}
}

func TestTargetRemove_All_RemovesAllTargets(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	claudePath := sb.CreateTarget("claude")
	codexPath := sb.CreateTarget("codex")

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets:
  claude:
    path: ` + claudePath + `
  codex:
    path: ` + codexPath + `
`)

	result := sb.RunCLI("target", "remove", "--all")

	result.AssertSuccess(t)

	// Verify config has no targets
	configContent := sb.ReadFile(sb.ConfigPath)
	if strings.Contains(configContent, "claude") || strings.Contains(configContent, "codex") {
		t.Error("all targets should be removed")
	}
}

func TestTargetRemove_MergeMode_RemovesSymlinks(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.CreateSkill("skill1", map[string]string{"SKILL.md": "# Skill 1"})
	sb.CreateSkill("skill2", map[string]string{"SKILL.md": "# Skill 2"})
	targetPath := sb.CreateTarget("claude")

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets:
  claude:
    path: ` + targetPath + `
`)

	// Sync to create symlinks
	syncResult := sb.RunCLI("sync")
	syncResult.AssertSuccess(t)

	// Verify symlinks exist
	for _, name := range []string{"skill1", "skill2"} {
		link := filepath.Join(targetPath, name)
		info, err := os.Lstat(link)
		if err != nil || info.Mode()&os.ModeSymlink == 0 {
			t.Fatalf("expected symlink for %s after sync", name)
		}
	}

	// Remove target
	result := sb.RunCLI("target", "remove", "claude")
	result.AssertSuccess(t)

	// Verify symlinks are gone (replaced with real copies or removed)
	for _, name := range []string{"skill1", "skill2"} {
		link := filepath.Join(targetPath, name)
		info, err := os.Lstat(link)
		if err != nil {
			continue // removed entirely is also OK
		}
		if info.Mode()&os.ModeSymlink != 0 {
			t.Errorf("symlink for %s should be removed after target remove", name)
		}
	}

	// Verify config updated
	configContent := sb.ReadFile(sb.ConfigPath)
	if strings.Contains(configContent, "claude") {
		t.Error("target should be removed from config")
	}
}

func TestTargetRemove_MergeMode_PreservesLocalSkills(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.CreateSkill("managed", map[string]string{"SKILL.md": "# Managed"})
	targetPath := sb.CreateTarget("claude")

	// Add a local (non-symlink) skill in the target
	localSkillPath := filepath.Join(targetPath, "local-skill")
	os.MkdirAll(localSkillPath, 0755)
	os.WriteFile(filepath.Join(localSkillPath, "SKILL.md"), []byte("# Local"), 0644)

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets:
  claude:
    path: ` + targetPath + `
`)

	// Sync to create managed symlink
	syncResult := sb.RunCLI("sync")
	syncResult.AssertSuccess(t)

	// Remove target
	result := sb.RunCLI("target", "remove", "claude")
	result.AssertSuccess(t)

	// Local skill should still exist
	if !sb.FileExists(filepath.Join(localSkillPath, "SKILL.md")) {
		t.Error("local (non-symlink) skill should be preserved after target remove")
	}
}

func TestTargetRemove_NotFound_ReturnsError(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)

	result := sb.RunCLI("target", "remove", "nonexistent")

	result.AssertFailure(t)
	result.AssertAnyOutputContains(t, "not found")
}

func TestTargetList_ShowsAllTargets(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	claudePath := sb.CreateTarget("claude")
	codexPath := sb.CreateTarget("codex")

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets:
  claude:
    path: ` + claudePath + `
  codex:
    path: ` + codexPath + `
`)

	result := sb.RunCLI("target", "list")

	result.AssertSuccess(t)
	result.AssertOutputContains(t, "claude")
	result.AssertOutputContains(t, "codex")
}

func TestTargetInfo_ShowsDetails(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	targetPath := sb.CreateTarget("claude")

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets:
  claude:
    path: ` + targetPath + `
`)

	result := sb.RunCLI("target", "claude")

	result.AssertSuccess(t)
	result.AssertOutputContains(t, "claude")
	result.AssertOutputContains(t, "Path:")
	result.AssertOutputContains(t, "Mode:")
}

func TestTargetInfo_CopyMode_ShowsManagedCount(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.CreateSkill("skill-a", map[string]string{"SKILL.md": "# A"})
	sb.CreateSkill("skill-b", map[string]string{"SKILL.md": "# B"})
	targetPath := sb.CreateTarget("claude")

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets:
  claude:
    path: ` + targetPath + `
    mode: copy
`)

	sb.RunCLI("sync").AssertSuccess(t)

	result := sb.RunCLI("target", "claude")
	result.AssertSuccess(t)
	result.AssertOutputContains(t, "copy")
	result.AssertOutputContains(t, "managed: 2")
}

func TestTargetMode_SetsMode(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	targetPath := sb.CreateTarget("claude")

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets:
  claude:
    path: ` + targetPath + `
`)

	result := sb.RunCLI("target", "claude", "--mode", "symlink")

	result.AssertSuccess(t)
	result.AssertOutputContains(t, "Changed")
	result.AssertOutputContains(t, "symlink")

	// Verify config was updated
	configContent := sb.ReadFile(sb.ConfigPath)
	if !strings.Contains(configContent, "symlink") {
		t.Error("mode should be updated in config")
	}
}

func TestTargetMode_InvalidMode_ReturnsError(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	targetPath := sb.CreateTarget("claude")

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets:
  claude:
    path: ` + targetPath + `
`)

	result := sb.RunCLI("target", "claude", "--mode", "invalid")

	result.AssertFailure(t)
	result.AssertAnyOutputContains(t, "invalid mode")
}

func TestTarget_NoSubcommand_ShowsUsage(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)

	result := sb.RunCLI("target")

	result.AssertFailure(t)
	result.AssertAnyOutputContains(t, "usage")
}
