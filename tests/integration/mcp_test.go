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

func TestMCPInlineLifecycle(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	sb.WriteConfig("targets: {}\n")
	sb.RunCLI("mcp", "add", "docs", "--url", "https://example.com/mcp", "--target", "claude", "-g").AssertSuccess(t)
	target := filepath.Join(sb.Home, ".claude.json")
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Fatal("add without --sync must only save the source")
	}
	sb.RunCLI("sync", "mcp", "--dry-run", "-g").AssertSuccess(t)
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Fatal("preview must not create native configuration")
	}
	sb.RunCLI("sync", "mcp", "-g").AssertSuccess(t)
	before := sb.ReadFile(target)
	if !strings.Contains(before, "https://example.com/mcp") {
		t.Fatal("server not written to Claude configuration")
	}
	sb.RunCLI("sync", "mcp", "-g").AssertSuccess(t)
	if got := sb.ReadFile(target); got != before {
		t.Fatal("repeated sync changed the target")
	}
	sb.RunCLI("mcp", "remove", "docs", "-g").AssertSuccess(t)
	sb.RunCLI("sync", "mcp", "-g").AssertSuccess(t)
	if strings.Contains(sb.ReadFile(target), "https://example.com/mcp") {
		t.Fatal("removed server remained in target")
	}
}

func TestMCPAdditionalClients(t *testing.T) {
	for _, target := range []string{"opencode", "grok"} {
		t.Run(target, func(t *testing.T) {
			sb := testutil.NewSandbox(t)
			defer sb.Cleanup()
			sb.WriteConfig("targets: {}\n")
			sb.RunCLI("mcp", "add", "docs", "--url", "https://example.com/mcp", "--target", target, "-g").AssertSuccess(t)
			sb.RunCLI("sync", "mcp", "--dry-run", "-g").AssertSuccess(t)
			sb.RunCLI("sync", "mcp", "-g").AssertSuccess(t)
			sb.RunCLI("mcp", "import", "--from", target, "--dry-run", "-g").AssertSuccess(t)
			sb.RunCLI("mcp", "remove", "docs", "-g").AssertSuccess(t)
			sb.RunCLI("sync", "mcp", "-g").AssertSuccess(t)
		})
	}
}

func TestMCPSyncAllIncludesMCP(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	sb.WriteConfig("targets: {}\nmcp:\n  targets: [claude]\n  servers:\n    docs:\n      url: https://example.com/mcp\n")
	r := sb.RunCLI("sync", "--all", "--dry-run", "--json", "-g")
	r.AssertSuccess(t)
	var output struct {
		MCP struct {
			Plan struct {
				Changes []any `json:"changes"`
			} `json:"plan"`
		} `json:"mcp"`
	}
	if err := json.Unmarshal([]byte(r.Stdout), &output); err != nil {
		t.Fatal(err)
	}
	if len(output.MCP.Plan.Changes) != 1 {
		t.Fatal("all-resources preview omitted MCP")
	}
	if _, err := os.Stat(filepath.Join(sb.Home, ".claude.json")); !os.IsNotExist(err) {
		t.Fatal("dry-run wrote native config")
	}
	sb.RunCLI("sync", "--all", "--json", "-g").AssertSuccess(t)
	if _, err := os.Stat(filepath.Join(sb.Home, ".claude.json")); err != nil {
		t.Fatal(err)
	}
}

func TestMCPMissingExternalSourceDoesNotPrune(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	sb.WriteConfig("targets: {}\nmcp:\n  targets: [claude]\n  servers:\n    docs:\n      url: https://example.com/mcp\n")
	sb.RunCLI("sync", "mcp", "-g").AssertSuccess(t)
	target := filepath.Join(sb.Home, ".claude.json")
	before := sb.ReadFile(target)
	sb.WriteConfig("targets: {}\nsources:\n  mcp: ./missing.yaml\nmcp:\n  targets: [claude]\n")
	sb.RunCLI("sync", "mcp", "-g").AssertFailure(t)
	if sb.ReadFile(target) != before {
		t.Fatal("missing external source must never prune native entries")
	}
}

func TestMCPImportIntoDifferentClient(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	sb.WriteConfig("targets: {}\n")
	path := filepath.Join(sb.Home, ".claude.json")
	before := `{"mcpServers":{"docs":{"url":"https://example.com/mcp","type":"http"}}}`
	if err := os.WriteFile(path, []byte(before), 0600); err != nil {
		t.Fatal(err)
	}
	sb.RunCLI("mcp", "import", "docs", "--from", "claude", "--target", "codex", "--sync", "-g").AssertSuccess(t)
	if sb.ReadFile(path) != before {
		t.Fatal("import changed an unselected client")
	}
	if _, err := os.Stat(filepath.Join(sb.Home, ".codex", "config.toml")); err != nil {
		t.Fatal(err)
	}
}

func TestMCPImportDoesNotRewriteSourceClientWithoutReplace(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	sb.WriteConfig("targets: {}\n")
	path := filepath.Join(sb.Home, ".claude.json")
	before := `{"mcpServers":{"github":{"type":"stdio","command":"github-mcp","env":{"GITHUB_TOKEN":"literal-token"}}}}`
	if err := os.WriteFile(path, []byte(before), 0600); err != nil {
		t.Fatal(err)
	}
	sb.RunCLI("mcp", "import", "github", "--from", "claude", "--target", "claude", "--sync", "-g").AssertFailure(t)
	if sb.ReadFile(path) != before {
		t.Fatal("import rewrote the working client entry without --replace")
	}
}
