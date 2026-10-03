//go:build !online

package integration

import (
	"os"
	"path/filepath"
	"testing"

	"skillshare/internal/install"
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
	result.AssertAnyOutputContains(t, "✓ Uninstall to-remove")

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
	result.AssertAnyOutputContains(t, "✓ Uninstall bye")
}

func TestUninstallProject_UpdatesConfig(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	projectRoot := sb.SetupProjectDir("claude")

	// Create remote skill with meta in centralized store
	sb.CreateProjectSkill(projectRoot, "remote", map[string]string{
		"SKILL.md": "# Remote",
	})
	skillsDir := filepath.Join(projectRoot, ".skillshare", "skills")
	metaStore := install.NewMetadataStore()
	metaStore.Set("remote", &install.MetadataEntry{Source: "org/skills/remote", Type: "github"})
	metaStore.Save(skillsDir)

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

	store, err := install.LoadMetadata(filepath.Join(projectRoot, ".skillshare", "skills"))
	if err != nil {
		t.Fatalf("load metadata: %v", err)
	}
	if store.Has("remote") {
		t.Error("metadata should not contain removed skill")
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
	result.AssertAnyOutputContains(t, "Dry run — nothing was written")

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

	store, err := install.LoadMetadata(filepath.Join(projectRoot, ".skillshare", "skills"))
	if err != nil {
		t.Fatalf("load metadata: %v", err)
	}
	if store.Has("skill-a") || store.Has("skill-b") {
		t.Error("metadata should not contain removed skills")
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

func TestUninstallProject_TrackedRepo_GitStatusErrorBlocksUninstall(t *testing.T) {
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
	result.AssertFailure(t)
	result.AssertAnyOutputContains(t, "failed to check git status")

	if !sb.FileExists(repoDir) {
		t.Error("tracked repo must stay in place when git status cannot be read")
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
	// Write metadata directly
	skillsDir := filepath.Join(projectRoot, ".skillshare", "skills")
	store := install.NewMetadataStore()
	store.Set("skill-a", &install.MetadataEntry{Source: "github.com/org/repo/skill-a", Group: "mygroup"})
	store.Set("skill-b", &install.MetadataEntry{Source: "github.com/org/repo/skill-b", Group: "mygroup"})
	store.Set("skill-c", &install.MetadataEntry{Source: "github.com/org/repo/skill-c", Group: "other"})
	store.Save(skillsDir)

	result := sb.RunCLIInDir(projectRoot, "uninstall", "mygroup", "--force", "-p")
	result.AssertSuccess(t)

	// Group directory should be removed from disk
	if sb.FileExists(filepath.Join(projectRoot, ".skillshare", "skills", "mygroup")) {
		t.Error("mygroup directory should be removed")
	}

	// Metadata should no longer contain mygroup skills
	store2, err := install.LoadMetadata(skillsDir)
	if err != nil {
		t.Fatalf("load metadata: %v", err)
	}
	if store2.Has("skill-a") {
		t.Error("metadata should not contain skill-a after group uninstall")
	}
	if store2.Has("skill-b") {
		t.Error("metadata should not contain skill-b after group uninstall")
	}
	if !store2.Has("skill-c") {
		t.Error("metadata should still contain skill-c from other group")
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
	// Write metadata directly to .metadata.json
	skillsDir := filepath.Join(projectRoot, ".skillshare", "skills")
	store := install.NewMetadataStore()
	store.Set("scan", &install.MetadataEntry{Source: "github.com/org/repo/scan", Group: "security"})
	store.Set("hardening", &install.MetadataEntry{Source: "github.com/org/repo/hardening", Group: "security"})
	store.Set("keep", &install.MetadataEntry{Source: "github.com/org/repo/keep", Group: "other"})
	store.Save(skillsDir)

	result := sb.RunCLIInDir(projectRoot, "uninstall", "security/", "--force", "-p")
	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, "✓ Uninstall security")

	store, err := install.LoadMetadata(filepath.Join(projectRoot, ".skillshare", "skills"))
	if err != nil {
		t.Fatalf("load metadata: %v", err)
	}
	if store.Has("scan") {
		t.Error("metadata should not contain scan after security/ uninstall")
	}
	if store.Has("hardening") {
		t.Error("metadata should not contain hardening after security/ uninstall")
	}
	if !store.Has("keep") {
		t.Error("metadata should still contain keep from other group")
	}
}
