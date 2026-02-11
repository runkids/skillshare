//go:build !online

package integration

import (
	"os"
	"path/filepath"
	"testing"

	"skillshare/internal/testutil"
)

func TestInstall_Into_Local(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	targetPath := sb.CreateTarget("claude")
	sb.WriteConfig(`source: ` + sb.SourcePath + `
mode: merge
targets:
  claude:
    path: ` + targetPath + `
`)

	// Create a local skill to install
	localSkill := filepath.Join(sb.Root, "pdf-skill")
	os.MkdirAll(localSkill, 0755)
	os.WriteFile(filepath.Join(localSkill, "SKILL.md"), []byte("# PDF Skill"), 0644)

	// Install with --into frontend
	result := sb.RunCLI("install", localSkill, "--into", "frontend")
	result.AssertSuccess(t)
	result.AssertOutputContains(t, "Into")
	result.AssertOutputContains(t, "frontend")

	// Verify skill was installed into subdirectory
	if !sb.FileExists(filepath.Join(sb.SourcePath, "frontend", "pdf-skill", "SKILL.md")) {
		t.Error("skill should be installed to source/frontend/pdf-skill/")
	}

	// Sync and verify flat name
	syncResult := sb.RunCLI("sync")
	syncResult.AssertSuccess(t)

	expectedLink := filepath.Join(targetPath, "frontend__pdf-skill")
	if !sb.IsSymlink(expectedLink) {
		t.Errorf("expected symlink %s to exist", expectedLink)
	}
}

func TestInstall_Into_MultiLevel(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)

	// Create a local skill
	localSkill := filepath.Join(sb.Root, "ui-skill")
	os.MkdirAll(localSkill, 0755)
	os.WriteFile(filepath.Join(localSkill, "SKILL.md"), []byte("# UI Skill"), 0644)

	// Install with multi-level --into
	result := sb.RunCLI("install", localSkill, "--into", "frontend/vue")
	result.AssertSuccess(t)

	// Verify nested directory was created
	if !sb.FileExists(filepath.Join(sb.SourcePath, "frontend", "vue", "ui-skill", "SKILL.md")) {
		t.Error("skill should be installed to source/frontend/vue/ui-skill/")
	}
}

func TestInstall_Into_Validation(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)

	localSkill := filepath.Join(sb.Root, "test-skill")
	os.MkdirAll(localSkill, 0755)
	os.WriteFile(filepath.Join(localSkill, "SKILL.md"), []byte("# Test"), 0644)

	// Test absolute path rejection
	result := sb.RunCLI("install", localSkill, "--into", "/absolute/path")
	result.AssertFailure(t)
	result.AssertAnyOutputContains(t, "must be relative")

	// Test path traversal rejection
	result = sb.RunCLI("install", localSkill, "--into", "../escape")
	result.AssertFailure(t)
	result.AssertAnyOutputContains(t, "invalid segment")

	// Test dot rejection
	result = sb.RunCLI("install", localSkill, "--into", ".")
	result.AssertFailure(t)
	result.AssertAnyOutputContains(t, "invalid segment")

	// Test empty segment rejection (double slash)
	result = sb.RunCLI("install", localSkill, "--into", "a//b")
	result.AssertFailure(t)
	result.AssertAnyOutputContains(t, "invalid segment")
}

func TestInstallProject_Into(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	projectRoot := sb.SetupProjectDir("claude-code")

	// Create a source skill to install
	sourceSkill := filepath.Join(sb.Root, "my-skill")
	os.MkdirAll(sourceSkill, 0755)
	os.WriteFile(filepath.Join(sourceSkill, "SKILL.md"), []byte("---\nname: my-skill\n---\n# My Skill"), 0644)

	// Install with --into in project mode
	result := sb.RunCLIInDir(projectRoot, "install", sourceSkill, "--into", "tools", "-p")
	result.AssertSuccess(t)

	// Verify skill was installed into the subdirectory
	skillPath := filepath.Join(projectRoot, ".skillshare", "skills", "tools", "my-skill", "SKILL.md")
	if !sb.FileExists(skillPath) {
		t.Error("skill should be installed to .skillshare/skills/tools/my-skill/")
	}

	// Verify .gitignore entry includes the nested path
	gitignorePath := filepath.Join(projectRoot, ".skillshare", ".gitignore")
	if !sb.FileExists(gitignorePath) {
		t.Fatal(".skillshare/.gitignore should exist")
	}
	content := sb.ReadFile(gitignorePath)
	if !contains(content, "skills/tools/my-skill/") {
		t.Errorf(".gitignore should contain 'skills/tools/my-skill/', got:\n%s", content)
	}
}

func TestInstallProject_Into_NoSource_Rejected(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	projectRoot := sb.SetupProjectDir("claude-code")

	// --into without source should fail
	result := sb.RunCLIInDir(projectRoot, "install", "--into", "tools", "-p")
	result.AssertFailure(t)
	result.AssertAnyOutputContains(t, "--into requires a source")
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(s) > 0 && containsSubstring(s, sub))
}

func containsSubstring(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
