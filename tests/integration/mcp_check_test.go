//go:build !online

package integration

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"skillshare/internal/testutil"
)

func TestMCPCheckGlobal(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	sb.WriteConfig("targets: {}\nmcp:\n  targets: [claude]\n  servers:\n    local:\n      command: sh\n      env:\n        API_KEY: {fromEnv: SKILLSHARE_CHECK_TOKEN}\n    parked:\n      url: https://example.com/mcp\n      targets: []\n")

	r := sb.RunCLI("mcp", "check", "--json", "--no-dns", "-g")
	r.AssertExitCode(t, 1)
	var report struct {
		Servers []struct {
			Name     string `json:"name"`
			OK       bool   `json:"ok"`
			Findings []struct {
				Level   string `json:"level"`
				Check   string `json:"check"`
				Target  string `json:"target"`
				Message string `json:"message"`
			} `json:"findings"`
		} `json:"servers"`
		Summary struct {
			Errors   int `json:"errors"`
			Warnings int `json:"warnings"`
		} `json:"summary"`
	}
	if err := json.Unmarshal([]byte(r.Stdout), &report); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, r.Stdout)
	}
	if len(report.Servers) != 2 || report.Servers[0].Name != "local" || report.Servers[0].OK || !report.Servers[1].OK {
		t.Fatalf("unexpected servers: %+v", report.Servers)
	}
	if report.Summary.Errors != 1 || report.Summary.Warnings != 1 {
		t.Fatalf("want the missing variable as the only error and the unsynced entry as the only warning: %+v", report)
	}

	sb.RunCLI("sync", "mcp", "-g").AssertSuccess(t)
	r = sb.RunCLIEnv(map[string]string{"SKILLSHARE_CHECK_TOKEN": "s3cr3t-value"}, "mcp", "check", "--no-dns", "-g")
	r.AssertSuccess(t)
	r.AssertOutputContains(t, "claude: in sync")
	r.AssertOutputContains(t, "kept in Skillshare only")
	if strings.Contains(r.Stdout, "s3cr3t-value") {
		t.Fatal("check printed a variable's value")
	}

	r = sb.RunCLI("mcp", "check", "nope", "-g")
	r.AssertExitCode(t, 1)
	r.AssertOutputContains(t, "known servers: local, parked")
}

func TestMCPCheckProjects(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	root := filepath.Join(sb.Home, "work", "app")
	if err := os.MkdirAll(root, 0755); err != nil {
		t.Fatal(err)
	}
	sb.WriteConfig("targets: {}\nmcp:\n  targets: [claude]\n  servers:\n    docs:\n      url: https://example.com/mcp\n  projects:\n    ~/work/app:\n      servers:\n        docs:\n          command: no-such-mcp-binary\n")

	r := sb.RunCLI("mcp", "check", "docs", "--json", "--no-dns", "-g")
	r.AssertExitCode(t, 1)
	var report struct {
		Servers []struct {
			Name    string `json:"name"`
			Project string `json:"project"`
			OK      bool   `json:"ok"`
		} `json:"servers"`
		Summary struct {
			Errors int `json:"errors"`
		} `json:"summary"`
	}
	if err := json.Unmarshal([]byte(r.Stdout), &report); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, r.Stdout)
	}
	if len(report.Servers) != 2 || report.Servers[0].Project != "" || !report.Servers[0].OK || report.Servers[1].Project != root || report.Servers[1].OK || report.Summary.Errors != 1 {
		t.Fatalf("want the global docs and the project's docs with its missing command: %+v", report)
	}
	if strings.Count(r.Stdout, `"project"`) != 1 {
		t.Fatalf("a global server must omit project:\n%s", r.Stdout)
	}

	r = sb.RunCLI("mcp", "check", "--no-dns", "-g")
	r.AssertExitCode(t, 1)
	r.AssertOutputContains(t, "docs  (project ~/work/app)")
	r.AssertOutputContains(t, "Checked 2 servers: 1 error")
}

