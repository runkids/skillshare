package mcp

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var expandedClients = []string{"amp", "claude-desktop", "cline", "copilot", "factory", "gemini", "goose", "junie", "kiro", "lmstudio", "warp", "windsurf"}

func TestExpandedScopeProtection(t *testing.T) {
	s := testService(t)
	s.Platform = "linux"
	if s.ClientPaths()["claude-desktop"] != "" {
		t.Fatal("unsupported Desktop platform exposed")
	}
	s.Platform = "windows"
	s.ConfigDirs = map[string]string{"appdata": filepath.Join(s.Home, "roaming"), "copilot": filepath.Join(s.Home, "copilot-custom")}
	for target, relative := range map[string]string{"claude-desktop": "roaming/Claude/claude_desktop_config.json", "goose": "roaming/Block/goose/config/config.yaml", "copilot": "copilot-custom/mcp-config.json"} {
		path, err := s.nativePath(target)
		if err != nil || path != filepath.Join(s.Home, relative) {
			t.Fatalf("%s: %s %v", target, path, err)
		}
	}
	s.ProjectRoot = t.TempDir()
	config := "mcp:\n  targets: [claude, copilot]\n  servers:\n    docs:\n      url: https://example.com/mcp\n"
	if err := os.WriteFile(s.ConfigPath, []byte(config), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Preview(); err == nil {
		t.Fatal("overlapping precedence should fail before either client writes")
	}
	if err := os.WriteFile(filepath.Join(s.ProjectRoot, ".mcp.json"), []byte("{}"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := s.nativePath("copilot"); err == nil {
		t.Fatal("shadowed Copilot file accepted")
	}
}

func TestExpandedNativeDialects(t *testing.T) {
	for _, tc := range []struct{ target, key, kind string }{
		{"amp", "url", ""}, {"cline", "url", "streamableHttp"}, {"copilot", "url", "http"},
		{"factory", "url", "http"}, {"gemini", "httpUrl", ""}, {"goose", "uri", "streamable_http"},
		{"junie", "url", ""}, {"kiro", "url", ""}, {"lmstudio", "url", ""}, {"warp", "url", ""}, {"windsurf", "serverUrl", ""},
	} {
		entry, err := Render(tc.target, Server{URL: "https://example.com/mcp"})
		if err != nil || entry[tc.key] != "https://example.com/mcp" {
			t.Fatalf("%s: %v %v", tc.target, entry, err)
		}
		if tc.kind == "" && entry["type"] != nil || tc.kind != "" && entry["type"] != tc.kind {
			t.Errorf("%s type: %v", tc.target, entry)
		}
		if tc.target == "amp" && nativeKey(tc.target) != "amp.mcpServers" {
			t.Fatal("Amp must use a literal dotted key")
		}
	}
	for _, target := range []string{"claude-desktop", "goose", "junie", "lmstudio", "warp"} {
		if _, err := Render(target, Server{Command: "tool", Env: map[string]Value{"KEY": {FromEnv: "KEY"}}}); err == nil {
			t.Errorf("unverified interpolation: %s", target)
		}
	}
	for target, reference := range map[string]string{"amp": "${KEY}", "cline": "${env:KEY}", "copilot": "${KEY}", "factory": "${KEY}", "gemini": "${KEY}", "kiro": "${KEY}", "windsurf": "${env:KEY}"} {
		entry, err := Render(target, Server{URL: "https://example.com/mcp", BearerToken: &Value{FromEnv: "KEY"}})
		if err != nil || entry["headers"].(map[string]string)["Authorization"] != "Bearer "+reference {
			t.Errorf("%s: %v %v", target, entry, err)
		}
	}
	items, err := Import("gemini", []byte(`{"mcpServers":{"sse":{"url":"https://example.com/sse"}}}`), "")
	if err != nil || len(items[0].Problems) == 0 {
		t.Fatal("Gemini SSE imported as HTTP")
	}
	items, err = Import("", []byte(`{"httpUrl":"https://example.com/mcp"}`), "docs")
	if err != nil || len(items[0].Problems) != 0 || items[0].Server.URL == "" {
		t.Fatalf("Gemini paste: %+v %v", items, err)
	}
}

func TestExpandedClientRoundTrip(t *testing.T) {
	for _, target := range expandedClients {
		t.Run(target, func(t *testing.T) {
			for _, server := range []Server{{Command: "npx", Args: []string{"-y", "@playwright/mcp@latest"}}, {URL: "https://example.com/mcp"}} {
				entry, err := Render(target, server)
				if target == "claude-desktop" && server.URL != "" {
					if err == nil {
						t.Fatal("Desktop remote file config accepted")
					}
					continue
				}
				if err != nil {
					t.Fatal(err)
				}
				n, err := ParseNative(target, nil)
				if err != nil {
					t.Fatal(err)
				}
				data, err := n.Edit(map[string]map[string]any{"browser": entry})
				if err != nil {
					t.Fatal(err)
				}
				items, err := Import(target, data, "")
				if err != nil || len(items) != 1 || len(items[0].Problems) != 0 || items[0].Server.Command != server.Command || items[0].Server.URL != server.URL {
					t.Fatalf("roundtrip %s: %+v %v", data, items, err)
				}
				if len(managedEntry(target, entry)) == 0 {
					t.Fatal("missing ownership fields")
				}
			}
		})
	}
}

func TestExpandedClientLifecycle(t *testing.T) {
	for _, target := range expandedClients {
		t.Run(target, func(t *testing.T) {
			s := testService(t)
			s.Platform = "darwin"
			config := "mcp:\n  targets: [" + target + "]\n  servers:\n    browser:\n      command: npx\n      args: [-y, '@playwright/mcp@latest']\n"
			if err := os.WriteFile(s.ConfigPath, []byte(config), 0600); err != nil {
				t.Fatal(err)
			}
			p, err := s.Preview()
			if err != nil || p.Blocked {
				t.Fatalf("preview %+v %v", p, err)
			}
			r, err := s.Apply(p.Revision)
			if err != nil {
				t.Fatal(err)
			}
			p, err = s.Preview()
			if err != nil || len(p.Changes) != 1 || p.Changes[0].Action != "unchanged" {
				t.Fatalf("idempotence %+v %v", p, err)
			}
			path, err := s.nativePath(target)
			if err != nil {
				t.Fatal(err)
			}
			before, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if target == "goose" && !strings.Contains(string(before), "name: browser") {
				t.Fatalf("Goose name: %s", before)
			}
			if err := os.WriteFile(path, []byte(strings.ReplaceAll(string(before), "npx", "changed")), 0600); err != nil {
				t.Fatal(err)
			}
			p, err = s.Preview()
			if err != nil || !p.Blocked {
				t.Fatalf("drift %+v %v", p, err)
			}
			if err := os.WriteFile(path, before, 0600); err != nil {
				t.Fatal(err)
			}
			p, err = s.PreviewRestore(r.BackupIDs[0])
			if err != nil {
				t.Fatal(err)
			}
			if _, err = s.Restore(r.BackupIDs[0], p.Revision); err != nil {
				t.Fatal(err)
			}
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			n, err := ParseNative(target, data)
			if err != nil || len(n.Entries) != 0 {
				t.Fatalf("restore %+v %v", n, err)
			}
		})
	}
}

func TestExpandedClientPaths(t *testing.T) {
	s := testService(t)
	s.Platform = "darwin"
	want := map[string]string{
		"amp": ".config/amp/settings.json", "claude-desktop": "Library/Application Support/Claude/claude_desktop_config.json",
		"cline":   "Library/Application Support/Code/User/globalStorage/saoudrizwan.claude-dev/settings/cline_mcp_settings.json",
		"copilot": ".copilot/mcp-config.json", "factory": ".factory/mcp.json", "gemini": ".gemini/settings.json",
		"goose": ".config/goose/config.yaml", "junie": ".junie/mcp/mcp.json", "kiro": ".kiro/settings/mcp.json",
		"lmstudio": ".lmstudio/mcp.json", "warp": ".warp/.mcp.json", "windsurf": ".codeium/windsurf/mcp_config.json",
	}
	for target, relative := range want {
		path, err := s.nativePath(target)
		if err != nil || path != filepath.Join(s.Home, relative) {
			t.Errorf("%s: %s %v", target, path, err)
		}
	}
	s.ProjectRoot = t.TempDir()
	for _, target := range []string{"claude-desktop", "cline", "goose", "lmstudio", "windsurf"} {
		if _, err := s.nativePath(target); err == nil {
			t.Errorf("unsupported project path: %s", target)
		}
		if s.ClientPaths()[target] != "" {
			t.Errorf("unavailable client shown: %s", target)
		}
	}
	for target, relative := range map[string]string{"amp": ".amp/settings.json", "copilot": ".github/mcp.json", "factory": ".factory/mcp.json", "gemini": ".gemini/settings.json", "junie": ".junie/mcp/mcp.json", "kiro": ".kiro/settings/mcp.json", "warp": ".warp/.mcp.json"} {
		path, err := s.nativePath(target)
		if err != nil || path != filepath.Join(s.ProjectRoot, relative) {
			t.Errorf("project %s: %s %v", target, path, err)
		}
	}
}

func TestGoosePreservesUnrelatedYAML(t *testing.T) {
	data := []byte("# user settings\nGOOSE_MODEL: test\nextensions:\n  developer:\n    type: builtin\n    name: developer\n    enabled: true\n")
	n, err := ParseNative("goose", data)
	if err != nil {
		t.Fatal(err)
	}
	entry, err := Render("goose", Server{Command: "npx"})
	if err != nil {
		t.Fatal(err)
	}
	updated, err := n.Edit(map[string]map[string]any{"browser": entry})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(updated), "# user settings") || !strings.Contains(string(updated), "GOOSE_MODEL: test") {
		t.Fatalf("lost user config: %s", updated)
	}
	n, err = ParseNative("goose", updated)
	if err != nil || n.Entries["developer"]["type"] != "builtin" {
		t.Fatalf("lost builtin: %s %v", updated, err)
	}
	for _, bad := range []string{"extensions: {}\nextensions: {}", "extensions: {}\n---\nother: true", "extensions: []"} {
		if _, err := ParseNative("goose", []byte(bad)); err == nil {
			t.Errorf("invalid YAML accepted: %s", bad)
		}
	}
}
