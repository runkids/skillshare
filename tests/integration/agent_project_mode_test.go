//go:build !online

package integration

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"skillshare/internal/testutil"
)

// setupProjectWithAgents creates a project directory with skills, agents, and config.
// Returns the project root path.
func setupProjectWithAgents(t *testing.T, sb *testutil.Sandbox) string {
	t.Helper()

	projectDir := filepath.Join(sb.Root, "myproject")
	skillsDir := filepath.Join(projectDir, ".skillshare", "skills")
	agentsDir := filepath.Join(projectDir, ".skillshare", "agents")
	os.MkdirAll(skillsDir, 0755)
	os.MkdirAll(agentsDir, 0755)

	// Create a skill
	skillDir := filepath.Join(skillsDir, "my-skill")
	os.MkdirAll(skillDir, 0755)
	os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("---\nname: my-skill\n---\n# Content"), 0644)

	// Create an agent
	os.WriteFile(filepath.Join(agentsDir, "tutor.md"), []byte("# Tutor agent"), 0644)

	// Write project config with a target that has agent path
	claudeAgents := filepath.Join(projectDir, ".claude", "agents")
	os.MkdirAll(claudeAgents, 0755)
	claudeSkills := filepath.Join(projectDir, ".claude", "skills")
	os.MkdirAll(claudeSkills, 0755)

	configContent := `targets:
  - name: claude
    skills:
      path: ` + claudeSkills + `
    agents:
      path: ` + claudeAgents + `
`
	os.WriteFile(filepath.Join(projectDir, ".skillshare", "config.yaml"), []byte(configContent), 0644)

	// Global config (needed by CLI)
	sb.WriteConfig(`source: ` + sb.SourcePath + "\ntargets: {}\n")

	return projectDir
}

func setupProjectWithMultipleAgentTargets(t *testing.T, sb *testutil.Sandbox) string {
	t.Helper()

	projectDir := filepath.Join(sb.Root, "multi-agent-project")
	skillsDir := filepath.Join(projectDir, ".skillshare", "skills")
	agentsDir := filepath.Join(projectDir, ".skillshare", "agents")
	claudeSkills := filepath.Join(projectDir, ".claude", "skills")
	claudeAgents := filepath.Join(projectDir, ".claude", "agents")
	cursorSkills := filepath.Join(projectDir, ".cursor", "skills")
	cursorAgents := filepath.Join(projectDir, ".cursor", "agents")

	for _, dir := range []string{skillsDir, agentsDir, claudeSkills, claudeAgents, cursorSkills, cursorAgents} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("failed to create %s: %v", dir, err)
		}
	}

	configContent := `targets:
  - name: claude
    skills:
      path: ` + claudeSkills + `
    agents:
      path: ` + claudeAgents + `
  - name: cursor
    skills:
      path: ` + cursorSkills + `
    agents:
      path: ` + cursorAgents + `
`
	if err := os.WriteFile(filepath.Join(projectDir, ".skillshare", "config.yaml"), []byte(configContent), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	sb.WriteConfig(`source: ` + sb.SourcePath + "\ntargets: {}\n")
	return projectDir
}

// --- status -p (always shows skills + agents) ---

func TestStatusProject_ShowsAgents(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	projectDir := setupProjectWithAgents(t, sb)

	result := sb.RunCLIInDir(projectDir, "status", "-p")
	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, "Source")   // source section
	result.AssertAnyOutputContains(t, "1 agents") // agents in source
	result.AssertAnyOutputContains(t, "agents")   // agents sub-item in targets
}

func TestStatusProject_JSON_IncludesAgents(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	projectDir := setupProjectWithAgents(t, sb)

	result := sb.RunCLIInDir(projectDir, "status", "-p", "--json")
	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, `"agents"`)
	result.AssertAnyOutputContains(t, `"count"`)
}

func TestSyncProject_AgentsUseRelativeSymlinks(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	projectDir := setupProjectWithAgents(t, sb)

	result := sb.RunCLIInDir(projectDir, "sync", "-p", "agents")
	result.AssertSuccess(t)

	link := filepath.Join(projectDir, ".claude", "agents", "tutor.md")
	if !sb.IsSymlink(link) {
		t.Fatal("agent should be a symlink")
	}
	if target := sb.SymlinkTarget(link); filepath.IsAbs(target) {
		t.Fatalf("project-mode agent symlink should be relative, got %q", target)
	}
}

