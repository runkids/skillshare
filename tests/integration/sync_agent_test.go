//go:build !online

package integration

import (
	"os"
	"path/filepath"
	"testing"

	"skillshare/internal/testutil"
)

func TestSync_Agents_IncludeFilter(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	agentsDir := filepath.Join(filepath.Dir(sb.SourcePath), "agents")
	os.MkdirAll(agentsDir, 0755)
	os.WriteFile(filepath.Join(agentsDir, "tutor.md"), []byte("# Tutor"), 0644)
	os.WriteFile(filepath.Join(agentsDir, "reviewer.md"), []byte("# Reviewer"), 0644)
	os.WriteFile(filepath.Join(agentsDir, "debugger.md"), []byte("# Debugger"), 0644)

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
        - "tutor"
        - "reviewer"
`)

	result := sb.RunCLI("sync", "agents")
	result.AssertSuccess(t)

	// Included agents should be synced
	if _, err := os.Lstat(filepath.Join(claudeAgents, "tutor.md")); err != nil {
		t.Error("tutor.md should be synced (included)")
	}
	if _, err := os.Lstat(filepath.Join(claudeAgents, "reviewer.md")); err != nil {
		t.Error("reviewer.md should be synced (included)")
	}

	// Excluded agent should NOT be synced
	if _, err := os.Lstat(filepath.Join(claudeAgents, "debugger.md")); !os.IsNotExist(err) {
		t.Error("debugger.md should NOT be synced (not in include list)")
	}
}

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

func TestSync_Agents_ExcludeFilter(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	agentsDir := filepath.Join(filepath.Dir(sb.SourcePath), "agents")
	os.MkdirAll(agentsDir, 0755)
	os.WriteFile(filepath.Join(agentsDir, "tutor.md"), []byte("# Tutor"), 0644)
	os.WriteFile(filepath.Join(agentsDir, "reviewer.md"), []byte("# Reviewer"), 0644)
	os.WriteFile(filepath.Join(agentsDir, "debugger.md"), []byte("# Debugger"), 0644)

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
      exclude:
        - "debugger"
`)

	result := sb.RunCLI("sync", "agents")
	result.AssertSuccess(t)

	// Non-excluded agents should be synced
	if _, err := os.Lstat(filepath.Join(claudeAgents, "tutor.md")); err != nil {
		t.Error("tutor.md should be synced (not excluded)")
	}
	if _, err := os.Lstat(filepath.Join(claudeAgents, "reviewer.md")); err != nil {
		t.Error("reviewer.md should be synced (not excluded)")
	}

	// Excluded agent should NOT be synced
	if _, err := os.Lstat(filepath.Join(claudeAgents, "debugger.md")); !os.IsNotExist(err) {
		t.Error("debugger.md should NOT be synced (excluded)")
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

func TestSync_Agents_GlobExcludePattern(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	agentsDir := filepath.Join(filepath.Dir(sb.SourcePath), "agents")
	os.MkdirAll(agentsDir, 0755)
	os.WriteFile(filepath.Join(agentsDir, "alpha.md"), []byte("# Alpha"), 0644)
	os.WriteFile(filepath.Join(agentsDir, "beta.md"), []byte("# Beta"), 0644)
	os.WriteFile(filepath.Join(agentsDir, "gamma.md"), []byte("# Gamma"), 0644)

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
      exclude:
        - "?eta"
        - "gamma"
`)

	result := sb.RunCLI("sync", "agents")
	result.AssertSuccess(t)

	// alpha doesn't match any exclude → synced
	if _, err := os.Lstat(filepath.Join(claudeAgents, "alpha.md")); err != nil {
		t.Error("alpha.md should be synced")
	}

	// beta matches ?eta → NOT synced
	if _, err := os.Lstat(filepath.Join(claudeAgents, "beta.md")); !os.IsNotExist(err) {
		t.Error("beta.md should NOT be synced (excluded by ?eta)")
	}

	// gamma matches gamma → NOT synced
	if _, err := os.Lstat(filepath.Join(claudeAgents, "gamma.md")); !os.IsNotExist(err) {
		t.Error("gamma.md should NOT be synced (excluded by gamma)")
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
	result.AssertAnyOutputContains(t, "skipped")
	result.AssertAnyOutputContains(t, "windsurf")
}
