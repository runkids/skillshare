//go:build !online

package integration

import (
	"os"
	"path/filepath"
	"testing"

	"skillshare/internal/testutil"
)

func TestSync_MergeMode_CreatesSymlinks(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	// Create a skill in source
	sb.CreateSkill("my-skill", map[string]string{
		"SKILL.md": "# My Skill\n\nDescription here.",
	})

	// Create target directory
	targetPath := sb.CreateTarget("claude")

	// Write config
	sb.WriteConfig(`source: ` + sb.SourcePath + `
mode: merge
targets:
  claude:
    path: ` + targetPath + `
`)

	result := sb.RunCLI("sync")

	result.AssertSuccess(t)
	result.AssertOutputContains(t, "merged")

	// Verify symlink was created
	skillLink := filepath.Join(targetPath, "my-skill")
	if !sb.IsSymlink(skillLink) {
		t.Error("skill should be a symlink")
	}

	expectedTarget := filepath.Join(sb.SourcePath, "my-skill")
	if got := sb.SymlinkTarget(skillLink); got != expectedTarget {
		t.Errorf("symlink target = %q, want %q", got, expectedTarget)
	}
}

func TestSync_MergeMode_PreservesLocalSkills(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	// Create source skill
	sb.CreateSkill("shared-skill", map[string]string{
		"SKILL.md": "# Shared",
	})

	// Create target with local skill
	targetPath := sb.CreateTarget("claude")
	localSkillPath := filepath.Join(targetPath, "local-skill")
	os.MkdirAll(localSkillPath, 0755)
	os.WriteFile(filepath.Join(localSkillPath, "SKILL.md"), []byte("# Local"), 0644)

	sb.WriteConfig(`source: ` + sb.SourcePath + `
mode: merge
targets:
  claude:
    path: ` + targetPath + `
`)

	result := sb.RunCLI("sync")

	result.AssertSuccess(t)

	// Verify local skill preserved (is still a directory, not symlink)
	if sb.IsSymlink(localSkillPath) {
		t.Error("local skill should not be converted to symlink")
	}
	if !sb.FileExists(filepath.Join(localSkillPath, "SKILL.md")) {
		t.Error("local skill files should be preserved")
	}

	// Verify shared skill is symlinked
	sharedSkillPath := filepath.Join(targetPath, "shared-skill")
	if !sb.IsSymlink(sharedSkillPath) {
		t.Error("shared skill should be a symlink")
	}
}

func TestSync_DryRun_NoChanges(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.CreateSkill("test-skill", map[string]string{
		"SKILL.md": "# Test",
	})
	targetPath := sb.CreateTarget("claude")

	sb.WriteConfig(`source: ` + sb.SourcePath + `
mode: merge
targets:
  claude:
    path: ` + targetPath + `
`)

	// Record initial state
	entriesBefore, _ := os.ReadDir(targetPath)

	// Execute with --dry-run
	result := sb.RunCLI("sync", "--dry-run")

	result.AssertSuccess(t)
	result.AssertOutputContains(t, "Dry run")

	// Verify no changes made
	entriesAfter, _ := os.ReadDir(targetPath)
	if len(entriesAfter) != len(entriesBefore) {
		t.Error("dry-run should not modify file system")
	}
}

func TestSync_NoConfig_ReturnsError(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	// Remove config file
	os.Remove(sb.ConfigPath)

	result := sb.RunCLI("sync")

	result.AssertFailure(t)
	result.AssertAnyOutputContains(t, "init")
}

func TestSync_SourceNotExist_ReturnsError(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	// Config points to non-existent source
	sb.WriteConfig(`source: /nonexistent/path
targets:
  claude:
    path: ` + filepath.Join(sb.Home, ".claude", "skills") + `
`)

	result := sb.RunCLI("sync")

	result.AssertFailure(t)
	result.AssertAnyOutputContains(t, "source directory does not exist")
}