func TestSyncProject_AgentsAliasTargetUsesBuiltinAgentPath(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	projectDir := filepath.Join(sb.Root, "factory-agent-project")
	agentsDir := filepath.Join(projectDir, ".skillshare", "agents")
	if err := os.MkdirAll(agentsDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(agentsDir, "droid.md"), []byte("# Droid"), 0644); err != nil {
		t.Fatal(err)
	}
	sb.WriteProjectConfig(projectDir, `targets:
  - factory
`)
	sb.WriteConfig(`source: ` + sb.SourcePath + "\ntargets: {}\n")

	result := sb.RunCLIInDir(projectDir, "sync", "-p", "agents")
	result.AssertSuccess(t)

	link := filepath.Join(projectDir, ".factory", "droids", "droid.md")
	if !sb.IsSymlink(link) {
		t.Fatal("factory alias should sync agents to droid builtin path")
	}
	if target := sb.SymlinkTarget(link); filepath.IsAbs(target) {
		t.Fatalf("project-mode factory agent symlink should be relative, got %q", target)
	}
}

func TestSyncProject_AgentsSymlinkModeUsesRelativeSymlink(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	projectDir := filepath.Join(sb.Root, "agent-symlink-project")
	agentsDir := filepath.Join(projectDir, ".skillshare", "agents")
	if err := os.MkdirAll(agentsDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(agentsDir, "tutor.md"), []byte("# Tutor"), 0644); err != nil {
		t.Fatal(err)
	}
	sb.WriteProjectConfig(projectDir, `targets:
  - name: claude
    agents:
      path: .claude/agents
      mode: symlink
`)
	sb.WriteConfig(`source: ` + sb.SourcePath + "\ntargets: {}\n")

	result := sb.RunCLIInDir(projectDir, "sync", "-p", "agents")
	result.AssertSuccess(t)

	link := filepath.Join(projectDir, ".claude", "agents")
	if !sb.IsSymlink(link) {
		t.Fatal("agents target should be a directory symlink")
	}
	if target := sb.SymlinkTarget(link); filepath.IsAbs(target) {
		t.Fatalf("project-mode agents directory symlink should be relative, got %q", target)
	}
}

// --- check -p agents ---

func TestCheckProject_Agents(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	projectDir := setupProjectWithAgents(t, sb)

	result := sb.RunCLIInDir(projectDir, "check", "-p", "agents")
	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, "tutor")
	result.AssertAnyOutputContains(t, "local")
}

func TestCheckProject_Agents_JSON(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	projectDir := setupProjectWithAgents(t, sb)

	result := sb.RunCLIInDir(projectDir, "check", "-p", "agents", "--json")
	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, `"name"`)
	result.AssertAnyOutputContains(t, `"status"`)
}

// --- diff -p agents ---

func TestDiffProject_Agents(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	projectDir := setupProjectWithAgents(t, sb)

	// Before sync, diff should show agents as "add"
	result := sb.RunCLIInDir(projectDir, "diff", "-p", "agents", "--no-tui")
	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, "tutor")
}

func TestDiffProject_Agents_JSON(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	projectDir := setupProjectWithAgents(t, sb)

	result := sb.RunCLIInDir(projectDir, "diff", "-p", "agents", "--json")
	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, `"agent"`)
}

// --- collect -p agents ---

func TestCollectProject_Agents_NoLocal(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	projectDir := setupProjectWithAgents(t, sb)

	// Sync agents first
	sb.RunCLIInDir(projectDir, "sync", "-p", "agents")

	// No local agents to collect
	result := sb.RunCLIInDir(projectDir, "collect", "-p", "agents")
	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, "No local agents")
}

