package adopt

import (
	"os"
	"path/filepath"
	"testing"
)

func writeLock(t *testing.T, dir, content string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, lockFileName), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

func TestReadLock_Missing(t *testing.T) {
	dir := t.TempDir()
	entries, err := ReadLock(dir)
	if err != nil {
		t.Fatalf("expected nil error for missing lockfile, got %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("expected empty map, got %d entries", len(entries))
	}
}

func TestReadLock_NestedSkillsObject(t *testing.T) {
	dir := t.TempDir()
	writeLock(t, dir, `{
		"skills": {
			"web-scraper": {"source": "firecrawl", "version": "1.0.0"},
			"gmail": {"sourceTool": "googleworkspace"}
		}
	}`)

	entries, err := ReadLock(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	if entries["web-scraper"].SourceTool != "firecrawl" {
		t.Errorf("web-scraper source tool = %q, want firecrawl", entries["web-scraper"].SourceTool)
	}
	if entries["web-scraper"].Name != "web-scraper" {
		t.Errorf("web-scraper name = %q, want web-scraper", entries["web-scraper"].Name)
	}
	if entries["gmail"].SourceTool != "googleworkspace" {
		t.Errorf("gmail source tool = %q, want googleworkspace", entries["gmail"].SourceTool)
	}
}

func TestReadLock_FlatMap(t *testing.T) {
	dir := t.TempDir()
	writeLock(t, dir, `{
		"web-scraper": {"tool": "firecrawl"},
		"gmail": {"source": "googleworkspace"}
	}`)

	entries, err := ReadLock(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	if entries["web-scraper"].SourceTool != "firecrawl" {
		t.Errorf("web-scraper source tool = %q, want firecrawl", entries["web-scraper"].SourceTool)
	}
}

func TestReadLock_Malformed(t *testing.T) {
	dir := t.TempDir()
	writeLock(t, dir, `{not valid json`)

	entries, err := ReadLock(dir)
	if err == nil {
		t.Fatalf("expected non-fatal error for malformed JSON")
	}
	if entries == nil {
		t.Fatalf("expected non-nil (empty) map even on malformed JSON")
	}
	if len(entries) != 0 {
		t.Fatalf("expected empty map on malformed JSON, got %d entries", len(entries))
	}
}

func TestProvenance(t *testing.T) {
	entries := map[string]LockEntry{
		"web-scraper": {Name: "web-scraper", SourceTool: "firecrawl"},
	}
	if got := Provenance(entries, "web-scraper"); got != "firecrawl" {
		t.Errorf("Provenance = %q, want firecrawl", got)
	}
	if got := Provenance(entries, "unknown"); got != "" {
		t.Errorf("Provenance for unknown = %q, want empty", got)
	}
}
