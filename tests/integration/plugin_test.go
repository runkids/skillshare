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

func TestPluginTernLifecycle(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	sb.WriteConfig("targets: {}\n")
	bin := filepath.Join(sb.Home, "bin")
	ternHome := filepath.Join(sb.Home, "tern")
	source := filepath.Join(sb.Home, "source")
	for _, dir := range []string{bin, ternHome, source} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
	}
	manifest := filepath.Join(source, "plugin.toml")
	if err := os.WriteFile(manifest, []byte("schema = 1\nid = 'demo'\nname = 'Demo'\nwindow = 'main.luau'\nversion = '1.0.0'\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "main.luau"), []byte("return {}"), 0644); err != nil {
		t.Fatal(err)
	}
	// Real Tern 0.9.18 list --json printed (with its installed plugin values):
	// {"plugins":[{"id":"tern-kata","name":"Kata","version":"0.17.0","description":"Example","icon":"puzzle","css":"","blocks":[],"lenses":[],"host":true,"window":false,"status":"ready"}],"problems":[],"jit":false,"canvas":true}
	fake := `#!/bin/sh
case "$*" in
  --version) echo 0.9.18 ;;
  'plugin --help') echo 'plugin install remove list' ;;
  'plugin list --json') if [ -f "$TERN_CONFIG_DIR/plugins/demo/main.luau" ]; then echo '{"plugins":[{"id":"demo","name":"Demo","version":"1.0.0","description":"","icon":"puzzle","css":"","blocks":[],"lenses":[],"host":false,"window":true,"status":"ready"}],"problems":[],"jit":false,"canvas":true}'; else echo '{"plugins":[],"problems":[],"jit":false,"canvas":true}'; fi ;;
  'plugin install '*) [ -f "$3/plugin.toml" ] && [ -f "$3/main.luau" ] || exit 1; printf '%s\n' "$*" >> "$TERN_CONFIG_DIR/commands"; if [ -d "$TERN_CONFIG_DIR/plugins/demo" ] && [ "$4" != '--force' ]; then exit 1; fi; mkdir -p "$TERN_CONFIG_DIR/plugins/demo"; cp -R "$3/." "$TERN_CONFIG_DIR/plugins/demo/" ;;
  'plugin remove demo') echo "$*" >> "$TERN_CONFIG_DIR/commands"; [ -d "$TERN_CONFIG_DIR/plugins/demo" ] || exit 1; rm -r "$TERN_CONFIG_DIR/plugins/demo" ;;
  *) echo "unexpected: $*" >&2; exit 1 ;;
esac
`
	if err := os.WriteFile(filepath.Join(bin, "tern"), []byte(fake), 0755); err != nil {
		t.Fatal(err)
	}
	env := map[string]string{"PATH": bin + ":" + os.Getenv("PATH"), "TERN_CONFIG_DIR": ternHome}
	sb.RunCLIEnv(env, "plugin", "add", source, "--target", "tern", "--no-tui", "-g").AssertSuccess(t)
	if err := os.WriteFile(filepath.Join(source, "extra.txt"), []byte("changed"), 0644); err != nil {
		t.Fatal(err)
	}
	sb.RunCLIEnv(env, "plugin", "check", "demo", "--json", "-g").AssertOutputContains(t, "update-available")
	sb.RunCLIEnv(env, "plugin", "update", "demo", "--no-tui", "-g").AssertSuccess(t)
	sb.RunCLIEnv(env, "plugin", "remove", "demo", "--no-tui", "-g").AssertSuccess(t)
	data, err := os.ReadFile(filepath.Join(ternHome, "commands"))
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 3 || !strings.HasPrefix(lines[0], "plugin install ") || !strings.HasSuffix(lines[1], " --force") || lines[2] != "plugin remove demo" || strings.Contains(lines[0], source) {
		t.Fatalf("unexpected native commands: %q", data)
	}
}

func TestPluginTernHelp(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	sb.RunCLI("plugin", "--help").AssertOutputContains(t, "tern")
}
