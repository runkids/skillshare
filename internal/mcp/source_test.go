package mcp

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSourceSaveBlockFormatting(t *testing.T) {
	for _, initial := range []string{"# keep\nmode: merge\n", "# keep\nmcp: {servers: {}}\n"} {
		path := filepath.Join(t.TempDir(), "config.yaml")
		if err := os.WriteFile(path, []byte(initial), 0600); err != nil {
			t.Fatal(err)
		}
		source, err := LoadSource(path)
		if err != nil {
			t.Fatal(err)
		}
		source.Servers = map[string]Server{"docs": {URL: "https://example.com/mcp"}}
		if err := source.save(); err != nil {
			t.Fatal(err)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(data), "mcp:\n  servers:\n    docs:\n") || !strings.Contains(string(data), "# keep") {
			t.Fatalf("unexpected formatting: %s", data)
		}
	}
}

func TestSourceSelection(t *testing.T) {
	for _, tc := range []struct {
		name, config, external string
		wantError              bool
	}{
		{"inline", "mcp:\n  targets: [claude]\n  servers:\n    docs:\n      url: https://example.com/mcp\n", "", false},
		{"external", "sources:\n  mcp: ./mcp.yaml\nmcp:\n  targets: [claude]\n", "servers:\n  docs:\n    url: https://example.com/mcp\n", false},
		{"both", "sources:\n  mcp: ./mcp.yaml\nmcp:\n  servers: {}\n", "servers: {}\n", true},
		{"missing", "sources:\n  mcp: ./missing.yaml\n", "", true},
		{"unknown", "mcp:\n  servers:\n    docs:\n      url: https://example.com/mcp\n      oauth: {}\n", "", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "config.yaml")
			if err := os.WriteFile(path, []byte(tc.config), 0600); err != nil {
				t.Fatal(err)
			}
			if tc.external != "" {
				if err := os.WriteFile(filepath.Join(dir, "mcp.yaml"), []byte(tc.external), 0600); err != nil {
					t.Fatal(err)
				}
			}
			source, err := LoadSource(path)
			if (err != nil) != tc.wantError {
				t.Fatalf("LoadSource error = %v", err)
			}
			if !tc.wantError && source.Servers["docs"].URL != "https://example.com/mcp" {
				t.Fatal("missing server")
			}
		})
	}
}

func TestRenderDoesNotResolveSecrets(t *testing.T) {
	t.Setenv("API_TOKEN", "never-write-this")
	server := Server{Command: "tool", Env: map[string]Value{"API_TOKEN": {FromEnv: "API_TOKEN"}}}
	for _, target := range []string{"claude", "codex", "cursor", "vscode"} {
		entry, err := Render(target, server)
		if err != nil {
			t.Fatal(err)
		}
		if entry["command"] != "tool" {
			t.Fatal("missing command")
		}
		if target == "codex" && entry["env"] != nil {
			t.Fatal("Codex must forward variables, not resolve them")
		}
	}
	server.Env = map[string]Value{"API_TOKEN": {FromEnv: "OTHER_TOKEN"}}
	if _, err := Render("codex", server); err == nil {
		t.Fatal("cannot silently rename Codex environment variables")
	}
}