func TestMCPCheckProjectModeReportsSyncState(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	sb.WriteConfig("targets: {}\n")
	root := sb.SetupProjectDir()
	sb.RunCLIInDir(root, "mcp", "add", "docs", "--target", "claude", "--url", "https://example.com/mcp", "--no-tui", "-p").AssertSuccess(t)

	r := sb.RunCLIInDir(root, "mcp", "check", "docs", "--no-dns", "-p")
	r.AssertSuccess(t)
	r.AssertOutputContains(t, "not synced yet")
}

// A project's Claude switch lives in ~/.claude.json, outside the project, but is still this scope.
func TestMCPProjectClaudeSwitchStaysInScope(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	sb.WriteConfig("targets: {}\n")
	root := sb.SetupProjectDir()
	sb.RunCLIInDir(root, "mcp", "add", "docs", "--target", "claude", "--disabled", "--no-tui", "-p").AssertSuccess(t)

	r := sb.RunCLIInDir(root, "mcp", "check", "docs", "--no-dns", "-p")
	r.AssertSuccess(t)
	r.AssertOutputContains(t, "not synced yet")

	sb.RunCLIInDir(root, "sync", "mcp", "-p").AssertSuccess(t)
	sb.RunCLIInDir(root, "mcp", "remove", "docs", "--keep-files", "--no-tui", "-p").AssertSuccess(t)
	r = sb.RunCLIInDir(root, "sync", "mcp", "-p", "--dry-run", "--json")
	r.AssertSuccess(t)
	r.AssertOutputNotContains(t, `"remove"`)
}

// tinyMCPServer answers server/discover and tools/list, one JSON-RPC message per line.
const tinyMCPServer = `#!/bin/sh
while IFS= read -r line; do
  id=$(printf '%s\n' "$line" | sed -n 's/.*"id":\([0-9][0-9]*\).*/\1/p')
  case "$line" in
    *'"server/discover"'*) printf '{"jsonrpc":"2.0","id":%s,"result":{"resultType":"complete","supportedVersions":["2026-07-28"],"capabilities":{"tools":{}},"_meta":{"io.modelcontextprotocol/serverInfo":{"name":"tiny","version":"0.1.0"}}}}\n' "$id" ;;
    *'"tools/list"'*) printf '{"jsonrpc":"2.0","id":%s,"result":{"resultType":"complete","tools":[{"name":"echo","inputSchema":{"type":"object"}}]}}\n' "$id" ;;
  esac
done
`

func TestMCPCheckLive(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	script := filepath.Join(sb.Home, "tiny-mcp.sh")
	if err := os.WriteFile(script, []byte(tinyMCPServer), 0755); err != nil {
		t.Fatal(err)
	}
	sb.WriteConfig("targets: {}\nmcp:\n  targets: [claude]\n  servers:\n    tiny:\n      command: sh\n      args: [" + script + "]\n    down:\n      url: http://127.0.0.1:1/mcp\n")

	r := sb.RunCLI("mcp", "check", "tiny", "--json", "-g")
	r.AssertSuccess(t)
	if strings.Contains(r.Stdout, `"live"`) {
		t.Fatalf("a check without --live probed the server:\n%s", r.Stdout)
	}

	r = sb.RunCLI("mcp", "check", "--live", "--timeout", "5s", "--json", "--no-dns", "-g")
	r.AssertExitCode(t, 1)
	var report struct {
		Servers []struct {
			Name string `json:"name"`
			OK   bool   `json:"ok"`
			Live *struct {
				ProtocolVersion string `json:"protocolVersion"`
				ServerInfo      struct {
					Name string `json:"name"`
				} `json:"serverInfo"`
				Tools int `json:"tools"`
			} `json:"live"`
		} `json:"servers"`
	}
	if err := json.Unmarshal([]byte(r.Stdout), &report); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, r.Stdout)
	}
	down, tiny := report.Servers[0], report.Servers[1]
	if tiny.Live == nil || tiny.Live.ServerInfo.Name != "tiny" || tiny.Live.ProtocolVersion != "2026-07-28" || tiny.Live.Tools != 1 || !tiny.OK {
		t.Fatalf("tiny: %+v", tiny)
	}
	if down.Live != nil || down.OK {
		t.Fatalf("an unreachable server must fail: %+v", down)
	}

	r = sb.RunCLI("mcp", "check", "tiny", "--live", "-g")
	r.AssertSuccess(t)
	r.AssertOutputContains(t, "responds: tiny 0.1.0, protocol 2026-07-28, 1 tool(s)")

	r = sb.RunCLI("mcp", "check", "--timeout", "5s", "-g")
	r.AssertFailure(t)
	r.AssertAnyOutputContains(t, "--timeout only applies with --live")
}

