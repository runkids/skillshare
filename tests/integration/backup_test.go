//go:build !online

package integration

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"skillshare/internal/testutil"
)

func TestBackup_AfterSync_SkipsSymlinks(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.CreateSkill("agent-browser", map[string]string{"SKILL.md": "# Agent Browser"})
	targetPath := sb.CreateTarget("claude")

	// Create a local skill that should be preserved in backup
	localSkillPath := filepath.Join(targetPath, "my-local")
	os.MkdirAll(localSkillPath, 0755)
	os.WriteFile(filepath.Join(localSkillPath, "SKILL.md"), []byte("# Local"), 0644)

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets:
  claude:
    path: ` + targetPath + `
`)

	// Sync first — creates symlink for agent-browser in target
	syncResult := sb.RunCLI("sync")
	syncResult.AssertSuccess(t)

	// Verify symlink was created
	agentPath := filepath.Join(targetPath, "agent-browser")
	info, err := os.Lstat(agentPath)
	if err != nil {
		t.Fatalf("agent-browser should exist after sync: %v", err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Fatal("agent-browser should be a symlink after sync")
	}

	// Backup should succeed despite symlinks in target
	backupResult := sb.RunCLI("backup")
	backupResult.AssertSuccess(t)

	// Verify backup contains local skill but not the symlinked one
	backupDir := filepath.Join(sb.Home, ".local", "share", "skillshare", "backups")
	entries, err := os.ReadDir(backupDir)
	if err != nil || len(entries) == 0 {
		t.Fatal("backup directory should contain a timestamp directory")
	}

	backupPath := filepath.Join(backupDir, entries[0].Name(), "claude")
	if _, err := os.Stat(filepath.Join(backupPath, "my-local", "SKILL.md")); err != nil {
		t.Error("local skill should be in backup")
	}
	if _, err := os.Lstat(filepath.Join(backupPath, "agent-browser")); !os.IsNotExist(err) {
		t.Error("symlinked skill should NOT be in backup")
	}
}

func TestBackup_CreatesBackup(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.CreateSkill("skill1", map[string]string{"SKILL.md": "# Skill 1"})
	targetPath := sb.CreateTarget("claude")

	// Create some files in target to backup
	localSkillPath := filepath.Join(targetPath, "local-skill")
	os.MkdirAll(localSkillPath, 0755)
	os.WriteFile(filepath.Join(localSkillPath, "SKILL.md"), []byte("# Local"), 0644)

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets:
  claude:
    path: ` + targetPath + `
`)

	result := sb.RunCLI("backup")

	result.AssertSuccess(t)
	result.AssertOutputContains(t, "claude")
}

func TestBackup_SpecificTarget_BackupsOnlyThat(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	claudePath := sb.CreateTarget("claude")
	codexPath := sb.CreateTarget("codex")

	// Create files in both targets
	os.MkdirAll(filepath.Join(claudePath, "skill"), 0755)
	os.WriteFile(filepath.Join(claudePath, "skill", "SKILL.md"), []byte("# Claude Skill"), 0644)
	os.MkdirAll(filepath.Join(codexPath, "skill"), 0755)
	os.WriteFile(filepath.Join(codexPath, "skill", "SKILL.md"), []byte("# Codex Skill"), 0644)

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets:
  claude:
    path: ` + claudePath + `
  codex:
    path: ` + codexPath + `
`)

	result := sb.RunCLI("backup", "--target", "claude")

	result.AssertSuccess(t)
	result.AssertOutputContains(t, "claude")
}

func TestBackup_EmptyTarget_ShowsNothing(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	targetPath := sb.CreateTarget("claude")
	// Target is empty

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets:
  claude:
    path: ` + targetPath + `
`)

	result := sb.RunCLI("backup")

	result.AssertSuccess(t)
	result.AssertOutputContains(t, "nothing to backup")
}

func TestBackup_SymlinkTarget_ShowsNothing(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.CreateSkill("skill1", map[string]string{"SKILL.md": "# Skill 1"})

	// Create target as symlink to source
	targetPath := filepath.Join(sb.Home, ".claude", "skills")
	os.MkdirAll(filepath.Dir(targetPath), 0755)
	os.Symlink(sb.SourcePath, targetPath)

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets:
  claude:
    path: ` + targetPath + `
    mode: symlink
`)

	result := sb.RunCLI("backup")

	result.AssertSuccess(t)
	result.AssertOutputContains(t, "nothing to backup")
}

func TestBackup_List_ShowsAllBackups(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)

	result := sb.RunCLI("backup", "--list")

	result.AssertSuccess(t)
	// May show "No backups found" if none exist
}

func TestBackup_List_Empty_ShowsNone(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)

	result := sb.RunCLI("backup", "--list")

	result.AssertSuccess(t)
	result.AssertOutputContains(t, "No backups")
}

func TestBackup_TargetNotFound_ReturnsError(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)

	result := sb.RunCLI("backup", "--target", "nonexistent")

	result.AssertFailure(t)
	result.AssertAnyOutputContains(t, "not found")
}

func TestBackup_Cleanup_Works(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)

	result := sb.RunCLI("backup", "--cleanup")

	result.AssertSuccess(t)
	result.AssertOutputContains(t, "Cleaning")
}

func TestBackup_DryRun_DoesNotCreateBackup(t *testing.T) {
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

	result := sb.RunCLI("backup", "--dry-run")

	result.AssertSuccess(t)
	result.AssertOutputContains(t, "Dry run")

	backupDir := filepath.Join(sb.Home, ".local", "share", "skillshare", "backups")
	entries, err := os.ReadDir(backupDir)
	if err != nil {
		t.Fatalf("failed to read backup dir: %v", err)
	}
	if len(entries) != 0 {
		t.Error("dry-run should not create backups")
	}
}

func TestBackup_CleanupDryRun_DoesNotRemoveBackups(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)

	backupRoot := filepath.Join(sb.Home, ".local", "share", "skillshare", "backups")
	timestampDir := filepath.Join(backupRoot, "2024-01-01_00-00-00")
	backupPath := filepath.Join(timestampDir, "claude", "old-skill")
	os.MkdirAll(backupPath, 0755)
	os.WriteFile(filepath.Join(backupPath, "SKILL.md"), []byte("# Old"), 0644)

	oldTime := time.Now().Add(-60 * 24 * time.Hour)
	os.Chtimes(timestampDir, oldTime, oldTime)

	result := sb.RunCLI("backup", "--cleanup", "--dry-run")

	result.AssertSuccess(t)
	result.AssertOutputContains(t, "Dry run")

	if !sb.FileExists(timestampDir) {
		t.Error("dry-run should not remove backups")
	}
}
