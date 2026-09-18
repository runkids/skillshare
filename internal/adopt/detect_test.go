package adopt

import (
	"os"
	"path/filepath"
	"testing"
)

// mkSkill creates a real skill directory with a SKILL.md file.
func mkSkill(t *testing.T, base, name string) string {
	t.Helper()
	dir := filepath.Join(base, name)
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("---\nname: "+name+"\n---\n# "+name), 0644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestDetectAdoptable_MissingAgentsPath(t *testing.T) {
	tmp := t.TempDir()
	agents := filepath.Join(tmp, "does-not-exist")
	source := filepath.Join(tmp, "source")

	cands, err := DetectAdoptable(agents, source, "merge", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cands) != 0 {
		t.Fatalf("expected empty candidates, got %d", len(cands))
	}
}

func TestDetectAdoptable_RealDirSelected_SymlinkSkipped(t *testing.T) {
	tmp := t.TempDir()
	agents := filepath.Join(tmp, "agents")
	source := filepath.Join(tmp, "source")
	if err := os.MkdirAll(agents, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(source, 0755); err != nil {
		t.Fatal(err)
	}

	// Real local skill dir.
	mkSkill(t, agents, "web-scraper")

	// A symlink that points back to source (synced) must be skipped.
	srcSkill := mkSkill(t, source, "managed")
	if err := os.Symlink(srcSkill, filepath.Join(agents, "managed")); err != nil {
		t.Fatal(err)
	}

	cands, err := DetectAdoptable(agents, source, "merge", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cands) != 1 {
		t.Fatalf("expected 1 candidate, got %d: %+v", len(cands), cands)
	}
	if cands[0].Name != "web-scraper" {
		t.Errorf("candidate name = %q, want web-scraper", cands[0].Name)
	}
	if cands[0].Conflict {
		t.Errorf("expected no conflict for web-scraper")
	}
}

func TestDetectAdoptable_ConflictWhenSameNameInSource(t *testing.T) {
	tmp := t.TempDir()
	agents := filepath.Join(tmp, "agents")
	source := filepath.Join(tmp, "source")
	mkSkill(t, agents, "web-scraper")
	mkSkill(t, source, "web-scraper")

	cands, err := DetectAdoptable(agents, source, "merge", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cands) != 1 {
		t.Fatalf("expected 1 candidate, got %d", len(cands))
	}
	if !cands[0].Conflict {
		t.Errorf("expected conflict=true for web-scraper (exists in source)")
	}
}

func TestDetectAdoptable_ExternalLinksDiscovered(t *testing.T) {
	tmp := t.TempDir()
	agents := filepath.Join(tmp, "agents")
	source := filepath.Join(tmp, "source")
	claude := filepath.Join(tmp, "claude")
	cursor := filepath.Join(tmp, "cursor")
	if err := os.MkdirAll(claude, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(cursor, 0755); err != nil {
		t.Fatal(err)
	}

	skillDir := mkSkill(t, agents, "web-scraper")

	// External tool symlinked the agents skill into claude (orphan symlink).
	if err := os.Symlink(skillDir, filepath.Join(claude, "web-scraper")); err != nil {
		t.Fatal(err)
	}
	// cursor has no link.

	allTargets := map[string]string{
		"universal": agents, // must be skipped
		"agents":    agents, // alias, also skipped
		"claude":    claude,
		"cursor":    cursor,
	}

	cands, err := DetectAdoptable(agents, source, "merge", allTargets)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cands) != 1 {
		t.Fatalf("expected 1 candidate, got %d", len(cands))
	}
	links := cands[0].ExternalLinks
	if len(links) != 1 {
		t.Fatalf("expected 1 external link, got %d: %v", len(links), links)
	}
	if links[0] != filepath.Join(claude, "web-scraper") {
		t.Errorf("external link = %q, want %q", links[0], filepath.Join(claude, "web-scraper"))
	}
}

func TestDetectAdoptable_ExternalLinksMissingTargetDirsOK(t *testing.T) {
	tmp := t.TempDir()
	agents := filepath.Join(tmp, "agents")
	source := filepath.Join(tmp, "source")
	mkSkill(t, agents, "web-scraper")

	allTargets := map[string]string{
		"claude": filepath.Join(tmp, "no-such-claude"),
	}

	cands, err := DetectAdoptable(agents, source, "merge", allTargets)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cands) != 1 {
		t.Fatalf("expected 1 candidate, got %d", len(cands))
	}
	if len(cands[0].ExternalLinks) != 0 {
		t.Errorf("expected no external links when target dir missing, got %v", cands[0].ExternalLinks)
	}
}
