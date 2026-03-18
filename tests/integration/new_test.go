//go:build !online

package integration

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"skillshare/internal/testutil"
)

func TestNew_CreatesSkill(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)

	result := sb.RunCLI("new", "my-skill")

	result.AssertSuccess(t)
	result.AssertOutputContains(t, "Created")
	result.AssertOutputContains(t, "my-skill")

	// Verify SKILL.md was created
	skillFile := filepath.Join(sb.SourcePath, "my-skill", "SKILL.md")
	if !sb.FileExists(skillFile) {
		t.Error("SKILL.md should be created")
	}

	// Verify content
	content, err := os.ReadFile(skillFile)
	if err != nil {
		t.Fatalf("failed to read SKILL.md: %v", err)
	}
	if !strings.Contains(string(content), "name: my-skill") {
		t.Error("SKILL.md should contain skill name")
	}
	if !strings.Contains(string(content), "# My Skill") {
		t.Error("SKILL.md should contain title")
	}
}

func TestNew_DryRun_NoChanges(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)

	result := sb.RunCLI("new", "dry-run-skill", "--dry-run")

	result.AssertSuccess(t)
	result.AssertOutputContains(t, "dry-run")
	result.AssertOutputContains(t, "Would create")

	// Verify skill was NOT created
	skillDir := filepath.Join(sb.SourcePath, "dry-run-skill")
	if sb.FileExists(skillDir) {
		t.Error("skill should not be created in dry-run mode")
	}
}

func TestNew_AlreadyExists_Errors(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	// Create existing skill
	sb.CreateSkill("existing-skill", map[string]string{"SKILL.md": "# Existing"})

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)

	result := sb.RunCLI("new", "existing-skill")

	result.AssertFailure(t)
	result.AssertAnyOutputContains(t, "already exists")
}

func TestNew_NoArgs_ShowsHelp(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)

	result := sb.RunCLI("new")

	result.AssertFailure(t)
	result.AssertAnyOutputContains(t, "skill name is required")
}

func TestNew_InvalidName_Errors(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)

	// Test uppercase
	result := sb.RunCLI("new", "MySkill")
	result.AssertFailure(t)
	result.AssertAnyOutputContains(t, "invalid skill name")

	// Test with spaces
	result = sb.RunCLI("new", "my skill")
	result.AssertFailure(t)
}

func TestNew_Help_ShowsUsage(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	result := sb.RunCLI("new", "--help")

	result.AssertSuccess(t)
	result.AssertOutputContains(t, "Usage:")
	result.AssertOutputContains(t, "--dry-run")
}

func TestNew_HyphenatedName_Works(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)

	result := sb.RunCLI("new", "code-review")

	result.AssertSuccess(t)
	result.AssertOutputContains(t, "Created")

	// Verify title is properly converted
	skillFile := filepath.Join(sb.SourcePath, "code-review", "SKILL.md")
	content, err := os.ReadFile(skillFile)
	if err != nil {
		t.Fatalf("failed to read SKILL.md: %v", err)
	}
	if !strings.Contains(string(content), "# Code Review") {
		t.Error("SKILL.md should have Title Case heading")
	}
}

func TestNew_PatternFlag_CreatesTemplate(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)

	result := sb.RunCLI("new", "my-reviewer", "-P", "reviewer")

	result.AssertSuccess(t)
	result.AssertOutputContains(t, "Created")

	// Verify SKILL.md contains pattern frontmatter and reviewer body
	skillFile := filepath.Join(sb.SourcePath, "my-reviewer", "SKILL.md")
	content, err := os.ReadFile(skillFile)
	if err != nil {
		t.Fatalf("failed to read SKILL.md: %v", err)
	}
	text := string(content)
	if !strings.Contains(text, "pattern: reviewer") {
		t.Error("SKILL.md should contain 'pattern: reviewer' in frontmatter")
	}
	if !strings.Contains(text, "review-checklist.md") {
		t.Error("SKILL.md should reference review-checklist.md")
	}

	// Verify scaffold directories
	refsDir := filepath.Join(sb.SourcePath, "my-reviewer", "references")
	if !sb.FileExists(refsDir) {
		t.Error("references/ directory should be created")
	}
	gitkeep := filepath.Join(refsDir, ".gitkeep")
	if !sb.FileExists(gitkeep) {
		t.Error("references/.gitkeep should be created")
	}
}

func TestNew_PatternFlag_Pipeline_ScaffoldDirs(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)

	result := sb.RunCLI("new", "my-pipeline", "-P", "pipeline")

	result.AssertSuccess(t)

	// Verify all three scaffold directories exist
	for _, dir := range []string{"references", "assets", "scripts"} {
		dirPath := filepath.Join(sb.SourcePath, "my-pipeline", dir)
		if !sb.FileExists(dirPath) {
			t.Errorf("%s/ directory should be created", dir)
		}
	}
}

func TestNew_PatternNone_NoPatternField(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)

	result := sb.RunCLI("new", "plain-skill", "-P", "none")

	result.AssertSuccess(t)

	// Verify SKILL.md does NOT contain pattern field
	skillFile := filepath.Join(sb.SourcePath, "plain-skill", "SKILL.md")
	content, err := os.ReadFile(skillFile)
	if err != nil {
		t.Fatalf("failed to read SKILL.md: %v", err)
	}
	if strings.Contains(string(content), "pattern:") {
		t.Error("SKILL.md should NOT contain 'pattern:' when using -P none")
	}
}

func TestNew_InvalidPattern_Errors(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)

	result := sb.RunCLI("new", "my-skill", "-P", "bogus")

	result.AssertFailure(t)
	result.AssertAnyOutputContains(t, "unknown pattern")
}

func TestNew_DryRun_WithPattern_ShowsDirs(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)

	result := sb.RunCLI("new", "my-skill", "-P", "pipeline", "--dry-run")

	result.AssertSuccess(t)
	result.AssertOutputContains(t, "Would create")
	result.AssertOutputContains(t, "scripts")

	// Verify skill directory was NOT actually created
	skillDir := filepath.Join(sb.SourcePath, "my-skill")
	if sb.FileExists(skillDir) {
		t.Error("skill should not be created in dry-run mode")
	}
}
