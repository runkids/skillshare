//go:build !online

package integration

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"skillshare/internal/sync"
	"skillshare/internal/testutil"
)

func TestTargetProject_AddKnownTarget(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	projectRoot := sb.SetupProjectDir("claude")

	result := sb.RunCLIInDir(projectRoot, "target", "add", "cursor", "-p")
	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, "Added target")

	cfg := sb.ReadFile(filepath.Join(projectRoot, ".skillshare", "config.yaml"))
	if !strings.Contains(cfg, "cursor") {
		t.Error("config should contain cursor")
	}
}

func TestTargetProject_AddCustomTarget(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	projectRoot := sb.SetupProjectDir("claude")

	result := sb.RunCLIInDir(projectRoot, "target", "add", "my-ide", ".my-ide/skills/", "-p")
	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, "Added target")
}

func TestTargetProject_AddDuplicate_Error(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	projectRoot := sb.SetupProjectDir("claude")

	result := sb.RunCLIInDir(projectRoot, "target", "add", "claude", "-p")
	result.AssertFailure(t)
	result.AssertAnyOutputContains(t, "already exists")
}

func TestTargetProject_RemoveTarget(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	projectRoot := sb.SetupProjectDir("claude", "cursor")

	result := sb.RunCLIInDir(projectRoot, "target", "remove", "cursor", "-p")
	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, "Removed target")

	cfg := sb.ReadFile(filepath.Join(projectRoot, ".skillshare", "config.yaml"))
	if strings.Contains(cfg, "cursor") {
		t.Error("cursor should be removed from config")
	}
}

func TestTargetProject_RemoveAll(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	projectRoot := sb.SetupProjectDir("claude", "cursor")

	result := sb.RunCLIInDir(projectRoot, "target", "remove", "--all", "-p")
	result.AssertSuccess(t)
}

func TestTargetProject_List(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	projectRoot := sb.SetupProjectDir("claude", "cursor")

	result := sb.RunCLIInDir(projectRoot, "target", "list", "-p")
	result.AssertSuccess(t)
	result.AssertOutputContains(t, "claude")
	result.AssertOutputContains(t, "cursor")
	result.AssertOutputContains(t, "merge")
}

func TestTargetProject_Info(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	projectRoot := sb.SetupProjectDir("claude")

	result := sb.RunCLIInDir(projectRoot, "target", "claude", "-p")
	result.AssertSuccess(t)
	result.AssertOutputContains(t, "claude")
	result.AssertOutputContains(t, "merge")
}

func TestTargetProject_SetMode(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	projectRoot := sb.SetupProjectDir("claude")

	result := sb.RunCLIInDir(projectRoot, "target", "claude", "--mode", "symlink", "-p")
	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, "Changed claude mode")

	cfg := sb.ReadFile(filepath.Join(projectRoot, ".skillshare", "config.yaml"))
	if !strings.Contains(cfg, "symlink") {
		t.Error("config should contain symlink mode")
	}

	// Verify info shows symlink mode
	info := sb.RunCLIInDir(projectRoot, "target", "claude", "-p")
	info.AssertSuccess(t)
	info.AssertOutputContains(t, "symlink")
}

func TestTargetProject_SetMode_InvalidMode(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	projectRoot := sb.SetupProjectDir("claude")

	result := sb.RunCLIInDir(projectRoot, "target", "claude", "--mode", "invalid", "-p")
	result.AssertFailure(t)
	result.AssertAnyOutputContains(t, "invalid mode")
}

func TestTargetProject_ListShowsMode(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	projectRoot := sb.SetupProjectDir("claude")

	// Set mode to symlink
	sb.RunCLIInDir(projectRoot, "target", "claude", "--mode", "symlink", "-p")

	result := sb.RunCLIInDir(projectRoot, "target", "list", "-p")
	result.AssertSuccess(t)
	result.AssertOutputContains(t, "symlink")
}

func TestTargetProject_SyncSymlinkMode(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	projectRoot := sb.SetupProjectDir("claude")

	// Create a skill
	sb.CreateProjectSkill(projectRoot, "test-skill", map[string]string{
		"SKILL.md": "---\nname: test-skill\n---\n# Test",
	})

	// Set mode to symlink
	sb.RunCLIInDir(projectRoot, "target", "claude", "--mode", "symlink", "-p")

	// Sync
	result := sb.RunCLIInDir(projectRoot, "sync", "-p")
	result.AssertSuccess(t)

	// Verify the target is a symlink to .skillshare/skills
	targetPath := filepath.Join(projectRoot, ".claude", "skills")
	info, err := os.Lstat(targetPath)
	if err != nil {
		t.Fatalf("target path should exist: %v", err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Error("target should be a symlink in symlink mode")
	}
}

func TestTargetProject_RemoveNotFound_Error(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	projectRoot := sb.SetupProjectDir("claude")

	result := sb.RunCLIInDir(projectRoot, "target", "remove", "nonexistent", "-p")
	result.AssertFailure(t)
	result.AssertAnyOutputContains(t, "not found")
}

func TestTargetProject_RemoveDryRun(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	projectRoot := sb.SetupProjectDir("claude", "cursor")

	result := sb.RunCLIInDir(projectRoot, "target", "remove", "cursor", "--dry-run", "-p")
	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, "Dry run")

	// Config unchanged
	cfg := sb.ReadFile(filepath.Join(projectRoot, ".skillshare", "config.yaml"))
	if !strings.Contains(cfg, "cursor") {
		t.Error("dry-run should not remove target from config")
	}
}

func TestTargetProject_RemoveCopyMode_CleansManifest(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	projectRoot := sb.SetupProjectDir("claude")

	sb.CreateProjectSkill(projectRoot, "my-skill", map[string]string{
		"SKILL.md": "# My Skill",
	})

	// Override project config to use copy mode
	sb.WriteProjectConfig(projectRoot, `targets:
  - name: claude
    mode: copy
`)

	// Sync to create copies + manifest
	sb.RunCLIInDir(projectRoot, "sync", "-p").AssertSuccess(t)

	targetPath := filepath.Join(projectRoot, ".claude", "skills")
	manifestPath := filepath.Join(targetPath, sync.ManifestFile)
	if !sb.FileExists(manifestPath) {
		t.Fatal("manifest should exist after copy sync")
	}

	// Remove target
	result := sb.RunCLIInDir(projectRoot, "target", "remove", "claude", "-p")
	result.AssertSuccess(t)

	// Manifest should be cleaned up
	if sb.FileExists(manifestPath) {
		t.Error("manifest should be removed after target remove")
	}
}
