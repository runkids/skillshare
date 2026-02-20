//go:build !online

package integration

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"skillshare/internal/sync"
	"skillshare/internal/testutil"
)

// --- basic copy mode ---

func TestSync_CopyMode_CopiesSkills(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.CreateSkill("skill-a", map[string]string{
		"SKILL.md": "---\nname: skill-a\n---\n# Skill A",
	})
	targetPath := sb.CreateTarget("claude")

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets:
  claude:
    path: ` + targetPath + `
    mode: copy
`)

	result := sb.RunCLI("sync")
	result.AssertSuccess(t)

	copied := filepath.Join(targetPath, "skill-a")
	if !sb.FileExists(filepath.Join(copied, "SKILL.md")) {
		t.Error("skill-a should be copied to target")
	}
	if sb.IsSymlink(copied) {
		t.Error("skill-a should be a real directory, not a symlink")
	}

	// Manifest should exist
	manifestPath := filepath.Join(targetPath, sync.ManifestFile)
	if !sb.FileExists(manifestPath) {
		t.Error("manifest file should exist")
	}

	// Verify manifest content
	var manifest sync.Manifest
	data, _ := os.ReadFile(manifestPath)
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatalf("failed to parse manifest: %v", err)
	}
	if _, ok := manifest.Managed["skill-a"]; !ok {
		t.Error("manifest should contain skill-a")
	}
}

func TestSync_CopyMode_SkipsUnchanged(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.CreateSkill("skill-a", map[string]string{
		"SKILL.md": "# Skill A",
	})
	targetPath := sb.CreateTarget("claude")

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets:
  claude:
    path: ` + targetPath + `
    mode: copy
`)

	// First sync
	sb.RunCLI("sync").AssertSuccess(t)

	// Second sync — should skip
	result := sb.RunCLI("sync")
	result.AssertSuccess(t)
	result.AssertOutputContains(t, "skipped")
}

func TestSync_CopyMode_UpdatesChanged(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	skillDir := sb.CreateSkill("skill-a", map[string]string{
		"SKILL.md": "# Original",
	})
	targetPath := sb.CreateTarget("claude")

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets:
  claude:
    path: ` + targetPath + `
    mode: copy
`)

	// First sync
	sb.RunCLI("sync").AssertSuccess(t)

	// Modify source skill
	os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("# Modified"), 0644)

	// Second sync — should update
	result := sb.RunCLI("sync")
	result.AssertSuccess(t)
	result.AssertOutputContains(t, "updated")

	// Verify target has new content
	got := sb.ReadFile(filepath.Join(targetPath, "skill-a", "SKILL.md"))
	if got != "# Modified" {
		t.Errorf("target content should be updated, got: %s", got)
	}
}

func TestSync_CopyMode_ForceOverwrites(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.CreateSkill("skill-a", map[string]string{
		"SKILL.md": "# Source",
	})
	targetPath := sb.CreateTarget("claude")

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets:
  claude:
    path: ` + targetPath + `
    mode: copy
`)

	// First sync
	sb.RunCLI("sync").AssertSuccess(t)

	// Force sync — should overwrite even though unchanged
	result := sb.RunCLI("sync", "--force")
	result.AssertSuccess(t)
	// With force, unchanged skills show as updated (not skipped)
	result.AssertOutputContains(t, "updated")
	result.AssertOutputNotContains(t, "up to date")
}

func TestSync_CopyMode_PreservesLocalSkills(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.CreateSkill("shared-skill", map[string]string{
		"SKILL.md": "# Shared",
	})
	targetPath := sb.CreateTarget("claude")

	// Place a local skill directly in target
	localDir := filepath.Join(targetPath, "local-only")
	os.MkdirAll(localDir, 0755)
	os.WriteFile(filepath.Join(localDir, "SKILL.md"), []byte("# Local"), 0644)

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets:
  claude:
    path: ` + targetPath + `
    mode: copy
`)

	result := sb.RunCLI("sync")
	result.AssertSuccess(t)

	// Local skill should be preserved
	if !sb.FileExists(filepath.Join(localDir, "SKILL.md")) {
		t.Error("local skill should be preserved")
	}
}

func TestSync_CopyMode_NestedSkills(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.CreateNestedSkill("_team/frontend/ui", map[string]string{
		"SKILL.md": "# UI Skill",
	})
	targetPath := sb.CreateTarget("claude")

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets:
  claude:
    path: ` + targetPath + `
    mode: copy
`)

	result := sb.RunCLI("sync")
	result.AssertSuccess(t)

	// Should use flat name
	flatDir := filepath.Join(targetPath, "_team__frontend__ui")
	if !sb.FileExists(filepath.Join(flatDir, "SKILL.md")) {
		t.Error("nested skill should be copied with flat name")
	}
	if sb.IsSymlink(flatDir) {
		t.Error("should be a real copy, not symlink")
	}
}

