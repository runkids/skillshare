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

func TestListProject_ShowsLocalAndRemote(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	projectRoot := sb.SetupProjectDir("claude")

	// Local skill (no meta)
	sb.CreateProjectSkill(projectRoot, "local-skill", map[string]string{
		"SKILL.md": "# Local",
	})

	// Remote skill (with meta)
	skillDir := sb.CreateProjectSkill(projectRoot, "remote-skill", map[string]string{
		"SKILL.md": "# Remote",
	})
	meta := map[string]interface{}{
		"source": "someone/skills/remote-skill",
		"type":   "github",
	}
	metaJSON, _ := json.Marshal(meta)
	os.WriteFile(filepath.Join(skillDir, ".skillshare-meta.json"), metaJSON, 0644)

	result := sb.RunCLIInDir(projectRoot, "list", "-p")
	result.AssertSuccess(t)
	result.AssertOutputContains(t, "local-skill")
	result.AssertOutputContains(t, "local")
	result.AssertOutputContains(t, "remote-skill")
	result.AssertOutputContains(t, "remote")
}

func TestListProject_Empty(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	projectRoot := sb.SetupProjectDir("claude")

	result := sb.RunCLIInDir(projectRoot, "list", "-p")
	result.AssertSuccess(t)
	result.AssertOutputContains(t, "No skills installed")
}

func TestListProject_TrackedRepo_ShowsSkills(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	projectRoot := sb.SetupProjectDir("claude")

	// Simulate a tracked repo with skills inside hidden directories (like openai/skills)
	skillsDir := filepath.Join(projectRoot, ".skillshare", "skills")
	for _, skill := range []struct{ dir, content string }{
		{filepath.Join(skillsDir, "_openai-skills", "skills", ".curated", "pdf"), "# PDF"},
		{filepath.Join(skillsDir, "_openai-skills", "skills", ".curated", "figma"), "# Figma"},
	} {
		os.MkdirAll(skill.dir, 0755)
		os.WriteFile(filepath.Join(skill.dir, "SKILL.md"), []byte(skill.content), 0644)
	}

	result := sb.RunCLIInDir(projectRoot, "list", "-p")
	result.AssertSuccess(t)
	result.AssertOutputContains(t, "pdf")
	result.AssertOutputContains(t, "figma")
	result.AssertOutputContains(t, "tracked")
}

func TestListProject_AutoDetectsMode(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	projectRoot := sb.SetupProjectDir("claude")
	sb.CreateProjectSkill(projectRoot, "skill", map[string]string{"SKILL.md": "# S"})

	result := sb.RunCLIInDir(projectRoot, "list")
	result.AssertSuccess(t)
	result.AssertOutputContains(t, "Installed skills (project)")
}

func TestListProject_PartialInit_RepairsMissingConfig(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	projectRoot := filepath.Join(sb.Root, "project")
	if err := os.MkdirAll(filepath.Join(projectRoot, ".skillshare", "skills"), 0755); err != nil {
		t.Fatalf("mkdir partial project skills dir: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(projectRoot, ".claude", "skills"), 0755); err != nil {
		t.Fatalf("mkdir project target dir: %v", err)
	}

	result := sb.RunCLIInDir(projectRoot, "list", "-p")
	result.AssertSuccess(t)
	result.AssertOutputContains(t, "No skills installed")

	cfgPath := filepath.Join(projectRoot, ".skillshare", "config.yaml")
	if !sb.FileExists(cfgPath) {
		t.Fatalf("expected repaired config file at %s", cfgPath)
	}
	cfg := sb.ReadFile(cfgPath)
	if !strings.Contains(cfg, "claude") {
		t.Fatalf("expected repaired config to include detected claude target, got:\n%s", cfg)
	}
}
