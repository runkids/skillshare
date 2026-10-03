//go:build !online

package integration

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"skillshare/internal/testutil"
)

// --- update agents ---

func TestUpdate_Agents_NoAgents(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	agentsDir := createAgentSource(t, sb, nil)
	_ = agentsDir

	sb.WriteConfig(`source: ` + sb.SourcePath + "\ntargets: {}\n")

	result := sb.RunCLI("update", "agents", "--all")
	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, "No agents found")
}

func TestUpdate_Agents_LocalOnly(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	createAgentSource(t, sb, map[string]string{
		"tutor.md": "# Tutor agent",
	})
	sb.WriteConfig(`source: ` + sb.SourcePath + "\ntargets: {}\n")

	result := sb.RunCLI("update", "agents", "--all")
	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, "local")
}

func TestUpdate_Agents_GroupInvalidDir(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	createAgentSource(t, sb, map[string]string{
		"tutor.md": "# Tutor agent",
	})
	sb.WriteConfig(`source: ` + sb.SourcePath + "\ntargets: {}\n")

	result := sb.RunCLI("update", "agents", "--group", "nonexistent")
	result.AssertFailure(t)
	result.AssertAnyOutputContains(t, "not found")
}

func TestUpdate_Agents_RequiresNameOrAll(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	createAgentSource(t, sb, map[string]string{
		"tutor.md": "# Tutor agent",
	})
	sb.WriteConfig(`source: ` + sb.SourcePath + "\ntargets: {}\n")

	result := sb.RunCLI("update", "agents")
	result.AssertFailure(t)
	result.AssertAnyOutputContains(t, "specify agent name")
}

func TestUpdate_Agents_HighFindingBlockedByThreshold(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.WriteConfig(`source: ` + sb.SourcePath + "\ntargets: {}\n")

	repoDir := filepath.Join(sb.Home, "agent-audit-repo")
	if err := os.MkdirAll(repoDir, 0o755); err != nil {
		t.Fatalf("mkdir repo: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoDir, "reviewer.md"), []byte("# Reviewer v1\n"), 0o644); err != nil {
		t.Fatalf("write initial agent: %v", err)
	}
	initGitRepo(t, repoDir)

	installResult := sb.RunCLI("install", "file://"+repoDir, "--kind", "agent", "--skip-audit")
	installResult.AssertSuccess(t)

	agentsDir := filepath.Join(filepath.Dir(sb.SourcePath), "agents")
	installedAgent := filepath.Join(agentsDir, "reviewer.md")

	if err := os.WriteFile(filepath.Join(repoDir, "reviewer.md"), []byte("# Reviewer v2\nsudo apt-get install -y jq\n"), 0o644); err != nil {
		t.Fatalf("write updated agent: %v", err)
	}
	for _, args := range [][]string{
		{"add", "reviewer.md"},
		{"commit", "-m", "introduce high finding"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = repoDir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v failed: %s %v", args, out, err)
		}
	}

	result := sb.RunCLI("update", "agents", "--all", "-T", "h")
	result.AssertFailure(t)
	result.AssertAnyOutputContains(t, "security audit")

	content, err := os.ReadFile(installedAgent)
	if err != nil {
		t.Fatalf("read installed agent: %v", err)
	}
	if string(content) != "# Reviewer v1\n" {
		t.Fatalf("expected blocked update to preserve original content, got %q", string(content))
	}
}

// --- uninstall agents ---

func TestUninstall_Agents_RemovesToTrash(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	agentsDir := createAgentSource(t, sb, map[string]string{
		"tutor.md": "# Tutor agent",
	})
	sb.WriteConfig(`source: ` + sb.SourcePath + "\ntargets: {}\n")

	result := sb.RunCLI("uninstall", "-g", "agents", "tutor", "--force")
	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, "Uninstalled 1 agent")
	result.AssertAnyOutputContains(t, "tutor")

	// Verify agent file was removed from source
	if _, err := os.Stat(filepath.Join(agentsDir, "tutor.md")); !os.IsNotExist(err) {
		t.Error("agent file should be removed from source")
	}
}

func TestUninstall_Agents_NotFound(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	createAgentSource(t, sb, nil)
	sb.WriteConfig(`source: ` + sb.SourcePath + "\ntargets: {}\n")

	result := sb.RunCLI("uninstall", "-g", "agents", "nonexistent", "--force")
	result.AssertFailure(t)
	result.AssertAnyOutputContains(t, "not found")
}

