//go:build !online

package integration

import (
	"os"
	"path/filepath"
	"testing"

	"skillshare/internal/testutil"
)

func TestSync_AgentsAliasTargetUsesBuiltinAgentPath(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	agentsDir := filepath.Join(filepath.Dir(sb.SourcePath), "agents")
	if err := os.MkdirAll(agentsDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(agentsDir, "droid.md"), []byte("# Droid"), 0644); err != nil {
		t.Fatal(err)
	}

	factorySkills := filepath.Join(sb.Home, ".factory", "skills")
	if err := os.MkdirAll(factorySkills, 0755); err != nil {
		t.Fatal(err)
	}

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets:
  factory:
    path: ` + factorySkills + `
`)

	result := sb.RunCLI("sync", "agents")
	result.AssertSuccess(t)

	if !sb.IsSymlink(filepath.Join(sb.Home, ".factory", "droids", "droid.md")) {
		t.Fatal("factory alias should sync agents to droid builtin path")
	}
}

func TestSync_Agents_IncludeExcludeCombined(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	agentsDir := filepath.Join(filepath.Dir(sb.SourcePath), "agents")
	os.MkdirAll(agentsDir, 0755)
	os.WriteFile(filepath.Join(agentsDir, "team-reviewer.md"), []byte("# Team Reviewer"), 0644)
	os.WriteFile(filepath.Join(agentsDir, "team-debugger.md"), []byte("# Team Debugger"), 0644)
	os.WriteFile(filepath.Join(agentsDir, "personal-tutor.md"), []byte("# Personal Tutor"), 0644)

	claudeSkills := filepath.Join(sb.Home, ".claude", "skills")
	claudeAgents := filepath.Join(sb.Home, ".claude", "agents")
	os.MkdirAll(claudeSkills, 0755)
	os.MkdirAll(claudeAgents, 0755)

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets:
  claude:
    skills:
      path: "` + claudeSkills + `"
    agents:
      path: "` + claudeAgents + `"
      include:
        - "team-*"
      exclude:
        - "*-debugger"
`)

	result := sb.RunCLI("sync", "agents")
	result.AssertSuccess(t)

	// team-reviewer matches include and not exclude → synced
	if _, err := os.Lstat(filepath.Join(claudeAgents, "team-reviewer.md")); err != nil {
		t.Error("team-reviewer.md should be synced (included, not excluded)")
	}

	// team-debugger matches include but also matches exclude → NOT synced
	if _, err := os.Lstat(filepath.Join(claudeAgents, "team-debugger.md")); !os.IsNotExist(err) {
		t.Error("team-debugger.md should NOT be synced (excluded by *-debugger)")
	}

	// personal-tutor does not match include → NOT synced
	if _, err := os.Lstat(filepath.Join(claudeAgents, "personal-tutor.md")); !os.IsNotExist(err) {
		t.Error("personal-tutor.md should NOT be synced (not in include list)")
	}
}

