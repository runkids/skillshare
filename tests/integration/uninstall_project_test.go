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

func TestUninstallProject_RemovesSkill(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	projectRoot := sb.SetupProjectDir("claude")
	sb.CreateProjectSkill(projectRoot, "to-remove", map[string]string{
		"SKILL.md": "# Remove Me",
	})

	result := sb.RunCLIInDirWithInput(projectRoot, "y\n", "uninstall", "to-remove", "-p")
	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, "Uninstalled")

	if sb.FileExists(filepath.Join(projectRoot, ".skillshare", "skills", "to-remove")) {
		t.Error("skill directory should be removed")
	}
}

func TestUninstallProject_Force_SkipsConfirmation(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	projectRoot := sb.SetupProjectDir("claude")
	sb.CreateProjectSkill(projectRoot, "bye", map[string]string{
		"SKILL.md": "# Bye",
	})

	result := sb.RunCLIInDir(projectRoot, "uninstall", "bye", "--force", "-p")
	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, "Uninstalled")
}

func TestUninstallProject_UpdatesConfig(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	projectRoot := sb.SetupProjectDir("claude")

	// Create remote skill with meta
	skillDir := sb.CreateProjectSkill(projectRoot, "remote", map[string]string{
		"SKILL.md": "# Remote",
	})
	meta := map[string]interface{}{"source": "org/skills/remote", "type": "github"}
	metaJSON, _ := json.Marshal(meta)
	os.WriteFile(filepath.Join(skillDir, ".skillshare-meta.json"), metaJSON, 0644)

	// Write config with the skill
	sb.WriteProjectConfig(projectRoot, `targets:
  - claude
skills:
  - name: remote
    source: org/skills/remote
`)

	result := sb.RunCLIInDir(projectRoot, "uninstall", "remote", "--force", "-p")
	result.AssertSuccess(t)

	cfg := sb.ReadFile(filepath.Join(projectRoot, ".skillshare", "config.yaml"))
	if strings.Contains(cfg, "remote") {
		t.Error("config should not contain removed skill")
	}
}

func TestUninstallProject_NotFound_Error(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	projectRoot := sb.SetupProjectDir("claude")

	result := sb.RunCLIInDir(projectRoot, "uninstall", "nonexistent", "--force", "-p")
	result.AssertFailure(t)
	result.AssertAnyOutputContains(t, "not found")
}

func TestUninstallProject_DryRun(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	projectRoot := sb.SetupProjectDir("claude")
	sb.CreateProjectSkill(projectRoot, "keep", map[string]string{
		"SKILL.md": "# Keep",
	})

	result := sb.RunCLIInDir(projectRoot, "uninstall", "keep", "--dry-run", "-p")
	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, "dry-run")

	if !sb.FileExists(filepath.Join(projectRoot, ".skillshare", "skills", "keep")) {
		t.Error("dry-run should not remove skill")
	}
}

func TestUninstallProject_MultipleSkills(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	projectRoot := sb.SetupProjectDir("claude")

	sb.CreateProjectSkill(projectRoot, "skill-a", map[string]string{"SKILL.md": "# A"})
	sb.CreateProjectSkill(projectRoot, "skill-b", map[string]string{"SKILL.md": "# B"})

	sb.WriteProjectConfig(projectRoot, `targets:
  - claude
skills:
  - name: skill-a
    source: local
  - name: skill-b
    source: local
`)

	result := sb.RunCLIInDir(projectRoot, "uninstall", "skill-a", "skill-b", "--force", "-p")
	result.AssertSuccess(t)

	if sb.FileExists(filepath.Join(projectRoot, ".skillshare", "skills", "skill-a")) {
		t.Error("skill-a should be removed")
	}
	if sb.FileExists(filepath.Join(projectRoot, ".skillshare", "skills", "skill-b")) {
		t.Error("skill-b should be removed")
	}

	cfg := sb.ReadFile(filepath.Join(projectRoot, ".skillshare", "config.yaml"))
	if strings.Contains(cfg, "skill-a") || strings.Contains(cfg, "skill-b") {
		t.Error("config should not contain removed skills")
	}
}

func TestUninstallProject_Group(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	projectRoot := sb.SetupProjectDir("claude")

	sb.CreateProjectSkill(projectRoot, "frontend/hooks", map[string]string{"SKILL.md": "# Hooks"})
	sb.CreateProjectSkill(projectRoot, "frontend/styles", map[string]string{"SKILL.md": "# Styles"})
	sb.CreateProjectSkill(projectRoot, "backend/api", map[string]string{"SKILL.md": "# API"})

	result := sb.RunCLIInDir(projectRoot, "uninstall", "--group", "frontend", "--force", "-p")
	result.AssertSuccess(t)

	if sb.FileExists(filepath.Join(projectRoot, ".skillshare", "skills", "frontend", "hooks")) {
		t.Error("frontend/hooks should be removed")
	}
	if sb.FileExists(filepath.Join(projectRoot, ".skillshare", "skills", "frontend", "styles")) {
		t.Error("frontend/styles should be removed")
	}
	if !sb.FileExists(filepath.Join(projectRoot, ".skillshare", "skills", "backend", "api")) {
		t.Error("backend/api should NOT be removed")
	}
}
