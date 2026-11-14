package install

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestCanonicalExtraRoot_RejectsDanglingLink(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("requires symlink privilege")
	}
	dir := t.TempDir()
	link := filepath.Join(dir, "broken")
	if err := os.Symlink(filepath.Join(dir, "missing"), link); err != nil {
		t.Fatal(err)
	}
	if _, err := canonicalExtraRoot(link); err == nil {
		t.Fatal("accepted dangling root link")
	}
}
