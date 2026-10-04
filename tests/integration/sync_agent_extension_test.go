//go:build !online

package integration

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"skillshare/internal/testutil"
)

// writeAgentExtension installs a directory-form extension into the global
// extensions dir and returns its name.
func writeAgentExtension(t *testing.T, sb *testutil.Sandbox, name, manifest string) string {
	t.Helper()
	extDir := filepath.Join(sb.Home, ".config", "skillshare", "extensions", name)
	if err := os.MkdirAll(extDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(extDir, "extension.yaml"), []byte(manifest), 0644); err != nil {
		t.Fatal(err)
	}
	return name
}

func agentExtensionConfig(sb *testutil.Sandbox, agentsPath, extra string) string {
	return `source: ` + sb.SourcePath + `
targets:
  claude:
    skills:
      path: ` + sb.CreateTarget("claude") + `
    agents:
      path: ` + agentsPath + `
` + extra
}

func TestSync_AgentsExtension_TransformsAndRenames(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	createAgentSource(t, sb, map[string]string{"tutor.md": "body"})
	agentsPath := createAgentTarget(t, sb, "codex")
	ext := writeAgentExtension(t, sb, "upper2toml", "run: [\"tr\", \"a-z\", \"A-Z\"]\noutput_ext: toml\n")
	sb.WriteConfig(agentExtensionConfig(sb, agentsPath, "      extension: "+ext+"\n"))

	sb.RunCLI("sync", "agents").AssertSuccess(t)

	out, err := os.ReadFile(filepath.Join(agentsPath, "tutor.toml"))
	if err != nil {
		t.Fatalf("expected tutor.toml: %v", err)
	}
	if string(out) != "BODY" {
		t.Errorf("output = %q, want BODY", string(out))
	}
}

// Refs #391: status compared extension outputs against the source name and
// content, so every converted agent counted as missing.
func TestStatus_AgentsExtension_CountsConvertedOutputs(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	createAgentSource(t, sb, map[string]string{"reviewer.md": "body", "editor.md": "body2"})
	agentsPath := createAgentTarget(t, sb, "codex")
	ext := writeAgentExtension(t, sb, "upper2toml", "run: [\"tr\", \"a-z\", \"A-Z\"]\noutput_ext: toml\n")
	sb.WriteConfig(agentExtensionConfig(sb, agentsPath, "      extension: "+ext+"\n"))

	sb.RunCLI("sync", "agents").AssertSuccess(t)

	res := sb.RunCLI("status", "--json")
	res.AssertSuccess(t)
	if !strings.Contains(res.Stdout, `"expected": 2`) || !strings.Contains(res.Stdout, `"linked": 2`) || !strings.Contains(res.Stdout, `"drift": false`) {
		t.Errorf("status --json should report 2/2 without drift, got:\n%s", res.Stdout)
	}

	doctor := sb.RunCLI("doctor")
	if !strings.Contains(doctor.Stdout, "2/2 linked") {
		t.Errorf("doctor should report 2/2 linked, got:\n%s", doctor.Stdout)
	}

	// A changed source is drift until it is converted again.
	createAgentSource(t, sb, map[string]string{"reviewer.md": "body v2"})
	res = sb.RunCLI("status", "--json")
	res.AssertSuccess(t)
	if !strings.Contains(res.Stdout, `"linked": 1`) || !strings.Contains(res.Stdout, `"drift": true`) {
		t.Errorf("status --json should report drift after a source change, got:\n%s", res.Stdout)
	}
	sb.RunCLI("sync", "agents").AssertSuccess(t)
	res = sb.RunCLI("status", "--json")
	if !strings.Contains(res.Stdout, `"linked": 2`) || !strings.Contains(res.Stdout, `"drift": false`) {
		t.Errorf("status --json should report 2/2 after resync, got:\n%s", res.Stdout)
	}
}

func TestSync_AgentsExtension_RejectsMergeMode(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	createAgentSource(t, sb, map[string]string{"tutor.md": "body"})
	agentsPath := createAgentTarget(t, sb, "codex")
	ext := writeAgentExtension(t, sb, "id", "run: [\"cat\"]\n")
	sb.WriteConfig(agentExtensionConfig(sb, agentsPath, "      mode: merge\n      extension: "+ext+"\n"))

	result := sb.RunCLI("sync", "agents")
	if result.ExitCode == 0 {
		t.Fatal("expected non-zero exit for extension + merge mode")
	}
}

