package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
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
	if len(d.Candidates) != 1 || len(d.Candidates[0].Targets) != 6 {
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
		if joined == "plugin marketplace list --json" {
			return []byte(`{"marketplaces":[{"name":"market","root":"/market"}]}`), nil
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

func TestManagedMarketplaceNameShowsThePlugin(t *testing.T) {
	root := fixture(t)
	for _, manifest := range []string{".claude-plugin/plugin.json", ".codex-plugin/plugin.json"} {
		if err := os.WriteFile(filepath.Join(root, manifest), []byte(`{"name":"my.demo","version":"1.0.0"}`), 0644); err != nil {
			t.Fatal(err)
		}
	}
	s, _, _ := fakeClaude(t)
	p, err := s.Preview(context.Background(), Request{Action: "add", Source: root, Plugin: "my.demo", Targets: []string{"claude"}})
	if err != nil || len(p.Changes) != 1 {
		t.Fatalf("preview failed: %+v %v", p, err)
	}
	// Codex rejects marketplace names outside ASCII letters, digits, `_`, and `-`.
	if id := p.Changes[0].ID; !regexp.MustCompile(`^my\.demo@skillshare-my-demo-[0-9a-f]{16}$`).MatchString(id) {
		t.Fatalf("marketplace name does not show the plugin: %s", id)
	}
}

func TestFailedNativeCommandOutcomeCarriesItsKey(t *testing.T) {
	s, _, _ := fakeClaude(t)
	run := s.Run
	s.Run = func(ctx context.Context, dir string, env []string, bin string, args ...string) ([]byte, error) {
		if len(args) > 1 && args[1] == "install" && !slices.Contains(args, "--help") {
			return nil, agentError{key: "plugins.error.commandFailed", message: "claude command failed"}
		}
		return run(ctx, dir, env, bin, args...)
	}
	r := Request{Action: "add", Source: fixture(t), Plugin: "demo", Targets: []string{"claude"}}
	p, err := s.Preview(context.Background(), r)
	if err != nil {
		t.Fatal(err)
	}
	result, err := s.Apply(context.Background(), r, p.Revision)
	if result == nil || len(result.Results) != 1 || result.Results[0].MessageKey != "plugins.error.commandFailed" {
		t.Fatalf("failed outcome lost its translation key: %+v %v", result, err)
	}
}

func TestExcludingAManagedPluginRemovesItsMarketplace(t *testing.T) {
	agents := &fakeAgents{version: "1.0.0"}
	s := agents.service(t)
	applyPluginRequest(t, s, Request{Action: "add", Source: fixture(t), Targets: []string{"claude", "codex"}})
	if len(agents.markets["claude"]) != 1 || len(agents.markets["codex"]) != 1 {
		t.Fatalf("install did not register marketplaces: %v", agents.markets)
	}
	applyPluginRequest(t, s, Request{Action: "disable", Name: "demo", Targets: []string{"claude", "codex"}})
	applyPluginRequest(t, s, Request{Action: "sync"})
	if len(agents.markets["claude"]) != 0 || len(agents.markets["codex"]) != 0 {
		t.Fatalf("Skillshare marketplaces were left behind: %v", agents.markets)
	}
}

func TestAddingAClaudePluginNamedLikeASkillFolderExplainsTheClash(t *testing.T) {
	agents := &fakeAgents{installed: map[string][]Installed{"claude": {{ID: "demo@skills-dir", PluginID: "demo@skills-dir", Installed: true, Enabled: true, Scope: "user"}}}}
	s := agents.service(t)
	p, err := s.Preview(context.Background(), Request{Action: "add", Source: fixture(t), Targets: []string{"claude"}})
	if err != nil || len(p.Changes) != 1 || p.Changes[0].Action != "install" || p.Changes[0].MessageKey != "plugins.note.skillsDirClash" {
		t.Fatalf("preview did not explain the skill folder clash: %+v %v", p, err)
	}
}

// legacyBinding records a plugin under the older skillshare-<hash> marketplace name, whose
// marketplace is still registered although the plugin itself is gone.
func legacyBinding(t *testing.T, s *Service, agents *fakeAgents, extra string) Binding {
	t.Helper()
	b := Binding{ID: "demo@skillshare-0123456789abcdef", Source: "https://example.com/demo", Plugin: "demo"}
	writePluginFile(t, filepath.Dir(s.ConfigPath), "config.yaml", "plugins:\n  packages:\n    demo:\n      bindings:\n        claude:\n          id: "+b.ID+"\n          source: "+b.Source+"\n          plugin: demo\n"+extra)
	agents.market("claude")["skillshare-0123456789abcdef"] = s.snapshotPath(b, "claude")
	return b
}

func TestExcludingAPluginAlreadyGoneStillRemovesItsMarketplace(t *testing.T) {
	agents := &fakeAgents{}
	s := agents.service(t)
	legacyBinding(t, s, agents, "          sync: false\n")
	p, err := s.Preview(context.Background(), Request{Action: "sync"})
	if err != nil || len(p.Changes) != 1 || p.Changes[0].Action != "uninstall" || p.Changes[0].MessageKey != "plugins.note.marketplaceCleanup" {
		t.Fatalf("sync did not plan the marketplace cleanup: %+v %v", p, err)
	}
	applyPluginRequest(t, s, Request{Action: "sync"})
	if len(agents.markets["claude"]) != 0 || !slices.Equal(agents.commands, []string{"claude plugin marketplace remove skillshare-0123456789abcdef"}) {
		t.Fatalf("cleanup ran %q, left %v", agents.commands, agents.markets)
	}
}

func TestRemovingAPluginAlreadyGoneRemovesItsMarketplace(t *testing.T) {
	agents := &fakeAgents{}
	s := agents.service(t)
	legacyBinding(t, s, agents, "")
	applyPluginRequest(t, s, Request{Action: "remove", Name: "demo"})
	inv, err := s.Packages()
	if err != nil || len(agents.markets["claude"]) != 0 || len(inv.Packages) != 0 {
		t.Fatalf("remove left %v, packages %v, err %v", agents.markets, inv, err)
	}
}

func TestFailedMarketplaceCleanupIsRetriedOnTheNextSync(t *testing.T) {
	agents := &fakeAgents{version: "1.0.0"}
	s := agents.service(t)
	applyPluginRequest(t, s, Request{Action: "add", Source: fixture(t), Targets: []string{"claude"}})
	applyPluginRequest(t, s, Request{Action: "disable", Name: "demo", Targets: []string{"claude"}})
	agents.failRemove = 1
	r := Request{Action: "sync"}
	p, err := s.Preview(context.Background(), r)
	if err != nil {
		t.Fatal(err)
	}
	result, _ := s.Apply(context.Background(), r, p.Revision)
	if result == nil || len(result.Results) != 1 || result.Results[0].MessageKey != "plugins.error.marketplaceCleanup" {
		t.Fatalf("cleanup failure was not reported: %+v", result)
	}
	applyPluginRequest(t, s, r)
	if len(agents.markets["claude"]) != 0 {
		t.Fatalf("retry left %v", agents.markets)
	}
}

func TestMarketplaceStillRegisteredAfterRemovalIsReported(t *testing.T) {
	agents := &fakeAgents{stuck: true}
	s := agents.service(t)
	legacyBinding(t, s, agents, "          sync: false\n")
	r := Request{Action: "sync"}
	p, err := s.Preview(context.Background(), r)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Apply(context.Background(), r, p.Revision); err == nil {
		t.Fatal("a marketplace that survived removal was reported as removed")
	}
}

func TestForeignMarketplaceWithTheSameNameIsLeftAlone(t *testing.T) {
	agents := &fakeAgents{}
	s := agents.service(t)
	legacyBinding(t, s, agents, "          sync: false\n")
	agents.market("claude")["skillshare-0123456789abcdef"] = "/elsewhere"
	applyPluginRequest(t, s, Request{Action: "sync"})
	if len(agents.commands) != 0 || agents.markets["claude"]["skillshare-0123456789abcdef"] != "/elsewhere" {
		t.Fatalf("foreign marketplace was touched: %q %v", agents.commands, agents.markets)
	}
}

func TestRemoveKeepsTheBindingWhileMarketplacesAreUnknown(t *testing.T) {
	agents := &fakeAgents{}
	s := agents.service(t)
	legacyBinding(t, s, agents, "")
	run := s.Run
	s.Run = func(ctx context.Context, dir string, env []string, bin string, args ...string) ([]byte, error) {
		if strings.Join(args, " ") == "plugin marketplace list --json" {
			return []byte(`not json`), nil
		}
		return run(ctx, dir, env, bin, args...)
	}
	r := Request{Action: "remove", Name: "demo"}
	p, err := s.Preview(context.Background(), r)
	if err != nil || len(p.Changes) == 0 || p.Changes[0].Action != "remove" {
		t.Fatalf("remove forgot a binding whose marketplace may remain: %+v %v", p, err)
	}
	_, _ = s.Apply(context.Background(), r, p.Revision)
	if inv, err := s.Packages(); err != nil || len(inv.Packages) != 1 {
		t.Fatalf("binding was dropped before its marketplace cleanup: %+v %v", inv, err)
	}
}

func TestHostReportsOnlyItsOwnManagedMarketplaces(t *testing.T) {
	agents := &fakeAgents{}
	s := agents.service(t)
	b := legacyBinding(t, s, agents, "")
	agents.market("claude")["skillshare-foreign"] = "/elsewhere/skillshare-foreign"
	agents.market("claude")["team"] = filepath.Join(filepath.Dir(s.snapshotPath(b, "claude")), "team")
	if got := s.host(context.Background(), "claude").ManagedMarketplaces; !slices.Equal(got, []string{"skillshare-0123456789abcdef"}) {
		t.Fatalf("managed marketplaces = %v", got)
	}
}

func TestMarketplaceAlsoDeclaredAtAnotherPathIsNotRemoved(t *testing.T) {
	agents := &fakeAgents{}
	s := agents.service(t)
	b := legacyBinding(t, s, agents, "          sync: false\n")
	run := s.Run
	s.Run = func(ctx context.Context, dir string, env []string, bin string, args ...string) ([]byte, error) {
		if strings.Join(args, " ") == "plugin marketplace list --json" {
			return json.Marshal([]map[string]string{{"name": "skillshare-0123456789abcdef", "installLocation": s.snapshotPath(b, "claude")}, {"name": "skillshare-0123456789abcdef", "installLocation": "/elsewhere"}})
		}
		return run(ctx, dir, env, bin, args...)
	}
	r := Request{Action: "sync"}
	p, err := s.Preview(context.Background(), r)
	if err != nil {
		t.Fatal(err)
	}
	result, _ := s.Apply(context.Background(), r, p.Revision)
	if slices.ContainsFunc(agents.commands, func(c string) bool { return strings.Contains(c, "marketplace remove") }) || result == nil || len(result.Results) != 1 || result.Results[0].MessageKey != "plugins.error.marketplaceCleanup" {
		t.Fatalf("removed a marketplace also declared elsewhere: %q %+v", agents.commands, result)
	}
}

func TestSyncSkipsAnImportWhoseNativeMarketplaceIsGone(t *testing.T) {
	agents := &fakeAgents{}
	s := agents.service(t)
	writePluginFile(t, filepath.Dir(s.ConfigPath), "config.yaml", "plugins:\n  packages:\n    demo:\n      source: https://example.com/demo\n      plugin: demo\n      bindings:\n        claude:\n          id: demo@team\n          pending: install\n")
	p, err := s.Preview(context.Background(), Request{Action: "sync"})
	if err != nil || p.Blocked || len(p.Changes) != 1 || p.Changes[0].Action != "skip" || p.Changes[0].MessageKey != "plugins.skip.marketplaceGone" {
		t.Fatalf("sync did not skip the import: %+v %v", p, err)
	}
	applyPluginRequest(t, s, Request{Action: "sync"})
	if len(agents.commands) != 0 {
		t.Fatalf("skipped import still ran %q", agents.commands)
	}
}

func TestUpdateRegistersAMissingManagedMarketplaceAgain(t *testing.T) {
	agents := &fakeAgents{version: "1.0.0"}
	s := agents.service(t)
	source := fixture(t)
	applyPluginRequest(t, s, Request{Action: "add", Source: source, Targets: []string{"claude"}})
	agents.markets["claude"] = nil
	bumpDemo(t, source, agents)
	applyPluginRequest(t, s, Request{Action: "update", Name: "demo", Targets: []string{"claude"}})
	if len(agents.markets["claude"]) != 1 || !strings.HasPrefix(agents.commands[0], "claude plugin marketplace add ") {
		t.Fatalf("update did not register the marketplace again: %q", agents.commands)
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
	// markets maps each Agent to the root of every marketplace registered with it.
	markets map[string]map[string]string
	// failRemove fails that many marketplace removals; stuck keeps the registration anyway,
	// as a copy declared in another Claude settings scope does.
	failRemove int
	stuck      bool
	commands   []string
}

func (f *fakeAgents) market(bin string) map[string]string {
	if f.markets[bin] == nil {
		f.markets[bin] = map[string]string{}
	}
	return f.markets[bin]
}

func (f *fakeAgents) service(t *testing.T) *Service {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	if f.installed == nil {
		f.installed = map[string][]Installed{}
	}
	if f.markets == nil {
		f.markets = map[string]map[string]string{}
	}
	s := &Service{ConfigPath: filepath.Join(home, "config.yaml"), StateDir: filepath.Join(home, "state")}
	s.Run = func(_ context.Context, _ string, _ []string, bin string, args ...string) ([]byte, error) {
		command := strings.Join(args, " ")
		switch {
		case command == "--version":
			if bin == "omp" {
				return []byte("omp/18.6.1"), nil
			}
			return []byte("test"), nil
		case strings.Contains(command, "--help"):
			return []byte("--json --scope upgrade"), nil
		case command == "plugin list --json" && bin == "codex":
			return json.Marshal(map[string]any{"installed": f.installed[bin], "available": []any{}})
		case command == "plugin list --json" && bin == "omp":
			// omp plugin list --json: npm/link plugins, then marketplace summaries per scope.
			npm, market := []map[string]any{}, []map[string]any{}
			for _, i := range f.installed[bin] {
				if !validID(i.ID) {
					npm = append(npm, map[string]any{"name": i.ID, "version": i.Version, "enabled": i.Enabled, "manifest": map[string]any{}})
					continue
				}
				scope := i.Scope
				if scope == "" {
					scope = "user"
				}
				market = append(market, map[string]any{"id": i.ID, "scope": scope, "entries": []map[string]any{{"scope": scope, "installPath": ompCachePath(filepath.Join(os.Getenv("HOME"), ".omp/plugins/cache/plugins"), i.ID, i.Version), "version": i.Version, "enabled": i.Enabled}}})
			}
			return json.Marshal(map[string]any{"npm": npm, "marketplace": market})
		case command == "plugin marketplace list" && bin == "omp":
			var text strings.Builder
			text.WriteString("Configured Marketplaces:\n\n")
			for name, root := range f.market(bin) {
				text.WriteString("  \x1b[36m" + name + "\x1b[39m  \x1b[2m" + root + "\x1b[22m\n")
			}
			return []byte(text.String()), nil
		case command == "plugin list --json":
			return json.Marshal(append([]Installed{}, f.installed[bin]...))
		case command == "plugin marketplace list --json":
			list := []map[string]string{}
			for name, root := range f.market(bin) {
				if bin == "codex" {
					list = append(list, map[string]string{"name": name, "root": root})
				} else {
					list = append(list, map[string]string{"name": name, "installLocation": root})
				}
			}
			if bin == "codex" {
				return json.Marshal(map[string]any{"marketplaces": list})
			}
			return json.Marshal(list)
		}
		f.commands = append(f.commands, bin+" "+command)
		switch {
		case args[1] == "marketplace" && args[2] == "add":
			f.market(bin)[filepath.Base(args[3])] = args[3]
		case args[1] == "marketplace" && args[2] == "remove":
			if f.failRemove > 0 {
				f.failRemove--
				return nil, agentError{key: "plugins.error.commandFailed", message: bin + " command failed"}
			}
			// Like Claude, a scoped removal fails when only known_marketplaces.json records it.
			if slices.Contains(args, "--scope") {
				return nil, agentError{key: "plugins.error.commandFailed", message: "Marketplace '" + args[3] + "' is not declared in user settings"}
			}
			if !f.stuck {
				delete(f.market(bin), args[3])
			}
		case args[1] == "add" || args[1] == "install" || args[1] == "update" || args[1] == "upgrade":
			scope := "user"
			if i := slices.Index(args, "--scope"); i >= 0 {
				scope = args[i+1]
			}
			enabled := true
			// OMP's upgrade reinstalls from the marketplace and keeps a disabled plugin disabled.
			if args[1] == "upgrade" {
				for _, i := range f.installed[bin] {
					if i.ID == args[2] {
						enabled = i.Enabled
					}
				}
			}
			f.installed[bin] = []Installed{{ID: args[2], PluginID: args[2], Installed: true, Enabled: enabled, Version: f.version, Scope: scope}}
		case args[1] == "remove" || args[1] == "uninstall":
			f.installed[bin] = nil
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

// Adding would turn a plugin disabled in Codex back on, so the update skips it, and the
// other Agents of the plugin are still updated.
func TestCodexUpdateSkipsADisabledPlugin(t *testing.T) {
	agents := &fakeAgents{version: "1.0.0"}
	s := agents.service(t)
	source := fixture(t)
	applyPluginRequest(t, s, Request{Action: "add", Source: source, Targets: []string{"claude", "codex"}})
	agents.installed["codex"][0].Enabled = false
	bumpDemo(t, source, agents)
	applyPluginRequest(t, s, Request{Action: "update", Name: "demo", Targets: []string{"claude", "codex"}})
	if slices.ContainsFunc(agents.commands, func(c string) bool { return strings.HasPrefix(c, "codex ") }) || !slices.Contains(agents.commands, "claude plugin update "+agents.installed["claude"][0].ID+" --scope user --json") {
		t.Fatalf("commands: %q", agents.commands)
	}
}

// An imported plugin has no reviewed source; Codex upgrades it with its marketplace.
func TestCodexUpdateOfAnImportUpgradesItsMarketplace(t *testing.T) {
	agents := &fakeAgents{installed: map[string][]Installed{"codex": {{PluginID: "demo@team", Installed: true, Enabled: true, Version: "1.0.0"}}}, markets: map[string]map[string]string{"codex": {"team": "/team"}}}
	s := agents.service(t)
	applyPluginRequest(t, s, Request{Action: "import", From: "codex", Plugin: "demo@team"})
	agents.commands = nil
	applyPluginRequest(t, s, Request{Action: "update", Name: "demo", Targets: []string{"codex"}})
	if !slices.Equal(agents.commands, []string{"codex plugin marketplace upgrade team"}) {
		t.Fatalf("commands: %q", agents.commands)
	}
}

// A skipped update stays pending, so a later sync still knows the plugin is behind.
func TestSkippedUpdateStaysPending(t *testing.T) {
	agents := &fakeAgents{version: "1.0.0"}
	s := agents.service(t)
	source := fixture(t)
	applyPluginRequest(t, s, Request{Action: "add", Source: source, Targets: []string{"codex"}})
	cfg, err := s.load()
	if err != nil {
		t.Fatal(err)
	}
	pack := cfg.packages["demo"]
	b := pack.Bindings["codex"]
	b.Pending = "update"
	pack.Bindings["codex"] = b
	cfg.packages["demo"] = pack
	if err := s.save(cfg); err != nil {
		t.Fatal(err)
	}
	agents.installed["codex"][0].Enabled = false
	applyPluginRequest(t, s, Request{Action: "sync"})
	if cfg, _ = s.load(); cfg.packages["demo"].Bindings["codex"].Pending != "update" {
		t.Fatalf("pending cleared: %+v", cfg.packages["demo"].Bindings["codex"])
	}
}
