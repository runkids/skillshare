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

func TestPluginCLIDiscoveryAndSelection(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	sb.WriteConfig("targets: {}\nplugins:\n  packages:\n    demo:\n      bindings:\n        codex:\n          id: demo@team\n")
	root := filepath.Join(sb.Home, "fixture")
	if err := os.MkdirAll(filepath.Join(root, ".codex-plugin"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".codex-plugin/plugin.json"), []byte(`{"name":"demo","version":"1.0.0"}`), 0644); err != nil {
		t.Fatal(err)
	}
	r := sb.RunCLI("plugin", "discover", root, "--json", "-g")
	r.AssertSuccess(t)
	var d struct {
		Candidates []struct {
			Name string `json:"name"`
		} `json:"candidates"`
	}
	if err := json.Unmarshal([]byte(r.Stdout), &d); err != nil {
		t.Fatal(err)
	}
	if len(d.Candidates) != 1 || d.Candidates[0].Name != "demo" {
		t.Fatal("source discovery failed")
	}
	sb.RunCLI("plugin", "disable", "demo", "--target", "codex", "--json", "-g").AssertSuccess(t)
	config := filepath.Join(sb.Home, ".config/skillshare/config.yaml")
	data, err := os.ReadFile(config)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "sync: false") {
		t.Fatalf("selection not saved: %s", data)
	}
	sb.RunCLI("plugin", "enable", "demo", "--target", "codex", "--dry-run", "--json", "-g").AssertSuccess(t)
	after, _ := os.ReadFile(config)
	if string(after) != string(data) {
		t.Fatal("preview changed configuration")
	}
	if _, err := os.Stat(filepath.Join(sb.Home, ".codex/config.toml")); !os.IsNotExist(err) {
		t.Fatal("selection created native config")
	}
	sb.RunCLI("sync", "plugins", "--help").AssertSuccess(t)
}
