package install

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func TestLockExtrasRoot_SerializesSymlinkAliases(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("requires symlink privilege")
	}
	base := t.TempDir()
	t.Setenv("XDG_STATE_HOME", filepath.Join(base, "state"))
	real := filepath.Join(base, "real")
	if err := os.Mkdir(real, 0755); err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(base, "alias")
	if err := os.Symlink(real, alias); err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(real, "extras")
	other := filepath.Join(alias, "extras")
	first, err := LockExtrasRoot(root)
	if err != nil {
		t.Fatal(err)
	}
	released := false
	defer func() {
		if !released {
			first()
		}
	}()
	waited := make(chan struct{}, 1)
	oldHook, oldWait := extraLockOnWait, checkoutLockWait
	extraLockOnWait = func() { waited <- struct{}{} }
	checkoutLockWait = 5 * time.Second
	defer func() { extraLockOnWait, checkoutLockWait = oldHook, oldWait }()
	done := make(chan error, 1)
	go func() {
		unlock, err := LockExtrasRoot(other)
		if err == nil {
			unlock()
		}
		done <- err
	}()
	select {
	case <-waited:
	case <-time.After(2 * time.Second):
		t.Fatal("second writer did not wait")
	}
	first()
	released = true
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("second writer did not acquire lock")
	}
}

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