func TestSync_AgentsExtension_PrunesByOutputName(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	createAgentSource(t, sb, map[string]string{"tutor.md": "body"})
	agentsPath := createAgentTarget(t, sb, "codex")
	// A plain copy left over from before the extension was configured.
	if err := os.WriteFile(filepath.Join(agentsPath, "tutor.md"), []byte("body"), 0644); err != nil {
		t.Fatal(err)
	}
	ext := writeAgentExtension(t, sb, "upper2toml", "run: [\"tr\", \"a-z\", \"A-Z\"]\noutput_ext: toml\n")
	sb.WriteConfig(agentExtensionConfig(sb, agentsPath, "      extension: "+ext+"\n"))

	sb.RunCLI("sync", "agents").AssertSuccess(t)

	if _, err := os.Stat(filepath.Join(agentsPath, "tutor.md")); !os.IsNotExist(err) {
		t.Error("stale tutor.md copy should be pruned once the target emits tutor.toml")
	}
}

func TestCollect_AgentsExtension_NeverPullsTransformedOutput(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	agentsSource := createAgentSource(t, sb, map[string]string{"tutor.md": "body"})
	agentsPath := createAgentTarget(t, sb, "codex")
	// Output keeps .md, so without a guard collect would see it as a local agent.
	ext := writeAgentExtension(t, sb, "upper", "run: [\"tr\", \"a-z\", \"A-Z\"]\n")
	sb.WriteConfig(agentExtensionConfig(sb, agentsPath, "      extension: "+ext+"\n"))
	sb.RunCLI("sync", "agents").AssertSuccess(t)

	sb.RunCLI("collect", "agents", "--all", "--force")

	src, err := os.ReadFile(filepath.Join(agentsSource, "tutor.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(src) != "body" {
		t.Errorf("source overwritten by transformed output: %q", string(src))
	}
}

func TestCollect_AgentsExtension_NamedTargetRefused(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	createAgentSource(t, sb, map[string]string{"tutor.md": "body"})
	agentsPath := createAgentTarget(t, sb, "codex")
	ext := writeAgentExtension(t, sb, "upper", "run: [\"tr\", \"a-z\", \"A-Z\"]\n")
	sb.WriteConfig(agentExtensionConfig(sb, agentsPath, "      extension: "+ext+"\n"))

	result := sb.RunCLI("collect", "agents", "claude")
	result.AssertFailure(t)
	result.AssertAnyOutputContains(t, "extension")
}

// opencodeAgentsExtension returns the bundled opencode-agents extension path,
// skipping when node is unavailable.
func opencodeAgentsExtension(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("node not available")
	}
	path, err := filepath.Abs(filepath.Join("..", "..", "extensions", "opencode-agents"))
	if err != nil {
		t.Fatal(err)
	}
	return path
}

func TestSync_AgentsExtension_OpencodeRejectsClaudeTools(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	createAgentSource(t, sb, map[string]string{"a.md": "---\ndescription: d\ntools: Read, Bash(git log)\n---\nbody\n"})
	agentsPath := createAgentTarget(t, sb, "opencode")
	sb.WriteConfig(agentExtensionConfig(sb, agentsPath, "      extension: "+opencodeAgentsExtension(t)+"\n"))

	result := sb.RunCLI("sync", "agents")
	result.AssertFailure(t)
	if !strings.Contains(result.Output(), "tools") {
		t.Errorf("error should name the tools field, got: %s", result.Output())
	}
	if _, err := os.Stat(filepath.Join(agentsPath, "a.md")); !os.IsNotExist(err) {
		t.Error("an agent whose tools can't be mapped must not be written")
	}
}

func TestSync_AgentsExtension_PartialFailureStillPrunes(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	agentsDir := createAgentSource(t, sb, map[string]string{"good.md": "ok", "bad.md": "FAIL", "gone.md": "old"})
	agentsPath := createAgentTarget(t, sb, "codex")
	// A copy synced earlier whose source agent was then removed.
	sb.WriteConfig(agentExtensionConfig(sb, agentsPath, "      mode: copy\n"))
	sb.RunCLI("sync", "agents").AssertSuccess(t)
	if err := os.Remove(filepath.Join(agentsDir, "gone.md")); err != nil {
		t.Fatal(err)
	}
	orphan := filepath.Join(agentsPath, "gone.md")
	ext := writeAgentExtension(t, sb, "picky", "run: [\"sh\", \"-c\", \"grep -q FAIL \\\"$SS_SRC_PATH\\\" && exit 1; cat\"]\n")
	sb.WriteConfig(agentExtensionConfig(sb, agentsPath, "      extension: "+ext+"\n"))

	sb.RunCLI("sync", "agents").AssertFailure(t)

	if _, err := os.Stat(filepath.Join(agentsPath, "good.md")); err != nil {
		t.Errorf("good.md should still sync: %v", err)
	}
	if _, err := os.Stat(orphan); !os.IsNotExist(err) {
		t.Error("orphans should be pruned even when one agent fails")
	}
}
