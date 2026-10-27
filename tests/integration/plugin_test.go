//go:build !online

package integration

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"skillshare/internal/plugin"
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

// Pi itself installs an npm package from pi.dev; the preview says it runs install scripts.
func TestPluginAddNpmPackageThroughPi(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	sb.WriteConfig("targets: {}\n")
	home := sb.Home
	bin := filepath.Join(home, "bin")
	agentDir := filepath.Join(home, ".pi/agent")
	fake := "#!/bin/sh\ncase \"$1\" in\n--version) echo 0.99.2 ;;\ninstall) [ \"$2\" = --help ] || printf '{\"packages\":[\"%s\"]}' \"$2\" > \"$PI_CODING_AGENT_DIR/settings.json\" ;;\nesac\n"
	if err := os.MkdirAll(bin, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(agentDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bin, "pi"), []byte(fake), 0755); err != nil {
		t.Fatal(err)
	}
	env := map[string]string{"PATH": bin + ":" + os.Getenv("PATH"), "PI_CODING_AGENT_DIR": agentDir}

	r := sb.RunCLIEnv(env, "plugin", "add", "npm:demo", "--target", "pi", "--no-tui", "-g")
	r.AssertSuccess(t)
	settings, _ := os.ReadFile(filepath.Join(agentDir, "settings.json"))
	if string(settings) != `{"packages":["npm:demo"]}` {
		t.Fatalf("Pi did not install the package: %s\n%s", settings, r.Stdout+r.Stderr)
	}
	config, _ := os.ReadFile(filepath.Join(sb.Home, ".config/skillshare/config.yaml"))
	if !strings.Contains(string(config), "source: npm:demo") || strings.Contains(string(config), "id:") || strings.Contains(string(config), "pending:") {
		t.Fatalf("shared intent lost source or exported native state: %s", config)
	}
	listed := sb.RunCLIEnv(env, "plugin", "list", "--json", "-g")
	listed.AssertSuccess(t)
	var inventory plugin.Inventory
	if err := json.Unmarshal([]byte(listed.Stdout), &inventory); err != nil {
		t.Fatal(err)
	}
	if binding := inventory.Packages["demo"].Bindings["pi"]; binding.ID != "npm:demo" || binding.Pending != "" {
		t.Fatalf("local binding not recorded: %+v", binding)
	}
}

func TestPluginDiscoverShowsSeparateOMPBuild(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	sb.WriteConfig("targets: {}\n")
	root := filepath.Join(sb.Home, "fixture")
	for path, data := range map[string]string{
		".claude-plugin/marketplace.json":                `{"name":"claude-market","plugins":[{"name":"kata","source":"./plugins/claude-code"}]}`,
		".omp-plugin/marketplace.json":                   `{"name":"omp-market","plugins":[{"name":"kata","source":"./plugins/oh-my-pi"}]}`,
		"plugins/claude-code/.claude-plugin/plugin.json": `{"name":"kata","version":"0.1.0","hooks":"./hooks/hooks.json"}`,
		"plugins/oh-my-pi/package.json":                  `{"name":"kata","version":"2.0.0","omp":{"extensions":["./kata.ts"]}}`,
		"plugins/oh-my-pi/kata.ts":                       "export default function () {}",
	} {
		file := filepath.Join(root, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(file), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(file, []byte(data), 0644); err != nil {
			t.Fatal(err)
		}
	}
	r := sb.RunCLI("plugin", "discover", root, "-g")
	r.AssertSuccess(t)
	var omp, claude bool
	for _, line := range strings.Split(r.Stdout, "\n") {
		if strings.Contains(line, "omp") && strings.Contains(line, "extensions") && strings.Contains(line, "plugins/oh-my-pi") && !strings.Contains(line, "hooks") && !strings.Contains(line, "claude") {
			omp = true
		}
		if strings.Contains(line, "claude") && strings.Contains(line, "hooks") && strings.Contains(line, "plugins/claude-code") && !strings.Contains(line, "omp") && !strings.Contains(line, "extensions") {
			claude = true
		}
	}
	if !omp || !claude {
		t.Fatalf("native builds were not shown separately:\n%s", r.Stdout)
	}
}

func TestPluginDiscoverNpmPointsToAdd(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	sb.WriteConfig("targets: {}\n")
	r := sb.RunCLI("plugin", "discover", "npm:demo", "--json", "-g")
	r.AssertFailure(t)
	r.AssertAnyOutputContains(t, "skillshare plugin add npm:demo --target pi")
}
