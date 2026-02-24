//go:build !online

package integration

import (
	"encoding/json"
	"strings"
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
	result.AssertOutputContains(t, "--type")
	result.AssertOutputContains(t, "--sort")
	result.AssertOutputContains(t, "[pattern]")
	result.AssertAnyOutputContains(t, "--no-tui")
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

// --- Search / Filter / Sort tests ---

func TestList_SearchByPattern(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.CreateSkill("react-helper", map[string]string{"SKILL.md": "# React"})
	sb.CreateSkill("vue-helper", map[string]string{"SKILL.md": "# Vue"})
	sb.CreateSkill("python-utils", map[string]string{"SKILL.md": "# Python"})

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)

	result := sb.RunCLI("list", "react")

	result.AssertSuccess(t)
	result.AssertOutputContains(t, "react-helper")
	result.AssertOutputNotContains(t, "vue-helper")
	result.AssertOutputNotContains(t, "python-utils")
	result.AssertOutputContains(t, `matching "react"`)
}

func TestList_SearchCaseInsensitive(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.CreateSkill("React-Helper", map[string]string{"SKILL.md": "# React"})
	sb.CreateSkill("other-skill", map[string]string{"SKILL.md": "# Other"})

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)

	// Search with lowercase should match capitalized skill name
	result := sb.RunCLI("list", "react")

	result.AssertSuccess(t)
	result.AssertOutputContains(t, "React-Helper")
	result.AssertOutputNotContains(t, "other-skill")
}

func TestList_SearchMatchesGroup(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.CreateNestedSkill("frontend/react-helper", map[string]string{"SKILL.md": "# React"})
	sb.CreateNestedSkill("frontend/vue-helper", map[string]string{"SKILL.md": "# Vue"})
	sb.CreateSkill("backend-api", map[string]string{"SKILL.md": "# Backend"})

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)

	// "frontend" matches via RelPath
	result := sb.RunCLI("list", "frontend")

	result.AssertSuccess(t)
	result.AssertOutputContains(t, "react-helper")
	result.AssertOutputContains(t, "vue-helper")
	result.AssertOutputNotContains(t, "backend-api")
}

func TestList_SearchNoResults(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.CreateSkill("alpha", map[string]string{"SKILL.md": "# Alpha"})
	sb.CreateSkill("beta", map[string]string{"SKILL.md": "# Beta"})

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)

	result := sb.RunCLI("list", "nonexistent")

	result.AssertSuccess(t)
	result.AssertOutputContains(t, "No skills matching")
	result.AssertOutputContains(t, "nonexistent")
}

func TestList_FilterByType_Local(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	// Local skill (no metadata)
	sb.CreateSkill("local-only", map[string]string{"SKILL.md": "# Local"})

	// GitHub skill (has metadata with source)
	sb.CreateSkill("from-github", map[string]string{
		"SKILL.md": "# GitHub",
		".skillshare-meta.json": `{
  "source": "github.com/user/repo",
  "type": "github",
  "installed_at": "2024-06-01T00:00:00Z"
}`,
	})

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)

	result := sb.RunCLI("list", "--type", "local")

	result.AssertSuccess(t)
	result.AssertOutputContains(t, "local-only")
	result.AssertOutputNotContains(t, "from-github")
}

func TestList_FilterByType_Github(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	// Local skill (no metadata)
	sb.CreateSkill("local-only", map[string]string{"SKILL.md": "# Local"})

	// GitHub skill (has metadata with source)
	sb.CreateSkill("from-github", map[string]string{
		"SKILL.md": "# GitHub",
		".skillshare-meta.json": `{
  "source": "github.com/user/repo",
  "type": "github",
  "installed_at": "2024-06-01T00:00:00Z"
}`,
	})

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)

	result := sb.RunCLI("list", "--type", "github")

	result.AssertSuccess(t)
	result.AssertOutputContains(t, "from-github")
	result.AssertOutputNotContains(t, "local-only")
}

func TestList_SortNewest(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.CreateSkill("old-skill", map[string]string{
		"SKILL.md": "# Old",
		".skillshare-meta.json": `{
  "source": "github.com/user/old",
  "type": "github",
  "installed_at": "2023-01-01T00:00:00Z"
}`,
	})
	sb.CreateSkill("new-skill", map[string]string{
		"SKILL.md": "# New",
		".skillshare-meta.json": `{
  "source": "github.com/user/new",
  "type": "github",
  "installed_at": "2025-12-01T00:00:00Z"
}`,
	})

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)

	result := sb.RunCLI("list", "--sort", "newest")

	result.AssertSuccess(t)
	// new-skill should appear before old-skill in the output
	out := result.Stdout
	newIdx := strings.Index(out, "new-skill")
	oldIdx := strings.Index(out, "old-skill")
	if newIdx < 0 || oldIdx < 0 {
		t.Fatal("expected both new-skill and old-skill in output")
	}
	if newIdx > oldIdx {
		t.Errorf("expected new-skill before old-skill with --sort newest, got new at %d, old at %d", newIdx, oldIdx)
	}
}

func TestList_InvalidType(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)

	result := sb.RunCLI("list", "--type", "invalid")

	result.AssertFailure(t)
	result.AssertAnyOutputContains(t, "invalid type")
}

