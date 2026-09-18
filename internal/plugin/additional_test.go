package plugin

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func writePluginFile(t *testing.T, root, name, content string) {
	t.Helper()
	p := filepath.Join(root, name)
	if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

func TestAdditionalPluginDiscovery(t *testing.T) {
	for _, tt := range []struct{ target, manifest, content string }{
		{"cursor", ".cursor-plugin/plugin.json", `{"name":"demo","version":"1"}`},
		{"antigravity", "plugin.json", `{"name":"demo","version":"1"}`},
		{"pi", "package.json", `{"name":"@team/demo","version":"1","pi":{"skills":["skills"]}}`},
		{"opencode", "package.json", `{"name":"demo","version":"1","main":"index.js","dependencies":{"@opencode-ai/plugin":"1"}}`},
	} {
		t.Run(tt.target, func(t *testing.T) {
			root := t.TempDir()
			writePluginFile(t, root, tt.manifest, tt.content)
			writePluginFile(t, root, "index.js", "export default async () => ({})")
			d, err := Discover(context.Background(), root)
			if err != nil {
				t.Fatal(err)
			}
			if !slices.Contains(d.Candidates[0].Targets, tt.target) {
				t.Fatalf("missing target: %+v", d)
			}
		})
	}
}

func TestAdditionalPluginScope(t *testing.T) {
	s := Service{ProjectRoot: t.TempDir()}
	for _, target := range []string{"cursor", "codex"} {
		h := s.host(context.Background(), target)
		if h.Error == "" {
			t.Fatalf("%s fell back to global", target)
		}
	}
}

func applyPluginRequest(t *testing.T, s *Service, r Request) {
	t.Helper()
	p, err := s.Preview(context.Background(), r)
	if err != nil {
		t.Fatal(err)
	}
	if p.Blocked {
		t.Fatalf("blocked: %+v", p.Changes)
	}
	result, err := s.Apply(context.Background(), r, p.Revision)
	if err != nil {
		t.Fatalf("apply: %+v %v", result, err)
	}
}

func TestLocalPluginLifecycleAndOwnership(t *testing.T) {
	for _, target := range []string{"cursor", "antigravity"} {
		t.Run(target, func(t *testing.T) {
			home := t.TempDir()
			t.Setenv("HOME", home)
			source := t.TempDir()
			manifest := "plugin.json"
			if target == "cursor" {
				manifest = ".cursor-plugin/plugin.json"
			}
			writePluginFile(t, source, manifest, `{"name":"demo","version":"1"}`)
			writePluginFile(t, source, "skills/demo/SKILL.md", "hello")
			s := &Service{ConfigPath: filepath.Join(home, "config.yaml"), StateDir: filepath.Join(home, "state")}
			applyPluginRequest(t, s, Request{Action: "add", Source: source, Targets: []string{target}})
			root, err := s.localRoot(target)
			if err != nil {
				t.Fatal(err)
			}
			installed := filepath.Join(root, "demo", "skills/demo/SKILL.md")
			if data, err := os.ReadFile(installed); err != nil || string(data) != "hello" {
				t.Fatal("whole plugin not installed", err)
			}
			applyPluginRequest(t, s, Request{Action: "disable", Name: "demo"})
			if _, err := os.Stat(installed); err != nil {
				t.Fatal("selection removed files before sync")
			}
			applyPluginRequest(t, s, Request{Action: "sync"})
			if _, err := os.Stat(installed); !os.IsNotExist(err) {
				t.Fatal("deselected target still installed")
			}
			applyPluginRequest(t, s, Request{Action: "enable", Name: "demo"})
			applyPluginRequest(t, s, Request{Action: "sync"})
			writePluginFile(t, source, "skills/demo/SKILL.md", "updated")
			applyPluginRequest(t, s, Request{Action: "update", Name: "demo"})
			if data, _ := os.ReadFile(installed); string(data) != "updated" {
				t.Fatal("source update not applied")
			}
			writePluginFile(t, root, "demo/skills/demo/SKILL.md", "user edit")
			p, err := s.Preview(context.Background(), Request{Action: "remove", Name: "demo"})
			if err != nil {
				t.Fatal(err)
			}
			_, err = s.Apply(context.Background(), Request{Action: "remove", Name: "demo"}, p.Revision)
			if err == nil {
				t.Fatal("removed locally edited files")
			}
			if data, _ := os.ReadFile(installed); string(data) != "user edit" {
				t.Fatal("user changes lost")
			}
		})
	}
}

func TestAntigravityProjectIsolation(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	project := t.TempDir()
	source := t.TempDir()
	writePluginFile(t, source, "plugin.json", `{"name":"demo"}`)
	s := &Service{ConfigPath: filepath.Join(project, ".skillshare/config.yaml"), StateDir: filepath.Join(home, "state"), ProjectRoot: project}
	applyPluginRequest(t, s, Request{Action: "add", Source: source, Targets: []string{"antigravity"}})
	if _, err := os.Stat(filepath.Join(project, ".agents/plugins/demo/plugin.json")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(home, ".gemini")); !os.IsNotExist(err) {
		t.Fatal("project operation wrote global config")
	}
	applyPluginRequest(t, s, Request{Action: "remove", Name: "demo"})
}

func TestOpenCodeConfigPreservesOtherEntries(t *testing.T) {
	for _, version := range []string{"1.18.31", "2.0.0"} {
		t.Run(version, func(t *testing.T) {
			home := t.TempDir()
			t.Setenv("HOME", home)
			t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
			t.Setenv("OPENCODE_CONFIG", "")
			t.Setenv("OPENCODE_CONFIG_DIR", "")
			t.Setenv("OPENCODE_CONFIG_CONTENT", "")
			source := t.TempDir()
			writePluginFile(t, source, "package.json", `{"name":"demo","main":"index.js","dependencies":{"@opencode-ai/plugin":"1"}}`)
			writePluginFile(t, source, "index.js", "export default async () => ({})")
			key, _ := openCodeKey(version)
			path := filepath.Join(home, ".config/opencode/opencode.jsonc")
			writePluginFile(t, home, ".config/opencode/opencode.jsonc", "{\n// keep me\n\"model\":\"unchanged\",\""+key+"\":[\"other-package\"],\n}")
			s := &Service{ConfigPath: filepath.Join(home, "config.yaml"), StateDir: filepath.Join(home, "state"), Run: func(_ context.Context, _, _ string, args ...string) ([]byte, error) { return []byte(version), nil }}
			applyPluginRequest(t, s, Request{Action: "add", Source: source, Targets: []string{"opencode"}})
			raw, _, entries, err := readPackageConfig(path, key)
			if err != nil || len(entries) != 2 || !strings.Contains(string(raw), "// keep me") || !strings.Contains(string(raw), "unchanged") {
				t.Fatalf("config not preserved: %s %v", raw, err)
			}
			applyPluginRequest(t, s, Request{Action: "disable", Name: "demo"})
			applyPluginRequest(t, s, Request{Action: "sync"})
			_, _, entries, err = readPackageConfig(path, key)
			if err != nil || len(entries) != 1 || string(entries[0]) != `"other-package"` {
				t.Fatal("unrelated plugin modified", err)
			}
			applyPluginRequest(t, s, Request{Action: "enable", Name: "demo"})
			applyPluginRequest(t, s, Request{Action: "sync"})
			applyPluginRequest(t, s, Request{Action: "remove", Name: "demo"})
		})
	}
}

func TestPiFilteredImportRejected(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("PI_CODING_AGENT_DIR", filepath.Join(home, ".pi/agent"))
	writePluginFile(t, home, ".pi/agent/settings.json", `{"packages":[{"source":"npm:demo","skills":[]}]}`)
	s := &Service{ConfigPath: filepath.Join(home, "config.yaml"), Run: func(_ context.Context, _, _ string, _ ...string) ([]byte, error) { return []byte("0.85.1"), nil }}
	if _, err := s.Preview(context.Background(), Request{Action: "import", From: "pi", Plugin: "npm:demo"}); err == nil {
		t.Fatal("resource filters would be lost")
	}
}

func TestPluginFormatsRemainDistinct(t *testing.T) {
	root := t.TempDir()
	writePluginFile(t, root, "plugin.json", `{"$schema":"https://agent-plugins.org/schemas/1.0.0/plugin.schema.json","name":"demo"}`)
	d, err := Discover(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(d.Candidates[0].Targets, []string{"codex", "copilot", "cursor"}) {
		t.Fatalf("silently converted portable plugin: %+v", d.Candidates)
	}
	writePluginFile(t, root, "plugin.json", `{"$schema":"https://antigravity.google/schemas/v1/plugin.json","name":"demo"}`)
	d, err = Discover(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(d.Candidates[0].Targets, []string{"antigravity", "antigravity-cli"}) {
		t.Fatalf("silently converted Antigravity plugin: %+v", d.Candidates)
	}
}

func TestOpenCodeStalePreviewAndSymlink(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("OPENCODE_CONFIG", "")
	t.Setenv("OPENCODE_CONFIG_DIR", "")
	t.Setenv("OPENCODE_CONFIG_CONTENT", "")
	path := filepath.Join(home, ".config/opencode/opencode.json")
	writePluginFile(t, home, ".config/opencode/opencode.json", `{"plugin":["demo"]}`)
	s := &Service{ConfigPath: filepath.Join(home, "config.yaml"), StateDir: filepath.Join(home, "state"), Run: func(_ context.Context, _, _ string, _ ...string) ([]byte, error) { return []byte("1.18.31"), nil }}
	r := Request{Action: "import", From: "opencode", Plugin: "demo"}
	p, err := s.Preview(context.Background(), r)
	if err != nil {
		t.Fatal(err)
	}
	writePluginFile(t, home, ".config/opencode/opencode.json", `{"plugin":["demo"],"model":"external-edit"}`)
	if _, err = s.Apply(context.Background(), r, p.Revision); err == nil {
		t.Fatal("stale native preview accepted")
	}
	original := filepath.Join(home, "elsewhere.json")
	writePluginFile(t, home, "elsewhere.json", `{"plugin":[]}`)
	if err = os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err = os.Symlink(original, path); err != nil {
		t.Fatal(err)
	}
	if h := s.host(context.Background(), "opencode"); h.Error == "" {
		t.Fatal("symlink config accepted")
	}
}

func TestPiRelativePackageIdentity(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("PI_CODING_AGENT_DIR", filepath.Join(home, ".pi/agent"))
	writePluginFile(t, home, ".pi/agent/settings.json", `{"packages":["../../snapshot/demo"]}`)
	s := &Service{}
	items, _, err := s.piInventory()
	if err != nil || len(items) != 1 || items[0].ID != filepath.Join(home, "snapshot/demo") {
		t.Fatalf("relative package identity: %+v %v", items, err)
	}
}

func TestUnavailableNativeClientDoesNotForgetBinding(t *testing.T) {
	home := t.TempDir()
	writePluginFile(t, home, "config.yaml", "plugins:\n  packages:\n    demo:\n      bindings:\n        pi:\n          id: npm:demo\n")
	s := &Service{ConfigPath: filepath.Join(home, "config.yaml"), Run: func(_ context.Context, _, _ string, _ ...string) ([]byte, error) {
		return nil, fmt.Errorf("Pi not installed")
	}}
	p, err := s.Preview(context.Background(), Request{Action: "remove", Name: "demo"})
	if err != nil {
		t.Fatal(err)
	}
	if !p.Blocked || p.Changes[0].Action != "blocked" {
		t.Fatalf("unavailable is not absent: %+v", p)
	}
}

func TestOpenCodeProjectImportedUpdateNeverUsesGlobalCLI(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("OPENCODE_CONFIG", "")
	t.Setenv("OPENCODE_CONFIG_DIR", "")
	t.Setenv("OPENCODE_CONFIG_CONTENT", "")
	project := t.TempDir()
	writePluginFile(t, project, "opencode.json", `{"plugins":["demo"]}`)
	s := &Service{ConfigPath: filepath.Join(project, "config.yaml"), StateDir: filepath.Join(home, "state"), ProjectRoot: project,
		Run: func(_ context.Context, _, _ string, args ...string) ([]byte, error) {
			if strings.Join(args, " ") != "--version" {
				t.Fatalf("unexpected native operation: %v", args)
			}
			return []byte("2.0.0"), nil
		}}
	applyPluginRequest(t, s, Request{Action: "import", From: "opencode", Plugin: "demo"})
	plan, err := s.Preview(context.Background(), Request{Action: "update", Name: "demo"})
	if err != nil || !plan.Blocked {
		t.Fatalf("project update must be blocked: %+v %v", plan, err)
	}
}
