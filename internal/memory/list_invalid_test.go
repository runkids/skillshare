package memory

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestListKeepsValidNotesBesideInvalidExternalFiles(t *testing.T) {
	root := t.TempDir()
	for name, content := range map[string][]byte{
		"valid.md":  []byte("# Valid\nFind this note."),
		"large.md":  []byte(strings.Repeat("x", MaxNoteBytes+1)),
		"binary.md": {0xff, 0xfe},
	} {
		if err := os.WriteFile(filepath.Join(root, name), content, 0644); err != nil {
			t.Fatal(err)
		}
	}
	notes, err := List(root, "")
	if err != nil || len(notes) != 3 {
		t.Fatalf("expected all files with valid notes still available: %v, %v", notes, err)
	}
	found, err := List(root, "find this")
	if err != nil || len(found) != 1 || found[0].Path != "valid.md" {
		t.Fatalf("valid note search: %+v, %v", found, err)
	}
	for _, path := range []string{"large.md", "binary.md"} {
		if _, err := Read(root, path); err == nil {
			t.Fatalf("invalid note remains unreadable: %s", path)
		}
	}
}