func TestUninstall_Agents_All(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	agentsDir := createAgentSource(t, sb, map[string]string{
		"tutor.md":    "# Tutor agent",
		"reviewer.md": "# Reviewer agent",
	})
	sb.WriteConfig(`source: ` + sb.SourcePath + "\ntargets: {}\n")

	result := sb.RunCLI("uninstall", "-g", "agents", "--all", "--force")
	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, "Uninstalled 2 agents")

	// Verify both files removed
	if _, err := os.Stat(filepath.Join(agentsDir, "tutor.md")); !os.IsNotExist(err) {
		t.Error("tutor.md should be removed")
	}
	if _, err := os.Stat(filepath.Join(agentsDir, "reviewer.md")); !os.IsNotExist(err) {
		t.Error("reviewer.md should be removed")
	}
}

// --- collect agents ---

func TestCollect_Agents_NoLocalAgents(t *testing.T) {
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

	// Sync agents first (creates symlinks)
	sb.RunCLI("sync", "agents")

	// Collect should find no local (non-symlinked) agents
	result := sb.RunCLI("collect", "agents")
	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, "No local agents")
}

func TestCollect_Agents_CollectsLocalFiles(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	createAgentSource(t, sb, nil)
	claudeAgents := createAgentTarget(t, sb, "claude")

	// Create a local (non-symlinked) agent in the target
	os.WriteFile(filepath.Join(claudeAgents, "local-agent.md"), []byte("# Local agent"), 0644)

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets:
  claude:
    skills:
      path: ` + sb.CreateTarget("claude") + `
    agents:
      path: ` + claudeAgents + `
`)

	agentsSource := filepath.Join(filepath.Dir(sb.SourcePath), "agents")

	result := sb.RunCLI("collect", "agents", "--force")
	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, "Collected 1 agent")

	// Verify the file was copied to agent source
	if _, err := os.Stat(filepath.Join(agentsSource, "local-agent.md")); err != nil {
		t.Error("local-agent.md should be collected to agent source")
	}
}

func TestCollect_Agents_SpecificTargetOnly(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	agentsSource := createAgentSource(t, sb, nil)
	claudeAgents := createAgentTarget(t, sb, "claude")
	cursorAgents := createAgentTarget(t, sb, "cursor")

	os.WriteFile(filepath.Join(claudeAgents, "claude-agent.md"), []byte("# Claude"), 0644)
	os.WriteFile(filepath.Join(cursorAgents, "cursor-agent.md"), []byte("# Cursor"), 0644)

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
    agents:
      path: ` + cursorAgents + `
`)

	result := sb.RunCLI("collect", "agents", "claude", "--force")
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

func TestCollect_Agents_MultipleTargets_RequiresAllOrName(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	agentsSource := createAgentSource(t, sb, nil)
	claudeAgents := createAgentTarget(t, sb, "claude")
	cursorAgents := createAgentTarget(t, sb, "cursor")

	os.WriteFile(filepath.Join(claudeAgents, "claude-agent.md"), []byte("# Claude"), 0644)
	os.WriteFile(filepath.Join(cursorAgents, "cursor-agent.md"), []byte("# Cursor"), 0644)

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
    agents:
      path: ` + cursorAgents + `
`)

	result := sb.RunCLI("collect", "agents")
	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, "Specify a target")

	if _, err := os.Stat(filepath.Join(agentsSource, "claude-agent.md")); !os.IsNotExist(err) {
		t.Error("claude-agent.md should not be collected without target selection")
	}
	if _, err := os.Stat(filepath.Join(agentsSource, "cursor-agent.md")); !os.IsNotExist(err) {
		t.Error("cursor-agent.md should not be collected without target selection")
	}
}

func TestCollect_Agents_DryRun_DoesNotWrite(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	agentsSource := createAgentSource(t, sb, nil)
	claudeAgents := createAgentTarget(t, sb, "claude")

	os.WriteFile(filepath.Join(claudeAgents, "local-agent.md"), []byte("# Local agent"), 0644)

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets:
  claude:
    skills:
      path: ` + sb.CreateTarget("claude") + `
    agents:
      path: ` + claudeAgents + `
`)

	result := sb.RunCLI("collect", "agents", "--dry-run")
	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, "Dry run")

	if _, err := os.Stat(filepath.Join(agentsSource, "local-agent.md")); !os.IsNotExist(err) {
		t.Error("dry-run should not collect local-agent.md")
	}
}

