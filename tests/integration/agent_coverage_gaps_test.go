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

// --- trash agents empty ---

func TestTrash_Agents_Empty(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	createAgentSource(t, sb, map[string]string{
		"tutor.md":    "# Tutor agent",
		"reviewer.md": "# Reviewer agent",
	})
	sb.WriteConfig(`source: ` + sb.SourcePath + "\ntargets: {}\n")

	// Uninstall both agents to trash
	sb.RunCLI("uninstall", "-g", "agents", "--all", "--force")

	// Verify trash has items
	listResult := sb.RunCLI("trash", "agents", "list", "--no-tui")
	listResult.AssertSuccess(t)
	listResult.AssertAnyOutputContains(t, "tutor")

	// Empty agent trash (use --force via input "y")
	emptyResult := sb.RunCLIWithInput("y\n", "trash", "agents", "empty")
	emptyResult.AssertSuccess(t)
	emptyResult.AssertAnyOutputContains(t, "Emptied trash")

	// Verify trash is now empty
	afterResult := sb.RunCLI("trash", "agents", "list", "--no-tui")
	afterResult.AssertSuccess(t)
	afterResult.AssertAnyOutputContains(t, "empty")
}

// --- trash agents delete ---

func TestTrash_Agents_Delete(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	createAgentSource(t, sb, map[string]string{
		"tutor.md": "# Tutor agent",
	})
	sb.WriteConfig(`source: ` + sb.SourcePath + "\ntargets: {}\n")

	sb.RunCLI("uninstall", "-g", "agents", "tutor", "--force")

	// Delete specific item from agent trash
	result := sb.RunCLI("trash", "agents", "delete", "tutor")
	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, "Permanently deleted")
}

// --- uninstall agents project mode ---

func TestUninstall_Agents_ProjectMode(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	// Setup project
	projectDir := filepath.Join(sb.Root, "myproject")
	os.MkdirAll(filepath.Join(projectDir, ".skillshare", "skills"), 0755)
	agentsDir := filepath.Join(projectDir, ".skillshare", "agents")
	os.MkdirAll(agentsDir, 0755)
	os.WriteFile(filepath.Join(agentsDir, "tutor.md"), []byte("# Tutor agent"), 0644)

	// Write project config
	projectCfgDir := filepath.Join(projectDir, ".skillshare")
	os.WriteFile(filepath.Join(projectCfgDir, "config.yaml"), []byte("targets:\n  - claude\n"), 0644)

	// Also need global config for the CLI to not error
	sb.WriteConfig(`source: ` + sb.SourcePath + "\ntargets: {}\n")

	result := sb.RunCLIInDir(projectDir, "uninstall", "-p", "agents", "tutor", "--force")
	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, "Uninstalled 1 agent")

	// Verify removed
	if _, err := os.Stat(filepath.Join(agentsDir, "tutor.md")); !os.IsNotExist(err) {
		t.Error("agent should be removed from project agents dir")
	}
}

// --- check all (combined) ---

func TestCheck_All_CombinedOutput(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.CreateSkill("my-skill", map[string]string{
		"SKILL.md": "---\nname: my-skill\n---\n# Content",
	})
	createAgentSource(t, sb, map[string]string{
		"tutor.md": "# Tutor agent",
	})
	sb.WriteConfig(`source: ` + sb.SourcePath + "\ntargets: {}\n")

	// "check --all" should show both skills and agents
	// Currently "check" defaults to skills-only, "check agents" is agents-only
	// "check --all" should combine both
	result := sb.RunCLI("check", "--all")
	result.AssertSuccess(t)
}

// --- multi-target agent config ---

