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

func antigravityCatalogFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	writeFile(t, root, ".claude-plugin/marketplace.json", `{"name":"claude-market","plugins":[{"name":"demo","source":"./plugins/claude-code"}]}`)
	writeFile(t, root, ".antigravity-plugin/marketplace.json", `{"name":"agy-market","plugins":[{"name":"demo","source":"./plugins/antigravity"}]}`)
	writeFile(t, root, "plugins/claude-code/.claude-plugin/plugin.json", `{"name":"demo","version":"0.1.0"}`)
	writeFile(t, root, "plugins/claude-code/hooks/hooks.json", `{}`)
	writeFile(t, root, "plugins/antigravity/.antigravity-plugin/plugin.json", `{"name":"demo","version":"2.0.0"}`)
	writeFile(t, root, "plugins/antigravity/hooks.json", `{}`)
	writeFile(t, root, "plugins/antigravity/skills/hello/SKILL.md", "---\nname: hello\n---\n")
	return root
}

func TestAntigravityManifestDiscoveryAndPrecedence(t *testing.T) {
	for _, manifest := range []string{".antigravity-plugin/plugin.json", ".gemini-plugin/plugin.json", "plugin.json"} {
		t.Run(manifest, func(t *testing.T) {
			root := t.TempDir()
			writeFile(t, root, manifest, `{"name":"demo","version":"2.0.0"}`)
			writeFile(t, root, "hooks.json", `{}`)
			d, err := Discover(context.Background(), root)
			if err != nil {
				t.Fatal(err)
			}
			if len(d.Candidates) != 1 {
				t.Fatalf("candidates: %+v", d.Candidates)
			}
			c := d.Candidates[0]
			if !slices.Equal(c.Targets, []string{"antigravity", "antigravity-cli"}) {
				t.Fatalf("unexpected targets: %+v", c.Targets)
			}
			for _, target := range []string{"antigravity", "antigravity-cli"} {
				info := c.TargetInfo[target]
				if info.Manifest != manifest || info.Version != "2.0.0" || !slices.Equal(info.Components, []string{"hooks"}) {
					t.Fatalf("%s target info: %+v", target, info)
				}
			}
		})
	}

	t.Run("precedence over root plugin.json", func(t *testing.T) {
		root := t.TempDir()
		writeFile(t, root, ".antigravity-plugin/plugin.json", `{"name":"demo","version":"2.0.0"}`)
		writeFile(t, root, ".gemini-plugin/plugin.json", `{"name":"demo","version":"1.5.0"}`)
		writeFile(t, root, "plugin.json", `{"name":"demo","version":"1.0.0"}`)
		d, err := Discover(context.Background(), root)
		if err != nil {
			t.Fatal(err)
		}
		info := d.Candidates[0].TargetInfo["antigravity"]
		if info.Manifest != ".antigravity-plugin/plugin.json" || info.Version != "2.0.0" {
			t.Fatalf("expected .antigravity-plugin/plugin.json to win, got %+v", info)
		}
	})

	t.Run("invalid native manifest does not fall back to claude", func(t *testing.T) {
		root := t.TempDir()
		writeFile(t, root, ".antigravity-plugin/plugin.json", `{`)
		writeFile(t, root, ".claude-plugin/plugin.json", `{"name":"demo","version":"1.0.0"}`)
		d, err := Discover(context.Background(), root)
		if err != nil {
			t.Fatal(err)
		}
		c := d.Candidates[0]
		if slices.Contains(c.Targets, "antigravity") || slices.Contains(c.Targets, "antigravity-cli") {
			t.Fatalf("invalid native manifest fell back to claude: %+v", c)
		}
		if !slices.Contains(c.Targets, "claude") {
			t.Fatalf("claude should remain installable: %+v", c)
		}
	})
}

