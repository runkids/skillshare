//go:build !online

package integration

import (
	"os"
	"path/filepath"
	"testing"

	"skillshare/internal/testutil"
)

// --- status agents ---

func TestStatus_Agents_ShowsAgentInfo(t *testing.T) {
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

	result := sb.RunCLI("status", "agents")
	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, "Agents")
	result.AssertAnyOutputContains(t, "2 agents")
}

func TestStatus_Agents_JSON_IncludesAgents(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	createAgentSource(t, sb, map[string]string{
		"tutor.md": "# Tutor agent",
	})

	sb.WriteConfig(`source: ` + sb.SourcePath + "\ntargets: {}\n")

	result := sb.RunCLI("status", "agents", "--json")
	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, `"agents"`)
	result.AssertAnyOutputContains(t, `"count"`)
}

func TestStatus_Default_NoAgentSection(t *testing.T) {
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
	// Default status should NOT include agent section
	result.AssertOutputNotContains(t, "Agents")
}

func TestStatus_All_ShowsBoth(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.CreateSkill("my-skill", map[string]string{
		"SKILL.md": "---\nname: my-skill\n---\n# Content",
	})
	createAgentSource(t, sb, map[string]string{
		"tutor.md": "# Tutor agent",
	})
	sb.WriteConfig(`source: ` + sb.SourcePath + "\ntargets: {}\n")

	result := sb.RunCLI("status", "all")
	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, "Source")  // skill section
	result.AssertAnyOutputContains(t, "Agents")  // agent section
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
	result.AssertAnyOutputContains(t, "Agents source")
	result.AssertAnyOutputContains(t, "1 agents")
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
	result.AssertAnyOutputContains(t, "1 agents")
	result.AssertOutputNotContains(t, "drift")
}
