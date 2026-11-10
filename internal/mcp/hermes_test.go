package mcp

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHermesRoundTrip(t *testing.T) {
	for _, server := range []Server{
		{Command: "npx", Args: []string{"-y", "example-mcp"}, Env: map[string]Value{"API_KEY": {FromEnv: "API_KEY"}}},
		{URL: "https://example.com/mcp", BearerToken: &Value{FromEnv: "API_KEY"}},
		{URL: "https://mcp.linear.app/mcp", Options: map[string]map[string]any{"hermes": {"auth": "oauth", "oauth": map[string]any{"client_id": "public-client", "redirect_port": 27890}}}},
	} {
		entry, err := Render("hermes", server)
		if err != nil {
			t.Fatal(err)
		}
		if entry["type"] != nil || entry["transport"] != nil {
			t.Fatalf("Hermes infers transport from command/url: %v", entry)
		}
		n, err := ParseNative("hermes", []byte("# personal settings\nmodel: example\nmcp_servers:\n  personal:\n    command: mine\n"))
		if err != nil {
			t.Fatal(err)
		}
		data, err := n.Edit(map[string]map[string]any{"docs": entry})
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(data), "# personal settings") || !strings.Contains(string(data), "command: mine") {
			t.Fatalf("lost unrelated settings: %s", data)
		}
		if DetectImportFormat(data) != "hermes" {
			t.Fatal("Hermes YAML not detected")
		}
		items, err := Import("hermes", data, "")
		if err != nil || len(items) != 2 || len(items[0].Problems) != 0 {
			t.Fatalf("import: %+v, %v", items, err)
		}
		again, err := Render("hermes", items[0].Server)
		if err != nil || piFieldHash(entry) != piFieldHash(again) {
			t.Fatalf("round trip: %v -> %v, %v", entry, again, err)
		}
	}
}

func TestHermesImportBoundaries(t *testing.T) {
	for _, tc := range []struct {
		entry string
		bad   bool
	}{
		{"url: https://example.com/mcp\n    auth: oauth", false},
		{"command: tool\n    env: {API_KEY: '${env:KEY}'}", false},
		{"url: https://example.com/sse\n    transport: sse", true},
		{"url: https://example.com/mcp\n    transport: unknown", true},
		{"url: https://example.com/mcp\n    enabled: false", true},
		{"url: https://example.com/mcp\n    oauth: {client_secret: private-value}", true},
	} {
		items, err := Import("hermes", []byte("mcp_servers:\n  docs:\n    "+tc.entry+"\n"), "")
		if err != nil || len(items) != 1 || (len(items[0].Problems) > 0) != tc.bad {
			t.Fatalf("%s: %+v, %v", tc.entry, items, err)
		}
	}
	for _, data := range []string{"mcp_servers: {docs: &server {command: tool}, other: *server}", "mcp_servers: {}\nmcp_servers: {}", "mcp_servers: []", "mcp_servers: {}\n---\nmodel: other"} {
		if _, err := ParseNative("hermes", []byte(data)); err == nil {
			t.Fatalf("unsafe YAML accepted: %s", data)
		}
	}
}

func TestHermesPaths(t *testing.T) {
	s := testService(t)
	path, err := s.nativePath("hermes")
	if err != nil || path != filepath.Join(s.Home, ".hermes", "config.yaml") {
		t.Fatalf("default: %s, %v", path, err)
	}
	t.Setenv("HERMES_HOME", filepath.Join(s.Home, "profile"))
	s.ConfigDirs = ConfigDirsFromEnv()
	path, err = s.nativePath("hermes")
	if err != nil || path != filepath.Join(s.Home, "profile", "config.yaml") {
		t.Fatalf("override: %s, %v", path, err)
	}
	s.ConfigDirs["hermes"] = "relative"
	if _, err := s.nativePath("hermes"); err == nil {
		t.Fatal("relative override accepted")
	}
	s.ConfigDirs = map[string]string{"localappdata": filepath.Join(s.Home, "local")}
	s.Platform = "windows"
	path, err = s.nativePath("hermes")
	if err != nil || path != filepath.Join(s.Home, "local", "hermes", "config.yaml") {
		t.Fatalf("Windows default: %s, %v", path, err)
	}
	s.ProjectRoot = t.TempDir()
	if _, err := s.nativePath("hermes"); err == nil {
		t.Fatal("project scope accepted")
	}
}

func TestHermesSyncLifecycle(t *testing.T) {
	s := testService(t)
	if err := os.WriteFile(s.ConfigPath, []byte("mcp:\n  targets: [hermes]\n  servers:\n    docs:\n      url: https://example.com/mcp\n      options:\n        hermes: {auth: oauth}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(s.Home, ".hermes", "config.yaml")
	os.MkdirAll(filepath.Dir(path), 0755)
	before := "# keep me\nmodel: example\nmcp_servers:\n  personal:\n    command: mine\n"
	if err := os.WriteFile(path, []byte(before), 0600); err != nil {
		t.Fatal(err)
	}
	plan, err := s.Preview()
	if err != nil || plan.Blocked {
		t.Fatalf("preview: %+v, %v", plan, err)
	}
	if data, _ := os.ReadFile(path); string(data) != before {
		t.Fatal("preview wrote native config")
	}
	if _, err := s.Apply(plan.Revision); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)
	if !strings.Contains(string(data), "auth: oauth") || !strings.Contains(string(data), "command: mine") || !strings.Contains(string(data), "# keep me") {
		t.Fatalf("sync lost config: %s", data)
	}
	plan, err = s.Preview()
	if err != nil || len(plan.Changes) != 1 || plan.Changes[0].Action != "unchanged" {
		t.Fatalf("idempotence: %+v, %v", plan, err)
	}
	backups, err := s.Backups()
	if err != nil || len(backups) == 0 {
		t.Fatalf("backups: %+v, %v", backups, err)
	}
	if err := os.WriteFile(path, []byte(strings.ReplaceAll(string(data), "example.com/mcp", "changed.example/mcp")), 0600); err != nil {
		t.Fatal(err)
	}
	plan, err = s.Preview()
	if err != nil || !plan.Blocked {
		t.Fatalf("native drift not blocked: %+v, %v", plan, err)
	}
	if _, err := s.Apply(plan.Revision); err == nil {
		t.Fatal("drifted config overwritten")
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Mutate(Mutation{Name: "docs", Remove: true}, "", true); err != nil {
		t.Fatal(err)
	}
	data, _ = os.ReadFile(path)
	if strings.Contains(string(data), "example.com/mcp") || !strings.Contains(string(data), "command: mine") {
		t.Fatalf("removal touched unmanaged config: %s", data)
	}
}