func TestSync_SymlinkMode_CreatesSingleSymlink(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.CreateSkill("skill1", map[string]string{"SKILL.md": "# Skill 1"})
	sb.CreateSkill("skill2", map[string]string{"SKILL.md": "# Skill 2"})

	targetPath := filepath.Join(sb.Home, ".claude", "skills")
	// Remove target directory if exists for symlink mode
	os.RemoveAll(targetPath)

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets:
  claude:
    path: ` + targetPath + `
    mode: symlink
`)

	result := sb.RunCLI("sync")

	result.AssertSuccess(t)

	// Verify target is a symlink to source
	if !sb.IsSymlink(targetPath) {
		t.Error("target should be a symlink")
	}
	if got := sb.SymlinkTarget(targetPath); got != sb.SourcePath {
		t.Errorf("symlink target = %q, want %q", got, sb.SourcePath)
	}
}

func TestSync_MultipleTargets_SyncsAll(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.CreateSkill("common-skill", map[string]string{
		"SKILL.md": "# Common Skill",
	})

	claudePath := sb.CreateTarget("claude")
	codexPath := sb.CreateTarget("codex")

	sb.WriteConfig(`source: ` + sb.SourcePath + `
mode: merge
targets:
  claude:
    path: ` + claudePath + `
  codex:
    path: ` + codexPath + `
`)

	result := sb.RunCLI("sync")

	result.AssertSuccess(t)

	// Verify skill synced to both targets
	if !sb.IsSymlink(filepath.Join(claudePath, "common-skill")) {
		t.Error("skill should be synced to claude")
	}
	if !sb.IsSymlink(filepath.Join(codexPath, "common-skill")) {
		t.Error("skill should be synced to codex")
	}
}

func TestSync_Force_OverwritesConflict(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.CreateSkill("skill", map[string]string{"SKILL.md": "# Skill"})

	// Create target as symlink to wrong location
	targetPath := filepath.Join(sb.Home, ".claude", "skills")
	wrongSource := filepath.Join(sb.Home, "wrong-source")
	os.MkdirAll(wrongSource, 0755)
	os.MkdirAll(filepath.Dir(targetPath), 0755)
	os.RemoveAll(targetPath)
	os.Symlink(wrongSource, targetPath)

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets:
  claude:
    path: ` + targetPath + `
    mode: symlink
`)

	// Execute without force - should fail
	result := sb.RunCLI("sync")
	result.AssertFailure(t)

	// Execute with force - should succeed
	result = sb.RunCLI("sync", "--force")
	result.AssertSuccess(t)

	// Verify symlink now points to correct source
	if got := sb.SymlinkTarget(targetPath); got != sb.SourcePath {
		t.Errorf("symlink target = %q, want %q", got, sb.SourcePath)
	}
}

