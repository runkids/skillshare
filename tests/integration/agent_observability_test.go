//go:build !online

package integration

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"skillshare/internal/testutil"
)

// --- status (always shows skills + agents) ---

func TestStatus_ShowsAgentInfo(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	createAgentSource(t, sb, map[string]string{
		"tutor.md":    "# Tutor agent",
		"reviewer.md": "# Reviewer agent",
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

	result := sb.RunCLI("status")
	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, "2 agents") // Source section
	result.AssertAnyOutputContains(t, "agents")   // Targets sub-item
	result.AssertAnyOutputContains(t, "! 0/2")    // agent sync status
}

func TestStatus_JSON_IncludesAgents(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	createAgentSource(t, sb, map[string]string{
		"tutor.md": "# Tutor agent",
	})

	sb.WriteConfig(`source: ` + sb.SourcePath + "\ntargets: {}\n")

	result := sb.RunCLI("status", "--json")
	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, `"agents"`)
	result.AssertAnyOutputContains(t, `"count"`)
}

func TestStatus_Default_ShowsBothSkillsAndAgents(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.CreateSkill("my-skill", map[string]string{
		"SKILL.md": "---\nname: my-skill\n---\n# Content",
	})
	createAgentSource(t, sb, map[string]string{
		"tutor.md": "# Tutor agent",
	})
	sb.WriteConfig(`source: ` + sb.SourcePath + "\ntargets: {}\n")

	result := sb.RunCLI("status")
	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, "Source")   // source section with skills + agents
	result.AssertRowContains(t, "skills", "~/.config/skillshare/skills  1 skill")
	result.AssertRowContains(t, "agents", "~/.config/skillshare/agents  1 agent")
}

// --- diff agents ---

func TestDiff_Agents_JSON_IncludesKind(t *testing.T) {
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

	// Diff before sync should show items with kind field
	result := sb.RunCLI("diff", "--json")
	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, `"kind"`)
	result.AssertAnyOutputContains(t, `"skill"`)
}

// --- doctor agents ---

func TestDoctor_ChecksAgentSource(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	createAgentSource(t, sb, map[string]string{
		"tutor.md": "# Tutor agent",
	})
	sb.WriteConfig(`source: ` + sb.SourcePath + "\ntargets: {}\n")

	result := sb.RunCLI("doctor")
	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, "Agents")
	result.AssertAnyOutputContains(t, "1 agent")
}

func TestDoctor_AgentTargetDrift(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	createAgentSource(t, sb, map[string]string{
		"tutor.md":    "# Tutor agent",
		"reviewer.md": "# Reviewer agent",
	})
	claudeAgents := createAgentTarget(t, sb, "claude")

	// Only sync one agent manually (create symlink for tutor only)
	agentsDir := filepath.Join(filepath.Dir(sb.SourcePath), "agents")
	os.Symlink(
		filepath.Join(agentsDir, "tutor.md"),
		filepath.Join(claudeAgents, "tutor.md"),
	)

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets:
  claude:
    skills:
      path: ` + sb.CreateTarget("claude") + `
    agents:
      path: ` + claudeAgents + `
`)

	result := sb.RunCLI("doctor")
	result.AssertSuccess(t)
	// Should detect drift (1/2 linked)
	result.AssertAnyOutputContains(t, "drift")
}

func TestDoctor_AgentTargetSynced(t *testing.T) {
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

	// Sync agents first
	sb.RunCLI("sync", "agents")

	result := sb.RunCLI("doctor")
	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, "1 agent")
	result.AssertOutputNotContains(t, "drift")
}

func TestCollect_Agents_WritesOplog(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	createAgentSource(t, sb, nil)
	claudeAgents := createAgentTarget(t, sb, "claude")
	if err := os.WriteFile(filepath.Join(claudeAgents, "local-agent.md"), []byte("# Local"), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets:
  claude:
    skills:
      path: ` + sb.CreateTarget("claude") + `
    agents:
      path: ` + claudeAgents + `
`)

	collectResult := sb.RunCLI("collect", "agents", "claude", "--force")
	collectResult.AssertSuccess(t)

	logResult := sb.RunCLI("log", "--json", "--cmd", "collect", "--tail", "1")
	logResult.AssertSuccess(t)

	line := strings.TrimSpace(logResult.Stdout)
	if line == "" {
		t.Fatal("expected collect oplog entry")
	}

	var entry map[string]any
	if err := json.Unmarshal([]byte(line), &entry); err != nil {
		t.Fatalf("failed to parse log entry: %v\nstdout=%s", err, logResult.Stdout)
	}

	if entry["cmd"] != "collect" {
		t.Fatalf("expected cmd=collect, got %v", entry["cmd"])
	}
	if entry["status"] != "ok" {
		t.Fatalf("expected status=ok, got %v", entry["status"])
	}

	args, ok := entry["args"].(map[string]any)
	if !ok {
		t.Fatalf("expected args object, got %T", entry["args"])
	}
	if args["kind"] != "agents" {
		t.Fatalf("expected kind=agents, got %v", args["kind"])
	}
	if args["pulled"] != float64(1) {
		t.Fatalf("expected pulled=1, got %v", args["pulled"])
	}
}