func TestCollectProject_Agents_CollectsLocal(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	projectDir := setupProjectWithAgents(t, sb)

	// Create a local agent directly in target (not via sync)
	claudeAgents := filepath.Join(projectDir, ".claude", "agents")
	os.WriteFile(filepath.Join(claudeAgents, "local-agent.md"), []byte("# Local"), 0644)

	result := sb.RunCLIInDir(projectDir, "collect", "-p", "agents", "--force")
	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, "collected")

	// Verify copied to project agents source
	agentsSource := filepath.Join(projectDir, ".skillshare", "agents")
	if _, err := os.Stat(filepath.Join(agentsSource, "local-agent.md")); err != nil {
		t.Error("local-agent.md should be collected to project agents source")
	}
}

func TestCollectProject_Agents_DryRun_DoesNotWrite(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	projectDir := setupProjectWithAgents(t, sb)
	claudeAgents := filepath.Join(projectDir, ".claude", "agents")
	os.WriteFile(filepath.Join(claudeAgents, "local-agent.md"), []byte("# Local"), 0644)

	result := sb.RunCLIInDir(projectDir, "collect", "-p", "agents", "--dry-run")
	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, "Dry run")

	agentsSource := filepath.Join(projectDir, ".skillshare", "agents")
	if _, err := os.Stat(filepath.Join(agentsSource, "local-agent.md")); !os.IsNotExist(err) {
		t.Error("dry-run should not collect local-agent.md")
	}
}

func TestCollectProject_Agents_JSON(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	projectDir := setupProjectWithAgents(t, sb)
	claudeAgents := filepath.Join(projectDir, ".claude", "agents")
	os.WriteFile(filepath.Join(claudeAgents, "local-agent.md"), []byte("# Local"), 0644)

	result := sb.RunCLIInDir(projectDir, "collect", "-p", "agents", "--json")
	result.AssertSuccess(t)

	var output map[string]any
	if err := json.Unmarshal([]byte(result.Stdout), &output); err != nil {
		t.Fatalf("invalid JSON output: %v\nStdout: %s", err, result.Stdout)
	}

	pulled, ok := output["pulled"].([]any)
	if !ok || len(pulled) != 1 || pulled[0] != "local-agent.md" {
		t.Fatalf("expected pulled=[local-agent.md], got %v", output["pulled"])
	}
}

func TestCollectProject_Agents_SpecificTargetOnly(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	projectDir := setupProjectWithMultipleAgentTargets(t, sb)
	claudeAgents := filepath.Join(projectDir, ".claude", "agents")
	cursorAgents := filepath.Join(projectDir, ".cursor", "agents")
	agentsSource := filepath.Join(projectDir, ".skillshare", "agents")

	os.WriteFile(filepath.Join(claudeAgents, "claude-agent.md"), []byte("# Claude"), 0644)
	os.WriteFile(filepath.Join(cursorAgents, "cursor-agent.md"), []byte("# Cursor"), 0644)

	result := sb.RunCLIInDir(projectDir, "collect", "-p", "agents", "claude", "--force")
	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, "claude-agent.md")
	result.AssertOutputNotContains(t, "cursor-agent.md")

	if _, err := os.Stat(filepath.Join(agentsSource, "claude-agent.md")); err != nil {
		t.Error("claude-agent.md should be collected")
	}
	if _, err := os.Stat(filepath.Join(agentsSource, "cursor-agent.md")); !os.IsNotExist(err) {
		t.Error("cursor-agent.md should not be collected")
	}
}

func TestCollectProject_Agents_MultipleTargets_RequiresAllOrName(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	projectDir := setupProjectWithMultipleAgentTargets(t, sb)
	claudeAgents := filepath.Join(projectDir, ".claude", "agents")
	cursorAgents := filepath.Join(projectDir, ".cursor", "agents")
	agentsSource := filepath.Join(projectDir, ".skillshare", "agents")

	os.WriteFile(filepath.Join(claudeAgents, "claude-agent.md"), []byte("# Claude"), 0644)
	os.WriteFile(filepath.Join(cursorAgents, "cursor-agent.md"), []byte("# Cursor"), 0644)

	result := sb.RunCLIInDir(projectDir, "collect", "-p", "agents")
	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, "Specify a target")

	if _, err := os.Stat(filepath.Join(agentsSource, "claude-agent.md")); !os.IsNotExist(err) {
		t.Error("claude-agent.md should not be collected without target selection")
	}
	if _, err := os.Stat(filepath.Join(agentsSource, "cursor-agent.md")); !os.IsNotExist(err) {
		t.Error("cursor-agent.md should not be collected without target selection")
	}
}

