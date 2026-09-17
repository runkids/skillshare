package mcp

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAntigravitySyncAndRestore(t *testing.T) {
	for _, project := range []bool{false, true} {
		s := testService(t)
		if project {
			s.ProjectRoot = t.TempDir()
		}
		if err := os.WriteFile(s.ConfigPath, []byte("mcp:\n  targets: [antigravity]\n  servers:\n    docs:\n      url: https://example.com/mcp\n"), 0600); err != nil {
			t.Fatal(err)
		}
		p, err := s.Preview()
		if err != nil {
			t.Fatal(err)
		}
		r, err := s.Apply(p.Revision)
		if err != nil {
			t.Fatal(err)
		}
		path, err := s.nativePath("antigravity")
		if err != nil {
			t.Fatal(err)
		}
		before, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		p, err = s.Preview()
		if err != nil || p.Changes[0].Action != "unchanged" {
			t.Fatalf("idempotency %v %v", p, err)
		}
		if err := os.WriteFile(path, []byte(strings.ReplaceAll(string(before), "example.com", "changed.example.com")), 0600); err != nil {
			t.Fatal(err)
		}
		p, err = s.Preview()
		if err != nil || !p.Blocked {
			t.Fatalf("drift %v %v", p, err)
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
		n, err := ParseNative("antigravity", data)
		if err != nil || len(n.Entries) != 0 {
			t.Fatalf("restore %v %v", n, err)
		}
	}
}

func TestAntigravityAdapter(t *testing.T) {
	s := testService(t)
	path, err := s.nativePath("antigravity")
	if err != nil || path != filepath.Join(s.Home, ".gemini", "config", "mcp_config.json") {
		t.Fatalf("global path %s: %v", path, err)
	}
	s.ProjectRoot = t.TempDir()
	path, err = s.nativePath("antigravity")
	if err != nil || path != filepath.Join(s.ProjectRoot, ".agents", "mcp_config.json") {
		t.Fatalf("project path %s: %v", path, err)
	}
	for _, server := range []Server{{URL: "https://example.com/mcp"}, {Command: "tool", Args: []string{"serve"}}} {
		entry, err := Render("antigravity", server)
		if err != nil {
			t.Fatal(err)
		}
		if entry["type"] != nil || entry["url"] != nil {
			t.Fatalf("unexpected fields: %v", entry)
		}
		if server.URL != "" && entry["serverUrl"] != server.URL {
			t.Fatal("missing serverUrl")
		}
		n, err := ParseNative("antigravity", []byte(`{"mcpServers":{"other":{"command":"other"}},"keep":true}`))
		if err != nil {
			t.Fatal(err)
		}
		data, err := n.Edit(map[string]map[string]any{"docs": entry})
		if err != nil {
			t.Fatal(err)
		}
		items, err := Import("antigravity", data, "")
		if err != nil {
			t.Fatal(err)
		}
		if len(items) != 2 || len(items[0].Problems) > 0 || items[0].Server.URL != server.URL || items[0].Server.Command != server.Command {
			t.Fatalf("roundtrip %+v", items)
		}
	}
}

func TestAntigravityRejectsUnverifiedReferences(t *testing.T) {
	for _, server := range []Server{
		{URL: "https://example.com/mcp", BearerToken: &Value{FromEnv: "TOKEN"}},
		{Command: "tool", Env: map[string]Value{"TOKEN": {FromEnv: "TOKEN"}}},
		{URL: "https://example.com/mcp", Headers: map[string]Value{"X-Key": {FromEnv: "TOKEN"}}},
	} {
		if _, err := Render("antigravity", server); err == nil {
			t.Fatal("unsupported reference accepted")
		}
	}
}

func TestAntigravityPastedRemote(t *testing.T) {
	items, err := Import("", []byte(`{"serverUrl":"https://example.com/mcp"}`), "docs")
	if err != nil || len(items) != 1 || len(items[0].Problems) > 0 || items[0].Server.URL != "https://example.com/mcp" {
		t.Fatalf("paste %+v %v", items, err)
	}
}
