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

func TestMCPNonInteractiveManagement(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	sb.WriteConfig("targets: {}\nmcp:\n  targets: [claude]\n  servers:\n    docs:\n      url: https://example.com/mcp\n      headers:\n        X-Team: engineering\n")
	for _, args := range [][]string{{"mcp", "--no-tui", "-g"}, {"mcp", "list", "--no-tui", "-g"}, {"mcp", "--json", "-g"}} {
		sb.RunCLI(args...).AssertSuccess(t)
	}
	sb.RunCLI("mcp", "edit", "docs", "--url", "https://updated.example/mcp", "--no-tui", "-g").AssertSuccess(t)
	data := sb.ReadFile(sb.ConfigPath)
	if !strings.Contains(data, "updated.example") || !strings.Contains(data, "X-Team: engineering") {
		t.Fatal("edit lost existing fields")
	}
	if _, err := os.Stat(filepath.Join(sb.Home, ".claude.json")); !os.IsNotExist(err) {
		t.Fatal("edit synchronized without --sync")
	}
	sb.RunCLI("mcp", "edit", "docs", "--url", "https://preview.example/mcp", "--dry-run", "--json", "-g").AssertSuccess(t)
	if sb.ReadFile(sb.ConfigPath) != data {
		t.Fatal("dry-run wrote source")
	}
	sb.RunCLI("mcp", "edit", "docs", "--no-tui", "-g", "--", "echo", "ready").AssertSuccess(t)
	data = sb.ReadFile(sb.ConfigPath)
	if strings.Contains(data, "url:") || strings.Contains(data, "headers:") || !strings.Contains(data, "command: echo") {
		t.Fatal("transport change retained incompatible fields")
	}
	for _, sub := range []string{"add", "edit", "remove", "restore", "import"} {
		r := sb.RunCLI("mcp", sub, "--no-tui", "-g")
		r.AssertFailure(t)
		if strings.Contains(r.Stderr, "unknown MCP argument") {
			t.Fatalf("--no-tui rejected: %s", r.Stderr)
		}
	}
	if sb.ReadFile(sb.ConfigPath) != data {
		t.Fatal("missing arguments changed source")
	}
}

func TestMCPImportJSONDoesNotSave(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	sb.WriteConfig("targets: {}\n")
	input := filepath.Join(sb.Home, "native.json")
	if err := os.WriteFile(input, []byte(`{"mcpServers":{"a":{"command":"echo"},"b":{"command":"echo"}}}`), 0600); err != nil {
		t.Fatal(err)
	}
	r := sb.RunCLI("mcp", "import", "--file", input, "--json", "--no-tui", "-g")
	r.AssertSuccess(t)
	var items []map[string]any
	if err := json.Unmarshal([]byte(r.Stdout), &items); err != nil || len(items) != 2 {
		t.Fatalf("invalid candidates: %s", r.Stdout)
	}
	if sb.ReadFile(sb.ConfigPath) != "targets: {}\n" {
		t.Fatal("listing candidates wrote source")
	}
}
