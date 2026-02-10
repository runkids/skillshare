//go:build !online

package integration

import (
	"os"
	"path/filepath"
	"testing"

	"skillshare/internal/testutil"
)

func TestUpgrade_NoConfig_ReturnsError(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	// Remove config
	os.Remove(sb.ConfigPath)

	// Test skill upgrade only (CLI upgrade doesn't need config)
	result := sb.RunCLI("upgrade", "--skill")

	result.AssertFailure(t)
	result.AssertAnyOutputContains(t, "config not found")
}

func TestUpgrade_DryRun_DoesNotModify(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	// Create config
	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)

	skillPath := filepath.Join(sb.SourcePath, "skillshare", "SKILL.md")

	// Create existing skill
	os.MkdirAll(filepath.Dir(skillPath), 0755)
	os.WriteFile(skillPath, []byte("# Old Content"), 0644)

	// Test skill upgrade only (CLI upgrade hits GitHub API rate limits in CI)
	result := sb.RunCLI("upgrade", "--skill", "--force", "--dry-run")

	result.AssertSuccess(t)
	result.AssertOutputContains(t, "Would upgrade")

	// Verify file was not changed
	content, _ := os.ReadFile(skillPath)
	if string(content) != "# Old Content" {
		t.Error("dry-run should not modify file")
	}
}

func TestUpgrade_Force_SkipsConfirmation(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	// Create config
	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)

	skillPath := filepath.Join(sb.SourcePath, "skillshare", "SKILL.md")

	// Create existing skill
	os.MkdirAll(filepath.Dir(skillPath), 0755)
	os.WriteFile(skillPath, []byte("# Old Content"), 0644)

	// Force upgrade skill only (don't upgrade CLI during tests!)
	result := sb.RunCLI("upgrade", "--skill", "--force")

	// Should either succeed (if network available) or fail with download error
	// But should NOT ask for confirmation
	if result.ExitCode == 0 {
		result.AssertOutputContains(t, "Upgraded")
	} else {
		result.AssertAnyOutputContains(t, "download")
	}
}

func TestUpgrade_NoExistingSkill_CreatesNew(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	// Create config
	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)

	skillPath := filepath.Join(sb.SourcePath, "skillshare", "SKILL.md")

	// Ensure skill doesn't exist
	os.RemoveAll(filepath.Dir(skillPath))

	// Upgrade skill only with --force to skip prompt (don't upgrade CLI during tests!)
	result := sb.RunCLI("upgrade", "--skill", "--force")

	// Should either succeed or fail with download error
	// But should create directory
	if result.ExitCode == 0 {
		if !sb.FileExists(skillPath) {
			t.Error("skill should be created on success")
		}
	}
}

func TestUpgrade_ShowsSourceURL(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	// Create config
	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)

	result := sb.RunCLI("upgrade", "--skill", "--force", "--dry-run")

	result.AssertSuccess(t)
	// Source URL appears in the logo banner
	result.AssertOutputContains(t, "github.com/runkids/skillshare")
}

func TestUpgrade_NoSkill_PromptDeclined(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	// Create config
	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)

	skillPath := filepath.Join(sb.SourcePath, "skillshare", "SKILL.md")

	// Ensure skill doesn't exist
	os.RemoveAll(filepath.Dir(skillPath))

	// Upgrade skill only without --force → prompt defaults to N (no stdin)
	result := sb.RunCLI("upgrade", "--skill")

	result.AssertSuccess(t)
	result.AssertOutputContains(t, "skipped")

	// Verify skill was NOT created
	if sb.FileExists(skillPath) {
		t.Error("skill should NOT be created when prompt is declined")
	}
}
