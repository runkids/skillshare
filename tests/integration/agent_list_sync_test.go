//go:build !online

package integration

import (
	"os"
	"path/filepath"
	"testing"

	"skillshare/internal/testutil"
)

// createAgentSource creates an agents source directory with the given agents.
// Each key is the filename (e.g., "tutor.md"), value is the content.
func createAgentSource(t *testing.T, sb *testutil.Sandbox, agents map[string]string) string {
	t.Helper()
	agentsDir := filepath.Join(filepath.Dir(sb.SourcePath), "agents")
	if err := os.MkdirAll(agentsDir, 0755); err != nil {
		t.Fatalf("failed to create agents dir: %v", err)
	}
	for name, content := range agents {
		agentPath := filepath.Join(agentsDir, name)
		if err := os.MkdirAll(filepath.Dir(agentPath), 0755); err != nil {
			t.Fatalf("failed to create agent parent dir for %s: %v", name, err)
		}
		if err := os.WriteFile(agentPath, []byte(content), 0644); err != nil {
			t.Fatalf("failed to write agent %s: %v", name, err)
		}
	}
	return agentsDir
}

// createAgentTarget creates an agent target directory for the given target name.
func createAgentTarget(t *testing.T, sb *testutil.Sandbox, name string) string {
	t.Helper()
	var path string
	switch name {
	case "claude":
		path = filepath.Join(sb.Home, ".claude", "agents")
	case "cursor":
		path = filepath.Join(sb.Home, ".cursor", "agents")
	case "opencode":
		path = filepath.Join(sb.Home, ".config", "opencode", "agents")
	default:
		path = filepath.Join(sb.Home, "."+name, "agents")
	}
	if err := os.MkdirAll(path, 0755); err != nil {
		t.Fatalf("failed to create agent target: %v", err)
	}
	return path
}

// --- list agents ---

func TestList_Agents_Empty(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	createAgentSource(t, sb, nil)
	sb.WriteConfig(`source: ` + sb.SourcePath + "\ntargets: {}\n")

	result := sb.RunCLI("list", "agents", "--no-tui")
	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, "No agents installed")
}

func TestList_Agents_ShowsAgents(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	createAgentSource(t, sb, map[string]string{
		"tutor.md":    "# Tutor agent",
		"reviewer.md": "# Reviewer agent",
	})
	sb.WriteConfig(`source: ` + sb.SourcePath + "\ntargets: {}\n")

	result := sb.RunCLI("list", "agents", "--no-tui")
	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, "tutor")
	result.AssertAnyOutputContains(t, "reviewer")
	result.AssertAnyOutputContains(t, "Installed agents")
}

func TestList_Agents_JSON_IncludesKind(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	createAgentSource(t, sb, map[string]string{
		"tutor.md": "# Tutor agent",
	})
	sb.WriteConfig(`source: ` + sb.SourcePath + "\ntargets: {}\n")

	result := sb.RunCLI("list", "agents", "--json")
	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, `"kind"`)
	result.AssertAnyOutputContains(t, `"agent"`)
	result.AssertAnyOutputContains(t, `"tutor"`)
}

func TestList_All_MixedOutput(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.CreateSkill("my-skill", map[string]string{
		"SKILL.md": "---\nname: my-skill\n---\n# Content",
	})
	createAgentSource(t, sb, map[string]string{
		"tutor.md": "# Tutor agent",
	})
	sb.WriteConfig(`source: ` + sb.SourcePath + "\ntargets: {}\n")

	result := sb.RunCLI("list", "--all", "--json")
	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, `"skill"`)
	result.AssertAnyOutputContains(t, `"agent"`)
}

func TestList_Default_SkillsOnly(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.CreateSkill("my-skill", map[string]string{
		"SKILL.md": "---\nname: my-skill\n---\n# Content",
	})
	createAgentSource(t, sb, map[string]string{
		"tutor.md": "# Tutor agent",
	})
	sb.WriteConfig(`source: ` + sb.SourcePath + "\ntargets: {}\n")

	// Default list should NOT include agents
	result := sb.RunCLI("list", "--json")
	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, `"my-skill"`)
	result.AssertOutputNotContains(t, `"tutor"`)
}

// --- sync agents ---

