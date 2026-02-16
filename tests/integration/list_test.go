//go:build !online

package integration

import (
	"testing"

	"skillshare/internal/testutil"
)

func TestList_ShowsInstalledSkills(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	// Create skills in source
	sb.CreateSkill("skill-one", map[string]string{"SKILL.md": "# One"})
	sb.CreateSkill("skill-two", map[string]string{"SKILL.md": "# Two"})

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)

	result := sb.RunCLI("list")

	result.AssertSuccess(t)
	result.AssertOutputContains(t, "skill-one")
	result.AssertOutputContains(t, "skill-two")
	result.AssertOutputContains(t, "Installed skills")
}

func TestList_Empty_ShowsMessage(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)

	result := sb.RunCLI("list")

	result.AssertSuccess(t)
	result.AssertOutputContains(t, "No skills installed")
}

func TestList_Verbose_ShowsDetails(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	// Create skill with metadata
	sb.CreateSkill("meta-skill", map[string]string{
		"SKILL.md": "# Meta Skill",
		".skillshare-meta.json": `{
  "source": "github.com/user/repo/path/to/skill",
  "type": "github-subdir",
  "installed_at": "2024-01-15T10:30:00Z"
}`,
	})

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)

	result := sb.RunCLI("list", "--verbose")

	result.AssertSuccess(t)
	result.AssertOutputContains(t, "meta-skill")
	result.AssertOutputContains(t, "github.com/user/repo/path/to/skill")
	result.AssertOutputContains(t, "github-subdir")
}

func TestList_TrackedRepo_HiddenDirs(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	// Simulate a tracked repo with skills inside hidden directories
	sb.CreateNestedSkill("_openai-skills/skills/.curated/pdf", map[string]string{
		"SKILL.md": "# PDF",
	})
	sb.CreateNestedSkill("_openai-skills/skills/.curated/figma", map[string]string{
		"SKILL.md": "# Figma",
	})

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)

	result := sb.RunCLI("list")

	result.AssertSuccess(t)
	result.AssertOutputContains(t, "pdf")
	result.AssertOutputContains(t, "figma")
	result.AssertOutputContains(t, "tracked")
}

func TestList_Help_ShowsUsage(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	result := sb.RunCLI("list", "--help")

	result.AssertSuccess(t)
	result.AssertOutputContains(t, "Usage:")
	result.AssertOutputContains(t, "--verbose")
}

func TestList_GroupedDisplay(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	// Create nested skills in two directories + one top-level skill
	sb.CreateNestedSkill("frontend/react-helper", map[string]string{"SKILL.md": "# React"})
	sb.CreateNestedSkill("frontend/vue-helper", map[string]string{"SKILL.md": "# Vue"})
	sb.CreateNestedSkill("utils/helper", map[string]string{"SKILL.md": "# Helper"})
	sb.CreateSkill("top-level", map[string]string{"SKILL.md": "# Top"})

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)

	result := sb.RunCLI("list")

	result.AssertSuccess(t)
	// Should show directory headers
	result.AssertOutputContains(t, "frontend/")
	result.AssertOutputContains(t, "utils/")
	// Should show base names within groups (not flat names)
	result.AssertOutputContains(t, "react-helper")
	result.AssertOutputContains(t, "vue-helper")
	result.AssertOutputContains(t, "helper")
	// Should show top-level skill
	result.AssertOutputContains(t, "top-level")
}

func TestList_GroupedDisplay_Verbose(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.CreateNestedSkill("frontend/react", map[string]string{"SKILL.md": "# React"})
	sb.CreateSkill("my-skill", map[string]string{"SKILL.md": "# Mine"})

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)

	result := sb.RunCLI("list", "--verbose")

	result.AssertSuccess(t)
	result.AssertOutputContains(t, "frontend/")
	result.AssertOutputContains(t, "react")
	result.AssertOutputContains(t, "my-skill")
}

func TestList_FlatDisplay_NoNesting(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	// All top-level skills — should not show directory headers
	sb.CreateSkill("alpha", map[string]string{"SKILL.md": "# Alpha"})
	sb.CreateSkill("beta", map[string]string{"SKILL.md": "# Beta"})

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)

	result := sb.RunCLI("list")

	result.AssertSuccess(t)
	result.AssertOutputContains(t, "alpha")
	result.AssertOutputContains(t, "beta")
	// Should NOT have directory separator in output for pure flat lists
	result.AssertOutputNotContains(t, "/")
}

func TestList_ShowsSourceInfo(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	// Create skill without metadata (local)
	sb.CreateSkill("local-skill", map[string]string{"SKILL.md": "# Local"})

	// Create skill with metadata (installed)
	sb.CreateSkill("installed-skill", map[string]string{
		"SKILL.md": "# Installed",
		".skillshare-meta.json": `{
  "source": "github.com/example/repo",
  "type": "github",
  "installed_at": "2024-01-15T10:30:00Z"
}`,
	})

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)

	result := sb.RunCLI("list")

	result.AssertSuccess(t)
	result.AssertOutputContains(t, "local-skill")
	result.AssertOutputContains(t, "local")
	result.AssertOutputContains(t, "installed-skill")
	result.AssertOutputContains(t, "github.com/example/repo")
}
