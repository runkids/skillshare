//go:build !online

package integration

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
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
	result.AssertRowContains(t, "claude", "1 linked")

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
	targetPath := sb.CreateTarget("claude")
	sb.WriteConfig(`source: /nonexistent/path
targets:
  claude:
    path: ` + targetPath + `
`)

	result := sb.RunCLI("sync")

	result.AssertFailure(t)
	result.AssertAnyOutputContains(t, "source path does not exist")
}

func TestSync_GlobalDefaultSourceWhenOmitted(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.CreateSkill("default-source-skill", map[string]string{
		"SKILL.md": "# Default Source Skill",
	})
	targetPath := sb.CreateTarget("claude")

	sb.WriteConfig(`targets:
  claude:
    skills:
      path: ` + targetPath + `
`)

	result := sb.RunCLI("sync", "--global")

	result.AssertSuccess(t)
	if !sb.IsSymlink(filepath.Join(targetPath, "default-source-skill")) {
		t.Error("skill from default source should be synced")
	}
}

func TestSync_SymlinkMode_CreatesSingleSymlink(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.CreateSkill("skill1", map[string]string{"SKILL.md": "# Skill 1"})
	sb.CreateSkill("skill2", map[string]string{"SKILL.md": "# Skill 2"})

	targetPath := sb.CreateTarget("claude")

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

func TestSync_PreservesRegistryEntries(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	// Create a local skill that sync will discover
	sb.CreateSkill("local-skill", map[string]string{"SKILL.md": "# Local"})

	targetPath := sb.CreateTarget("claude")
	sb.WriteConfig(`source: ` + sb.SourcePath + `
mode: merge
targets:
  claude:
    path: ` + targetPath + `
`)

	// Write a registry with a remote-installed skill that has NO files on disk.
	// Sync must NOT prune this entry — registry is the source of truth for installations.
	registryPath := filepath.Join(sb.SourcePath, "registry.yaml")
	registryContent := "skills:\n  - name: remote-tool\n    source: github.com/someone/remote-tool\n"
	os.WriteFile(registryPath, []byte(registryContent), 0644)

	result := sb.RunCLI("sync")
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

// A broken link that points outside the source and that skillshare did not
// create belongs to another tool or the user, so sync keeps it.
func TestSync_Pruning_KeepsBrokenExternalSymlinkItDidNotCreate(t *testing.T) {
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

	sb.RunCLI("sync").AssertSuccess(t)

	foreign := filepath.Join(targetPath, "other-tool-skill")
	if err := os.Symlink(filepath.Join(sb.Home, "other-tool", "skills", "gone"), foreign); err != nil {
		t.Fatal(err)
	}

	result := sb.RunCLI("sync")
	result.AssertSuccess(t)

	// IsSymlink uses Lstat; FileExists follows the link and is false for a broken one.
	if !sb.IsSymlink(foreign) {
		t.Error("broken link skillshare did not create should be kept")
	}
	result.AssertAnyOutputContains(t, "skillshare did not create")
	if !sb.IsSymlink(filepath.Join(targetPath, "skill-a")) {
		t.Error("skill-a should still be a symlink")
	}
}

// After the source moves, the links sync made earlier point into the old
// location. The manifest records them, so sync removes the ones left broken.
func TestSync_Pruning_RemovesBrokenExternalSymlinkItCreated(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.CreateSkill("skill-a", map[string]string{"SKILL.md": "# Skill A"})
	sb.CreateSkill("migrated-skill", map[string]string{"SKILL.md": "# Migrated"})
	targetPath := sb.CreateTarget("claude")
	config := func(source string) string {
		return `source: ` + source + `
mode: merge
targets:
  claude:
    path: ` + targetPath + `
`
	}

	sb.WriteConfig(config(sb.SourcePath))
	sb.RunCLI("sync").AssertSuccess(t)

	// The source moves, and migrated-skill does not come along.
	newSource := filepath.Join(sb.Home, "new-source")
	if err := os.MkdirAll(filepath.Join(newSource, "skill-a"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(newSource, "skill-a", "SKILL.md"), []byte("# Skill A"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(sb.SourcePath); err != nil {
		t.Fatal(err)
	}
	sb.WriteConfig(config(newSource))
	migrated := filepath.Join(targetPath, "migrated-skill")
	if !sb.IsSymlink(migrated) {
		t.Fatal("setup: migrated-skill should be a broken link into the old source")
	}

	sb.RunCLI("sync").AssertSuccess(t)

	if _, err := os.Lstat(migrated); !os.IsNotExist(err) {
		t.Errorf("broken link recorded in the manifest should be pruned: %v", err)
	}
	if !sb.FileExists(filepath.Join(targetPath, "skill-a", "SKILL.md")) {
		t.Error("skill-a should be re-pointed to the new source")
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

func TestSync_IgnoredSkillsTextOutput(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.CreateSkill("keep-me", map[string]string{
		"SKILL.md": "---\nname: keep-me\n---\nKeep",
	})
	sb.CreateSkill("ignore-me", map[string]string{
		"SKILL.md": "---\nname: ignore-me\n---\nIgnored",
	})
	targetPath := sb.CreateTarget("claude")
	sb.WriteConfig(`source: ` + sb.SourcePath + `
mode: merge
targets:
  claude:
    path: ` + targetPath + `
`)

	os.WriteFile(filepath.Join(sb.SourcePath, ".skillignore"), []byte("ignore-me\n"), 0644)

	result := sb.RunCLI("sync")
	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, "1 skill(s) ignored by .skillignore")
	result.AssertAnyOutputContains(t, "ignore-me")
}

func TestSync_IgnoredSkillsJSONOutput(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.CreateSkill("keep-me", map[string]string{
		"SKILL.md": "---\nname: keep-me\n---\nKeep",
	})
	sb.CreateSkill("ignore-me", map[string]string{
		"SKILL.md": "---\nname: ignore-me\n---\nIgnored",
	})
	targetPath := sb.CreateTarget("claude")
	sb.WriteConfig(`source: ` + sb.SourcePath + `
mode: merge
targets:
  claude:
    path: ` + targetPath + `
`)

	os.WriteFile(filepath.Join(sb.SourcePath, ".skillignore"), []byte("ignore-me\n"), 0644)

	result := sb.RunCLI("sync", "--json")
	result.AssertSuccess(t)

	var output map[string]any
	if err := json.Unmarshal([]byte(result.Stdout), &output); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}

	ignoredCount := int(output["ignored_count"].(float64))
	if ignoredCount != 1 {
		t.Errorf("expected ignored_count=1, got %d", ignoredCount)
	}

	ignoredSkills := output["ignored_skills"].([]any)
	if len(ignoredSkills) != 1 || ignoredSkills[0].(string) != "ignore-me" {
		t.Errorf("expected ignored_skills=[ignore-me], got %v", ignoredSkills)
	}
}

func TestSync_NoSkillignore_NoIgnoredOutput(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.CreateSkill("my-skill", map[string]string{
		"SKILL.md": "---\nname: my-skill\n---\nContent",
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
	result.AssertOutputNotContains(t, "ignored by .skillignore")
}

func TestSync_SkillignoreLocal_OverridesBase(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.CreateSkill("keep-me", map[string]string{
		"SKILL.md": "---\nname: keep-me\n---\nKeep",
	})
	sb.CreateSkill("private-mine", map[string]string{
		"SKILL.md": "---\nname: private-mine\n---\nMine",
	})
	sb.CreateSkill("private-other", map[string]string{
		"SKILL.md": "---\nname: private-other\n---\nOther",
	})
	targetPath := sb.CreateTarget("claude")
	sb.WriteConfig(`source: ` + sb.SourcePath + `
mode: merge
targets:
  claude:
    path: ` + targetPath + `
`)

	// .skillignore blocks all private-* skills
	os.WriteFile(filepath.Join(sb.SourcePath, ".skillignore"), []byte("private-*\n"), 0644)
	// .skillignore.local un-ignores private-mine
	os.WriteFile(filepath.Join(sb.SourcePath, ".skillignore.local"), []byte("!private-mine\n"), 0644)

	result := sb.RunCLI("sync")
	result.AssertSuccess(t)

	// private-mine should be synced (un-ignored by .local)
	if _, err := os.Stat(filepath.Join(targetPath, "private-mine")); os.IsNotExist(err) {
		t.Error("private-mine should be synced (un-ignored by .skillignore.local)")
	}

	// private-other should NOT be synced (still ignored)
	if _, err := os.Stat(filepath.Join(targetPath, "private-other")); err == nil {
		t.Error("private-other should NOT be synced (still ignored)")
	}

	// keep-me should be synced
	if _, err := os.Stat(filepath.Join(targetPath, "keep-me")); os.IsNotExist(err) {
		t.Error("keep-me should be synced")
	}

	// Output should show 1 ignored skill and .local hint
	result.AssertAnyOutputContains(t, "1 skill(s) ignored by .skillignore")
	result.AssertAnyOutputContains(t, "private-other")
	result.AssertAnyOutputContains(t, ".local")
}

// A built-in target listed without a path takes its path from targets.yaml.
// antigravity-cli used to be an alias, and DefaultTargets() is not alias-aware,
// so a path-less entry failed validation instead of resolving.
func TestSync_BuiltinTargetWithoutPath_UsesDefaultPath(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.CreateSkill("my-skill", map[string]string{
		"SKILL.md": "# My Skill",
	})
	targetPath := sb.CreateTarget("antigravity-cli")

	sb.WriteConfig(`source: ` + sb.SourcePath + `
mode: merge
targets:
  antigravity-cli:
`)

	result := sb.RunCLI("sync")
	result.AssertSuccess(t)

	if !sb.IsSymlink(filepath.Join(targetPath, "my-skill")) {
		t.Errorf("symlink not created at %s", filepath.Join(targetPath, "my-skill"))
	}
}
