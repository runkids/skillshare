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