func TestMCPCheckLiveToolPins(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	script := filepath.Join(sb.Home, "tiny-mcp.sh")
	writeTools := func(tool string) {
		t.Helper()
		content := strings.Replace(tinyMCPServer, `{"name":"echo","inputSchema":{"type":"object"}}`, tool, 1)
		if err := os.WriteFile(script, []byte(content), 0755); err != nil {
			t.Fatal(err)
		}
	}
	writeTools(`{"name":"echo","inputSchema":{"type":"object"}}`)
	sb.WriteConfig("targets: {}\nmcp:\n  targets: [claude]\n  servers:\n    tiny:\n      command: sh\n      args: [" + script + "]\n")

	// pinLevels runs a live check and returns the level of each tool-change finding.
	pinLevels := func(extra ...string) []string {
		t.Helper()
		r := sb.RunCLI(append([]string{"mcp", "check", "tiny", "--live", "--no-dns", "--json", "-g"}, extra...)...)
		r.AssertSuccess(t)
		var report struct {
			Servers []struct {
				Findings []struct{ Level, Check string } `json:"findings"`
			} `json:"servers"`
		}
		if err := json.Unmarshal([]byte(r.Stdout), &report); err != nil {
			t.Fatalf("invalid JSON: %v\n%s", err, r.Stdout)
		}
		var levels []string
		for _, f := range report.Servers[0].Findings {
			if f.Check == "tool-change" {
				levels = append(levels, f.Level)
			}
		}
		return levels
	}
	expect := func(got []string, want ...string) {
		t.Helper()
		if !slices.Equal(got, want) {
			t.Fatalf("tool-change levels = %v, want %v", got, want)
		}
	}

	expect(pinLevels(), "info")
	writeTools(`{"name":"echo","description":"now reads ~/.ssh","inputSchema":{"type":"object"}}`)
	expect(pinLevels(), "warning")
	expect(pinLevels("--accept-tools"), "info")
	expect(pinLevels())

	// Accepting changed definitions is the one part of a check that is logged.
	logged := sb.RunCLI("log", "--json", "--tail", "1", "-g")
	logged.AssertSuccess(t)
	if out := logged.Stdout; !strings.Contains(out, `"cmd":"mcp check --accept-tools"`) || !strings.Contains(out, `"accepted":["tiny"]`) {
		t.Fatalf("operation log = %s", out)
	}

	r := sb.RunCLI("mcp", "check", "--accept-tools", "-g")
	r.AssertFailure(t)
	r.AssertAnyOutputContains(t, "--accept-tools only applies with --live")
}

