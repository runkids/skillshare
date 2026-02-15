//go:build !online

package integration

import (
	"os"
	"path/filepath"
	"testing"

	"skillshare/internal/testutil"
)

func TestCollectProject_FindsLocalSkills(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	projectRoot := sb.SetupProjectDir("claude")

	// Place a local (non-symlinked) skill in the target
	targetSkillDir := filepath.Join(projectRoot, ".claude", "skills", "local-skill")
	os.MkdirAll(targetSkillDir, 0755)
	os.WriteFile(filepath.Join(targetSkillDir, "SKILL.md"), []byte("# Local Skill"), 0644)

	result := sb.RunCLIInDir(projectRoot, "collect", "-p", "--dry-run")

	result.AssertSuccess(t)
	result.AssertOutputContains(t, "local-skill")
}

func TestCollectProject_DryRun(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	projectRoot := sb.SetupProjectDir("claude")
	sourcePath := filepath.Join(projectRoot, ".skillshare", "skills")

	// Place a local skill in the target
	targetSkillDir := filepath.Join(projectRoot, ".claude", "skills", "dry-skill")
	os.MkdirAll(targetSkillDir, 0755)
	os.WriteFile(filepath.Join(targetSkillDir, "SKILL.md"), []byte("# Dry Skill"), 0644)

	result := sb.RunCLIInDir(projectRoot, "collect", "-p", "--dry-run")

	result.AssertSuccess(t)
	result.AssertOutputContains(t, "Dry run")

	// Verify skill was NOT copied to source
	if sb.FileExists(filepath.Join(sourcePath, "dry-skill", "SKILL.md")) {
		t.Error("dry-run should not copy skills to source")
	}
}

func TestCollectProject_Force(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	projectRoot := sb.SetupProjectDir("claude")
	sourcePath := filepath.Join(projectRoot, ".skillshare", "skills")

	// Create existing skill in source
	existingDir := filepath.Join(sourcePath, "my-skill")
	os.MkdirAll(existingDir, 0755)
	os.WriteFile(filepath.Join(existingDir, "SKILL.md"), []byte("# Old Version"), 0644)

	// Place updated version in target
	targetSkillDir := filepath.Join(projectRoot, ".claude", "skills", "my-skill")
	os.MkdirAll(targetSkillDir, 0755)
	os.WriteFile(filepath.Join(targetSkillDir, "SKILL.md"), []byte("# New Version"), 0644)

	result := sb.RunCLIInDir(projectRoot, "collect", "-p", "--force")

	result.AssertSuccess(t)
	result.AssertOutputContains(t, "copied to source")

	// Verify source was overwritten
	content, err := os.ReadFile(filepath.Join(sourcePath, "my-skill", "SKILL.md"))
	if err != nil {
		t.Fatalf("failed to read collected skill: %v", err)
	}
	if string(content) != "# New Version" {
		t.Errorf("source should be overwritten, got: %s", string(content))
	}
}

func TestCollectProject_NoLocalSkills(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	projectRoot := sb.SetupProjectDir("claude")

	// Create a skill in source and symlink it (no local skills)
	sb.CreateProjectSkill(projectRoot, "synced-skill", map[string]string{"SKILL.md": "# Synced"})
	targetDir := filepath.Join(projectRoot, ".claude", "skills")
	os.MkdirAll(targetDir, 0755)
	os.Symlink(
		filepath.Join(projectRoot, ".skillshare", "skills", "synced-skill"),
		filepath.Join(targetDir, "synced-skill"),
	)

	result := sb.RunCLIInDir(projectRoot, "collect", "-p")

	result.AssertSuccess(t)
	result.AssertOutputContains(t, "No local skills")
}
