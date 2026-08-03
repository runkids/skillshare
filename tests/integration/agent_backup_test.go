//go:build !online

package integration

import (
	"os"
	"path/filepath"
	"testing"

	"skillshare/internal/testutil"
)

func TestBackup_Agents_CreatesBackup(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	createAgentSource(t, sb, map[string]string{
		"tutor.md": "# Tutor agent",
	})
	claudeAgents := createAgentTarget(t, sb, "claude")

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets:
  claude:
    skills:
      path: ` + sb.CreateTarget("claude") + `
    agents:
      path: ` + claudeAgents + `
`)

	// Sync agents first so there's something to backup
	sb.RunCLI("sync", "agents")

	result := sb.RunCLI("backup", "agents")
	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, "agent backup")
}

func TestBackup_Agents_DryRun(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	createAgentSource(t, sb, map[string]string{
		"tutor.md": "# Tutor agent",
	})
	claudeAgents := createAgentTarget(t, sb, "claude")

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets:
  claude:
    skills:
      path: ` + sb.CreateTarget("claude") + `
    agents:
      path: ` + claudeAgents + `
`)

	sb.RunCLI("sync", "agents")

	result := sb.RunCLI("backup", "agents", "--dry-run")
	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, "Dry run")
}

func TestBackup_Agents_RestoreRoundTrip(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	createAgentSource(t, sb, map[string]string{
		"tutor.md": "# Tutor agent",
	})
	claudeAgents := createAgentTarget(t, sb, "claude")

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets:
  claude:
    skills:
      path: ` + sb.CreateTarget("claude") + `
    agents:
      path: ` + claudeAgents + `
`)

	// A local agent that only exists in the target — this is what a backup
	// protects. Synced agents are symlinks into the source and are recreated
	// by `sync`, so they are deliberately not backed up (issue #252).
	localPath := filepath.Join(claudeAgents, "local-helper.md")
	sb.WriteFile(localPath, "# Local helper")

	// Sync then backup
	sb.RunCLI("sync", "agents")
	sb.RunCLI("backup", "agents")

	// Verify symlink exists
	linkPath := filepath.Join(claudeAgents, "tutor.md")
	if _, err := os.Lstat(linkPath); err != nil {
		t.Fatalf("expected agent symlink at %s", linkPath)
	}

	// Delete the local agent from target
	if err := os.Remove(localPath); err != nil {
		t.Fatalf("remove local agent: %v", err)
	}

	// Restore
	result := sb.RunCLI("restore", "agents", "claude", "--force")
	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, "Restored")

	if _, err := os.Stat(localPath); err != nil {
		t.Fatalf("expected restored agent at %s: %v", localPath, err)
	}
}

func TestBackup_Default_DoesNotBackupAgents(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.CreateSkill("my-skill", map[string]string{
		"SKILL.md": "---\nname: my-skill\n---\n# Content",
	})
	claudeSkills := sb.CreateTarget("claude")

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets:
  claude:
    skills:
      path: ` + claudeSkills + `
`)

	// Default backup should only backup skills, not mention agents
	result := sb.RunCLI("backup")
	result.AssertSuccess(t)
	result.AssertOutputNotContains(t, "agent")
}

func TestBackup_Agents_ProjectMode_CreatesBackup(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	projectDir := setupProjectWithAgents(t, sb)

	// Local agent — synced agents are symlinks and are not backed up (#252),
	// so without local content there is nothing to snapshot.
	sb.WriteFile(
		filepath.Join(projectDir, ".claude", "agents", "local-helper.md"),
		"# Local helper",
	)

	// Sync agents first
	result := sb.RunCLIInDir(projectDir, "sync", "-p", "agents")
	result.AssertSuccess(t)

	// Backup project agents
	result = sb.RunCLIInDir(projectDir, "backup", "-p", "agents")
	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, "agent backup")

	// Verify backup was created under .skillshare/backups/
	backupDir := filepath.Join(projectDir, ".skillshare", "backups")
	entries, err := os.ReadDir(backupDir)
	if err != nil {
		t.Fatalf("expected backup dir at %s: %v", backupDir, err)
	}
	if len(entries) == 0 {
		t.Fatal("expected at least one backup timestamp directory")
	}
}

func TestBackup_Agents_ProjectMode_DryRun(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	projectDir := setupProjectWithAgents(t, sb)
	sb.RunCLIInDir(projectDir, "sync", "-p", "agents")

	result := sb.RunCLIInDir(projectDir, "backup", "-p", "agents", "--dry-run")
	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, "Dry run")

	// Backup dir should NOT exist
	backupDir := filepath.Join(projectDir, ".skillshare", "backups")
	if _, err := os.Stat(backupDir); !os.IsNotExist(err) {
		t.Fatal("backup dir should not exist in dry run mode")
	}
}

func TestBackup_Agents_ProjectMode_RestoreRoundTrip(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	projectDir := setupProjectWithAgents(t, sb)

	// A local agent that only exists in the target — this is what a backup
	// protects. Synced agents are symlinks into the source and are recreated
	// by `sync`, so they are deliberately not backed up (issue #252).
	claudeAgents := filepath.Join(projectDir, ".claude", "agents")
	localPath := filepath.Join(claudeAgents, "local-helper.md")
	sb.WriteFile(localPath, "# Local helper")

	// Sync → backup
	sb.RunCLIInDir(projectDir, "sync", "-p", "agents")
	sb.RunCLIInDir(projectDir, "backup", "-p", "agents")

	// Verify agent symlink exists
	linkPath := filepath.Join(claudeAgents, "tutor.md")
	if _, err := os.Lstat(linkPath); err != nil {
		t.Fatalf("expected agent symlink at %s", linkPath)
	}

	// Delete the local agent from target
	if err := os.Remove(localPath); err != nil {
		t.Fatalf("remove local agent: %v", err)
	}

	// Restore
	result := sb.RunCLIInDir(projectDir, "restore", "-p", "agents", "claude", "--force")
	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, "Restored")

	// Verify the local agent is back
	if _, err := os.Stat(localPath); err != nil {
		t.Fatalf("expected restored agent at %s: %v", localPath, err)
	}
}

func TestRestore_Agents_SkillsProjectModeRejected(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.WriteConfig(`source: ` + sb.SourcePath + "\ntargets: {}\n")

	// Skills restore in project mode should still be rejected
	result := sb.RunCLI("restore", "-p", "claude")
	result.AssertFailure(t)
	result.AssertAnyOutputContains(t, "not supported in project mode")
}