func TestMCPToolCostsReachAnalyzeAndSync(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	script := filepath.Join(sb.Home, "tiny-mcp.sh")
	tool := `{"name":"echo","description":"Echo the text back to the caller, unchanged, so a client can test the connection.","inputSchema":{"type":"object"}}`
	if err := os.WriteFile(script, []byte(strings.Replace(tinyMCPServer, `{"name":"echo","inputSchema":{"type":"object"}}`, tool, 1)), 0755); err != nil {
		t.Fatal(err)
	}
	sb.CreateSkill("demo", map[string]string{"SKILL.md": "---\nname: demo\ndescription: Use when testing\n---\n# Demo\n"})
	sb.WriteConfig("targets: {}\ncontext_budget:\n  warn_mcp_tool_tokens: 10\nmcp:\n  targets: [claude]\n  servers:\n    tiny:\n      command: sh\n      args: [" + script + "]\n")

	r := sb.RunCLI("analyze", "--json", "-g")
	r.AssertSuccess(t)
	var before struct {
		MCP struct {
			Groups []struct {
				Measured int `json:"measured"`
			} `json:"groups"`
		} `json:"mcp"`
	}
	if err := json.Unmarshal([]byte(r.Stdout), &before); err != nil || len(before.MCP.Groups) != 1 || before.MCP.Groups[0].Measured != 0 {
		t.Fatalf("before a live check, tiny must be listed uncounted: %v\n%s", err, r.Stdout)
	}

	sb.RunCLI("mcp", "check", "--live", "--timeout", "5s", "-g").AssertSuccess(t)

	r = sb.RunCLI("analyze", "--json", "-g")
	r.AssertSuccess(t)
	var after struct {
		MCP struct {
			Budget int `json:"budget"`
			Groups []struct {
				Target string `json:"target"`
				Tokens int    `json:"tokens"`
			} `json:"groups"`
		} `json:"mcp"`
	}
	if err := json.Unmarshal([]byte(r.Stdout), &after); err != nil || after.MCP.Budget != 10 || len(after.MCP.Groups) != 1 || after.MCP.Groups[0].Target != "claude" || after.MCP.Groups[0].Tokens <= 10 {
		t.Fatalf("analyze --json mcp: %v\n%s", err, r.Stdout)
	}

	r = sb.RunCLI("sync", "mcp", "-g")
	r.AssertSuccess(t)
	r.AssertAnyOutputContains(t, "claude: MCP tool definitions are ~")
	r.AssertAnyOutputContains(t, "(budget: 10)")
}

// MCP servers load whether or not any skill exists, so analyze reports them without one.
func TestAnalyzeReportsMCPWithoutSkills(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	sb.WriteConfig("targets: {}\nmcp:\n  targets: [claude]\n  servers:\n    tiny:\n      command: tiny-server\n")

	r := sb.RunCLI("analyze", "--json", "-g")
	r.AssertSuccess(t)
	var out struct {
		MCP *struct {
			Groups []struct {
				Target string `json:"target"`
			} `json:"groups"`
		} `json:"mcp"`
	}
	if err := json.Unmarshal([]byte(r.Stdout), &out); err != nil || out.MCP == nil || len(out.MCP.Groups) != 1 || out.MCP.Groups[0].Target != "claude" {
		t.Fatalf("analyze --json with no skills must still list the MCP target: %v\n%s", err, r.Stdout)
	}
}

// A project counts its own servers under its own budget, apart from a global server of the same name.
func TestMCPToolCostsInProjectMode(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	script := filepath.Join(sb.Home, "tiny-mcp.sh")
	if err := os.WriteFile(script, []byte(tinyMCPServer), 0755); err != nil {
		t.Fatal(err)
	}
	sb.WriteConfig("targets: {}\nmcp:\n  targets: [claude]\n  servers:\n    tiny:\n      command: tiny-global\n")
	root := sb.SetupProjectDir("claude")
	sb.WriteProjectConfig(root, "targets:\n  - claude\ncontext_budget:\n  warn_mcp_tool_tokens: 1\nmcp:\n  targets: [claude]\n  servers:\n    tiny:\n      command: sh\n      args: ["+script+"]\n")

	sb.RunCLIInDir(root, "mcp", "check", "--live", "--timeout", "5s", "-p").AssertSuccess(t)

	type costs struct {
		MCP struct {
			Budget int `json:"budget"`
			Groups []struct {
				Measured int `json:"measured"`
				Tokens   int `json:"tokens"`
			} `json:"groups"`
		} `json:"mcp"`
	}
	r := sb.RunCLIInDir(root, "analyze", "--json", "-p")
	r.AssertSuccess(t)
	var project costs
	if err := json.Unmarshal([]byte(r.Stdout), &project); err != nil || project.MCP.Budget != 1 || len(project.MCP.Groups) != 1 || project.MCP.Groups[0].Measured != 1 || project.MCP.Groups[0].Tokens < 1 {
		t.Fatalf("analyze -p --json mcp: %v\n%s", err, r.Stdout)
	}
	r = sb.RunCLI("analyze", "--json", "-g")
	r.AssertSuccess(t)
	var global costs
	if err := json.Unmarshal([]byte(r.Stdout), &global); err != nil || len(global.MCP.Groups) != 1 || global.MCP.Groups[0].Measured != 0 {
		t.Fatalf("the project's count must not reach the global tiny: %v\n%s", err, r.Stdout)
	}
}
