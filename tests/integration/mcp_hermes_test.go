//go:build !online

package integration

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"skillshare/internal/testutil"
)

func TestMCPHermesOAuthImportAndSync(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	sb.WriteConfig("targets: {}\n")
	profile := filepath.Join(sb.Home, "hermes-profile")
	if err := os.MkdirAll(profile, 0755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(profile, "config.yaml")
	before := "# personal settings\nmodel: example\nmcp_servers:\n  linear:\n    url: https://mcp.linear.app/mcp\n    auth: oauth\n  personal:\n    command: mine\n"
	if err := os.WriteFile(path, []byte(before), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(profile, "mcp-tokens"), 0755); err != nil {
		t.Fatal(err)
	}
	privateFiles := map[string]string{
		filepath.Join(profile, ".env"):                      "API_KEY=hermes-only-test-value\n",
		filepath.Join(profile, "mcp-tokens", "linear.json"): `{"access_token":"hermes-only-test-token"}`,
	}
	for file, content := range privateFiles {
		if err := os.WriteFile(file, []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
	}
	env := map[string]string{"HERMES_HOME": profile}
	sb.RunCLIEnv(env, "mcp", "import", "--from", "hermes", "linear", "--target", "claude", "--target", "codex", "--target", "hermes", "-g").AssertSuccess(t)
	sb.RunCLIEnv(env, "sync", "mcp", "--dry-run", "-g").AssertSuccess(t)
	if sb.ReadFile(path) != before {
		t.Fatal("preview rewrote native config")
	}
	sb.RunCLIEnv(env, "sync", "mcp", "-g").AssertSuccess(t)
	for _, file := range []string{filepath.Join(sb.Home, ".claude.json"), filepath.Join(sb.Home, ".codex", "config.toml"), path} {
		if !strings.Contains(sb.ReadFile(file), "https://mcp.linear.app/mcp") {
			t.Fatalf("missing server in %s", file)
		}
	}
	if !strings.Contains(sb.ReadFile(path), "auth: oauth") {
		t.Fatal("Hermes OAuth setting lost")
	}
	sb.RunCLIEnv(env, "mcp", "remove", "linear", "--sync", "-g").AssertSuccess(t)
	after := sb.ReadFile(path)
	if strings.Contains(after, "mcp.linear.app") || !strings.Contains(after, "command: mine") || !strings.Contains(after, "# personal settings") {
		t.Fatalf("removal did not preserve unmanaged config: %s", after)
	}
	if _, err := os.Stat(filepath.Join(sb.Home, ".hermes", "config.yaml")); !os.IsNotExist(err) {
		t.Fatal("HERMES_HOME wrote the default profile")
	}
	for file, content := range privateFiles {
		if sb.ReadFile(file) != content {
			t.Fatalf("Hermes credentials changed: %s", file)
		}
	}
	if strings.Contains(sb.ReadFile(sb.ConfigPath), "hermes-only-test") {
		t.Fatal("Hermes credentials copied into source")
	}
}
