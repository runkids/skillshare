package plugin

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestPiNativeLockImmutableWindowsIdentity(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json.lock")
	lock, err := acquirePiNativeLock(path)
	if err != nil {
		t.Fatal(err)
	}
	released := false
	defer func() {
		if !released {
			lock.release()
		}
	}()
	old, err := openPiLockAnchor(path)
	if err != nil {
		t.Fatal(err)
	}
	defer old.Close()
	if err := os.Rename(path, path+".previous"); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(path, 0o755); err != nil {
		t.Fatal(err)
	}
	// A new owner can have the same timestamp; identity alone must reject it.
	if err := os.Chtimes(path, lock.mtime, lock.mtime); err != nil {
		t.Fatal(err)
	}
	if err := lock.verify(); !errors.Is(err, ErrPiExtensionsBusy) {
		t.Fatalf("replacement accepted with matching mtime: %v", err)
	}
	lock.release() // Verify that release also leaves the replacement untouched.
	released = true
	if _, err := os.Lstat(path); err != nil {
		t.Fatalf("replacement removed: %v", err)
	}
}
