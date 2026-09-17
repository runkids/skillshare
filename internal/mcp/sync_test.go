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
	if err := os.WriteFile(filepath.Join(s.Home, ".claude.json"), []byte(`{"personal":true}`), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Apply(plan.Revision); err == nil {
		t.Fatal("stale preview accepted")
	}
	if _, err := os.Stat(filepath.Join(s.Home, ".codex", "config.toml")); !os.IsNotExist(err) {
		t.Fatal("other target was written")
	}
}

func TestUnmanagedIdenticalEntryRequiresAdoption(t *testing.T) {
	s := testService(t)
	if err := os.WriteFile(filepath.Join(s.Home, ".claude.json"), []byte(`{"mcpServers":{"docs":{"type":"http","url":"https://example.com/mcp"}}}`), 0600); err != nil {
		t.Fatal(err)
	}
	plan, err := s.Preview()
	if err != nil {
		t.Fatal(err)
	}
	if !plan.Blocked {
		t.Fatal("identical unmanaged entry must not be silently adopted")
	}
}
