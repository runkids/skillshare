//go:build !online

package integration

import (
	"os"
	"path/filepath"
	"testing"

	"skillshare/internal/testutil"
)

func TestSyncExtras_MergeMode(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	// Create a skill so config.Load() has a valid source
	sb.CreateSkill("placeholder", map[string]string{
		"SKILL.md": "# Placeholder",
	})
	targetPath := sb.CreateTarget("claude")

	// Create extras source: rules directory with 2 .md files
	sourceRoot := filepath.Dir(sb.SourcePath) // ~/.config/skillshare/
	rulesSource := filepath.Join(sourceRoot, "rules")
	os.MkdirAll(rulesSource, 0755)
	os.WriteFile(filepath.Join(rulesSource, "coding.md"), []byte("# Coding Rules"), 0644)
	os.WriteFile(filepath.Join(rulesSource, "testing.md"), []byte("# Testing Rules"), 0644)

	// Create extras target directory
	rulesTarget := filepath.Join(sb.Home, ".claude", "rules")
	os.MkdirAll(rulesTarget, 0755)

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets:
  claude:
    path: ` + targetPath + `
extras:
  - name: rules
    targets:
      - path: ` + rulesTarget + `
`)

	result := sb.RunCLI("sync", "extras")

	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, "Rules")
	result.AssertAnyOutputContains(t, "2 files")

	// Verify files are symlinks
	codingLink := filepath.Join(rulesTarget, "coding.md")
	if !sb.IsSymlink(codingLink) {
		t.Error("coding.md should be a symlink in merge mode")
	}

	testingLink := filepath.Join(rulesTarget, "testing.md")
	if !sb.IsSymlink(testingLink) {
		t.Error("testing.md should be a symlink in merge mode")
	}

	// Verify symlink targets point to the source files
	expectedCoding := filepath.Join(rulesSource, "coding.md")
	if got := sb.SymlinkTarget(codingLink); got != expectedCoding {
		t.Errorf("coding.md symlink target = %q, want %q", got, expectedCoding)
	}
}

func TestSyncExtras_CopyMode(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.CreateSkill("placeholder", map[string]string{
		"SKILL.md": "# Placeholder",
	})
	targetPath := sb.CreateTarget("claude")

	// Create extras source
	sourceRoot := filepath.Dir(sb.SourcePath)
	rulesSource := filepath.Join(sourceRoot, "rules")
	os.MkdirAll(rulesSource, 0755)
	os.WriteFile(filepath.Join(rulesSource, "coding.md"), []byte("# Coding Rules"), 0644)

	// Create extras target
	rulesTarget := filepath.Join(sb.Home, ".cursor", "rules")
	os.MkdirAll(rulesTarget, 0755)

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets:
  claude:
    path: ` + targetPath + `
extras:
  - name: rules
    targets:
      - path: ` + rulesTarget + `
        mode: copy
`)

	result := sb.RunCLI("sync", "extras")

	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, "Rules")

	// Verify file exists and is a real copy (not a symlink)
	copiedFile := filepath.Join(rulesTarget, "coding.md")
	if !sb.FileExists(copiedFile) {
		t.Fatal("coding.md should exist in target")
	}
	if sb.IsSymlink(copiedFile) {
		t.Error("coding.md should be a real copy, not a symlink, in copy mode")
	}

	// Verify content matches
	content := sb.ReadFile(copiedFile)
	if content != "# Coding Rules" {
		t.Errorf("copied file content = %q, want %q", content, "# Coding Rules")
	}
}

func TestSyncExtras_NoExtrasConfigured(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.CreateSkill("placeholder", map[string]string{
		"SKILL.md": "# Placeholder",
	})
	targetPath := sb.CreateTarget("claude")

	// Config with no extras section
	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets:
  claude:
    path: ` + targetPath + `
`)

	result := sb.RunCLI("sync", "extras")

	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, "No extras configured")
}

func TestSyncExtras_PrunesOrphans(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.CreateSkill("placeholder", map[string]string{
		"SKILL.md": "# Placeholder",
	})
	targetPath := sb.CreateTarget("claude")

	// Create extras source with 1 file
	sourceRoot := filepath.Dir(sb.SourcePath)
	rulesSource := filepath.Join(sourceRoot, "rules")
	os.MkdirAll(rulesSource, 0755)
	os.WriteFile(filepath.Join(rulesSource, "keep.md"), []byte("# Keep"), 0644)

	// Create extras target with an orphan symlink pointing to non-existent source
	rulesTarget := filepath.Join(sb.Home, ".claude", "rules")
	os.MkdirAll(rulesTarget, 0755)

	orphanSource := filepath.Join(rulesSource, "deleted.md")
	orphanLink := filepath.Join(rulesTarget, "deleted.md")
	os.Symlink(orphanSource, orphanLink)

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets:
  claude:
    path: ` + targetPath + `
extras:
  - name: rules
    targets:
      - path: ` + rulesTarget + `
`)

	result := sb.RunCLI("sync", "extras")

	result.AssertSuccess(t)

	// Verify orphan is removed
	if sb.FileExists(orphanLink) {
		t.Error("orphan symlink should have been pruned")
	}

	// Verify real file is synced
	keepLink := filepath.Join(rulesTarget, "keep.md")
	if !sb.IsSymlink(keepLink) {
		t.Error("keep.md should be synced as a symlink")
	}
}

func TestSync_AllFlag(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	// Setup skills
	sb.CreateSkill("my-skill", map[string]string{
		"SKILL.md": "# My Skill\n\nDescription here.",
	})
	targetPath := sb.CreateTarget("claude")

	// Setup extras
	sourceRoot := filepath.Dir(sb.SourcePath)
	rulesSource := filepath.Join(sourceRoot, "rules")
	os.MkdirAll(rulesSource, 0755)
	os.WriteFile(filepath.Join(rulesSource, "coding.md"), []byte("# Coding"), 0644)

	rulesTarget := filepath.Join(sb.Home, ".claude", "rules")
	os.MkdirAll(rulesTarget, 0755)

	sb.WriteConfig(`source: ` + sb.SourcePath + `
mode: merge
targets:
  claude:
    path: ` + targetPath + `
extras:
  - name: rules
    targets:
      - path: ` + rulesTarget + `
`)

	result := sb.RunCLI("sync", "--all")

	result.AssertSuccess(t)

	// Verify skill sync happened
	result.AssertAnyOutputContains(t, "merged")

	// Verify extras sync happened
	result.AssertAnyOutputContains(t, "Rules")

	// Verify skill symlink
	if !sb.IsSymlink(filepath.Join(targetPath, "my-skill")) {
		t.Error("skill should be synced to target")
	}

	// Verify extras symlink
	if !sb.IsSymlink(filepath.Join(rulesTarget, "coding.md")) {
		t.Error("extras rule should be synced to target")
	}
}

func TestSyncExtras_SourceNotExist(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.CreateSkill("placeholder", map[string]string{
		"SKILL.md": "# Placeholder",
	})
	targetPath := sb.CreateTarget("claude")

	// Config with extras pointing to non-existent source directory
	// The source name "nonexistent" resolves to ~/.config/skillshare/nonexistent
	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets:
  claude:
    path: ` + targetPath + `
extras:
  - name: nonexistent
    targets:
      - path: ` + filepath.Join(sb.Home, ".claude", "rules") + `
`)

	result := sb.RunCLI("sync", "extras")

	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, "does not exist")
}
