package mcp

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func testService(t *testing.T) *Service {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte("mcp:\n  targets: [claude, codex, cursor, vscode]\n  servers:\n    docs:\n      url: https://example.com/mcp\n"), 0600); err != nil {
		t.Fatal(err)
	}
	return &Service{ConfigPath: path, Home: dir, StateDir: filepath.Join(dir, "state"), Platform: "linux"}
}

func TestSyncLifecycleAndDrift(t *testing.T) {
	s := testService(t)
	plan, err := s.Preview()
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Changes) != 4 {
		t.Fatalf("got %d changes", len(plan.Changes))
	}
	if _, err := s.Apply(plan.Revision); err != nil {
		t.Fatal(err)
	}
	plan, err = s.Preview()
	if err != nil {
		t.Fatal(err)
	}
	for _, change := range plan.Changes {
		if change.Action != "unchanged" {
			t.Fatalf("not idempotent: %+v", change)
		}
	}
	path := filepath.Join(s.Home, ".claude.json")
	data, _ := os.ReadFile(path)
	if err := os.WriteFile(path, []byte(strings.ReplaceAll(string(data), "https://example.com/mcp", "https://changed.example/mcp")), 0600); err != nil {
		t.Fatal(err)
	}
	plan, err = s.Preview()
	if err != nil {
		t.Fatal(err)
	}
	if !plan.Blocked {
		t.Fatal("local edit must conflict")
	}
	if _, err := s.Apply(plan.Revision); err == nil {
		t.Fatal("must not apply conflicted plan")
	}
}

func TestPreviewRejectsConcurrentChanges(t *testing.T) {
	s := testService(t)
	plan, err := s.Preview()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(s.Home, ".claude.json"), []byte(`{"mcpServers":{"docs":{"command":"mine"}}}`), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Apply(plan.Revision); err == nil {
		t.Fatal("stale preview accepted")
	}
	if _, err := os.Stat(filepath.Join(s.Home, ".codex", "config.toml")); !os.IsNotExist(err) {
		t.Fatal("other target was written")
	}
}

func TestApplyKeepsSettingsWrittenAfterPreview(t *testing.T) {
	s := testService(t)
	plan, err := s.Preview()
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(s.Home, ".claude.json")
	if err := os.WriteFile(path, []byte(`{"theme":"dark"}`), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Apply(plan.Revision); err != nil {
		t.Fatalf("unrelated Agent setting blocked apply: %v", err)
	}
	data, _ := os.ReadFile(path)
	if !strings.Contains(string(data), `"theme"`) || !strings.Contains(string(data), "example.com") {
		t.Fatalf("apply lost the Agent setting or the entry: %s", data)
	}
}

func TestIdenticalUnmanagedEntryConverges(t *testing.T) {
	s := testService(t)
	path := filepath.Join(s.Home, ".claude.json")
	if err := os.WriteFile(path, []byte(`{"mcpServers":{"docs":{"type":"http","url":"https://example.com/mcp"}}}`), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Apply(""); err != nil {
		t.Fatalf("identical entry must not conflict: %v", err)
	}
	if err := os.WriteFile(s.ConfigPath, []byte("mcp:\n  targets: [claude, codex, cursor, vscode]\n  servers: {}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Apply(""); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)
	if !strings.Contains(string(data), "example.com") {
		t.Fatal("removing the server deleted an entry Skillshare never owned")
	}
}

func TestAgentDefaultsAreNotDrift(t *testing.T) {
	s := testService(t)
	if _, err := s.Apply(""); err != nil {
		t.Fatal(err)
	}
	// Agents may rewrite an entry without the default type and with empty fields.
	if err := os.WriteFile(filepath.Join(s.Home, ".claude.json"), []byte(`{"mcpServers":{"docs":{"url":"https://example.com/mcp","headers":{}}}}`), 0600); err != nil {
		t.Fatal(err)
	}
	plan, err := s.Preview()
	if err != nil || plan.Blocked {
		t.Fatalf("equivalent entry reported as drift: %v %+v", err, plan)
	}
}

func TestDisabledEntryIsDrift(t *testing.T) {
	s := testService(t)
	if _, err := s.Apply(""); err != nil {
		t.Fatal(err)
	}
	path := s.ClientPaths()["cursor"]
	data, _ := os.ReadFile(path)
	if err := os.WriteFile(path, []byte(strings.Replace(string(data), `"url"`, `"disabled": true, "url"`, 1)), 0600); err != nil {
		t.Fatal(err)
	}
	plan, err := s.Preview()
	if err != nil || !plan.Blocked {
		t.Fatalf("disabling a managed server was not reported: %v", err)
	}
}

func TestPulledChangeConverges(t *testing.T) {
	s := testService(t)
	if _, err := s.Apply(""); err != nil {
		t.Fatal(err)
	}
	// A teammate's commit changes the source and the Agent file together.
	for _, path := range []string{s.ConfigPath, filepath.Join(s.Home, ".claude.json")} {
		data, _ := os.ReadFile(path)
		if err := os.WriteFile(path, []byte(strings.ReplaceAll(string(data), "example.com", "moved.example")), 0600); err != nil {
			t.Fatal(err)
		}
	}
	plan, err := s.Preview()
	if err != nil || plan.Blocked {
		t.Fatalf("pulled change must not conflict: %v %+v", err, plan)
	}
}

func TestSyncKeepsAgentSpecificFields(t *testing.T) {
	s := testService(t)
	if _, err := s.Apply(""); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(s.Home, ".codex", "config.toml")
	data, _ := os.ReadFile(path)
	if err := os.WriteFile(path, append(data, "startup_timeout_sec = 30\n"...), 0600); err != nil {
		t.Fatal(err)
	}
	config, _ := os.ReadFile(s.ConfigPath)
	if err := os.WriteFile(s.ConfigPath, []byte(strings.ReplaceAll(string(config), "example.com", "moved.example")), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Apply(""); err != nil {
		t.Fatal(err)
	}
	data, _ = os.ReadFile(path)
	if !strings.Contains(string(data), "startup_timeout_sec = 30") || !strings.Contains(string(data), "moved.example") {
		t.Fatalf("Agent-specific field lost or entry not updated: %s", data)
	}
}

func TestSyncLaysOutAnOwnedEntryThatIsStillOnOneLine(t *testing.T) {
	s := testService(t)
	plan, err := s.Preview()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Apply(plan.Revision); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(s.Home, ".cursor", "mcp.json")
	if err := os.WriteFile(path, []byte(`{"mcpServers":{"docs":{"url":"https://example.com/mcp"}}}`), 0600); err != nil {
		t.Fatal(err)
	}
	plan, err = s.Preview()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Apply(plan.Revision); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(path)
	if want := "{\n  \"mcpServers\": {\n    \"docs\": {\n      \"url\": \"https://example.com/mcp\"\n    }\n  }\n}"; string(got) != want { // no newline at the end: the file had none
		t.Fatalf("got %q", got)
	}
	// Laid out once, it is left alone: Sync has nothing more to do.
	plan, err = s.Preview()
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range plan.Changes {
		if c.Action != "unchanged" {
			t.Fatalf("%s %s is %q after the layout", c.Target, c.Name, c.Action)
		}
	}
}
