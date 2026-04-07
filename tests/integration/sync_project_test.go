//go:build !online

package integration

import (
	"os"
	"path/filepath"
	"strings"
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

func TestSyncProject_PreservesRegistryEntries(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	projectRoot := sb.SetupProjectDir("claude")

	// Create a skill on disk that sync will discover
	sb.CreateProjectSkill(projectRoot, "local-skill", map[string]string{
		"SKILL.md": "# Local Skill",
	})

	// Write a registry with a remote-installed skill that has NO files on disk.
	// Sync must NOT prune this entry — the registry is the source of truth for installations.
	registryPath := filepath.Join(projectRoot, ".skillshare", "registry.yaml")
	registryContent := "skills:\n  - name: remote-tool\n    source: github.com/someone/remote-tool\n"
	os.WriteFile(registryPath, []byte(registryContent), 0644)

	result := sb.RunCLIInDir(projectRoot, "sync", "-p")
	result.AssertSuccess(t)

	// Verify registry still contains the remote-tool entry
	data, err := os.ReadFile(registryPath)
	if err != nil {
		t.Fatalf("failed to read registry: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "remote-tool") {
		t.Errorf("sync should preserve registry entry for installed skill without local files, got:\n%s", content)
	}
	if !strings.Contains(content, "github.com/someone/remote-tool") {
		t.Errorf("sync should preserve source in registry entry, got:\n%s", content)
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

func TestSyncProject_RelativeSymlinks(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	projectRoot := sb.SetupProjectDir("claude")
	sb.CreateProjectSkill(projectRoot, "my-skill", map[string]string{
		"SKILL.md": "# My Skill\n\nDescription here.",
	})

	result := sb.RunCLIInDir(projectRoot, "sync", "-p")
	result.AssertSuccess(t)

	link := filepath.Join(projectRoot, ".claude", "skills", "my-skill")
	if !sb.IsSymlink(link) {
		t.Fatal("skill should be a symlink")
	}

	// Project-mode symlinks must be relative (not absolute)
	target := sb.SymlinkTarget(link)
	if filepath.IsAbs(target) {
		t.Errorf("project-mode symlink should be relative, got absolute: %q", target)
	}

	// Verify the symlink resolves to the correct skill directory
	resolved, err := filepath.EvalSymlinks(link)
	if err != nil {
		t.Fatalf("symlink should resolve: %v", err)
	}
	expected := filepath.Join(projectRoot, ".skillshare", "skills", "my-skill")
	if resolved != expected {
		t.Errorf("resolved symlink = %q, want %q", resolved, expected)
	}
}

func TestSync_GlobalMode_AbsoluteSymlinks(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.CreateSkill("my-skill", map[string]string{
		"SKILL.md": "# My Skill\n\nDescription here.",
	})
	targetPath := sb.CreateTarget("claude")

	sb.WriteConfig(`source: ` + sb.SourcePath + `
mode: merge
targets:
  claude:
    path: ` + targetPath + `
`)

	result := sb.RunCLI("sync")
	result.AssertSuccess(t)

	link := filepath.Join(targetPath, "my-skill")
	if !sb.IsSymlink(link) {
		t.Fatal("skill should be a symlink")
	}

	// Global-mode symlinks must be absolute
	target := sb.SymlinkTarget(link)
	if !filepath.IsAbs(target) {
		t.Errorf("global-mode symlink should be absolute, got relative: %q", target)
	}

	// Verify the symlink points directly to the source skill
	expected := filepath.Join(sb.SourcePath, "my-skill")
	if target != expected {
		t.Errorf("symlink target = %q, want %q", target, expected)
	}
}