// --- audit -p agents ---

func TestAuditProject_Agents(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	projectDir := setupProjectWithAgents(t, sb)

	result := sb.RunCLIInDir(projectDir, "audit", "-p", "agents")
	result.AssertSuccess(t)
	// Audit should scan agents, not error
	result.AssertOutputNotContains(t, "not yet supported")
}

func TestSyncProject_All_NestedAgentsSameBasename_FlattensAndStaysStable(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	projectDir := filepath.Join(sb.Root, "nested-agents-project")
	skillsDir := filepath.Join(projectDir, ".skillshare", "skills")
	agentsDir := filepath.Join(projectDir, ".skillshare", "agents")
	claudeAgents := filepath.Join(projectDir, ".claude", "agents")
	cursorAgents := filepath.Join(projectDir, ".cursor", "agents")
	claudeSkills := filepath.Join(projectDir, ".claude", "skills")
	cursorSkills := filepath.Join(projectDir, ".cursor", "skills")

	for _, dir := range []string{
		filepath.Join(skillsDir, "sample-skill"),
		filepath.Join(agentsDir, "team-a"),
		filepath.Join(agentsDir, "team-b"),
		claudeAgents,
		cursorAgents,
		claudeSkills,
		cursorSkills,
	} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", dir, err)
		}
	}

	if err := os.WriteFile(filepath.Join(skillsDir, "sample-skill", "SKILL.md"), []byte("---\nname: sample-skill\n---\n# Sample"), 0o644); err != nil {
		t.Fatalf("write sample skill: %v", err)
	}
	if err := os.WriteFile(filepath.Join(agentsDir, "team-a", "helper.md"), []byte("# Team A"), 0o644); err != nil {
		t.Fatalf("write team-a helper: %v", err)
	}
	if err := os.WriteFile(filepath.Join(agentsDir, "team-b", "helper.md"), []byte("# Team B"), 0o644); err != nil {
		t.Fatalf("write team-b helper: %v", err)
	}

	configContent := `targets:
  - name: claude
    skills:
      path: ` + claudeSkills + `
    agents:
      path: ` + claudeAgents + `
  - name: cursor
    skills:
      path: ` + cursorSkills + `
    agents:
      path: ` + cursorAgents + `
`
	if err := os.WriteFile(filepath.Join(projectDir, ".skillshare", "config.yaml"), []byte(configContent), 0o644); err != nil {
		t.Fatalf("write project config: %v", err)
	}

	sb.WriteConfig(`source: ` + sb.SourcePath + "\ntargets: {}\n")

	first := sb.RunCLIInDir(projectDir, "sync", "-p", "--all")
	first.AssertSuccess(t)
	first.AssertAnyOutputContains(t, "Agent sync complete")
	first.AssertAnyOutputContains(t, "0 updated")

	second := sb.RunCLIInDir(projectDir, "sync", "-p", "--all")
	second.AssertSuccess(t)
	second.AssertAnyOutputContains(t, "Agent sync complete")
	second.AssertAnyOutputContains(t, "0 updated")

	for _, base := range []string{claudeAgents, cursorAgents} {
		for _, name := range []string{"team-a__helper.md", "team-b__helper.md"} {
			if _, err := os.Lstat(filepath.Join(base, name)); err != nil {
				t.Fatalf("expected synced agent %s in %s: %v", name, base, err)
			}
		}
	}
}

// --- default -p shows both skills and agents ---

func TestStatusProject_Default_ShowsBoth(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	projectDir := setupProjectWithAgents(t, sb)

	// status always shows both skills and agents in unified layout
	result := sb.RunCLIInDir(projectDir, "status", "-p")
	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, "Source")
	result.AssertAnyOutputContains(t, "1 agents") // agents in source section
	result.AssertAnyOutputContains(t, "agents")   // agents sub-item in targets
}
