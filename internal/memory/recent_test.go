package memory

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestRecentNotesCountsVaultAndLimitsContentCandidates(t *testing.T) {
	root := t.TempDir()
	if err := Init(root); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 12; i++ {
		path := filepath.Join(root, fmt.Sprintf("note-%02d.md", i))
		if err := os.WriteFile(path, []byte(fmt.Sprintf("# Note %d\n", i)), 0644); err != nil {
			t.Fatal(err)
		}
		modified := time.Date(2026, 10, 12, 0, i, 0, 0, time.UTC)
		if err := os.Chtimes(path, modified, modified); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, ".hidden.md"), []byte("# Hidden"), 0644); err != nil {
		t.Fatal(err)
	}
	reads := []string{}
	result, err := recent(root, func(root, path string) (Note, error) {
		reads = append(reads, path)
		return Read(root, path)
	})
	if err != nil || !result.Initialized || result.Count != 14 || len(result.Notes) != 4 {
		t.Fatalf("recent: %+v, %v", result, err)
	}
	for i, note := range result.Notes {
		if note.Path != fmt.Sprintf("note-%02d.md", 11-i) || note.Title != fmt.Sprintf("Note %d", 11-i) || note.Content != "" || note.ModifiedAt == "" || note.Starter {
			t.Fatalf("note %d: %+v", i, note)
		}
	}
	if len(reads) != 6 {
		t.Fatalf("expected only two starters and four candidates to be read: %v", reads)
	}
}

func TestRecentNotesStarterAndIncompleteStates(t *testing.T) {
	root := t.TempDir()
	empty, err := Recent(root)
	if err != nil || empty.Initialized || empty.Count != 0 || len(empty.Notes) != 0 {
		t.Fatalf("empty: %+v, %v", empty, err)
	}
	if err := Init(root); err != nil {
		t.Fatal(err)
	}
	starter, err := Recent(root)
	if err != nil || !starter.Initialized || starter.Count != 2 || len(starter.Notes) != 2 || !starter.Notes[0].Starter || !starter.Notes[1].Starter {
		t.Fatalf("starters: %+v, %v", starter, err)
	}
	if err := os.Remove(filepath.Join(root, "LEARNED.md")); err != nil {
		t.Fatal(err)
	}
	partialStarter, err := Recent(root)
	if err != nil || partialStarter.Initialized || partialStarter.Count != 1 || len(partialStarter.Notes) != 1 || !partialStarter.Notes[0].Starter {
		t.Fatalf("partial starter: %+v, %v", partialStarter, err)
	}
	if _, err := Write(root, "INDEX.md", "# Edited index\n", starter.Notes[0].Version); err != nil {
		t.Fatal(err)
	}
	incomplete, err := Recent(root)
	if err != nil || incomplete.Initialized || incomplete.Count != 1 || len(incomplete.Notes) != 1 || incomplete.Notes[0].Starter {
		t.Fatalf("incomplete: %+v, %v", incomplete, err)
	}
}
