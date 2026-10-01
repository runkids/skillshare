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

// Codex and universal share ~/.agents/skills. Removing one must leave the links
// the other still syncs there, not turn them into copies it no longer manages.
func TestTargetRemove_SharedPath_KeepsOtherTargetsSkills(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.CreateSkill("skill1", map[string]string{"SKILL.md": "# Skill 1"})
	shared := sb.CreateTarget("agents")

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets:
  universal:
    path: ` + shared + `
  codex:
    path: ` + shared + `
`)
	sb.RunCLI("sync").AssertSuccess(t)

	preview := sb.RunCLI("target", "remove", "codex", "--dry-run")
	preview.AssertSuccess(t)
	preview.AssertOutputContains(t, "universal uses the same folder")

	sb.RunCLI("target", "remove", "codex").AssertSuccess(t)

	info, err := os.Lstat(filepath.Join(shared, "skill1"))
	if err != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Errorf("universal's link was not kept: %v", err)
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

func TestTargetInfo_CopyMode_ManagedCountMatchesDisk(t *testing.T) {
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

	// Delete one managed skill from disk
	os.RemoveAll(filepath.Join(targetPath, "skill-b"))

	// managed count should reflect actual disk state (1, not 2)
	result := sb.RunCLI("target", "claude")
	result.AssertSuccess(t)
	result.AssertOutputContains(t, "managed: 1")
	result.AssertOutputNotContains(t, "managed: 2")
}

func TestTargetInfo_CopyMode_ManagedCountIgnoresNonDirectory(t *testing.T) {
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

	// Replace one managed skill directory with a regular file
	os.RemoveAll(filepath.Join(targetPath, "skill-b"))
	os.WriteFile(filepath.Join(targetPath, "skill-b"), []byte("not-a-dir"), 0644)

	result := sb.RunCLI("target", "claude")
	result.AssertSuccess(t)
	result.AssertOutputContains(t, "managed: 1")
	result.AssertOutputNotContains(t, "managed: 2")
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

func TestTargetList_NoTUI_FallsBackToPlainText(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	claudePath := sb.CreateTarget("claude")

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets:
  claude:
    path: ` + claudePath + `
`)

	result := sb.RunCLI("target", "list", "--no-tui")

	result.AssertSuccess(t)
	result.AssertOutputContains(t, "claude")
	result.AssertOutputContains(t, "Mode:")
	result.AssertOutputContains(t, "merge")
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

// A second account of an Agent: the target names the Agent and its other config directory,
// and skills land where that Agent looks for them there.
func TestTargetAdd_AgentConfigDir_SyncsIntoThatDirectory(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)
	sb.CreateSkill("hello", map[string]string{"SKILL.md": "---\nname: hello\n---\n# Hello"})
	dir := filepath.Join(sb.Home, ".claude-work")

	result := sb.RunCLI("target", "add", "claude-work", "--agent", "claude", "--config-dir", dir)
	result.AssertSuccess(t)
	result.AssertOutputContains(t, filepath.Join(dir, "skills"))
	if content := sb.ReadFile(sb.ConfigPath); !strings.Contains(content, "agent: claude") || strings.Contains(content, "path:") {
		t.Fatalf("config:\n%s", content)
	}

	sb.RunCLI("sync").AssertSuccess(t)
	if _, err := os.Stat(filepath.Join(dir, "skills", "hello", "SKILL.md")); err != nil {
		t.Fatalf("skill not synced into the config directory: %v", err)
	}

	sb.RunCLI("target", "add", "claude-other", "--agent", "cursor", "--config-dir", filepath.Join(sb.Home, ".x")).AssertFailure(t)
}

// --cli names a compatible executable for the account's plugin commands, such as omo for Pi.
func TestTargetAdd_AgentConfigDir_CLI(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)
	sb.RunCLI("target", "add", "omo", "--agent", "pi", "--config-dir", filepath.Join(sb.Home, ".omo", "agent"), "--cli", "omo").AssertSuccess(t)
	if content := sb.ReadFile(sb.ConfigPath); !strings.Contains(content, "cli: omo") {
		t.Fatalf("config:\n%s", content)
	}

	sb.RunCLI("target", "add", "omo-rel", "--agent", "pi", "--config-dir", filepath.Join(sb.Home, ".omo-rel"), "--cli", "bin/omo").AssertFailure(t)
}
