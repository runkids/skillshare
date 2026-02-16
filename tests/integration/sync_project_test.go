//go:build !online

package integration

import (
	"os"
	"path/filepath"
	"testing"

	"skillshare/internal/testutil"
)

func TestSyncProject_CreatesSymlinks(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	projectRoot := sb.SetupProjectDir("claude")
	sb.CreateProjectSkill(projectRoot, "my-skill", map[string]string{
		"SKILL.md": "# My Skill",
	})

	result := sb.RunCLIInDir(projectRoot, "sync", "-p")
	result.AssertSuccess(t)

	link := filepath.Join(projectRoot, ".claude", "skills", "my-skill")
	if !sb.IsSymlink(link) {
		t.Error("should create symlink")
	}
}

func TestSyncProject_MultipleTargets(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	projectRoot := sb.SetupProjectDir("claude", "cursor")
	sb.CreateProjectSkill(projectRoot, "shared", map[string]string{
		"SKILL.md": "# Shared",
	})

	result := sb.RunCLIInDir(projectRoot, "sync", "-p")
	result.AssertSuccess(t)

	if !sb.IsSymlink(filepath.Join(projectRoot, ".claude", "skills", "shared")) {
		t.Error("symlink in claude target missing")
	}
	if !sb.IsSymlink(filepath.Join(projectRoot, ".cursor", "skills", "shared")) {
		t.Error("symlink in cursor target missing")
	}
}

func TestSyncProject_PreservesLocalSkills(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	projectRoot := sb.SetupProjectDir("claude")
	sb.CreateProjectSkill(projectRoot, "remote-skill", map[string]string{
		"SKILL.md": "# Remote",
	})

	// Place local skill directly in target
	localDir := filepath.Join(projectRoot, ".claude", "skills", "local-only")
	os.MkdirAll(localDir, 0755)
	os.WriteFile(filepath.Join(localDir, "SKILL.md"), []byte("# Local"), 0644)

	result := sb.RunCLIInDir(projectRoot, "sync", "-p")
	result.AssertSuccess(t)

	if sb.IsSymlink(localDir) {
		t.Error("local skill should not become symlink")
	}
	if !sb.FileExists(filepath.Join(localDir, "SKILL.md")) {
		t.Error("local skill should be preserved")
	}
}

func TestSyncProject_PrunesOrphanLinks(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	projectRoot := sb.SetupProjectDir("claude")
	sb.CreateProjectSkill(projectRoot, "skill-a", map[string]string{"SKILL.md": "# A"})
	sb.CreateProjectSkill(projectRoot, "skill-b", map[string]string{"SKILL.md": "# B"})

	// First sync
	sb.RunCLIInDir(projectRoot, "sync", "-p").AssertSuccess(t)

	// Remove skill-b from source
	os.RemoveAll(filepath.Join(projectRoot, ".skillshare", "skills", "skill-b"))

	// Second sync prunes
	result := sb.RunCLIInDir(projectRoot, "sync", "-p")
	result.AssertSuccess(t)

	if sb.FileExists(filepath.Join(projectRoot, ".claude", "skills", "skill-b")) {
		t.Error("skill-b should be pruned")
	}
	if !sb.IsSymlink(filepath.Join(projectRoot, ".claude", "skills", "skill-a")) {
		t.Error("skill-a should remain")
	}
}

func TestSyncProject_DryRun_NoChanges(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	projectRoot := sb.SetupProjectDir("claude")
	sb.CreateProjectSkill(projectRoot, "test", map[string]string{"SKILL.md": "# Test"})

	result := sb.RunCLIInDir(projectRoot, "sync", "-p", "--dry-run")
	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, "Dry run")

	if sb.IsSymlink(filepath.Join(projectRoot, ".claude", "skills", "test")) {
		t.Error("dry-run should not create symlinks")
	}
}

func TestSyncProject_AutoDetectsMode(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	projectRoot := sb.SetupProjectDir("claude")
	sb.CreateProjectSkill(projectRoot, "auto", map[string]string{"SKILL.md": "# Auto"})

	// No -p flag; auto-detects from .skillshare/config.yaml
	result := sb.RunCLIInDir(projectRoot, "sync")
	result.AssertSuccess(t)

	if !sb.IsSymlink(filepath.Join(projectRoot, ".claude", "skills", "auto")) {
		t.Error("auto-detect should trigger project mode sync")
	}
}