func TestSync_CopyMode_Pruning(t *testing.T) {
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

	// First sync
	sb.RunCLI("sync").AssertSuccess(t)

	// Remove skill-b from source
	os.RemoveAll(filepath.Join(sb.SourcePath, "skill-b"))

	// Second sync — should prune skill-b
	result := sb.RunCLI("sync")
	result.AssertSuccess(t)

	if sb.FileExists(filepath.Join(targetPath, "skill-b")) {
		t.Error("skill-b should be pruned from target")
	}
	if !sb.FileExists(filepath.Join(targetPath, "skill-a", "SKILL.md")) {
		t.Error("skill-a should remain")
	}

	// Manifest should not contain skill-b
	var manifest sync.Manifest
	data, _ := os.ReadFile(filepath.Join(targetPath, sync.ManifestFile))
	json.Unmarshal(data, &manifest)
	if _, ok := manifest.Managed["skill-b"]; ok {
		t.Error("manifest should not contain pruned skill-b")
	}
}

func TestSync_CopyMode_IncludeExclude(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.CreateSkill("team-alpha", map[string]string{"SKILL.md": "# Alpha"})
	sb.CreateSkill("team-beta", map[string]string{"SKILL.md": "# Beta"})
	sb.CreateSkill("personal", map[string]string{"SKILL.md": "# Personal"})
	targetPath := sb.CreateTarget("claude")

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets:
  claude:
    path: ` + targetPath + `
    mode: copy
    include:
      - "team-*"
`)

	result := sb.RunCLI("sync")
	result.AssertSuccess(t)

	if !sb.FileExists(filepath.Join(targetPath, "team-alpha", "SKILL.md")) {
		t.Error("team-alpha should be copied")
	}
	if !sb.FileExists(filepath.Join(targetPath, "team-beta", "SKILL.md")) {
		t.Error("team-beta should be copied")
	}
	if sb.FileExists(filepath.Join(targetPath, "personal")) {
		t.Error("personal should NOT be copied (excluded by include filter)")
	}
}

func TestSync_CopyMode_DryRun(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.CreateSkill("skill-a", map[string]string{"SKILL.md": "# A"})
	targetPath := sb.CreateTarget("claude")

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets:
  claude:
    path: ` + targetPath + `
    mode: copy
`)

	result := sb.RunCLI("sync", "--dry-run")
	result.AssertSuccess(t)

	// Should not actually copy
	if sb.FileExists(filepath.Join(targetPath, "skill-a")) {
		t.Error("dry run should not copy files")
	}

	// Manifest should not be created
	if sb.FileExists(filepath.Join(targetPath, sync.ManifestFile)) {
		t.Error("dry run should not create manifest")
	}
}

// --- mode conversion ---

func TestSync_ModeConversion_MergeToCopy(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.CreateSkill("skill-a", map[string]string{"SKILL.md": "# A"})
	targetPath := sb.CreateTarget("claude")

	// Start with merge mode
	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets:
  claude:
    path: ` + targetPath + `
    mode: merge
`)
	sb.RunCLI("sync").AssertSuccess(t)

	// Verify it's a symlink
	if !sb.IsSymlink(filepath.Join(targetPath, "skill-a")) {
		t.Fatal("merge mode should create symlink")
	}

	// Switch to copy mode
	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets:
  claude:
    path: ` + targetPath + `
    mode: copy
`)
	result := sb.RunCLI("sync")
	result.AssertSuccess(t)

	// Should now be a real copy, not symlink
	skillPath := filepath.Join(targetPath, "skill-a")
	if sb.IsSymlink(skillPath) {
		t.Error("should be a real copy after conversion")
	}
	if !sb.FileExists(filepath.Join(skillPath, "SKILL.md")) {
		t.Error("skill content should exist")
	}
	if !sb.FileExists(filepath.Join(targetPath, sync.ManifestFile)) {
		t.Error("manifest should be created")
	}
}

func TestSync_ModeConversion_CopyToMerge(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.CreateSkill("skill-a", map[string]string{"SKILL.md": "# A"})
	targetPath := sb.CreateTarget("claude")

	// Start with copy mode
	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets:
  claude:
    path: ` + targetPath + `
    mode: copy
`)
	sb.RunCLI("sync").AssertSuccess(t)

	// Switch to merge mode with force (to replace copied dirs with symlinks)
	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets:
  claude:
    path: ` + targetPath + `
    mode: merge
`)
	result := sb.RunCLI("sync", "--force")
	result.AssertSuccess(t)

	// Should now be a symlink
	if !sb.IsSymlink(filepath.Join(targetPath, "skill-a")) {
		t.Error("should be a symlink after copy→merge conversion with --force")
	}
	// Manifest should be removed
	if sb.FileExists(filepath.Join(targetPath, sync.ManifestFile)) {
		t.Error("manifest should be removed after copy→merge conversion")
	}
}

// --- project mode copy ---

func TestSync_CopyMode_Project(t *testing.T) {
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

	result := sb.RunCLIInDir(projectRoot, "sync", "-p")
	result.AssertSuccess(t)

	copied := filepath.Join(projectRoot, ".claude", "skills", "my-skill")
	if !sb.FileExists(filepath.Join(copied, "SKILL.md")) {
		t.Error("skill should be copied in project mode")
	}
	if sb.IsSymlink(copied) {
		t.Error("should be a real copy, not symlink")
	}
}
