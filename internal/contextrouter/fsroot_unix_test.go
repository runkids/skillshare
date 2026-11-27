//go:build !windows

package contextrouter

import (
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

func TestReadRegularFIFO(t *testing.T) {
	dir := t.TempDir()
	if err := syscall.Mkfifo(filepath.Join(dir, "pipe.md"), 0o644); err != nil {
		t.Skipf("mkfifo: %v", err)
	}
	root := openRoot(t, dir)
	done := make(chan string, 1)
	go func() {
		_, code := readRegular(root, "pipe.md", 10)
		done <- code
	}()
	select {
	case code := <-done:
		if code != ErrNotFile {
			t.Fatalf("got %q, want %q", code, ErrNotFile)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("opening a FIFO blocked")
	}
}
