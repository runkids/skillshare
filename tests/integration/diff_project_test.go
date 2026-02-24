//go:build !online

package integration

import (
	"os"
	"path/filepath"
	"testing"

	"skillshare/internal/testutil"
)

func TestDiffProject_InSync(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	projectRoot := sb.SetupProjectDir("claude")

	// Create skill in project source
	sb.CreateProjectSkill(projectRoot, "synced-skill", map[string]string{
		"SKILL.md": "# Synced Skill",
	})

	// Symlink from target to source (simulates synced state)
	targetDir := filepath.Join(projectRoot, ".claude", "skills")
	os.MkdirAll(targetDir, 0755)
	os.Symlink(
		filepath.Join(projectRoot, ".skillshare", "skills", "synced-skill"),
		filepath.Join(targetDir, "synced-skill"),
	)

	result := sb.RunCLIInDir(projectRoot, "diff", "-p")
	result.AssertSuccess(t)
	result.AssertOutputContains(t, "claude")
}

func TestDiffProject_SkillOnlyInSource(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	projectRoot := sb.SetupProjectDir("claude")

	// Create skill in source but NOT in target
	sb.CreateProjectSkill(projectRoot, "new-skill", map[string]string{
		"SKILL.md": "# New Skill",
	})

	result := sb.RunCLIInDir(projectRoot, "diff", "-p")
	result.AssertSuccess(t)
	result.AssertOutputContains(t, "claude")
}
