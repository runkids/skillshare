//go:build !online

package integration

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"skillshare/internal/install"
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
	})
	writeListMeta(t, sb.SourcePath, "meta-skill", &install.MetadataEntry{
		Source: "github.com/user/repo/path/to/skill",
		Type:   "github-subdir",
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
	})
	writeListMeta(t, sb.SourcePath, "installed-skill", &install.MetadataEntry{
		Source: "github.com/example/repo",
		Type:   "github",
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
	})
	writeListMeta(t, sb.SourcePath, "from-github", &install.MetadataEntry{
		Source: "github.com/user/repo",
		Type:   "github",
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
	})
	writeListMeta(t, sb.SourcePath, "from-github", &install.MetadataEntry{
		Source: "github.com/user/repo",
		Type:   "github",
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
	})
	oldTime, _ := time.Parse(time.RFC3339, "2023-01-01T00:00:00Z")
	writeListMeta(t, sb.SourcePath, "old-skill", &install.MetadataEntry{
		Source:      "github.com/user/old",
		Type:        "github",
		InstalledAt: oldTime,
	})
	sb.CreateSkill("new-skill", map[string]string{
		"SKILL.md": "# New",
	})
	newTime, _ := time.Parse(time.RFC3339, "2025-12-01T00:00:00Z")
	writeListMeta(t, sb.SourcePath, "new-skill", &install.MetadataEntry{
		Source:      "github.com/user/new",
		Type:        "github",
		InstalledAt: newTime,
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
	})
	writeListMeta(t, sb.SourcePath, "react-remote", &install.MetadataEntry{
		Source: "github.com/user/react-kit",
		Type:   "github",
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
	})
	writeListMeta(t, sb.SourcePath, "beta", &install.MetadataEntry{
		Source: "github.com/user/repo",
		Type:   "github",
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

// --- --status tests ---

// setupStatusSandbox creates one enabled and one disabled skill ("off-skill"
// is disabled through .skillignore) and writes the config.
func setupStatusSandbox(t *testing.T) *testutil.Sandbox {
	t.Helper()
	sb := testutil.NewSandbox(t)
	sb.CreateSkill("on-skill", map[string]string{"SKILL.md": "# On"})
	sb.CreateSkill("off-skill", map[string]string{"SKILL.md": "# Off"})
	sb.WriteFile(filepath.Join(sb.SourcePath, ".skillignore"), "off-skill\n")
	sb.WriteConfig(`source: ` + sb.SourcePath + "\ntargets: {}\n")
	return sb
}

func TestList_FilterByStatus_Enabled(t *testing.T) {
	sb := setupStatusSandbox(t)
	defer sb.Cleanup()

	result := sb.RunCLI("list", "--no-tui", "--status", "enabled")

	result.AssertSuccess(t)
	result.AssertOutputContains(t, "on-skill")
	result.AssertOutputNotContains(t, "off-skill")
	result.AssertAnyOutputContains(t, "1 of 2 skills (status: enabled)")
}

func TestList_FilterByStatus_Disabled(t *testing.T) {
	sb := setupStatusSandbox(t)
	defer sb.Cleanup()

	result := sb.RunCLI("list", "--no-tui", "--status=disabled")

	result.AssertSuccess(t)
	result.AssertOutputContains(t, "off-skill")
	result.AssertOutputNotContains(t, "on-skill")
	result.AssertAnyOutputContains(t, "1 of 2 skills (status: disabled)")
}

func TestList_FilterByStatus_All_EqualsDefault(t *testing.T) {
	sb := setupStatusSandbox(t)
	defer sb.Cleanup()

	def := sb.RunCLI("list", "--json")
	def.AssertSuccess(t)
	explicit := sb.RunCLI("list", "--json", "--status", "all")
	explicit.AssertSuccess(t)

	if def.Stdout != explicit.Stdout {
		t.Errorf("--status all differs from default:\ndefault:\n%s\nexplicit:\n%s", def.Stdout, explicit.Stdout)
	}
}

func TestList_FilterByStatus_JSON(t *testing.T) {
	sb := setupStatusSandbox(t)
	defer sb.Cleanup()

	result := sb.RunCLI("list", "--json", "--status", "enabled")
	result.AssertSuccess(t)

	var skills []map[string]interface{}
	if err := json.Unmarshal([]byte(result.Stdout), &skills); err != nil {
		t.Fatalf("invalid JSON output: %v\nOutput: %s", err, result.Stdout)
	}
	if len(skills) != 1 {
		t.Fatalf("expected 1 skill, got %d", len(skills))
	}
	if skills[0]["name"] != "on-skill" {
		t.Errorf("expected on-skill, got %v", skills[0]["name"])
	}
}

func TestList_FilterByStatus_NoMatch(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.CreateSkill("on-skill", map[string]string{"SKILL.md": "# On"})
	sb.WriteConfig(`source: ` + sb.SourcePath + "\ntargets: {}\n")

	result := sb.RunCLI("list", "--no-tui", "--status", "disabled")
	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, `No skills matching status "disabled"`)

	jsonResult := sb.RunCLI("list", "--json", "--status", "disabled")
	jsonResult.AssertSuccess(t)
	if strings.TrimSpace(jsonResult.Stdout) != "[]" {
		t.Errorf("expected empty JSON array, got %q", jsonResult.Stdout)
	}
}

func TestList_FilterByStatus_CombinesWithPattern(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.CreateSkill("react-on", map[string]string{"SKILL.md": "# On"})
	sb.CreateSkill("react-off", map[string]string{"SKILL.md": "# Off"})
	sb.CreateSkill("vue-off", map[string]string{"SKILL.md": "# Vue"})
	sb.WriteFile(filepath.Join(sb.SourcePath, ".skillignore"), "react-off\nvue-off\n")
	sb.WriteConfig(`source: ` + sb.SourcePath + "\ntargets: {}\n")

	result := sb.RunCLI("list", "--no-tui", "react", "--status", "disabled")

	result.AssertSuccess(t)
	result.AssertOutputContains(t, "react-off")
	result.AssertOutputNotContains(t, "react-on")
	result.AssertOutputNotContains(t, "vue-off")
}

func TestList_FilterByStatus_HidesTrackedReposSummary(t *testing.T) {
	sb := setupStatusSandbox(t)
	defer sb.Cleanup()

	result := sb.RunCLI("list", "--no-tui", "--status", "enabled")

	result.AssertSuccess(t)
	result.AssertOutputNotContains(t, "Tracked repositories")
}

func TestList_Agents_FilterByStatus(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	agentsDir := createAgentSource(t, sb, map[string]string{
		"tutor.md":    "# Tutor agent",
		"reviewer.md": "# Reviewer agent",
	})
	sb.WriteFile(filepath.Join(agentsDir, ".agentignore"), "reviewer.md\n")
	sb.WriteConfig(`source: ` + sb.SourcePath + "\ntargets: {}\n")

	result := sb.RunCLI("list", "agents", "--no-tui", "--status", "disabled")

	result.AssertSuccess(t)
	result.AssertOutputContains(t, "reviewer")
	result.AssertOutputNotContains(t, "tutor")
}

func TestList_InvalidStatus(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.WriteConfig(`source: ` + sb.SourcePath + "\ntargets: {}\n")

	result := sb.RunCLI("list", "--status", "bogus")

	result.AssertFailure(t)
	result.AssertAnyOutputContains(t, "invalid status")
	result.AssertAnyOutputContains(t, "all, enabled, or disabled")
}

// writeListMeta writes a metadata entry to the centralized .metadata.json in sourceDir.
func writeListMeta(t *testing.T, sourceDir, skillName string, entry *install.MetadataEntry) {
	t.Helper()
	store, err := install.LoadMetadata(sourceDir)
	if err != nil {
		t.Fatalf("writeListMeta: load: %v", err)
	}
	store.Set(skillName, entry)
	if err := store.Save(sourceDir); err != nil {
		t.Fatalf("writeListMeta: save: %v", err)
	}
}
