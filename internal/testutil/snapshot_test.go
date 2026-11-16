package testutil

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"skillshare/internal/utils"
)

func TestSnapshotTree_DirectoryLink(t *testing.T) {
	root := t.TempDir()
	source := t.TempDir()
	if err := os.WriteFile(filepath.Join(source, "content.txt"), []byte("external"), 0600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "link")
	if runtime.GOOS == "windows" {
		if err := utils.CreateJunction(link, source); err != nil {
			t.Fatal(err)
		}
	} else if err := os.Symlink(source, link); err != nil {
		t.Fatal(err)
	}
	snapshot := SnapshotTree(t, false, root)
	if len(snapshot) != 2 || !strings.Contains(snapshot[link], source) {
		t.Fatalf("directory link was followed or not recorded: %v", snapshot)
	}
}
