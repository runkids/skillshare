package projects

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func TestLockConfig_SymlinkSharesLock(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("config symlink fixture needs symlink privilege")
	}
	dir := t.TempDir()
	t.Setenv("XDG_STATE_HOME", filepath.Join(dir, "state"))
	path := filepath.Join(dir, "config.yaml")
	alias := filepath.Join(dir, "alias.yaml")
	if err := os.WriteFile(path, []byte("targets: {}\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(path, alias); err != nil {
		t.Fatal(err)
	}
	release, err := LockConfig(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	if unlock, err := LockConfig(ctx, alias); !errors.Is(err, context.DeadlineExceeded) {
		if unlock != nil {
			unlock()
		}
		t.Fatalf("alias bypassed lock: %v", err)
	}
	release()
	release, err = LockConfig(context.Background(), alias)
	if err != nil {
		t.Fatal(err)
	}
	release()
}

func TestLockConfig_StaysOutOfConfigFolder(t *testing.T) {
	dir := t.TempDir()
	state := filepath.Join(t.TempDir(), "state")
	t.Setenv("XDG_STATE_HOME", state)
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte("targets: {}\n"), 0644); err != nil {
		t.Fatal(err)
	}
	release, err := LockConfig(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	release()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("lock left files beside config: %v", entries)
	}
	locks, _ := filepath.Glob(filepath.Join(state, "skillshare", "projects", "*.lock"))
	if len(locks) != 1 {
		t.Fatalf("expected one lock in state dir, got %v", locks)
	}
}
