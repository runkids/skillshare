package plugin

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Opt-in because this runs real native CLIs. HOME and all host configuration
// roots are isolated; the fixture contains only a harmless greeting skill.
func TestPluginNativeLifecycle(t *testing.T) {
	if os.Getenv("SKILLSHARE_PLUGIN_NATIVE_E2E") != "1" {
		t.Skip("set SKILLSHARE_PLUGIN_NATIVE_E2E=1 with native CLIs on PATH")
	}
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("XDG_STATE_HOME", filepath.Join(home, ".local/state"))
	t.Setenv("CODEX_HOME", filepath.Join(home, ".codex"))
	t.Setenv("CLAUDE_CONFIG_DIR", filepath.Join(home, ".claude"))
	t.Setenv("CLAUDECODE", "")
	if err := os.MkdirAll(filepath.Join(home, ".codex"), 0700); err != nil {
		t.Fatal(err)
	}
	root := fixture(t)
	s := &Service{ConfigPath: filepath.Join(home, ".config/skillshare/config.yaml"), StateDir: filepath.Join(home, "state")}
	apply := func(r Request) {
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
			t.Fatalf("apply: %+v; %v", result, err)
		}
	}
	apply(Request{Action: "add", Source: root, Targets: []string{"claude", "codex"}})
	apply(Request{Action: "disable", Name: "demo", Targets: []string{"codex"}})
	apply(Request{Action: "sync"})
	apply(Request{Action: "enable", Name: "demo", Targets: []string{"codex"}})
	apply(Request{Action: "sync"})
	// Local source changes are reviewed and installed through Claude's native update.
	for _, manifest := range []string{".claude-plugin/plugin.json", ".codex-plugin/plugin.json"} {
		path := filepath.Join(root, manifest)
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		data = []byte(strings.ReplaceAll(string(data), "1.0.0", "1.0.1"))
		if err := os.WriteFile(path, data, 0644); err != nil {
			t.Fatal(err)
		}
	}
	apply(Request{Action: "update", Name: "demo", Targets: []string{"claude"}})
	apply(Request{Action: "remove", Name: "demo"})
	inventory, err := s.Inventory(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(inventory.Packages) != 0 {
		t.Fatal("packages remain")
	}
	for _, h := range inventory.Hosts {
		if h.Target != "claude" && h.Target != "codex" {
			continue
		}
		if h.Error != "" || len(h.Installed) != 0 {
			t.Fatalf("native cleanup: %+v", h)
		}
	}
	project := filepath.Join(home, "project")
	if err := os.MkdirAll(project, 0755); err != nil {
		t.Fatal(err)
	}
	s = &Service{ConfigPath: filepath.Join(project, ".skillshare/config.yaml"), StateDir: filepath.Join(home, "state"), ProjectRoot: project}
	apply(Request{Action: "add", Source: root, Targets: []string{"claude"}})
	apply(Request{Action: "remove", Name: "demo"})
}

// No model calls or user profiles: Pi receives a harmless skill package, and
// OpenCode only registers a self-contained module in an isolated config.
func TestAdditionalNativeLifecycle(t *testing.T) {
	if os.Getenv("SKILLSHARE_PLUGIN_EXTRA_E2E") != "1" {
		t.Skip("set SKILLSHARE_PLUGIN_EXTRA_E2E=1 with pi and opencode on PATH")
	}
	for _, target := range []string{"pi", "opencode"} {
		t.Run(target, func(t *testing.T) {
			home := t.TempDir()
			t.Setenv("HOME", home)
			t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
			t.Setenv("PI_CODING_AGENT_DIR", filepath.Join(home, ".pi/agent"))
			t.Setenv("OPENCODE_CONFIG", "")
			t.Setenv("OPENCODE_CONFIG_DIR", "")
			t.Setenv("OPENCODE_CONFIG_CONTENT", "")
			root := t.TempDir()
			manifest := `{"name":"ss-demo","version":"1.0.0","pi":{"skills":["skills"]}}`
			if target == "opencode" {
				manifest = `{"name":"ss-demo","version":"1.0.0","main":"index.js","devDependencies":{"@opencode-ai/plugin":"1"}}`
			}
			writePluginFile(t, root, "package.json", manifest)
			writePluginFile(t, root, "index.js", "export default async () => ({})")
			writePluginFile(t, root, "skills/hello/SKILL.md", "---\nname: hello\ndescription: Greet\n---\nSay hello.")
			s := &Service{ConfigPath: filepath.Join(home, "config.yaml"), StateDir: filepath.Join(home, "state")}
			applyPluginRequest(t, s, Request{Action: "add", Source: root, Targets: []string{target}})
			applyPluginRequest(t, s, Request{Action: "disable", Name: "ss-demo"})
			applyPluginRequest(t, s, Request{Action: "sync"})
			applyPluginRequest(t, s, Request{Action: "enable", Name: "ss-demo"})
			applyPluginRequest(t, s, Request{Action: "sync"})
			writePluginFile(t, root, "skills/hello/SKILL.md", "---\nname: hello\ndescription: Greet\n---\nSay hello again.")
			applyPluginRequest(t, s, Request{Action: "update", Name: "ss-demo"})
			applyPluginRequest(t, s, Request{Action: "remove", Name: "ss-demo"})
			h := s.host(context.Background(), target)
			if h.Error != "" || len(h.Installed) != 0 {
				t.Fatalf("cleanup: %+v", h)
			}
		})
	}
}