func TestCollect_Agents_ExistingSource_SkipsWithoutForce(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	agentsSource := createAgentSource(t, sb, map[string]string{
		"local-agent.md": "# Source version",
	})
	claudeAgents := createAgentTarget(t, sb, "claude")

	os.WriteFile(filepath.Join(claudeAgents, "local-agent.md"), []byte("# Target version"), 0644)

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets:
  claude:
    skills:
      path: ` + sb.CreateTarget("claude") + `
    agents:
      path: ` + claudeAgents + `
`)

	result := sb.RunCLIWithInput("y\n", "collect", "agents", "claude")
	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, "skipped")

	content, err := os.ReadFile(filepath.Join(agentsSource, "local-agent.md"))
	if err != nil {
		t.Fatalf("failed to read source agent: %v", err)
	}
	if string(content) != "# Source version" {
		t.Errorf("source agent should not be overwritten, got %q", string(content))
	}
}

func TestCollect_Agents_Force_OverwritesSource(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	agentsSource := createAgentSource(t, sb, map[string]string{
		"local-agent.md": "# Source version",
	})
	claudeAgents := createAgentTarget(t, sb, "claude")

	os.WriteFile(filepath.Join(claudeAgents, "local-agent.md"), []byte("# Target version"), 0644)

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets:
  claude:
    skills:
      path: ` + sb.CreateTarget("claude") + `
    agents:
      path: ` + claudeAgents + `
`)

	result := sb.RunCLI("collect", "agents", "claude", "--force")
	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, "copied to source")

	content, err := os.ReadFile(filepath.Join(agentsSource, "local-agent.md"))
	if err != nil {
		t.Fatalf("failed to read source agent: %v", err)
	}
	if string(content) != "# Target version" {
		t.Errorf("source agent should be overwritten, got %q", string(content))
	}
}

// --- trash agents ---

func TestTrash_Agents_ListEmpty(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.WriteConfig(`source: ` + sb.SourcePath + "\ntargets: {}\n")

	result := sb.RunCLI("trash", "agents", "list", "--no-tui")
	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, "empty")
}

func TestTrash_Agents_ListAfterUninstall(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	createAgentSource(t, sb, map[string]string{
		"tutor.md": "# Tutor agent",
	})
	sb.WriteConfig(`source: ` + sb.SourcePath + "\ntargets: {}\n")

	// Uninstall to trash
	sb.RunCLI("uninstall", "-g", "agents", "tutor", "--force")

	// List agent trash
	result := sb.RunCLI("trash", "agents", "list", "--no-tui")
	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, "tutor")
}

func TestTrash_Agents_Restore(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	agentsDir := createAgentSource(t, sb, map[string]string{
		"tutor.md": "# Tutor agent",
	})
	sb.WriteConfig(`source: ` + sb.SourcePath + "\ntargets: {}\n")

	// Uninstall
	sb.RunCLI("uninstall", "-g", "agents", "tutor", "--force")

	// Verify removed
	if _, err := os.Stat(filepath.Join(agentsDir, "tutor.md")); !os.IsNotExist(err) {
		t.Fatal("should be removed after uninstall")
	}

	// Restore
	result := sb.RunCLI("trash", "agents", "restore", "tutor")
	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, "✓ Restore")

	// Verify restored to agent source
	if _, err := os.Stat(filepath.Join(agentsDir, "tutor.md")); err != nil {
		t.Error("tutor.md should be restored to agent source")
	}
}

func TestTrash_Agents_Restore_Nested_DoesNotGoToSkills(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	agentsDir := createAgentSource(t, sb, map[string]string{
		"demo/code-archaeologist.md": "# Code Archaeologist",
	})
	sb.WriteConfig(`source: ` + sb.SourcePath + "\ntargets: {}\n")

	sb.RunCLI("uninstall", "-g", "agents", "demo/code-archaeologist", "--force")

	result := sb.RunCLI("trash", "agents", "restore", "demo/code-archaeologist")
	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, "✓ Restore")

	if _, err := os.Stat(filepath.Join(agentsDir, "demo", "code-archaeologist.md")); err != nil {
		t.Fatalf("nested agent should be restored to agents source: %v", err)
	}

	wrongSkillsPath := filepath.Join(sb.SourcePath, "agents", "demo", "code-archaeologist", "code-archaeologist.md")
	if _, err := os.Stat(wrongSkillsPath); err == nil {
		t.Fatalf("nested agent should not be restored into skills tree: %s", wrongSkillsPath)
	}
}

// --- default behavior unchanged ---

func TestTrash_Default_SkillsOnly(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.WriteConfig(`source: ` + sb.SourcePath + "\ntargets: {}\n")

	// Default trash list should check skill trash (not agent trash)
	result := sb.RunCLI("trash", "list", "--no-tui")
	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, "empty")
}

func TestTrash_Default_SkillsOnly_IgnoresAgentTrash(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	createAgentSource(t, sb, map[string]string{
		"demo/tutor.md": "# Tutor agent",
	})
	sb.WriteConfig(`source: ` + sb.SourcePath + "\ntargets: {}\n")

	sb.RunCLI("uninstall", "-g", "agents", "demo/tutor", "--force")

	result := sb.RunCLI("trash", "list", "--no-tui")
	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, "empty")

	restore := sb.RunCLI("trash", "restore", "demo/tutor")
	restore.AssertFailure(t)
	restore.AssertAnyOutputContains(t, "not found in trash")
}
