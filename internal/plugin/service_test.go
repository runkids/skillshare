package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func fixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	for name, content := range map[string]string{".claude-plugin/plugin.json": `{"name":"demo","version":"1.0.0"}`, ".codex-plugin/plugin.json": `{"name":"demo","version":"1.0.0","skills":"./skills"}`, "skills/demo/SKILL.md": "---\nname: demo\ndescription: Demo\n---\nHello"} {
		p := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func TestDiscoverPreservesComponentsAndLeavesOutEscapes(t *testing.T) {
	root := fixture(t)
	d, err := Discover(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if len(d.Candidates) != 1 || len(d.Candidates[0].Targets) != 5 {
		t.Fatalf("unexpected discovery: %+v", d)
	}
	if err := os.Symlink("/etc/passwd", filepath.Join(root, "escape")); err != nil {
		t.Fatal(err)
	}
	if escaped, err := Discover(context.Background(), root); err != nil || escaped.Digest != d.Digest || len(escaped.Warnings) != 1 {
		t.Fatalf("external symlink not left out: %+v %v", escaped, err)
	}
}

func TestNativeInventoryStrict(t *testing.T) {
	for _, target := range []string{"claude", "codex"} {
		if _, err := parseInventory(target, []byte(`{"unexpected":[]}`), ""); err == nil {
			t.Fatal("unknown schema accepted")
		}
	}
	items, err := parseInventory("codex", []byte(`{"installed":[{"pluginId":"demo@market","name":"demo","marketplaceName":"market","installed":true,"enabled":false,"version":"1"}],"available":[]}`), "")
	if err != nil || len(items) != 1 || items[0].Enabled {
		t.Fatalf("%+v %v", items, err)
	}
}

func TestClaudeInventoryIgnoresOtherScopesBeforeValidating(t *testing.T) {
	data := []byte(`[{"id":"demo@market","scope":"user"},{"id":"(suppressed)@skills-dir","scope":"project","projectPath":"/home/me"}]`)
	items, err := parseInventory("claude", data, "")
	if err != nil || len(items) != 1 || items[0].ID != "demo@market" {
		t.Fatalf("%+v %v", items, err)
	}
	if _, err := parseInventory("claude", data, "/home/me"); err == nil {
		t.Fatal("unsupported identifier in the active project accepted")
	}
}

func TestPreviewCancelStaleAndPartialRetry(t *testing.T) {
	root := fixture(t)
	home := t.TempDir()
	config := filepath.Join(home, "config.yaml")
	if err := os.WriteFile(config, []byte("# keep me\nmode: merge\n"), 0644); err != nil {
		t.Fatal(err)
	}
	installed := map[string][]Installed{"claude": {}, "codex": {}}
	failCodex := true
	mutations := 0
	runner := func(ctx context.Context, dir string, env []string, bin string, args ...string) ([]byte, error) {
		joined := strings.Join(args, " ")
		if joined == "--version" {
			return []byte("test-version"), nil
		}
		if strings.Contains(joined, "--help") {
			return []byte("install add remove uninstall update enable disable --json --scope"), nil
		}
		if joined == "plugin list --json" {
			if bin == "codex" {
				return json.Marshal(map[string]any{"installed": installed[bin], "available": []any{}})
			}
			return json.Marshal(installed[bin])
		}
		if joined == "plugin marketplace list --json" {
			if bin == "codex" {
				return []byte(`{"marketplaces":[]}`), nil
			}
			return []byte(`[]`), nil
		}
		mutations++
		if bin == "codex" && failCodex {
			return nil, fmt.Errorf("offline")
		}
		if len(args) > 2 && (args[1] == "install" || args[1] == "add") {
			installed[bin] = append(installed[bin], Installed{ID: args[2], PluginID: args[2], Installed: true, Enabled: true, Scope: "user"})
		}
		return []byte(`{}`), nil
	}
	s := &Service{ConfigPath: config, StateDir: filepath.Join(home, "state"), Run: runner}
	req := Request{Action: "add", Source: root, Plugin: "demo", Targets: []string{"claude", "codex"}}
	p, err := s.Preview(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if mutations != 0 {
		t.Fatal("preview mutated native state")
	}
	if _, err := os.Stat(s.StateDir); !os.IsNotExist(err) {
		t.Fatal("preview created state")
	}
	if err := os.WriteFile(config, []byte("# changed\nmode: merge\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Apply(context.Background(), req, p.Revision); err == nil {
		t.Fatal("stale preview accepted")
	}
	p, err = s.Preview(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	result, err := s.Apply(context.Background(), req, p.Revision)
	if err == nil || len(result.Results) != 2 {
		t.Fatalf("expected partial result: %+v %v", result, err)
	}
	failCodex = false
	req = Request{Action: "sync"}
	p, err = s.Preview(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	before := mutations
	result, err = s.Apply(context.Background(), req, p.Revision)
	if err != nil {
		t.Fatal(err)
	}
	if mutations-before != 2 {
		t.Fatalf("retry repeated successful target: %d", mutations-before)
	}
	data, _ := os.ReadFile(config)
	if !strings.Contains(string(data), "# changed") {
		t.Fatal("lost user comment")
	}
}

func TestProjectCodexNeverFallsBackToGlobal(t *testing.T) {
	s := &Service{ConfigPath: filepath.Join(t.TempDir(), "config.yaml"), ProjectRoot: t.TempDir(), StateDir: t.TempDir()}
	p, err := s.Preview(context.Background(), Request{Action: "add", Source: fixture(t), Plugin: "demo", Targets: []string{"codex"}})
	if err != nil {
		t.Fatal(err)
	}
	if !p.Blocked {
		t.Fatal("project Codex install was not blocked")
	}
}

func TestSyncSelectionDoesNotToggleNativeEnabledState(t *testing.T) {
	dir := t.TempDir()
	config := filepath.Join(dir, "config.yaml")
	err := os.WriteFile(config, []byte("plugins:\n  packages:\n    demo:\n      bindings:\n        codex:\n          id: demo@market\n"), 0600)
	if err != nil {
		t.Fatal(err)
	}
	installed := true
	mutations := []string{}
	s := &Service{ConfigPath: config, StateDir: filepath.Join(dir, "state"), Run: func(ctx context.Context, dir string, env []string, target string, args ...string) ([]byte, error) {
		joined := strings.Join(args, " ")
		if joined == "--version" {
			return []byte("0.154.0"), nil
		}
		if strings.Contains(joined, "--help") {
			return []byte("--json"), nil
		}
		if joined == "plugin list --json" {
			if installed {
				return []byte(`{"installed":[{"pluginId":"demo@market","installed":true,"enabled":false}],"available":[]}`), nil
			}
			return []byte(`{"installed":[],"available":[]}`), nil
		}
		mutations = append(mutations, joined)
		if strings.HasPrefix(joined, "plugin remove") {
			installed = false
		}
		if strings.HasPrefix(joined, "plugin add") {
			installed = true
		}
		return []byte(`{}`), nil
	}}
	apply := func(action string) {
		t.Helper()
		r := Request{Action: action, Name: "demo", Targets: []string{"codex"}}
		p, e := s.Preview(context.Background(), r)
		if e != nil {
			t.Fatal(e)
		}
		if _, e = s.Apply(context.Background(), r, p.Revision); e != nil {
			t.Fatal(e)
		}
	}
	apply("disable")
	if len(mutations) != 0 || !installed {
		t.Fatal("deselect changed native installation")
	}
	apply("sync")
	if installed {
		t.Fatal("sync did not remove excluded plugin")
	}
	d, err := s.load()
	if err != nil {
		t.Fatal(err)
	}
	b, ok := d.packages["demo"].Bindings["codex"]
	if !ok || b.Selected() {
		t.Fatal("excluded definition was lost")
	}
	apply("enable")
	if installed {
		t.Fatal("select installed immediately")
	}
	apply("sync")
	if !installed {
		t.Fatal("sync did not restore installation")
	}
	if len(mutations) != 2 || strings.Contains(strings.Join(mutations, " "), "enable") {
		t.Fatalf("unexpected native mutation: %v", mutations)
	}
}

func TestAddWithoutTargetsKeepsPackageForLater(t *testing.T) {
	root := fixture(t)
	s, installed, mutations := fakeClaude(t)
	apply := func(r Request) *Plan {
		t.Helper()
		p, err := s.Preview(context.Background(), r)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := s.Apply(context.Background(), r, p.Revision); err != nil {
			t.Fatal(err)
		}
		return p
	}
	writeFile(t, root, ".codex-plugin/plugin.json", `{"name":"demo","version":"1.0.0","skills":"./skills","interface":{"logo":"./logo.png"}}`)
	writeFile(t, root, "logo.png", "png")
	p := apply(Request{Action: "add", Source: root, Plugin: "demo"})
	if len(p.Changes) != 1 || p.Changes[0].Action != "record" || *mutations != 0 {
		t.Fatalf("add without targets touched an Agent: %+v %d", p.Changes, *mutations)
	}
	d, _ := s.load()
	if pack, ok := d.packages["demo"]; !ok || pack.Source == "" || len(pack.Bindings) != 0 {
		t.Fatalf("package not kept for later: %+v", d.packages)
	}
	if p.Changes[0].Logo != "data:image/png;base64,cG5n" {
		t.Fatalf("preview lost the Codex logo: %q", p.Changes[0].Logo)
	}
	if d.packages["demo"].Version != "1.0.0" {
		t.Fatalf("version not recorded without an Agent: %+v", d.packages["demo"])
	}
	apply(Request{Action: "add", Name: "demo", Source: root, Plugin: "demo", Targets: []string{"claude"}})
	apply(Request{Action: "remove", Name: "demo", Targets: []string{"claude"}})
	if d, _ = s.load(); len(*installed) != 0 || d.packages["demo"].Source == "" {
		t.Fatalf("removing the last Agent dropped the package: %+v %+v", *installed, d.packages)
	}
	apply(Request{Action: "remove", Name: "demo"})
	if d, _ = s.load(); len(d.packages) != 0 {
		t.Fatalf("remove kept the package: %+v", d.packages)
	}
}

// fakeClaude answers like a Claude Code CLI with an empty inventory, and counts the calls that change it.
func fakeClaude(t *testing.T) (*Service, *[]Installed, *int) {
	home := t.TempDir()
	installed := []Installed{}
	mutations := 0
	s := &Service{ConfigPath: filepath.Join(home, "config.yaml"), StateDir: filepath.Join(home, "state"), Run: func(ctx context.Context, dir string, env []string, bin string, args ...string) ([]byte, error) {
		joined := strings.Join(args, " ")
		switch {
		case joined == "--version":
			return []byte("test-version"), nil
		case strings.Contains(joined, "--help"):
			return []byte("install add remove uninstall update enable disable --json --scope"), nil
		case joined == "plugin list --json":
			return json.Marshal(installed)
		case joined == "plugin marketplace list --json":
			return []byte(`[]`), nil
		}
		mutations++
		if len(args) > 2 && args[1] == "install" {
			installed = append(installed, Installed{ID: args[2], PluginID: args[2], Installed: true, Enabled: true, Scope: "user"})
		}
		if len(args) > 2 && args[1] == "uninstall" {
			installed = []Installed{}
		}
		return []byte(`{}`), nil
	}}
	return s, &installed, &mutations
}

func TestCheckReportsTheSourceVersion(t *testing.T) {
	root := fixture(t)
	s, _, _ := fakeClaude(t)
	r := Request{Action: "add", Source: root, Plugin: "demo", Targets: []string{"claude"}}
	p, err := s.Preview(context.Background(), r)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Apply(context.Background(), r, p.Revision); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".claude-plugin/plugin.json"), []byte(`{"name":"demo","version":"1.1.0"}`), 0644); err != nil {
		t.Fatal(err)
	}
	p, err = s.Preview(context.Background(), Request{Action: "check"})
	if err != nil || len(p.Changes) != 1 || p.Changes[0].Action != "update-available" || p.Changes[0].Binding.Version != "1.1.0" {
		t.Fatalf("check did not report the new version: %+v %v", p, err)
	}
}

func TestRecordedSourceDoesNotBlockAnotherDistribution(t *testing.T) {
	s, _, _ := fakeClaude(t)
	r := Request{Action: "add", Source: fixture(t), Plugin: "demo"}
	p, err := s.Preview(context.Background(), r)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Apply(context.Background(), r, p.Revision); err != nil {
		t.Fatal(err)
	}
	p, err = s.Preview(context.Background(), Request{Action: "add", Source: fixture(t), Plugin: "demo", Name: "demo", Targets: []string{"claude"}})
	if err != nil || p.Blocked {
		t.Fatalf("another source for one Agent was blocked: %+v %v", p, err)
	}
}

// fakeAgents installs plugins like Claude and Codex: installing or updating puts the
// marketplace's current version in place, and Codex's add always enables the plugin.
type fakeAgents struct {
	version   string
	installed map[string][]Installed
	commands  []string
}

func (f *fakeAgents) service(t *testing.T) *Service {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	native := filepath.Join(home, "native-codex")
	t.Setenv("CODEX_HOME", native)
	markets := map[string]string{}
	if f.installed == nil {
		f.installed = map[string][]Installed{}
	}
	s := &Service{ConfigPath: filepath.Join(home, "config.yaml"), StateDir: filepath.Join(home, "state")}
	s.Run = func(_ context.Context, _ string, _ []string, bin string, args ...string) ([]byte, error) {
		command := strings.Join(args, " ")
		switch {
		case command == "--version":
			return []byte("test"), nil
		case strings.Contains(command, "--help"):
			return []byte("--json --scope upgrade --config"), nil
		case command == "plugin list --json" && bin == "codex":
			if _, cfg, err := readCodexConfig(native); err == nil {
				for i := range f.installed[bin] {
					item := &f.installed[bin][i]
					if setting, ok := cfg.Plugins[item.PluginID]; ok && setting.Enabled != nil {
						item.Enabled = *setting.Enabled
					}
				}
			}
			return json.Marshal(map[string]any{"installed": f.installed[bin], "available": []any{}})
		case command == "plugin list --json":
			return json.Marshal(append([]Installed{}, f.installed[bin]...))
		case command == "plugin marketplace list --json" && bin == "codex":
			rows := []map[string]string{}
			for name, root := range markets {
				rows = append(rows, map[string]string{"name": name, "root": root})
			}
			return json.Marshal(map[string]any{"marketplaces": rows})
		case command == "plugin marketplace list --json":
			return []byte(`[]`), nil
		}
		f.commands = append(f.commands, bin+" "+command)
		if bin == "codex" && len(args) > 3 && args[1] == "marketplace" && args[2] == "add" {
			markets[filepath.Base(args[3])] = args[3]
		}
		if bin == "codex" && len(args) > 2 && args[1] == "add" {
			name, market, _ := strings.Cut(args[2], "@")
			root := markets[market]
			for _, arg := range args {
				if key, value, ok := strings.Cut(arg, "="); ok && strings.HasSuffix(key, ".source") {
					if err := json.Unmarshal([]byte(value), &root); err != nil {
						return nil, err
					}
				}
			}
			var catalog struct {
				Plugins []struct{ Name, Source string }
			}
			data, err := os.ReadFile(filepath.Join(root, ".agents/plugins/marketplace.json"))
			if err != nil {
				return nil, err
			}
			if err := json.Unmarshal(data, &catalog); err != nil {
				return nil, err
			}
			payload := ""
			for _, entry := range catalog.Plugins {
				if entry.Name == name {
					payload = filepath.Join(root, entry.Source)
				}
			}
			if payload == "" {
				return nil, fmt.Errorf("plugin missing from catalog")
			}
			cache := filepath.Join(native, "plugins/cache", market, name)
			if err := os.RemoveAll(cache); err != nil {
				return nil, err
			}
			if err := copyTree(payload, filepath.Join(cache, f.version)); err != nil {
				return nil, err
			}
			config, _, err := readCodexConfig(native)
			if err != nil && !os.IsNotExist(err) {
				return nil, err
			}
			if len(f.installed[bin]) == 0 {
				config = append(config, []byte(fmt.Sprintf("\n[plugins.%q]\nenabled = true\n", args[2]))...)
			} else {
				config, err = codexEnabledConfig(config, args[2])
				if err != nil {
					return nil, err
				}
			}
			writeFile(t, native, "config.toml", string(config))
		}
		if args[1] == "add" || args[1] == "install" || args[1] == "update" {
			f.installed[bin] = []Installed{{ID: args[2], PluginID: args[2], Installed: true, Enabled: true, Version: f.version, Scope: "user"}}
		}
		return []byte(`{}`), nil
	}
	return s
}

// bumpDemo releases version 2.0.0 of the fixture's plugin.
func bumpDemo(t *testing.T, source string, agents *fakeAgents) {
	t.Helper()
	writeFile(t, source, ".claude-plugin/plugin.json", `{"name":"demo","version":"2.0.0"}`)
	writeFile(t, source, ".codex-plugin/plugin.json", `{"name":"demo","version":"2.0.0","skills":"./skills"}`)
	agents.version, agents.commands = "2.0.0", nil
}

// Codex has no update command, so an update adds the plugin again from the new snapshot.
func TestCodexUpdateAddsThePluginAgain(t *testing.T) {
	agents := &fakeAgents{version: "1.0.0"}
	s := agents.service(t)
	source := fixture(t)
	applyPluginRequest(t, s, Request{Action: "add", Source: source, Targets: []string{"codex"}})
	bumpDemo(t, source, agents)
	applyPluginRequest(t, s, Request{Action: "update", Name: "demo", Targets: []string{"codex"}})
	if len(agents.commands) != 1 || !strings.HasPrefix(agents.commands[0], "codex plugin add demo@") {
		t.Fatalf("commands: %q", agents.commands)
	}
}

// Native add enables the selected plugin; the transaction restores its disabled state
// while still updating the package's other Agents.
func TestCodexUpdatePreservesADisabledPlugin(t *testing.T) {
	agents := &fakeAgents{version: "1.0.0"}
	s := agents.service(t)
	source := fixture(t)
	applyPluginRequest(t, s, Request{Action: "add", Source: source, Targets: []string{"claude", "codex"}})
	native := os.Getenv("CODEX_HOME")
	before, _, err := readCodexConfig(native)
	if err != nil {
		t.Fatal(err)
	}
	before = []byte(strings.Replace(string(before), "enabled = true", "enabled = false", 1))
	writeFile(t, native, "config.toml", string(before))
	bumpDemo(t, source, agents)
	applyPluginRequest(t, s, Request{Action: "update", Name: "demo", Targets: []string{"claude", "codex"}})
	if !slices.ContainsFunc(agents.commands, func(c string) bool { return strings.HasPrefix(c, "codex plugin add ") }) || !slices.Contains(agents.commands, "claude plugin update "+agents.installed["claude"][0].ID+" --scope user --json") {
		t.Fatalf("commands: %q", agents.commands)
	}
	after, _, err := readCodexConfig(native)
	if err != nil || string(before) != string(after) {
		t.Fatalf("disabled config changed: %s %v", after, err)
	}
	if item := agents.installed["codex"][0]; item.Enabled || item.Version != "2.0.0" {
		t.Fatalf("disabled plugin not updated safely: %+v", item)
	}
}

// Imported updates require reviewable native registration; an inventory alone is insufficient.
func TestCodexUpdateOfAnImportRequiresReviewableRegistration(t *testing.T) {
	agents := &fakeAgents{installed: map[string][]Installed{"codex": {{PluginID: "demo@team", Installed: true, Enabled: true, Version: "1.0.0"}}}}
	s := agents.service(t)
	applyPluginRequest(t, s, Request{Action: "import", From: "codex", Plugin: "demo@team"})
	agents.commands = nil
	p, err := s.Preview(context.Background(), Request{Action: "update", Name: "demo", Targets: []string{"codex"}})
	if err != nil || p.Blocked || len(p.Changes) != 1 || p.Changes[0].Action != "skip" || len(agents.commands) != 0 {
		t.Fatalf("unreviewable update was not skipped: %+v %v %q", p, err, agents.commands)
	}
	p, err = s.Preview(context.Background(), Request{Action: "check", Name: "demo", Targets: []string{"codex"}})
	if err != nil || !p.Blocked || len(p.Changes) != 1 || p.Changes[0].Action != "blocked" {
		t.Fatalf("unreviewable check did not remain blocked: %+v %v", p, err)
	}
}

func TestCodexUpdatePreviewFailureSkipsAndUpdatesOtherAgents(t *testing.T) {
	agents := &fakeAgents{
		version: "1.0.0",
		installed: map[string][]Installed{
			"codex": {{ID: "demo@team", PluginID: "demo@team", Installed: true, EnabledKnown: true, Enabled: true, Version: "1.0.0", Scope: "user"}},
		},
	}
	s := agents.service(t)
	source := fixture(t)
	applyPluginRequest(t, s, Request{Action: "add", Source: source, Targets: []string{"claude"}})

	config, err := s.load()
	if err != nil {
		t.Fatal(err)
	}
	pack := config.packages["demo"]
	pack.Bindings["codex"] = Binding{ID: "demo@team", Version: "1.0.0"}
	config.packages["demo"] = pack
	if err := s.save(config); err != nil {
		t.Fatal(err)
	}
	writeFile(t, os.Getenv("CODEX_HOME"), "config.toml", "[plugins.\"demo@team\"]\nenabled = true\n")
	bumpDemo(t, source, agents)
	request := Request{Action: "update", Name: "demo", Targets: []string{"claude", "codex"}}

	plan, err := s.Preview(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Blocked {
		t.Fatalf("unreviewable Codex update blocked the independent Claude update: %+v", plan.Changes)
	}
	if len(plan.Changes) != 2 {
		t.Fatalf("unexpected update preview: %+v", plan.Changes)
	}
	for _, change := range plan.Changes {
		switch change.Target {
		case "claude":
			if change.Action != "update" {
				t.Fatalf("Claude update missing from preview: %+v", change)
			}
		case "codex":
			if change.Action != "skip" || !strings.Contains(change.Message, "marketplace is not registered") {
				t.Fatalf("Codex preview failure was not reported as a skip: %+v", change)
			}
		default:
			t.Fatalf("unexpected update target: %+v", change)
		}
	}

	result, err := s.Apply(context.Background(), request, plan.Revision)
	if err != nil {
		t.Fatalf("apply aborted after the Codex preview failure: %+v %v", result, err)
	}
	statuses := map[string]Outcome{}
	for _, outcome := range result.Results {
		statuses[outcome.Target] = outcome
	}
	if statuses["claude"].Status != "installed" {
		t.Fatalf("Claude was not updated: %+v", result.Results)
	}
	if statuses["codex"].Status != "skipped" || !strings.Contains(statuses["codex"].Message, "marketplace is not registered") {
		t.Fatalf("Codex failure was not retained in the result: %+v", result.Results)
	}
	if !slices.ContainsFunc(agents.commands, func(command string) bool { return strings.HasPrefix(command, "claude plugin update ") }) {
		t.Fatalf("Claude native update did not run: %q", agents.commands)
	}
	if slices.ContainsFunc(agents.commands, func(command string) bool { return strings.HasPrefix(command, "codex plugin ") }) {
		t.Fatalf("Codex mutated native state despite missing marketplace registration: %q", agents.commands)
	}
	config, err = s.load()
	if err != nil {
		t.Fatal(err)
	}
	if pending := config.packages["demo"].Bindings["codex"].Pending; pending != "update" {
		t.Fatalf("Codex update was not left pending: %q", pending)
	}
}

// A skipped update stays pending, so a later sync still knows the plugin is behind.
func TestSkippedUpdateStaysPending(t *testing.T) {
	home := t.TempDir()
	s := &Service{ConfigPath: filepath.Join(home, "config.yaml"), StateDir: filepath.Join(home, "state")}
	writeFile(t, home, "config.yaml", "plugins:\n  packages:\n    demo:\n      bindings:\n        antigravity-cli:\n          id: demo\n          pending: update\n")
	s.Run = func(_ context.Context, _ string, _ []string, bin string, args ...string) ([]byte, error) {
		command := strings.Join(args, " ")
		switch command {
		case "--version":
			return []byte("test"), nil
		case "plugin list":
			return []byte(`{"imports":[{"name":"demo","version":"1.0.0","enabled":true}]}`), nil
		case "plugin --help":
			return []byte("install"), nil
		}
		return nil, fmt.Errorf("unexpected native mutation: %s %s", bin, command)
	}
	applyPluginRequest(t, s, Request{Action: "sync", Targets: []string{"antigravity-cli"}})
	cfg, err := s.load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.packages["demo"].Bindings["antigravity-cli"].Pending != "update" {
		t.Fatalf("pending cleared: %+v", cfg.packages["demo"].Bindings["antigravity-cli"])
	}
}
