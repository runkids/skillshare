//go:build !online

package integration

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"skillshare/internal/testutil"
)

func TestInstall_Into_RecordsGroupField(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)

	// Create a local skill
	localSkill := filepath.Join(sb.Root, "pdf-skill")
	os.MkdirAll(localSkill, 0755)
	os.WriteFile(filepath.Join(localSkill, "SKILL.md"), []byte("# PDF Skill"), 0644)

	// Install with --into frontend
	result := sb.RunCLI("install", localSkill, "--into", "frontend")
	result.AssertSuccess(t)

	// Verify skill was installed into subdirectory
	if !sb.FileExists(filepath.Join(sb.SourcePath, "frontend", "pdf-skill", "SKILL.md")) {
		t.Error("skill should be installed to source/frontend/pdf-skill/")
	}

	// Read registry and verify group field
	registryPath := filepath.Join(filepath.Dir(sb.ConfigPath), "registry.yaml")
	registryContent := sb.ReadFile(registryPath)
	if !strings.Contains(registryContent, "group: frontend") {
		t.Errorf("registry should contain 'group: frontend', got:\n%s", registryContent)
	}
	// Name should be the bare name, not "frontend/pdf-skill"
	if strings.Contains(registryContent, "name: frontend/pdf-skill") {
		t.Errorf("registry should NOT contain legacy slash name 'frontend/pdf-skill', got:\n%s", registryContent)
	}
	if !strings.Contains(registryContent, "name: pdf-skill") {
		t.Errorf("registry should contain bare 'name: pdf-skill', got:\n%s", registryContent)
	}
}

func TestInstall_Into_MultiLevel_RecordsGroupField(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)

	localSkill := filepath.Join(sb.Root, "ui-skill")
	os.MkdirAll(localSkill, 0755)
	os.WriteFile(filepath.Join(localSkill, "SKILL.md"), []byte("# UI Skill"), 0644)

	// Install with multi-level --into
	result := sb.RunCLI("install", localSkill, "--into", "frontend/vue")
	result.AssertSuccess(t)

	// Read registry and verify group field
	registryPath := filepath.Join(filepath.Dir(sb.ConfigPath), "registry.yaml")
	registryContent := sb.ReadFile(registryPath)
	if !strings.Contains(registryContent, "group: frontend/vue") {
		t.Errorf("registry should contain 'group: frontend/vue', got:\n%s", registryContent)
	}
	if !strings.Contains(registryContent, "name: ui-skill") {
		t.Errorf("registry should contain bare 'name: ui-skill', got:\n%s", registryContent)
	}
}

func TestInstall_ConfigBased_WithGroup(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	// Create a local skill to use as source
	sourceSkill := filepath.Join(sb.Root, "source-pdf")
	os.MkdirAll(sourceSkill, 0755)
	os.WriteFile(filepath.Join(sourceSkill, "SKILL.md"), []byte("# PDF Skill"), 0644)

	// First install normally with --into to populate config
	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)
	result := sb.RunCLI("install", sourceSkill, "--into", "frontend")
	result.AssertSuccess(t)

	// Verify skill exists
	skillPath := filepath.Join(sb.SourcePath, "frontend", "source-pdf", "SKILL.md")
	if !sb.FileExists(skillPath) {
		t.Fatal("skill should exist after initial install")
	}

	// Remove the installed skill (simulate fresh machine)
	os.RemoveAll(filepath.Join(sb.SourcePath, "frontend"))

	// Now run config-based install — this is the bug fix test
	result = sb.RunCLI("install")
	result.AssertSuccess(t)

	// Verify skill was recreated in the correct group directory
	if !sb.FileExists(skillPath) {
		t.Error("config-based install should recreate skill at frontend/source-pdf/")
	}
}

func TestInstall_LegacySlashName_BackwardCompat(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	// Write config with legacy format (name contains slash, no group field)
	// This is the format that existed before the group field was added
	sourceSkill := filepath.Join(sb.Root, "source-pdf")
	os.MkdirAll(sourceSkill, 0755)
	os.WriteFile(filepath.Join(sourceSkill, "SKILL.md"), []byte("# PDF"), 0644)

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
skills:
  - name: frontend/pdf
    source: ` + sourceSkill + `
`)

	// Config-based install with legacy slash name should still work
	result := sb.RunCLI("install")
	result.AssertSuccess(t)

	// Verify skill was installed correctly
	if !sb.FileExists(filepath.Join(sb.SourcePath, "frontend", "pdf", "SKILL.md")) {
		t.Error("legacy slash-name install should place skill at frontend/pdf/")
	}
}

func TestInstallProject_Into_RecordsGroupField(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	projectRoot := sb.SetupProjectDir("claude")

	// Create a source skill
	sourceSkill := filepath.Join(sb.Root, "my-skill")
	os.MkdirAll(sourceSkill, 0755)
	os.WriteFile(filepath.Join(sourceSkill, "SKILL.md"), []byte("---\nname: my-skill\n---\n# My Skill"), 0644)

	// Install with --into in project mode
	result := sb.RunCLIInDir(projectRoot, "install", sourceSkill, "--into", "tools", "-p")
	result.AssertSuccess(t)

	// Read project registry and verify group field
	registryPath := filepath.Join(projectRoot, ".skillshare", "registry.yaml")
	registryContent := sb.ReadFile(registryPath)
	if !strings.Contains(registryContent, "group: tools") {
		t.Errorf("project registry should contain 'group: tools', got:\n%s", registryContent)
	}
	if !strings.Contains(registryContent, "name: my-skill") {
		t.Errorf("project registry should contain bare 'name: my-skill', got:\n%s", registryContent)
	}
}
