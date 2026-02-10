//go:build !online

package integration

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"skillshare/internal/testutil"
)

func TestRestore_NoBackup_ReturnsError(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	targetPath := sb.CreateTarget("claude")

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets:
  claude:
    path: ` + targetPath + `
`)

	result := sb.RunCLI("restore", "claude")

	result.AssertFailure(t)
	result.AssertAnyOutputContains(t, "no backup found")
}

func TestRestore_TargetNotInConfig_ReturnsError(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)

	result := sb.RunCLI("restore", "nonexistent")

	result.AssertFailure(t)
	result.AssertAnyOutputContains(t, "not found")
}

func TestRestore_NoArgs_ReturnsUsage(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)

	result := sb.RunCLI("restore")

	result.AssertFailure(t)
	result.AssertAnyOutputContains(t, "usage")
}

func TestRestore_FromLatest_RestoresBackup(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	targetPath := sb.CreateTarget("claude")

	// Create backup directory manually
	backupDir := filepath.Join(sb.Home, ".config", "skillshare", "backups")
	timestamp := time.Now().Format("20060102-150405")
	backupPath := filepath.Join(backupDir, timestamp, "claude")
	os.MkdirAll(backupPath, 0755)

	// Create a skill in backup
	skillPath := filepath.Join(backupPath, "backed-up-skill")
	os.MkdirAll(skillPath, 0755)
	os.WriteFile(filepath.Join(skillPath, "SKILL.md"), []byte("# Backed Up"), 0644)

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets:
  claude:
    path: ` + targetPath + `
`)

	result := sb.RunCLI("restore", "claude")

	result.AssertSuccess(t)
	result.AssertOutputContains(t, "Restored")

	// Verify skill was restored
	restoredSkillPath := filepath.Join(targetPath, "backed-up-skill", "SKILL.md")
	if !sb.FileExists(restoredSkillPath) {
		t.Error("skill should be restored from backup")
	}
}

func TestRestore_FromTimestamp_RestoresSpecific(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	targetPath := sb.CreateTarget("claude")

	// Create backup directory manually with specific timestamp
	backupDir := filepath.Join(sb.Home, ".config", "skillshare", "backups")
	timestamp := "20240101-120000"
	backupPath := filepath.Join(backupDir, timestamp, "claude")
	os.MkdirAll(backupPath, 0755)

	// Create a skill in backup
	skillPath := filepath.Join(backupPath, "specific-skill")
	os.MkdirAll(skillPath, 0755)
	os.WriteFile(filepath.Join(skillPath, "SKILL.md"), []byte("# Specific"), 0644)

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets:
  claude:
    path: ` + targetPath + `
`)

	result := sb.RunCLI("restore", "claude", "--from", timestamp)

	result.AssertSuccess(t)
	result.AssertOutputContains(t, "Restored")
	result.AssertOutputContains(t, timestamp)
}

func TestRestore_OverwritesSymlink(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	// Create target as symlink
	targetPath := filepath.Join(sb.Home, ".claude", "skills")
	os.MkdirAll(filepath.Dir(targetPath), 0755)
	os.Symlink(sb.SourcePath, targetPath)

	// Create backup
	backupDir := filepath.Join(sb.Home, ".config", "skillshare", "backups")
	timestamp := time.Now().Format("20060102-150405")
	backupPath := filepath.Join(backupDir, timestamp, "claude")
	os.MkdirAll(backupPath, 0755)

	skillPath := filepath.Join(backupPath, "restored-skill")
	os.MkdirAll(skillPath, 0755)
	os.WriteFile(filepath.Join(skillPath, "SKILL.md"), []byte("# Restored"), 0644)

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets:
  claude:
    path: ` + targetPath + `
`)

	result := sb.RunCLI("restore", "claude")

	result.AssertSuccess(t)

	// Verify target is no longer a symlink
	if sb.IsSymlink(targetPath) {
		t.Error("target should no longer be a symlink after restore")
	}
}

func TestRestore_DryRun_DoesNotRestore(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	targetPath := sb.CreateTarget("claude")

	backupDir := filepath.Join(sb.Home, ".config", "skillshare", "backups")
	timestamp := time.Now().Format("20060102-150405")
	backupPath := filepath.Join(backupDir, timestamp, "claude")
	os.MkdirAll(backupPath, 0755)

	skillPath := filepath.Join(backupPath, "dry-run-skill")
	os.MkdirAll(skillPath, 0755)
	os.WriteFile(filepath.Join(skillPath, "SKILL.md"), []byte("# Dry Run"), 0644)

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets:
  claude:
    path: ` + targetPath + `
`)

	result := sb.RunCLI("restore", "claude", "--dry-run")

	result.AssertSuccess(t)
	result.AssertOutputContains(t, "Dry run")

	restoredSkillPath := filepath.Join(targetPath, "dry-run-skill", "SKILL.md")
	if sb.FileExists(restoredSkillPath) {
		t.Error("dry-run should not restore files")
	}
}