func TestSync_Agents_DisabledAgentsNotSynced(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	agentsDir := filepath.Join(filepath.Dir(sb.SourcePath), "agents")
	os.MkdirAll(agentsDir, 0755)
	os.WriteFile(filepath.Join(agentsDir, "active.md"), []byte("# Active"), 0644)
	os.WriteFile(filepath.Join(agentsDir, "disabled-one.md"), []byte("# Disabled One"), 0644)
	os.WriteFile(filepath.Join(agentsDir, "disabled-two.md"), []byte("# Disabled Two"), 0644)

	// Disable two agents via .agentignore
	os.WriteFile(filepath.Join(agentsDir, ".agentignore"), []byte("disabled-one.md\ndisabled-two.md\n"), 0644)

	claudeSkills := filepath.Join(sb.Home, ".claude", "skills")
	claudeAgents := filepath.Join(sb.Home, ".claude", "agents")
	os.MkdirAll(claudeSkills, 0755)
	os.MkdirAll(claudeAgents, 0755)

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets:
  claude:
    skills:
      path: "` + claudeSkills + `"
    agents:
      path: "` + claudeAgents + `"
`)

	result := sb.RunCLI("sync", "agents")
	result.AssertSuccess(t)

	// Active agent should be synced
	if _, err := os.Lstat(filepath.Join(claudeAgents, "active.md")); err != nil {
		t.Error("active.md should be synced")
	}

	// Disabled agents should NOT be synced
	if _, err := os.Lstat(filepath.Join(claudeAgents, "disabled-one.md")); !os.IsNotExist(err) {
		t.Error("disabled-one.md should NOT be synced (disabled via .agentignore)")
	}
	if _, err := os.Lstat(filepath.Join(claudeAgents, "disabled-two.md")); !os.IsNotExist(err) {
		t.Error("disabled-two.md should NOT be synced (disabled via .agentignore)")
	}
}

func TestSync_Agents_DisabledByGlobPattern(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	agentsDir := filepath.Join(filepath.Dir(sb.SourcePath), "agents")
	os.MkdirAll(agentsDir, 0755)
	os.WriteFile(filepath.Join(agentsDir, "prod-reviewer.md"), []byte("# Prod"), 0644)
	os.WriteFile(filepath.Join(agentsDir, "draft-experiment.md"), []byte("# Draft 1"), 0644)
	os.WriteFile(filepath.Join(agentsDir, "draft-wip.md"), []byte("# Draft 2"), 0644)

	// Glob pattern disables all draft-* agents
	os.WriteFile(filepath.Join(agentsDir, ".agentignore"), []byte("draft-*\n"), 0644)

	claudeSkills := filepath.Join(sb.Home, ".claude", "skills")
	claudeAgents := filepath.Join(sb.Home, ".claude", "agents")
	os.MkdirAll(claudeSkills, 0755)
	os.MkdirAll(claudeAgents, 0755)

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets:
  claude:
    skills:
      path: "` + claudeSkills + `"
    agents:
      path: "` + claudeAgents + `"
`)

	result := sb.RunCLI("sync", "agents")
	result.AssertSuccess(t)

	// Non-draft agent should be synced
	if _, err := os.Lstat(filepath.Join(claudeAgents, "prod-reviewer.md")); err != nil {
		t.Error("prod-reviewer.md should be synced")
	}

	// Draft agents should NOT be synced
	if _, err := os.Lstat(filepath.Join(claudeAgents, "draft-experiment.md")); !os.IsNotExist(err) {
		t.Error("draft-experiment.md should NOT be synced (disabled by draft-* pattern)")
	}
	if _, err := os.Lstat(filepath.Join(claudeAgents, "draft-wip.md")); !os.IsNotExist(err) {
		t.Error("draft-wip.md should NOT be synced (disabled by draft-* pattern)")
	}
}

func TestSync_Agents_DisabledNestedAgent(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	agentsDir := filepath.Join(filepath.Dir(sb.SourcePath), "agents")
	os.MkdirAll(filepath.Join(agentsDir, "team"), 0755)
	os.WriteFile(filepath.Join(agentsDir, "top-level.md"), []byte("# Top"), 0644)
	os.WriteFile(filepath.Join(agentsDir, "team", "reviewer.md"), []byte("# Reviewer"), 0644)
	os.WriteFile(filepath.Join(agentsDir, "team", "debugger.md"), []byte("# Debugger"), 0644)

	// Disable one nested agent
	os.WriteFile(filepath.Join(agentsDir, ".agentignore"), []byte("team/debugger.md\n"), 0644)

	claudeSkills := filepath.Join(sb.Home, ".claude", "skills")
	claudeAgents := filepath.Join(sb.Home, ".claude", "agents")
	os.MkdirAll(claudeSkills, 0755)
	os.MkdirAll(claudeAgents, 0755)

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets:
  claude:
    skills:
      path: "` + claudeSkills + `"
    agents:
      path: "` + claudeAgents + `"
`)

	result := sb.RunCLI("sync", "agents")
	result.AssertSuccess(t)

	// Top-level and enabled nested agent should be synced
	if _, err := os.Lstat(filepath.Join(claudeAgents, "top-level.md")); err != nil {
		t.Error("top-level.md should be synced")
	}
	if _, err := os.Lstat(filepath.Join(claudeAgents, "team__reviewer.md")); err != nil {
		t.Error("team__reviewer.md should be synced")
	}

	// Disabled nested agent should NOT be synced
	if _, err := os.Lstat(filepath.Join(claudeAgents, "team__debugger.md")); !os.IsNotExist(err) {
		t.Error("team__debugger.md should NOT be synced (disabled via .agentignore)")
	}
}

func TestSync_Agents_SkipsTargetsWithoutAgentsPath(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	// Create agents source with an agent
	agentsDir := filepath.Join(filepath.Dir(sb.SourcePath), "agents")
	os.MkdirAll(agentsDir, 0755)
	os.WriteFile(filepath.Join(agentsDir, "helper.md"), []byte("# Helper"), 0644)

	// Configure a target WITH agents path and one WITHOUT
	claudeSkills := filepath.Join(sb.Home, ".claude", "skills")
	claudeAgents := filepath.Join(sb.Home, ".claude", "agents")
	windsurf := filepath.Join(sb.Home, ".windsurf", "skills")

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets:
  claude:
    skills:
      path: "` + claudeSkills + `"
    agents:
      path: "` + claudeAgents + `"
  windsurf:
    skills:
      path: "` + windsurf + `"
`)

	result := sb.RunCLI("sync", "agents")
	result.AssertSuccess(t)

	// Agent should be synced to claude
	if !sb.FileExists(filepath.Join(claudeAgents, "helper.md")) {
		t.Error("agent should be synced to claude agents dir")
	}

	// Warning should mention windsurf was skipped
	result.AssertAnyOutputContains(t, "No agents folder: windsurf")
}

