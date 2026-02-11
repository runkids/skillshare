//go:build !online

package integration

import (
	"path/filepath"
	"testing"

	"skillshare/internal/testutil"
)

func TestUninstall_ExistingSkill_Removes(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	// Create existing skill in source
	sb.CreateSkill("my-skill", map[string]string{"SKILL.md": "# My Skill"})

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)

	// Uninstall with --force to skip confirmation
	result := sb.RunCLI("uninstall", "my-skill", "--force")

	result.AssertSuccess(t)
	result.AssertOutputContains(t, "Uninstalled")
	result.AssertOutputContains(t, "my-skill")

	// Verify skill was removed
	skillPath := filepath.Join(sb.SourcePath, "my-skill")
	if sb.FileExists(skillPath) {
		t.Error("skill should be removed after uninstall")
	}
}

func TestUninstall_NotFound_Errors(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)

	result := sb.RunCLI("uninstall", "nonexistent-skill", "--force")

	result.AssertFailure(t)
	result.AssertAnyOutputContains(t, "not found")
}

func TestUninstall_DryRun_NoChanges(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	// Create existing skill in source
	sb.CreateSkill("dry-run-skill", map[string]string{"SKILL.md": "# Dry Run"})

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)

	result := sb.RunCLI("uninstall", "dry-run-skill", "--dry-run")

	result.AssertSuccess(t)
	result.AssertOutputContains(t, "dry-run")
	result.AssertOutputContains(t, "would move to trash")

	// Verify skill was NOT removed
	skillPath := filepath.Join(sb.SourcePath, "dry-run-skill")
	if !sb.FileExists(skillPath) {
		t.Error("skill should not be removed in dry-run mode")
	}
}

func TestUninstall_Force_SkipsConfirm(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	// Create existing skill
	sb.CreateSkill("force-skill", map[string]string{"SKILL.md": "# Force"})

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)

	// Without --force, would wait for stdin (but RunCLI provides no input)
	// With --force, should complete immediately
	result := sb.RunCLI("uninstall", "force-skill", "--force")

	result.AssertSuccess(t)
	result.AssertOutputContains(t, "Uninstalled")

	// Verify skill was removed
	skillPath := filepath.Join(sb.SourcePath, "force-skill")
	if sb.FileExists(skillPath) {
		t.Error("skill should be removed with --force")
	}
}

func TestUninstall_Help_ShowsUsage(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	result := sb.RunCLI("uninstall", "--help")

	result.AssertSuccess(t)
	result.AssertOutputContains(t, "Usage:")
	result.AssertOutputContains(t, "--force")
	result.AssertOutputContains(t, "--dry-run")
}

func TestUninstall_NoArgs_ShowsHelp(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)

	result := sb.RunCLI("uninstall")

	result.AssertFailure(t)
	result.AssertAnyOutputContains(t, "skill name is required")
}

func TestUninstall_NestedSkill_ResolvesByBasename(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	// Create nested skill in organizational folder
	sb.CreateSkill("frontend/vue/vue-best-practices", map[string]string{
		"SKILL.md": "# Vue Best Practices",
	})

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)

	// Uninstall by short name (basename only)
	result := sb.RunCLI("uninstall", "vue-best-practices", "--force")
	result.AssertSuccess(t)
	result.AssertOutputContains(t, "Uninstalled")

	// Verify nested skill was removed
	skillPath := filepath.Join(sb.SourcePath, "frontend", "vue", "vue-best-practices")
	if sb.FileExists(skillPath) {
		t.Error("nested skill should be removed after uninstall by basename")
	}
}

func TestUninstall_NestedSkill_FullPathAlsoWorks(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.CreateSkill("backend/go-patterns", map[string]string{
		"SKILL.md": "# Go Patterns",
	})

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)

	// Uninstall by full nested path
	result := sb.RunCLI("uninstall", "backend/go-patterns", "--force")
	result.AssertSuccess(t)
	result.AssertOutputContains(t, "Uninstalled")
}

func TestUninstall_ShowsMetadata(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	// Create skill with metadata (simulating installed skill)
	sb.CreateSkill("meta-skill", map[string]string{
		"SKILL.md": "# Meta Skill",
		".skillshare-meta.json": `{
  "source": "github.com/user/repo",
  "type": "github",
  "installed_at": "2024-01-15T10:30:00Z"
}`,
	})

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)

	result := sb.RunCLI("uninstall", "meta-skill", "--dry-run")

	result.AssertSuccess(t)
	result.AssertOutputContains(t, "github.com/user/repo")
}
