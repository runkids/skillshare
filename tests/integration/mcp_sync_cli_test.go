//go:build !online

package integration

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"skillshare/internal/testutil"
)

const mcpDocsConfig = "mcp:\n  targets: [claude]\n  servers:\n    docs:\n      url: https://example.com/mcp\n"

func TestMCPSyncKindAfterFlags(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	sb.CreateSkill("demo", map[string]string{"SKILL.md": "---\nname: demo\n---\n# Demo"})
	sb.WriteConfig("source: " + sb.SourcePath + "\ntargets:\n  claude:\n    path: " + sb.CreateTarget("claude") + "\n" + mcpDocsConfig)

	r := sb.RunCLI("sync", "-g", "--dry-run", "mcp")
	r.AssertSuccess(t)
	r.AssertOutputContains(t, "MCP source:")
	r.AssertOutputNotContains(t, "Syncing skills")
}

func TestMCPSyncAllJSONConflict(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	sb.WriteConfig("targets: {}\n" + mcpDocsConfig)
	sb.WriteFile(filepath.Join(sb.Home, ".claude.json"), `{"mcpServers":{"docs":{"type":"http","url":"https://other.example/mcp"}}}`)

	r := sb.RunCLI("sync", "--all", "--json", "-g")
	r.AssertFailure(t)
	var output struct {
		Error string `json:"error"`
		MCP   struct {
			Plan struct {
				Changes []struct {
					Action string `json:"action"`
				} `json:"changes"`
			} `json:"plan"`
		} `json:"mcp"`
	}
	if err := json.Unmarshal([]byte(r.Stdout), &output); err != nil {
		t.Fatalf("stdout is not JSON: %v\n%s", err, r.Stdout)
	}
	if output.Error == "" || len(output.MCP.Plan.Changes) != 1 || output.MCP.Plan.Changes[0].Action != "conflict" {
		t.Fatalf("blocked preflight lacks error or conflict details: %s", r.Stdout)
	}
}

func TestMCPSyncAllSkipsMCPAfterResourceFailure(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	sb.CreateSkill("demo", map[string]string{"SKILL.md": "---\nname: demo\n---\n# Demo"})
	target := sb.CreateTarget("broken")
	// An unreadable manifest fails this target's skills sync.
	if err := os.Mkdir(filepath.Join(target, ".skillshare-manifest.json"), 0755); err != nil {
		t.Fatal(err)
	}
	sb.WriteConfig("source: " + sb.SourcePath + "\ntargets:\n  broken:\n    path: " + target + "\n" + mcpDocsConfig)
	claudeJSON := filepath.Join(sb.Home, ".claude.json")

	r := sb.RunCLI("sync", "--all", "-g")
	r.AssertFailure(t)
	r.AssertOutputContains(t, "MCP settings were not applied because resource sync failed")
	if sb.FileExists(claudeJSON) {
		t.Fatal("MCP applied after resource sync failure")
	}

	r = sb.RunCLI("sync", "--all", "--json", "-g")
	r.AssertFailure(t)
	var output map[string]json.RawMessage
	if err := json.Unmarshal([]byte(r.Stdout), &output); err != nil {
		t.Fatalf("stdout is not JSON: %v\n%s", err, r.Stdout)
	}
	if _, ok := output["mcp"]; ok {
		t.Fatalf("JSON reports an MCP plan that was never applied: %s", r.Stdout)
	}
}
