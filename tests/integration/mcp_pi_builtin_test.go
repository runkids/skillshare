//go:build !online

package integration

import (
	"path/filepath"
	"strings"
	"testing"

	"skillshare/internal/testutil"
)

func TestMCPPiBuiltinLifecycle(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	sb.WriteConfig("targets: {}\n")
	sb.RunCLI("mcp", "add", "docs", "--target", "pi", "--pi-options", `{"exposure":"deferred","custom":{"flag":true}}`, "--url", "https://example.com/mcp", "--no-tui", "-g").AssertSuccess(t)
	sb.RunCLI("sync", "mcp", "-g").AssertSuccess(t)
	path := filepath.Join(sb.Home, ".pi", "agent", "mcp.json")
	before := sb.ReadFile(path)
	if !strings.Contains(before, `"deferred"`) || strings.Contains(before, `"transport"`) {
		t.Fatal(before)
	}
	sb.RunCLI("sync", "mcp", "-g").AssertSuccess(t)
	if sb.ReadFile(path) != before {
		t.Fatal("re-sync changed native config")
	}
	sb.RunCLI("mcp", "import", "--from", "pi", "--dry-run", "--json", "-g").AssertSuccess(t)
	sb.RunCLI("mcp", "remove", "docs", "-g").AssertSuccess(t)
	sb.RunCLI("sync", "mcp", "-g").AssertSuccess(t)
	if strings.Contains(sb.ReadFile(path), `"docs"`) {
		t.Fatal("managed entry remained")
	}
}

// A server added for Pi is written to the built-in file of its scope.
func TestMCPPiBuiltinFollowsScope(t *testing.T) {
	t.Run("global", func(t *testing.T) {
		sb := testutil.NewSandbox(t)
		defer sb.Cleanup()
		sb.WriteConfig("targets: {}\n")
		sb.RunCLI("mcp", "add", "docs", "--target", "pi", "--url", "https://example.com/mcp", "--no-tui", "-g").AssertSuccess(t)
		if strings.Contains(sb.ReadFile(sb.ConfigPath), "piExtension") {
			t.Fatal(sb.ReadFile(sb.ConfigPath))
		}
		sb.RunCLI("sync", "mcp", "-g").AssertSuccess(t)
		if !strings.Contains(sb.ReadFile(filepath.Join(sb.Home, ".pi", "agent", "mcp.json")), `"docs"`) {
			t.Fatal("global Pi file lacks docs")
		}
	})
	t.Run("project mode", func(t *testing.T) {
		sb := testutil.NewSandbox(t)
		defer sb.Cleanup()
		sb.WriteConfig("targets: {}\n")
		root := sb.SetupProjectDir()
		sb.RunCLIInDir(root, "mcp", "add", "docs", "--target", "pi", "--url", "https://example.com/mcp", "--no-tui", "-p").AssertSuccess(t)
		if strings.Contains(sb.ReadFile(filepath.Join(root, ".skillshare", "config.yaml")), "piExtension") {
			t.Fatal(sb.ReadFile(filepath.Join(root, ".skillshare", "config.yaml")))
		}
		sb.RunCLIInDir(root, "sync", "mcp", "-p").AssertSuccess(t)
		if !strings.Contains(sb.ReadFile(filepath.Join(root, ".pi", "mcp.json")), `"docs"`) {
			t.Fatal("project Pi file lacks docs")
		}
		if sb.FileExists(filepath.Join(sb.Home, ".pi", "agent", "mcp.json")) {
			t.Fatal("project server reached the global Pi file")
		}
	})
	t.Run("mcp.projects", func(t *testing.T) {
		sb := testutil.NewSandbox(t)
		defer sb.Cleanup()
		root := filepath.Join(sb.Root, "work")
		sb.WriteConfig("targets: {}\nmcp:\n  projects:\n    " + root + ":\n      servers:\n        docs:\n          url: https://example.com/mcp\n          targets: [pi]\n")
		sb.RunCLI("sync", "mcp", "-g").AssertSuccess(t)
		if !strings.Contains(sb.ReadFile(filepath.Join(root, ".pi", "mcp.json")), `"docs"`) {
			t.Fatal("mcp.projects Pi file lacks docs")
		}
		if sb.FileExists(filepath.Join(sb.Home, ".pi", "agent", "mcp.json")) {
			t.Fatal("project server reached the global Pi file")
		}
	})
}

// A server set up for pi-mcp-adapter before 0.23.0 syncs to Pi's built-in mcp.json with a
// notice. An entry the user wrote in mcp-adapter.json is never touched.
func TestMCPPiAdapterServerSyncsToBuiltin(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	dir := filepath.Join(sb.Home, ".pi", "agent")
	builtin, adapter := filepath.Join(dir, "mcp.json"), filepath.Join(dir, "mcp-adapter.json")
	manual := `{"mcpServers":{"mine":{"command":"mine"}}}`
	sb.WriteFile(adapter, manual)
	sb.WriteConfig("targets: {}\nmcp:\n  targets: [pi]\n  servers:\n    docs:\n      url: https://example.com/mcp\n      piExtension: pi-mcp-adapter\n")
	r := sb.RunCLI("sync", "mcp", "-g")
	r.AssertSuccess(t)
	r.AssertAnyOutputContains(t, "piExtension is ignored since 0.23.0")
	if !strings.Contains(sb.ReadFile(builtin), `"docs"`) || sb.ReadFile(adapter) != manual {
		t.Fatalf("mcp.json: %s\nmcp-adapter.json: %s", sb.ReadFile(builtin), sb.ReadFile(adapter))
	}
}

func TestMCPRemovedPiFlagsExplainTheChange(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	sb.WriteConfig("targets: {}\n")
	for _, flag := range []string{"--pi-extension", "--pi-options-prune", "--direct-tools"} {
		r := sb.RunCLI("mcp", "add", "docs", "--target", "pi", flag, "builtin", "--url", "https://example.com/mcp", "--no-tui", "-g")
		r.AssertFailure(t)
		r.AssertAnyOutputContains(t, flag+" was removed in 0.23.0")
	}
}
