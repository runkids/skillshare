package filepreview

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// A config folder kept in a dotfiles repo is often a symlink; its files must still list.
func TestListFollowsSymlinkedDir(t *testing.T) {
	base := t.TempDir()
	real := filepath.Join(base, "real")
	if err := os.MkdirAll(filepath.Join(real, "ext"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(real, "ext", "run.sh"), []byte("echo"), 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(base, "link")
	if err := os.Symlink(real, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	files, err := List(link)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(files, []string{"ext/run.sh"}) {
		t.Fatalf("List(link) = %q, want [ext/run.sh]", files)
	}
}
