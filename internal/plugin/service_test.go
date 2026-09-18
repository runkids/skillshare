package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
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

func TestDiscoverPreservesComponentsAndRejectsEscapes(t *testing.T) {
	root := fixture(t)
	d, err := Discover(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if len(d.Candidates) != 1 || len(d.Candidates[0].Targets) != 2 {
		t.Fatalf("unexpected discovery: %+v", d)
	}
	if err := os.Symlink("/etc/passwd", filepath.Join(root, "escape")); err != nil {
		t.Fatal(err)
	}
	if _, err := Discover(context.Background(), root); err == nil {
		t.Fatal("external symlink accepted")
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
	runner := func(ctx context.Context, dir, bin string, args ...string) ([]byte, error) {
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
	s := &Service{ConfigPath: config, StateDir: filepath.Join(dir, "state"), Run: func(ctx context.Context, dir, target string, args ...string) ([]byte, error) {
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