func TestSync_Agents_CreatesSymlinks(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	createAgentSource(t, sb, map[string]string{
		"tutor.md": "# Tutor agent",
	})
	claudeAgents := createAgentTarget(t, sb, "claude")

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets:
  claude:
    skills:
      path: ` + sb.CreateTarget("claude") + `
    agents:
      path: ` + claudeAgents + `
`)

	result := sb.RunCLI("sync", "agents")
	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, "Syncing agents")

	// Verify symlink was created
	linkPath := filepath.Join(claudeAgents, "tutor.md")
	if _, err := os.Lstat(linkPath); err != nil {
		t.Errorf("expected agent symlink at %s, got error: %v", linkPath, err)
	}
}

func TestSync_Agents_DryRun(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	createAgentSource(t, sb, map[string]string{
		"tutor.md": "# Tutor agent",
	})
	claudeAgents := createAgentTarget(t, sb, "claude")

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets:
  claude:
    skills:
      path: ` + sb.CreateTarget("claude") + `
    agents:
      path: ` + claudeAgents + `
`)

	result := sb.RunCLI("sync", "agents", "--dry-run")
	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, "Dry run")

	// Verify NO symlink was created
	linkPath := filepath.Join(claudeAgents, "tutor.md")
	if _, err := os.Lstat(linkPath); !os.IsNotExist(err) {
		t.Error("expected no agent symlink in dry-run mode")
	}
}

func TestSync_Agents_AfterFlags(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	createAgentSource(t, sb, map[string]string{
		"tutor.md": "# Tutor agent",
	})
	claudeAgents := createAgentTarget(t, sb, "claude")

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets:
  claude:
    skills:
      path: ` + sb.CreateTarget("claude") + `
    agents:
      path: ` + claudeAgents + `
`)

	result := sb.RunCLI("sync", "--dry-run", "agents")
	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, "Syncing agents")
	result.AssertOutputNotContains(t, "Syncing skills")
}

func TestSync_Default_SkillsOnly_NoAgentSync(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.CreateSkill("my-skill", map[string]string{
		"SKILL.md": "---\nname: my-skill\n---\n# Content",
	})
	createAgentSource(t, sb, map[string]string{
		"tutor.md": "# Tutor agent",
	})

	claudeSkills := sb.CreateTarget("claude")
	claudeAgents := createAgentTarget(t, sb, "claude")

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets:
  claude:
    skills:
      path: ` + claudeSkills + `
    agents:
      path: ` + claudeAgents + `
`)

	// Default sync should only sync skills, NOT agents
	result := sb.RunCLI("sync")
	result.AssertSuccess(t)
	result.AssertOutputNotContains(t, "Syncing agents")

	// Skill symlink should exist
	if _, err := os.Lstat(filepath.Join(claudeSkills, "my-skill")); err != nil {
		t.Error("expected skill symlink")
	}
	// Agent symlink should NOT exist
	if _, err := os.Lstat(filepath.Join(claudeAgents, "tutor.md")); !os.IsNotExist(err) {
		t.Error("expected no agent symlink from default sync")
	}
}

func TestSync_All_SyncsSkillsAndAgents(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.CreateSkill("my-skill", map[string]string{
		"SKILL.md": "---\nname: my-skill\n---\n# Content",
	})
	createAgentSource(t, sb, map[string]string{
		"tutor.md": "# Tutor agent",
	})

	claudeSkills := sb.CreateTarget("claude")
	claudeAgents := createAgentTarget(t, sb, "claude")

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets:
  claude:
    skills:
      path: ` + claudeSkills + `
    agents:
      path: ` + claudeAgents + `
`)

	result := sb.RunCLI("sync", "--all")
	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, "Syncing skills")
	result.AssertAnyOutputContains(t, "Syncing agents")

	// Both should be synced
	if _, err := os.Lstat(filepath.Join(claudeSkills, "my-skill")); err != nil {
		t.Error("expected skill symlink")
	}
	if _, err := os.Lstat(filepath.Join(claudeAgents, "tutor.md")); err != nil {
		t.Error("expected agent symlink")
	}
}

// --- --all flag ---

func TestSync_All_Flag(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.CreateSkill("my-skill", map[string]string{
		"SKILL.md": "---\nname: my-skill\n---\n# Content",
	})
	claudeSkills := sb.CreateTarget("claude")

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets:
  claude:
    skills:
      path: ` + claudeSkills + `
`)

	// "sync --all" should still sync skills even without agents configured
	result := sb.RunCLI("sync", "--all")
	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, "Syncing skills")
}
