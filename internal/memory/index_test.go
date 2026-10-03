package memory

import (
	"errors"
	"strings"
	"testing"
)

func TestIndexAssistancePreservesCustomContentAndChecksVersion(t *testing.T) {
	root := t.TempDir()
	index, err := Write(root, "INDEX.md", "# My custom index\n\nKeep this exact text.\n\n[Missing](wiki/missing.md)\n\n```md\n[Example](fake.md)\n```\n", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Write(root, "wiki/my note.md", "# Storage [decision]\n", ""); err != nil {
		t.Fatal(err)
	}
	status := InspectIndex(root, mustList(t, root))
	if len(status.Unindexed) != 1 || status.Unindexed[0] != "wiki/my note.md" || len(status.BrokenLinks) != 1 || status.BrokenLinks[0] != "wiki/missing.md" {
		t.Fatalf("index status: %+v", status)
	}
	if _, err := LinkFromIndex(root, "wiki/my note.md", "stale"); !errors.Is(err, ErrConflict) {
		t.Fatalf("stale index: %v", err)
	}
	updated, err := LinkFromIndex(root, "wiki/my note.md", index.Version)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(updated.Content, index.Content) || !strings.Contains(updated.Content, "wiki/my%20note.md") {
		t.Fatalf("custom index changed: %s", updated.Content)
	}
	status = InspectIndex(root, mustList(t, root))
	if len(status.Unindexed) != 0 {
		t.Fatalf("link not recognized: %+v", status)
	}
	unchanged, err := LinkFromIndex(root, "wiki/my note.md", updated.Version)
	if err != nil || unchanged.Content != updated.Content {
		t.Fatalf("duplicate link: %+v %v", unchanged, err)
	}
}

func mustList(t *testing.T, root string) []Note {
	t.Helper()
	notes, err := List(root, "")
	if err != nil {
		t.Fatal(err)
	}
	return notes
}
