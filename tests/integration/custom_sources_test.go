//go:build !online

package integration

import (
	"os"
	"path/filepath"
	"testing"

	"skillshare/internal/testutil"
)

func TestSyncProject_CustomSkillsSource(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	projectRoot := sb.SetupProjectDir("claude")

	// Create skill in custom source directory (not .skillshare/skills)
	customSkillsDir := filepath.Join(projectRoot, "docs", "skills")
	os.MkdirAll(filepath.Join(customSkillsDir, "custom-skill"), 0755)
	os.WriteFile(filepath.Join(customSkillsDir, "custom-skill", "SKILL.md"), []byte("# Custom"), 0644)

	sb.WriteProjectConfig(projectRoot, `sources:
  skills: ./docs/skills
targets:
  - claude
`)

	result := sb.RunCLIInDir(projectRoot, "sync", "-p")
	result.AssertSuccess(t)

	link := filepath.Join(projectRoot, ".claude", "skills", "custom-skill")
	if !sb.IsSymlink(link) {
		t.Error("sync should create symlink from custom skills source")
	}
}

func TestSyncProject_CustomSkillsSource_DefaultFallback(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	projectRoot := sb.SetupProjectDir("claude")
	sb.CreateProjectSkill(projectRoot, "default-skill", map[string]string{
		"SKILL.md": "# Default",
	})

	result := sb.RunCLIInDir(projectRoot, "sync", "-p")
	result.AssertSuccess(t)

	link := filepath.Join(projectRoot, ".claude", "skills", "default-skill")
	if !sb.IsSymlink(link) {
		t.Error("sync should still work with default source when sources not configured")
	}
}

func TestStatusProject_CustomSkillsSource(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	projectRoot := sb.SetupProjectDir("claude")

	customSkillsDir := filepath.Join(projectRoot, "docs", "skills")
	os.MkdirAll(filepath.Join(customSkillsDir, "my-skill"), 0755)
	os.WriteFile(filepath.Join(customSkillsDir, "my-skill", "SKILL.md"), []byte("# My Skill"), 0644)

	sb.WriteProjectConfig(projectRoot, `sources:
  skills: ./docs/skills
targets:
  - claude
`)

	result := sb.RunCLIInDir(projectRoot, "status", "-p")
	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, "docs/skills")
}

func TestListProject_CustomSkillsSource(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	projectRoot := sb.SetupProjectDir("claude")

	customSkillsDir := filepath.Join(projectRoot, "docs", "skills")
	os.MkdirAll(filepath.Join(customSkillsDir, "listed-skill"), 0755)
	os.WriteFile(filepath.Join(customSkillsDir, "listed-skill", "SKILL.md"), []byte("---\nname: listed-skill\ndescription: test\n---\n# Listed"), 0644)

	sb.WriteProjectConfig(projectRoot, `sources:
  skills: ./docs/skills
targets:
  - claude
`)

	result := sb.RunCLIInDir(projectRoot, "list", "-p", "--no-tui")
	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, "listed-skill")
}
