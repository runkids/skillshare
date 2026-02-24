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

	// Write config and registry with the skill
	sb.WriteProjectConfig(projectRoot, `targets:
  - claude
`)
	os.WriteFile(filepath.Join(projectRoot, ".skillshare", "registry.yaml"), []byte(`skills:
  - name: remote
    source: org/skills/remote
`), 0644)

	result := sb.RunCLIInDir(projectRoot, "uninstall", "remote", "--force", "-p")
	result.AssertSuccess(t)

	registryContent := sb.ReadFile(filepath.Join(projectRoot, ".skillshare", "registry.yaml"))
	if strings.Contains(registryContent, "remote") {
		t.Error("registry should not contain removed skill")
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
`)
	os.WriteFile(filepath.Join(projectRoot, ".skillshare", "registry.yaml"), []byte(`skills:
  - name: skill-a
    source: local
  - name: skill-b
    source: local
`), 0644)

	result := sb.RunCLIInDir(projectRoot, "uninstall", "skill-a", "skill-b", "--force", "-p")
	result.AssertSuccess(t)

	if sb.FileExists(filepath.Join(projectRoot, ".skillshare", "skills", "skill-a")) {
		t.Error("skill-a should be removed")
	}
	if sb.FileExists(filepath.Join(projectRoot, ".skillshare", "skills", "skill-b")) {
		t.Error("skill-b should be removed")
	}

	registryContent := sb.ReadFile(filepath.Join(projectRoot, ".skillshare", "registry.yaml"))
	if strings.Contains(registryContent, "skill-a") || strings.Contains(registryContent, "skill-b") {
		t.Error("registry should not contain removed skills")
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

func TestUninstallProject_TrackedRepo_GitStatusErrorWarnsAndContinues(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	projectRoot := sb.SetupProjectDir("claude")

	repoDir := sb.CreateProjectSkill(projectRoot, "_broken-repo", map[string]string{
		"SKILL.md": "# Broken Repo",
	})

	// Mark as tracked for uninstall resolution, but keep it invalid so `git status` fails.
	if err := os.MkdirAll(filepath.Join(repoDir, ".git"), 0755); err != nil {
		t.Fatalf("failed to create fake .git dir: %v", err)
	}

	result := sb.RunCLIInDirWithInput(projectRoot, "y\n", "uninstall", "broken-repo", "-p")
	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, "Could not check git status")

	if sb.FileExists(repoDir) {
		t.Error("tracked repo should still be uninstalled when git status check fails")
	}
}

// TestUninstallProject_GroupDir_RemovesConfigEntries verifies that uninstalling
// a group directory removes all member skills from the project config.yaml.
func TestUninstallProject_GroupDir_RemovesConfigEntries(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	projectRoot := sb.SetupProjectDir("claude")

	sb.CreateProjectSkill(projectRoot, "mygroup/skill-a", map[string]string{"SKILL.md": "# A"})
	sb.CreateProjectSkill(projectRoot, "mygroup/skill-b", map[string]string{"SKILL.md": "# B"})
	sb.CreateProjectSkill(projectRoot, "other/skill-c", map[string]string{"SKILL.md": "# C"})

	sb.WriteProjectConfig(projectRoot, `targets:
  - claude
`)
	os.WriteFile(filepath.Join(projectRoot, ".skillshare", "registry.yaml"), []byte(`skills:
  - name: skill-a
    source: github.com/org/repo/skill-a
    group: mygroup
  - name: skill-b
    source: github.com/org/repo/skill-b
    group: mygroup
  - name: skill-c
    source: github.com/org/repo/skill-c
    group: other
`), 0644)

	result := sb.RunCLIInDir(projectRoot, "uninstall", "mygroup", "--force", "-p")
	result.AssertSuccess(t)

	// Group directory should be removed from disk
	if sb.FileExists(filepath.Join(projectRoot, ".skillshare", "skills", "mygroup")) {
		t.Error("mygroup directory should be removed")
	}

	// Registry should no longer contain mygroup skills
	registryContent := sb.ReadFile(filepath.Join(projectRoot, ".skillshare", "registry.yaml"))
	if strings.Contains(registryContent, "skill-a") {
		t.Error("registry should not contain skill-a after group uninstall")
	}
	if strings.Contains(registryContent, "skill-b") {
		t.Error("registry should not contain skill-b after group uninstall")
	}
	if !strings.Contains(registryContent, "skill-c") {
		t.Error("registry should still contain skill-c from other group")
	}
}

func TestUninstallProject_GroupDirWithTrailingSlash_RemovesConfigEntries(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	projectRoot := sb.SetupProjectDir("claude")

	sb.CreateProjectSkill(projectRoot, "security/scan", map[string]string{"SKILL.md": "# Scan"})
	sb.CreateProjectSkill(projectRoot, "security/hardening", map[string]string{"SKILL.md": "# Hardening"})
	sb.CreateProjectSkill(projectRoot, "other/keep", map[string]string{"SKILL.md": "# Keep"})

	sb.WriteProjectConfig(projectRoot, `targets:
  - claude
`)
	os.WriteFile(filepath.Join(projectRoot, ".skillshare", "registry.yaml"), []byte(`skills:
  - name: scan
    source: github.com/org/repo/scan
    group: security
  - name: hardening
    source: github.com/org/repo/hardening
    group: security
  - name: keep
    source: github.com/org/repo/keep
    group: other
`), 0644)

	result := sb.RunCLIInDir(projectRoot, "uninstall", "security/", "--force", "-p")
	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, "Uninstalled group: security")

	registryContent := sb.ReadFile(filepath.Join(projectRoot, ".skillshare", "registry.yaml"))
	if strings.Contains(registryContent, "scan") {
		t.Error("registry should not contain scan after security/ uninstall")
	}
	if strings.Contains(registryContent, "hardening") {
		t.Error("registry should not contain hardening after security/ uninstall")
	}
	if !strings.Contains(registryContent, "keep") {
		t.Error("registry should still contain keep from other group")
	}
}
