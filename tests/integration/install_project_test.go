//go:build !online

package integration

import (
	"os"
	"path/filepath"
	"testing"

	"skillshare/internal/testutil"
)

func TestInstallProject_LocalPath(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	projectRoot := sb.SetupProjectDir("claude")

	// Create a source skill to install from
	sourceSkill := filepath.Join(sb.Root, "external-skill")
	os.MkdirAll(sourceSkill, 0755)
	os.WriteFile(filepath.Join(sourceSkill, "SKILL.md"), []byte("---\nname: external-skill\n---\n# External"), 0644)

	result := sb.RunCLIInDir(projectRoot, "install", sourceSkill, "-p")
	result.AssertSuccess(t)

	// Skill name comes from directory name, not frontmatter
	if !sb.FileExists(filepath.Join(projectRoot, ".skillshare", "skills", "external-skill", "SKILL.md")) {
		t.Error("skill should be installed to .skillshare/skills/")
	}
}

func TestInstallProject_CustomName(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	projectRoot := sb.SetupProjectDir("claude")

	sourceSkill := filepath.Join(sb.Root, "my-source")
	os.MkdirAll(sourceSkill, 0755)
	os.WriteFile(filepath.Join(sourceSkill, "SKILL.md"), []byte("---\nname: original\n---\n# S"), 0644)

	result := sb.RunCLIInDir(projectRoot, "install", sourceSkill, "--name", "custom", "-p")
	result.AssertSuccess(t)

	if !sb.FileExists(filepath.Join(projectRoot, ".skillshare", "skills", "custom", "SKILL.md")) {
		t.Error("skill should be installed with custom name")
	}
}

func TestInstallProject_DryRun(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	projectRoot := sb.SetupProjectDir("claude")

	sourceSkill := filepath.Join(sb.Root, "dry-skill")
	os.MkdirAll(sourceSkill, 0755)
	os.WriteFile(filepath.Join(sourceSkill, "SKILL.md"), []byte("---\nname: dry\n---\n# D"), 0644)

	result := sb.RunCLIInDir(projectRoot, "install", sourceSkill, "--dry-run", "-p")
	result.AssertSuccess(t)

	if sb.FileExists(filepath.Join(projectRoot, ".skillshare", "skills", "dry")) {
		t.Error("dry-run should not install skill")
	}
}

func TestInstallProject_TrackRequiresGitSource(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	projectRoot := sb.SetupProjectDir("claude")

	// --track with a non-git source should fail with git-related error
	result := sb.RunCLIInDir(projectRoot, "install", "/some/path", "--track", "-p")
	result.AssertFailure(t)
	result.AssertAnyOutputContains(t, "git repository source")
}

func TestInstallProject_FromConfig_SkipsExisting(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	projectRoot := sb.SetupProjectDir("claude")

	// Create an already-installed skill
	sb.CreateProjectSkill(projectRoot, "already-here", map[string]string{
		"SKILL.md": "# Already",
	})

	// Write config referencing it
	sb.WriteProjectConfig(projectRoot, `targets:
  - claude
skills:
  - name: already-here
    source: someone/skills/already-here
`)

	// install (no args) → should skip existing
	result := sb.RunCLIInDir(projectRoot, "install", "-p")
	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, "skipped")
}

func TestInstallProject_FromConfig_EmptySkills(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	projectRoot := sb.SetupProjectDir("claude")

	result := sb.RunCLIInDir(projectRoot, "install", "-p")
	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, "No remote skills")
}

func TestInstallProject_NoSource_WithName_Errors(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	projectRoot := sb.SetupProjectDir("claude")

	result := sb.RunCLIInDir(projectRoot, "install", "-p", "--name", "custom")
	result.AssertFailure(t)
	result.AssertAnyOutputContains(t, "require a source argument")
}

func TestInstallProject_NoSource_WithExclude_Errors(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	projectRoot := sb.SetupProjectDir("claude")

	result := sb.RunCLIInDir(projectRoot, "install", "-p", "--exclude", "skip-me")
	result.AssertFailure(t)
	result.AssertAnyOutputContains(t, "require a source argument")
}