func TestList_InvalidSort(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)

	result := sb.RunCLI("list", "--sort", "invalid")

	result.AssertFailure(t)
	result.AssertAnyOutputContains(t, "invalid sort")
}

func TestList_SearchWithFilter(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	// Local skills
	sb.CreateSkill("react-local", map[string]string{"SKILL.md": "# React Local"})
	sb.CreateSkill("vue-local", map[string]string{"SKILL.md": "# Vue Local"})

	// GitHub skill with "react" in source
	sb.CreateSkill("react-remote", map[string]string{
		"SKILL.md": "# React Remote",
		".skillshare-meta.json": `{
  "source": "github.com/user/react-kit",
  "type": "github",
  "installed_at": "2024-06-01T00:00:00Z"
}`,
	})

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)

	// Search "react" + type "local" → only react-local
	result := sb.RunCLI("list", "react", "--type", "local")

	result.AssertSuccess(t)
	result.AssertOutputContains(t, "react-local")
	result.AssertOutputNotContains(t, "vue-local")
	result.AssertOutputNotContains(t, "react-remote")
}

func TestList_JSON_OutputsValidJSON(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.CreateSkill("alpha", map[string]string{"SKILL.md": "# Alpha"})
	sb.CreateSkill("beta", map[string]string{
		"SKILL.md": "# Beta",
		".skillshare-meta.json": `{"source":"github.com/user/repo","type":"github","installed_at":"2024-06-01T00:00:00Z"}`,
	})

	sb.WriteConfig(`source: ` + sb.SourcePath + "\ntargets: {}\n")

	result := sb.RunCLI("list", "--json")
	result.AssertSuccess(t)

	var skills []map[string]interface{}
	if err := json.Unmarshal([]byte(result.Stdout), &skills); err != nil {
		t.Fatalf("invalid JSON output: %v\nOutput: %s", err, result.Stdout)
	}
	if len(skills) != 2 {
		t.Fatalf("expected 2 skills, got %d", len(skills))
	}

	// Verify fields exist
	for _, s := range skills {
		if _, ok := s["name"]; !ok {
			t.Error("missing 'name' field in JSON output")
		}
		if _, ok := s["relPath"]; !ok {
			t.Error("missing 'relPath' field in JSON output")
		}
	}
}

func TestList_JSON_WithFilter(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.CreateSkill("react-skill", map[string]string{"SKILL.md": "# React"})
	sb.CreateSkill("vue-skill", map[string]string{"SKILL.md": "# Vue"})

	sb.WriteConfig(`source: ` + sb.SourcePath + "\ntargets: {}\n")

	result := sb.RunCLI("list", "react", "--json")
	result.AssertSuccess(t)

	var skills []map[string]interface{}
	if err := json.Unmarshal([]byte(result.Stdout), &skills); err != nil {
		t.Fatalf("invalid JSON output: %v", err)
	}
	if len(skills) != 1 {
		t.Fatalf("expected 1 skill, got %d", len(skills))
	}
	if skills[0]["name"] != "react-skill" {
		t.Errorf("expected react-skill, got %v", skills[0]["name"])
	}
}

func TestList_JSON_Empty(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.WriteConfig(`source: ` + sb.SourcePath + "\ntargets: {}\n")

	result := sb.RunCLI("list", "--json")
	result.AssertSuccess(t)

	var skills []map[string]interface{}
	if err := json.Unmarshal([]byte(result.Stdout), &skills); err != nil {
		t.Fatalf("invalid JSON output: %v", err)
	}
	if len(skills) != 0 {
		t.Fatalf("expected 0 skills, got %d", len(skills))
	}
}

// --- --no-tui tests ---

func TestList_NoTUI_ShowsPlainText(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.CreateSkill("my-skill", map[string]string{
		"SKILL.md": "---\nname: my-skill\n---\n# My Skill",
	})

	sb.WriteConfig(`source: ` + sb.SourcePath + "\ntargets: {}\n")

	result := sb.RunCLI("list", "--no-tui")
	if result.ExitCode != 0 {
		t.Errorf("expected exit code 0, got %d\n\tstdout: %s\n\tstderr: %s", result.ExitCode, result.Stdout, result.Stderr)
	}
	if !strings.Contains(result.Stdout, "my-skill") {
		t.Errorf("expected output to contain 'my-skill', got:\n%s", result.Stdout)
	}
}

func TestList_NoTUI_WithPattern(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.CreateSkill("react-helper", map[string]string{
		"SKILL.md": "---\nname: react-helper\n---\n# React",
	})
	sb.CreateSkill("vue-helper", map[string]string{
		"SKILL.md": "---\nname: vue-helper\n---\n# Vue",
	})

	sb.WriteConfig(`source: ` + sb.SourcePath + "\ntargets: {}\n")

	result := sb.RunCLI("list", "--no-tui", "react")
	if result.ExitCode != 0 {
		t.Errorf("expected exit code 0, got %d", result.ExitCode)
	}
	if !strings.Contains(result.Stdout, "react-helper") {
		t.Errorf("expected output to contain 'react-helper'")
	}
	if strings.Contains(result.Stdout, "vue-helper") {
		t.Errorf("should not contain 'vue-helper' when filtered")
	}
}