func TestSync_Agents_TargetFailureExitsNonZero(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	agentsDir := filepath.Join(filepath.Dir(sb.SourcePath), "agents")
	os.MkdirAll(agentsDir, 0755)
	os.WriteFile(filepath.Join(agentsDir, "helper.md"), []byte("# Helper"), 0644)

	cursorAgents := filepath.Join(sb.Home, ".cursor", "agents")
	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets:
  claude:
    skills:
      path: "` + filepath.Join(sb.Home, ".claude", "skills") + `"
    agents:
      path: "` + filepath.Join(sb.Home, ".claude", "agents") + `"
      include: ["["]
  cursor:
    skills:
      path: "` + filepath.Join(sb.Home, ".cursor", "skills") + `"
    agents:
      path: "` + cursorAgents + `"
`)

	result := sb.RunCLI("sync", "agents")
	result.AssertFailure(t)
	result.AssertRowContains(t, "claude", "invalid agent filter")
	result.AssertAnyOutputContains(t, "some agent targets failed to sync")

	if !sb.FileExists(filepath.Join(cursorAgents, "helper.md")) {
		t.Error("healthy target should still sync when another target fails")
	}
}

func TestSync_Agents_FrontmatterTargetsRestrictsTargets(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	agentsDir := filepath.Join(filepath.Dir(sb.SourcePath), "agents")
	os.MkdirAll(agentsDir, 0755)
	os.WriteFile(filepath.Join(agentsDir, "shared.md"), []byte("# Shared"), 0644)
	os.WriteFile(filepath.Join(agentsDir, "claude-only.md"), []byte("---\ntargets: [claude]\n---\n# Claude only"), 0644)

	claudeAgents := filepath.Join(sb.Home, ".claude", "agents")
	cursorAgents := filepath.Join(sb.Home, ".cursor", "agents")
	os.MkdirAll(claudeAgents, 0755)
	os.MkdirAll(cursorAgents, 0755)

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets:
  claude:
    skills:
      path: "` + filepath.Join(sb.Home, ".claude", "skills") + `"
    agents:
      path: "` + claudeAgents + `"
  cursor:
    skills:
      path: "` + filepath.Join(sb.Home, ".cursor", "skills") + `"
    agents:
      path: "` + cursorAgents + `"
`)

	sb.RunCLI("sync", "agents").AssertSuccess(t)

	if _, err := os.Lstat(filepath.Join(claudeAgents, "claude-only.md")); err != nil {
		t.Error("claude-only.md should be synced to claude")
	}
	if _, err := os.Lstat(filepath.Join(cursorAgents, "shared.md")); err != nil {
		t.Error("shared.md (no targets field) should be synced to cursor")
	}
	if _, err := os.Lstat(filepath.Join(cursorAgents, "claude-only.md")); !os.IsNotExist(err) {
		t.Error("claude-only.md should NOT be synced to cursor")
	}
}
