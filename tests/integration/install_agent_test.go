//go:build !online

package integration

import (
	"os"
	"path/filepath"
	"testing"

	"skillshare/internal/testutil"
)

func TestInstall_AgentFlag_ParsesCorrectly(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)

	// --kind with invalid value should error
	result := sb.RunCLI("install", "--kind", "invalid", "test")
	result.AssertFailure(t)
	result.AssertAnyOutputContains(t, "must be 'skill' or 'agent'")
}

func TestInstall_AgentFlagShort_ParsesCorrectly(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)

	// -a without value should error
	result := sb.RunCLI("install", "-a")
	result.AssertFailure(t)
	result.AssertAnyOutputContains(t, "requires agent name")
}

func TestCheck_Agents_EmptyDir(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)

	// Create agents source dir
	agentsDir := filepath.Join(filepath.Dir(sb.SourcePath), "agents")
	os.MkdirAll(agentsDir, 0755)

	result := sb.RunCLI("check", "agents")
	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, "No agents found")
}

func TestCheck_Agents_LocalAgent(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)

	// Create agents source dir with a local agent
	agentsDir := filepath.Join(filepath.Dir(sb.SourcePath), "agents")
	os.MkdirAll(agentsDir, 0755)
	os.WriteFile(filepath.Join(agentsDir, "tutor.md"), []byte("# Tutor agent"), 0644)

	result := sb.RunCLI("check", "agents")
	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, "tutor")
	result.AssertAnyOutputContains(t, "local")
}

func TestCheck_Agents_JsonOutput(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)

	agentsDir := filepath.Join(filepath.Dir(sb.SourcePath), "agents")
	os.MkdirAll(agentsDir, 0755)
	os.WriteFile(filepath.Join(agentsDir, "tutor.md"), []byte("# Tutor"), 0644)

	result := sb.RunCLI("check", "agents", "--json")
	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, `"name"`)
	result.AssertAnyOutputContains(t, `"status"`)
}

func TestEnable_KindAgent_ParsesCorrectly(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)

	// Create agents source dir
	agentsDir := filepath.Join(filepath.Dir(sb.SourcePath), "agents")
	os.MkdirAll(agentsDir, 0755)

	// Disable an agent — --kind goes after -g (mode flag)
	result := sb.RunCLI("disable", "-g", "--kind", "agent", "tutor")
	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, ".agentignore")

	// Verify .agentignore was created
	agentIgnorePath := filepath.Join(agentsDir, ".agentignore")
	if !sb.FileExists(agentIgnorePath) {
		t.Error(".agentignore should be created")
	}
}

func TestUninstall_KindAgent_ParsesFlag(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)

	// --kind agent with nonexistent agent — should error with agent not found
	result := sb.RunCLI("uninstall", "-g", "--kind", "agent", "nonexistent")
	// May fail because agent doesn't exist, but should parse flags correctly (no "unknown option")
	result.AssertOutputNotContains(t, "unknown option")
}