func TestSync_Agents_SkipsTargetsWithoutAgentPath(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	createAgentSource(t, sb, map[string]string{
		"tutor.md": "# Tutor agent",
	})
	claudeAgents := createAgentTarget(t, sb, "claude")

	// claude has agent path, cursor does NOT
	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets:
  claude:
    skills:
      path: ` + sb.CreateTarget("claude") + `
    agents:
      path: ` + claudeAgents + `
  cursor:
    skills:
      path: ` + sb.CreateTarget("cursor") + `
`)

	result := sb.RunCLI("sync", "agents")
	result.AssertSuccess(t)

	// Claude agents should be synced
	if _, err := os.Lstat(filepath.Join(claudeAgents, "tutor.md")); err != nil {
		t.Error("claude agent should be synced")
	}
}

func TestDiff_Default_ShowsAgentPruneAfterUninstallAll(t *testing.T) {
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

	sb.RunCLI("sync", "-g", "agents").AssertSuccess(t)
	sb.RunCLI("uninstall", "-g", "agents", "--all", "--force").AssertSuccess(t)

	result := sb.RunCLI("diff", "-g", "--json")
	result.AssertSuccess(t)

	output := parseJSON(t, result.Stdout)
	targets, ok := output["targets"].([]any)
	if !ok || len(targets) == 0 {
		t.Fatalf("expected diff targets, got %v", output["targets"])
	}

	foundPrune := false
	for _, rawTarget := range targets {
		target, ok := rawTarget.(map[string]any)
		if !ok || target["name"] != "claude" {
			continue
		}
		items, _ := target["items"].([]any)
		for _, rawItem := range items {
			item, ok := rawItem.(map[string]any)
			if !ok {
				continue
			}
			if item["name"] == "tutor.md" && item["kind"] == "agent" && item["action"] == "remove" {
				foundPrune = true
			}
		}
	}

	if !foundPrune {
		pretty, _ := json.MarshalIndent(output, "", "  ")
		t.Fatalf("expected agent prune in diff output, got:\n%s", string(pretty))
	}
}

func TestSync_Agents_PrunesTargetAfterUninstallAll(t *testing.T) {
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

	sb.RunCLI("sync", "-g", "agents").AssertSuccess(t)
	sb.RunCLI("uninstall", "-g", "agents", "--all", "--force").AssertSuccess(t)

	syncResult := sb.RunCLI("sync", "-g", "agents")
	syncResult.AssertSuccess(t)
	syncResult.AssertAnyOutputContains(t, "1 pruned")

	if _, err := os.Lstat(filepath.Join(claudeAgents, "tutor.md")); !os.IsNotExist(err) {
		t.Fatalf("expected tutor.md to be pruned from target, got err=%v", err)
	}
}

// --- list agents JSON with kind field ---

func TestList_Agents_JSON_AllEntriesHaveKind(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.CreateSkill("my-skill", map[string]string{
		"SKILL.md": "---\nname: my-skill\n---\n# Content",
	})
	createAgentSource(t, sb, map[string]string{
		"tutor.md": "# Tutor agent",
	})
	sb.WriteConfig(`source: ` + sb.SourcePath + "\ntargets: {}\n")

	// "list --all --json" should have kind on every entry
	result := sb.RunCLI("list", "--all", "--json")
	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, `"kind": "skill"`)
	result.AssertAnyOutputContains(t, `"kind": "agent"`)
}

// --- status agents JSON with targets ---

func TestStatus_Agents_JSON_WithTargets(t *testing.T) {
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

	// Sync agents
	sb.RunCLI("sync", "agents")

	result := sb.RunCLI("status", "agents", "--json")
	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, `"agents"`)
	result.AssertAnyOutputContains(t, `"expected"`)
	result.AssertAnyOutputContains(t, `"linked"`)
}

// Refs #395: status counted every discovered agent, so a target that excludes
// some of them was reported as drifted after a complete sync.
func TestStatus_Agents_JSON_AppliesSyncFilters(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	agentsDir := createAgentSource(t, sb, map[string]string{
		"tutor.md":    "# Tutor",
		"reviewer.md": "# Reviewer",
		"ignored.md":  "# Ignored",
		"cursor.md":   "---\ntargets: [cursor]\n---\n# Cursor only",
	})
	sb.WriteFile(filepath.Join(agentsDir, ".agentignore"), "ignored.md\n")
	claudeAgents := createAgentTarget(t, sb, "claude")

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets:
  claude:
    skills:
      path: ` + sb.CreateTarget("claude") + `
    agents:
      path: ` + claudeAgents + `
      exclude: [reviewer]
`)

	sb.RunCLI("sync", "agents").AssertSuccess(t)

	result := sb.RunCLI("status", "--json")
	result.AssertSuccess(t)
	if !strings.Contains(result.Stdout, `"expected": 1`) || !strings.Contains(result.Stdout, `"linked": 1`) || !strings.Contains(result.Stdout, `"drift": false`) {
		t.Errorf("status --json should expect only tutor (1/1, no drift), got:\n%s", result.Stdout)
	}

	human := sb.RunCLI("status")
	human.AssertSuccess(t)
	if strings.Contains(human.Stdout, "1/4") {
		t.Errorf("status should not report 1/4 agents, got:\n%s", human.Stdout)
	}
}