func TestSync_NestedSkills_FlatNaming(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	// Create nested skill structure
	// Source: personal/writing/email/SKILL.md -> Target: personal__writing__email
	sb.CreateNestedSkill("personal/writing/email", map[string]string{
		"SKILL.md": "# Email Writing Skill",
	})

	// Also create a regular flat skill for comparison
	sb.CreateSkill("my-helper", map[string]string{
		"SKILL.md": "# My Helper",
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

	// Verify flat skill is symlinked normally
	flatSkillLink := filepath.Join(targetPath, "my-helper")
	if !sb.IsSymlink(flatSkillLink) {
		t.Error("flat skill should be a symlink")
	}

	// Verify nested skill is symlinked with flat naming
	nestedSkillLink := filepath.Join(targetPath, "personal__writing__email")
	if !sb.IsSymlink(nestedSkillLink) {
		t.Errorf("nested skill should be a symlink at %s", nestedSkillLink)
	}

	// Verify symlink points to correct nested source
	expectedNestedTarget := filepath.Join(sb.SourcePath, "personal", "writing", "email")
	if got := sb.SymlinkTarget(nestedSkillLink); got != expectedNestedTarget {
		t.Errorf("nested symlink target = %q, want %q", got, expectedNestedTarget)
	}
}

func TestSync_TrackedRepoSkills_FlatNaming(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	// Create a tracked repo structure with nested skills
	// Source: _team-repo/frontend/ui/SKILL.md -> Target: _team-repo__frontend__ui
	sb.CreateNestedSkill("_team-repo/frontend/ui", map[string]string{
		"SKILL.md": "# UI Components",
	})

	// Another skill in the same tracked repo
	sb.CreateNestedSkill("_team-repo/backend/api", map[string]string{
		"SKILL.md": "# API Utilities",
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

	// Verify tracked repo skills are symlinked with flat naming
	uiSkillLink := filepath.Join(targetPath, "_team-repo__frontend__ui")
	if !sb.IsSymlink(uiSkillLink) {
		t.Errorf("tracked repo skill should be a symlink at %s", uiSkillLink)
	}

	apiSkillLink := filepath.Join(targetPath, "_team-repo__backend__api")
	if !sb.IsSymlink(apiSkillLink) {
		t.Errorf("tracked repo skill should be a symlink at %s", apiSkillLink)
	}

	// Verify symlinks point to correct nested sources
	expectedUITarget := filepath.Join(sb.SourcePath, "_team-repo", "frontend", "ui")
	if got := sb.SymlinkTarget(uiSkillLink); got != expectedUITarget {
		t.Errorf("UI symlink target = %q, want %q", got, expectedUITarget)
	}
}

func TestSync_TrackedRepoSkills_HiddenDirs(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	// Simulate a tracked repo with skills inside hidden directories (like openai/skills)
	// Structure: _openai-skills/.curated/pdf/SKILL.md
	//            _openai-skills/.system/figma/SKILL.md
	sb.CreateNestedSkill("_openai-skills/.curated/pdf", map[string]string{
		"SKILL.md": "# PDF",
	})
	sb.CreateNestedSkill("_openai-skills/.system/figma", map[string]string{
		"SKILL.md": "# Figma",
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

	// Verify skills inside hidden dirs are discovered and synced
	pdfLink := filepath.Join(targetPath, "_openai-skills__.curated__pdf")
	if !sb.IsSymlink(pdfLink) {
		t.Errorf("skill inside .curated/ should be synced: %s", pdfLink)
	}

	figmaLink := filepath.Join(targetPath, "_openai-skills__.system__figma")
	if !sb.IsSymlink(figmaLink) {
		t.Errorf("skill inside .system/ should be synced: %s", figmaLink)
	}
}

func TestSync_Pruning_RemovesOrphanLinks(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	// Create initial skills
	sb.CreateSkill("skill-a", map[string]string{"SKILL.md": "# Skill A"})
	sb.CreateSkill("skill-b", map[string]string{"SKILL.md": "# Skill B"})
	sb.CreateNestedSkill("nested/skill-c", map[string]string{"SKILL.md": "# Skill C"})

	targetPath := sb.CreateTarget("claude")

	sb.WriteConfig(`source: ` + sb.SourcePath + `
mode: merge
targets:
  claude:
    path: ` + targetPath + `
`)

	// First sync - creates all symlinks
	result := sb.RunCLI("sync")
	result.AssertSuccess(t)

	// Verify all symlinks exist
	if !sb.IsSymlink(filepath.Join(targetPath, "skill-a")) {
		t.Error("skill-a should be a symlink")
	}
	if !sb.IsSymlink(filepath.Join(targetPath, "skill-b")) {
		t.Error("skill-b should be a symlink")
	}
	if !sb.IsSymlink(filepath.Join(targetPath, "nested__skill-c")) {
		t.Error("nested__skill-c should be a symlink")
	}

	// Remove skill-b from source
	os.RemoveAll(filepath.Join(sb.SourcePath, "skill-b"))

	// Second sync - should prune skill-b
	result = sb.RunCLI("sync")
	result.AssertSuccess(t)
	result.AssertOutputContains(t, "pruned")

	// Verify skill-b is removed, others remain
	if !sb.IsSymlink(filepath.Join(targetPath, "skill-a")) {
		t.Error("skill-a should still be a symlink")
	}
	if sb.FileExists(filepath.Join(targetPath, "skill-b")) {
		t.Error("skill-b should have been pruned")
	}
	if !sb.IsSymlink(filepath.Join(targetPath, "nested__skill-c")) {
		t.Error("nested__skill-c should still be a symlink")
	}
}

func TestSync_Pruning_PreservesLocalDirectories(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	// Create source skill
	sb.CreateSkill("shared-skill", map[string]string{"SKILL.md": "# Shared"})

	targetPath := sb.CreateTarget("claude")

	// Create a local directory in target (not a symlink)
	localSkillPath := filepath.Join(targetPath, "my-local-skill")
	os.MkdirAll(localSkillPath, 0755)
	os.WriteFile(filepath.Join(localSkillPath, "SKILL.md"), []byte("# Local"), 0644)

	sb.WriteConfig(`source: ` + sb.SourcePath + `
mode: merge
targets:
  claude:
    path: ` + targetPath + `
`)

	result := sb.RunCLI("sync")
	result.AssertSuccess(t)

	// Local directory should be preserved (warning issued but not deleted)
	if !sb.FileExists(localSkillPath) {
		t.Error("local skill directory should be preserved")
	}
}

func TestSync_MergeMode_IncludeFilter(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.CreateSkill("codex-plan", map[string]string{"SKILL.md": "# Codex"})
	sb.CreateSkill("claude-help", map[string]string{"SKILL.md": "# Claude"})
	targetPath := sb.CreateTarget("claude")

	sb.WriteConfig(`source: ` + sb.SourcePath + `
mode: merge
targets:
  claude:
    path: ` + targetPath + `
    include: [codex-*]
`)

	result := sb.RunCLI("sync")
	result.AssertSuccess(t)

	if !sb.IsSymlink(filepath.Join(targetPath, "codex-plan")) {
		t.Error("included skill should be symlinked")
	}
	if sb.FileExists(filepath.Join(targetPath, "claude-help")) {
		t.Error("non-included skill should not be synced")
	}
}

func TestSync_MergeMode_ExcludeFilter(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.CreateSkill("codex-plan", map[string]string{"SKILL.md": "# Codex"})
	sb.CreateSkill("claude-help", map[string]string{"SKILL.md": "# Claude"})
	targetPath := sb.CreateTarget("claude")

	sb.WriteConfig(`source: ` + sb.SourcePath + `
mode: merge
targets:
  claude:
    path: ` + targetPath + `
    exclude: [claude-*]
`)

	result := sb.RunCLI("sync")
	result.AssertSuccess(t)

	if !sb.IsSymlink(filepath.Join(targetPath, "codex-plan")) {
		t.Error("non-excluded skill should be symlinked")
	}
	if sb.FileExists(filepath.Join(targetPath, "claude-help")) {
		t.Error("excluded skill should not be synced")
	}
}

func TestSync_MergeMode_IncludeThenExclude(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.CreateSkill("codex-plan", map[string]string{"SKILL.md": "# Plan"})
	sb.CreateSkill("codex-test", map[string]string{"SKILL.md": "# Test"})
	sb.CreateSkill("claude-help", map[string]string{"SKILL.md": "# Claude"})
	targetPath := sb.CreateTarget("claude")

	sb.WriteConfig(`source: ` + sb.SourcePath + `
mode: merge
targets:
  claude:
    path: ` + targetPath + `
    include: [codex-*]
    exclude: ["*-test"]
`)

	result := sb.RunCLI("sync")
	result.AssertSuccess(t)

	if !sb.IsSymlink(filepath.Join(targetPath, "codex-plan")) {
		t.Error("included skill should be symlinked")
	}
	if sb.FileExists(filepath.Join(targetPath, "codex-test")) {
		t.Error("exclude should be applied after include")
	}
	if sb.FileExists(filepath.Join(targetPath, "claude-help")) {
		t.Error("skills outside include should not be synced")
	}
}

func TestSync_Pruning_RemovesExcludedSourceLinkedSkill(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.CreateSkill("keep-me", map[string]string{"SKILL.md": "# Keep"})
	sb.CreateSkill("exclude-me", map[string]string{"SKILL.md": "# Exclude"})
	targetPath := sb.CreateTarget("claude")

	// Initial sync with no filters so both links exist.
	sb.WriteConfig(`source: ` + sb.SourcePath + `
mode: merge
targets:
  claude:
    path: ` + targetPath + `
`)
	sb.RunCLI("sync").AssertSuccess(t)

	// Add an exclude filter and sync again.
	sb.WriteConfig(`source: ` + sb.SourcePath + `
mode: merge
targets:
  claude:
    path: ` + targetPath + `
    exclude: [exclude-*]
`)
	result := sb.RunCLI("sync")
	result.AssertSuccess(t)

	if !sb.IsSymlink(filepath.Join(targetPath, "keep-me")) {
		t.Error("non-excluded skill should stay synced")
	}
	if sb.FileExists(filepath.Join(targetPath, "exclude-me")) {
		t.Error("excluded source-linked skill should be removed by sync")
	}
}

func TestSync_Pruning_PreservesExcludedSourceLocalCopy(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.CreateSkill("exclude-me", map[string]string{"SKILL.md": "# Source"})
	targetPath := sb.CreateTarget("claude")

	// Create local copy in target (not symlink)
	localSkillPath := filepath.Join(targetPath, "exclude-me")
	if err := os.MkdirAll(localSkillPath, 0755); err != nil {
		t.Fatalf("failed to create local skill dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(localSkillPath, "SKILL.md"), []byte("# Local"), 0644); err != nil {
		t.Fatalf("failed to write local skill: %v", err)
	}

	// First sync keeps local copy (no force).
	sb.WriteConfig(`source: ` + sb.SourcePath + `
mode: merge
targets:
  claude:
    path: ` + targetPath + `
`)
	sb.RunCLI("sync").AssertSuccess(t)
	if !sb.FileExists(localSkillPath) {
		t.Fatal("local copy should exist before exclude")
	}

	// Add exclude and sync again - local copy should stay.
	sb.WriteConfig(`source: ` + sb.SourcePath + `
mode: merge
targets:
  claude:
    path: ` + targetPath + `
    exclude: [exclude-*]
`)
	result := sb.RunCLI("sync")
	result.AssertSuccess(t)

	if !sb.FileExists(localSkillPath) {
		t.Error("excluded source skill local copy should be preserved")
	}
}

func TestSync_Pruning_RemovesBrokenExternalSymlinks(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	// Create a skill and sync it
	sb.CreateSkill("skill-a", map[string]string{"SKILL.md": "# Skill A"})
	targetPath := sb.CreateTarget("claude")

	sb.WriteConfig(`source: ` + sb.SourcePath + `
mode: merge
targets:
  claude:
    path: ` + targetPath + `
`)

	result := sb.RunCLI("sync")
	result.AssertSuccess(t)

	// Simulate data migration: create a symlink pointing to a non-existent
	// external path (as if the old source directory was moved/deleted)
	oldPath := filepath.Join(sb.Home, "old-config", "skillshare", "skills", "migrated-skill")
	os.Symlink(oldPath, filepath.Join(targetPath, "migrated-skill"))

	if !sb.IsSymlink(filepath.Join(targetPath, "migrated-skill")) {
		t.Fatal("setup: migrated-skill symlink should exist")
	}

	// Sync again — broken external symlink should be auto-removed
	result = sb.RunCLI("sync")
	result.AssertSuccess(t)

	if sb.FileExists(filepath.Join(targetPath, "migrated-skill")) {
		t.Error("broken external symlink should have been pruned")
	}
	// Valid skill should still exist
	if !sb.IsSymlink(filepath.Join(targetPath, "skill-a")) {
		t.Error("skill-a should still be a symlink")
	}
}

func TestSync_Pruning_ForceRemovesExternalSymlinks(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.CreateSkill("skill-a", map[string]string{"SKILL.md": "# Skill A"})
	targetPath := sb.CreateTarget("claude")

	sb.WriteConfig(`source: ` + sb.SourcePath + `
mode: merge
targets:
  claude:
    path: ` + targetPath + `
`)

	result := sb.RunCLI("sync")
	result.AssertSuccess(t)

	// Create a valid external symlink (target exists but outside source dir)
	externalDir := filepath.Join(sb.Home, "external-skills", "ext-skill")
	os.MkdirAll(externalDir, 0755)
	os.WriteFile(filepath.Join(externalDir, "SKILL.md"), []byte("# External"), 0644)
	os.Symlink(externalDir, filepath.Join(targetPath, "ext-skill"))

	// Sync without force — external symlink should be preserved (with warning)
	result = sb.RunCLI("sync")
	result.AssertSuccess(t)

	if !sb.IsSymlink(filepath.Join(targetPath, "ext-skill")) {
		t.Error("valid external symlink should be preserved without --force")
	}

	// Sync with --force — external symlink should be removed
	result = sb.RunCLI("sync", "--force")
	result.AssertSuccess(t)

	if sb.FileExists(filepath.Join(targetPath, "ext-skill")) {
		t.Error("external symlink should have been removed with --force")
	}
	// Valid source skill should still exist
	if !sb.IsSymlink(filepath.Join(targetPath, "skill-a")) {
		t.Error("skill-a should still be a symlink")
	}
}

func TestSync_MergeMode_InvalidFilterPatternFails(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.CreateSkill("skill-a", map[string]string{"SKILL.md": "# Skill"})
	targetPath := sb.CreateTarget("claude")

	sb.WriteConfig(`source: ` + sb.SourcePath + `
mode: merge
targets:
  claude:
    path: ` + targetPath + `
    include: ["["]
`)

	result := sb.RunCLI("sync")
	result.AssertFailure(t)
	result.AssertAnyOutputContains(t, "invalid include pattern")
}
