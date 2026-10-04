package trash

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"skillshare/internal/sourcefs"
)

func TestRestore_RefusesNestedNameThroughLink(t *testing.T) {
	tmpDir := t.TempDir()
	trashBase := filepath.Join(tmpDir, "trash")
	destDir := filepath.Join(tmpDir, "skills")
	external := filepath.Join(tmpDir, "external")
	os.MkdirAll(destDir, 0755)
	os.MkdirAll(external, 0755)
	if err := os.Symlink(external, filepath.Join(destDir, "org")); err != nil {
		t.Fatal(err)
	}

	srcDir := filepath.Join(tmpDir, "src", "org", "_team-skills")
	os.MkdirAll(srcDir, 0755)
	os.WriteFile(filepath.Join(srcDir, "README.md"), []byte("# Team"), 0644)
	if _, err := MoveToTrash(srcDir, "org/_team-skills", trashBase); err != nil {
		t.Fatal(err)
	}
	entry := FindByName(trashBase, "org/_team-skills")
	if entry == nil {
		t.Fatal("expected to find org/_team-skills in trash")
	}

	if err := Restore(entry, destDir); !errors.Is(err, sourcefs.ErrLink) {
		t.Fatalf("restore below a link: got %v, want ErrLink", err)
	}
	if entries, _ := os.ReadDir(external); len(entries) != 0 {
		t.Fatalf("external tree changed: %v", entries)
	}
	if FindByName(trashBase, "org/_team-skills") == nil {
		t.Fatal("trash entry should stay after a refused restore")
	}
}
