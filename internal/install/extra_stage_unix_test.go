//go:build linux || darwin

package install

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

func TestStageFetchedTree_RejectsFIFO(t *testing.T) {
	root := t.TempDir()
	if err := syscall.Mkfifo(filepath.Join(root, "pipe"), 0600); err != nil {
		t.Fatal(err)
	}
	dst := filepath.Join(t.TempDir(), "stage")
	if err := stageFetchedTree(root, "", dst); err == nil {
		t.Fatal("accepted FIFO")
	}
	if _, err := os.Stat(dst); !os.IsNotExist(err) {
		t.Fatalf("wrote before validation: %v", err)
	}
}