func TestAntigravityDiscoverUsesItsOwnCatalogPackage(t *testing.T) {
	root := antigravityCatalogFixture(t)
	d, err := Discover(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if len(d.Candidates) != 1 {
		t.Fatalf("candidates: %+v", d.Candidates)
	}
	c := d.Candidates[0]
	for _, target := range []string{"antigravity", "antigravity-cli"} {
		info := c.TargetInfo[target]
		if c.Problem != "" || !slices.Contains(c.Targets, target) || c.pathFor(target) != "plugins/antigravity" || info.Manifest != ".antigravity-plugin/plugin.json" || info.Version != "2.0.0" || !slices.Equal(info.Components, []string{"hooks", "skills"}) {
			t.Fatalf("%s did not select native Antigravity package: %+v", target, c)
		}
	}
	if c.pathFor("claude") != "plugins/claude-code" || c.TargetInfo["claude"].Version != "0.1.0" || !slices.Equal(c.TargetInfo["claude"].Components, []string{"hooks"}) {
		t.Fatalf("Claude package changed: %+v", c)
	}
}

func TestAntigravityCatalogPreventsClaudeFallback(t *testing.T) {
	for _, catalog := range []string{
		`{"name":"agy-market","plugins":[]}`,
		`{"name":"agy-market","plugins":[{"name":"demo","source":"./missing"}]}`,
		`{"name":"agy-market","plugins":[{"name":"demo","source":"https://example.com/demo.git"}]}`,
		`{"name":"agy-market","plugins":[{"name":"demo","source":"../outside"}]}`,
		`{`,
	} {
		t.Run(catalog, func(t *testing.T) {
			root := antigravityCatalogFixture(t)
			writeFile(t, root, ".antigravity-plugin/marketplace.json", catalog)
			d, err := Discover(context.Background(), root)
			if err != nil {
				t.Fatal(err)
			}
			c := d.Candidates[0]
			if slices.Contains(c.Targets, "antigravity") || slices.Contains(c.Targets, "antigravity-cli") || !slices.Contains(c.Targets, "claude") || c.Problem != "" {
				t.Fatalf("broken/missing native entry fell back to Claude: %+v", c)
			}
		})
	}
}

func TestAntigravityCatalogRejectsUnsupportedNativePackage(t *testing.T) {
	root := antigravityCatalogFixture(t)
	if err := os.Remove(filepath.Join(root, "plugins/antigravity/.antigravity-plugin/plugin.json")); err != nil {
		t.Fatal(err)
	}
	writeFile(t, root, "plugins/antigravity/package.json", `{"name":"demo","pi":{"skills":["./skills"]}}`)
	d, err := Discover(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	c := d.Candidates[0]
	if slices.Contains(c.Targets, "antigravity") || slices.Contains(c.Targets, "antigravity-cli") || !slices.Contains(c.Targets, "claude") {
		t.Fatalf("unsupported native package inherited Claude Antigravity metadata: %+v", c)
	}
}

func TestAntigravityCLIFallsBackToClaudeWithoutNativeCatalog(t *testing.T) {
	root := antigravityCatalogFixture(t)
	if err := os.Remove(filepath.Join(root, ".antigravity-plugin/marketplace.json")); err != nil {
		t.Fatal(err)
	}
	d, err := Discover(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	c := d.Candidates[0]
	if !slices.Contains(c.Targets, "antigravity-cli") || c.pathFor("antigravity-cli") != "plugins/claude-code" {
		t.Fatalf("expected Claude fallback when no native catalog exists: %+v", c)
	}
}

func TestAntigravityCatalogInstallMaterializesNativePath(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	root := antigravityCatalogFixture(t)
	s := &Service{ConfigPath: filepath.Join(home, "config.yaml"), StateDir: filepath.Join(home, "state")}
	var installedPath string
	s.Run = func(_ context.Context, _ string, _ []string, _ string, args ...string) ([]byte, error) {
		command := strings.Join(args, " ")
		switch {
		case command == "--version":
			return []byte("1.0.0"), nil
		case strings.Contains(command, "--help"):
			return []byte("Usage: agy plugin install uninstall list"), nil
		case command == "plugin list --json":
			if installedPath == "" {
				return []byte(`[]`), nil
			}
			return json.Marshal([]map[string]any{{"name": "demo", "enabled": true, "version": "2.0.0"}})
		case len(args) >= 3 && args[1] == "install":
			installedPath = args[2]
			return nil, nil
		default:
			return nil, fmt.Errorf("unexpected command: %s", command)
		}
	}

	applyPluginRequest(t, s, Request{Action: "add", Source: root, Plugin: "demo", Targets: []string{"antigravity-cli"}})
	if !strings.HasSuffix(filepath.ToSlash(installedPath), "/plugins/antigravity") {
		t.Fatalf("antigravity-cli installed wrong path: %q", installedPath)
	}

	applyPluginRequest(t, s, Request{Action: "add", Source: root, Plugin: "demo", Targets: []string{"antigravity"}})
	items, _, err := s.localInventory("antigravity")
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].Name != "demo" || items[0].Version != "2.0.0" {
		t.Fatalf("localInventory did not recognize .antigravity-plugin/plugin.json: %+v", items)
	}
}
