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
	legacy := sb.ReadFile(sb.ConfigPath)
	dry := sb.RunCLI("sync", "mcp", "-g", "--dry-run")
	dry.AssertSuccess(t)
	dry.AssertAnyOutputContains(t, "Pi now uses its built-in MCP; the next sync updates the config: docs")
	dry.AssertAnyOutputContains(t, "Pi's built-in MCP needs Pi 0.99.0 or later")
	sb.RunCLI("sync", "mcp", "-g", "--dry-run", "--json").AssertAnyOutputContains(t, "is still installed in Pi, remove it")
	if sb.ReadFile(sb.ConfigPath) != legacy {
		t.Fatal("dry run changed the config")
	}
	r := sb.RunCLI("sync", "mcp", "-g")
	r.AssertSuccess(t)
	r.AssertAnyOutputContains(t, "Pi's built-in MCP needs Pi 0.99.0 or later")
	assertConfigMigrated(t, sb, r, sb.ConfigPath, legacy)
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

// assertConfigMigrated checks a sync saved path without the settings 0.23.0 retired, named
// the backup it kept of legacy, and the next sync has nothing left to say about them.
func assertConfigMigrated(t *testing.T, sb *testutil.Sandbox, r *testutil.Result, path, legacy string) {
	t.Helper()
	output := r.Stdout + r.Stderr
	prefix := "Updated config.yaml for 0.23.0 (backup: "
	start := strings.Index(output, prefix)
	if start < 0 {
		t.Fatalf("no migration line:\n%s", output)
	}
	backup := output[start+len(prefix):]
	backup = backup[:strings.Index(backup, ")")]
	if sb.ReadFile(backup) != legacy {
		t.Fatalf("backup %s: %s", backup, sb.ReadFile(backup))
	}
	if saved := sb.ReadFile(path); strings.Contains(saved, "piExtension") || strings.Contains(saved, "piOptionsPrune") {
		t.Fatalf("saved config:\n%s", saved)
	}
	dir := filepath.Dir(filepath.Dir(path))
	args := []string{"sync", "mcp", "--dry-run", "-g"}
	if filepath.Base(filepath.Dir(path)) == ".skillshare" {
		args[3] = "-p"
	} else {
		dir = sb.Home
	}
	again := sb.RunCLIInDir(dir, args...)
	again.AssertSuccess(t)
	again.AssertOutputNotContains(t, "0.23.0")
}

// Nothing in an Agent file changes, but the sync still saves the config, from sync --all
// and in project mode too.
func TestSyncSavesTheConfigWithoutRetiredSettings(t *testing.T) {
	t.Run("sync", func(t *testing.T) {
		sb := testutil.NewSandbox(t)
		defer sb.Cleanup()
		sb.WriteConfig("source: " + sb.SourcePath + "\ntargets: {}\nmcp:\n  servers:\n    docs:\n      url: https://example.com/mcp\n      targets: []\n      piOptionsPrune: true\n")
		legacy := sb.ReadFile(sb.ConfigPath)
		sb.RunCLI("sync", "--all", "-g", "--dry-run").AssertSuccess(t)
		if sb.ReadFile(sb.ConfigPath) != legacy {
			t.Fatal("dry run changed the config")
		}
		r := sb.RunCLI("sync", "--all", "-g")
		r.AssertSuccess(t)
		assertConfigMigrated(t, sb, r, sb.ConfigPath, legacy)
	})
	t.Run("project mode", func(t *testing.T) {
		sb := testutil.NewSandbox(t)
		defer sb.Cleanup()
		sb.WriteConfig("targets: {}\n")
		root := sb.SetupProjectDir()
		sb.WriteProjectConfig(root, "targets: []\nmcp:\n  targets: [pi]\n  servers:\n    docs:\n      url: https://example.com/mcp\n      piExtension: pi-mcp-adapter\n")
		path := filepath.Join(root, ".skillshare", "config.yaml")
		legacy := sb.ReadFile(path)
		r := sb.RunCLIInDir(root, "sync", "mcp", "-p")
		r.AssertSuccess(t)
		assertConfigMigrated(t, sb, r, path, legacy)
		if !strings.Contains(sb.ReadFile(filepath.Join(root, ".pi", "mcp.json")), `"docs"`) {
			t.Fatal("project Pi file lacks docs")
		}
	})
}
