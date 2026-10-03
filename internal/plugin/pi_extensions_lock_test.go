package plugin

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestPiNativeLockReclaimPreservesRacingOwner(t *testing.T) {
	for _, kind := range []string{"renewed", "replaced", "new content"} {
		t.Run(kind, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "settings.json.lock")
			if err := os.Mkdir(path, 0o755); err != nil {
				t.Fatal(err)
			}
			old := time.Now().Add(-time.Minute)
			if err := os.Chtimes(path, old, old); err != nil {
				t.Fatal(err)
			}
			var expected os.FileInfo
			piBeforeLockReclaim = func(path string) {
				switch kind {
				case "renewed":
					now := time.Now()
					if err := os.Chtimes(path, now, now); err != nil {
						t.Fatal(err)
					}
				case "replaced":
					if err := os.Rename(path, path+".previous"); err != nil {
						t.Fatal(err)
					}
					if err := os.Mkdir(path, 0o755); err != nil {
						t.Fatal(err)
					}
					if err := os.Chtimes(path, old, old); err != nil {
						t.Fatal(err)
					}
				case "new content":
					if err := os.WriteFile(filepath.Join(path, "keep"), []byte("keep"), 0o600); err != nil {
						t.Fatal(err)
					}
					if err := os.Chtimes(path, old, old); err != nil {
						t.Fatal(err)
					}
				}
				expected, _ = os.Lstat(path)
			}
			t.Cleanup(func() { piBeforeLockReclaim = func(string) {} })
			if _, err := acquirePiNativeLock(path); !errors.Is(err, ErrPiExtensionsBusy) {
				t.Fatalf("err=%v", err)
			}
			after, err := os.Lstat(path)
			if err != nil || !os.SameFile(expected, after) || !expected.ModTime().Equal(after.ModTime()) {
				t.Fatalf("racing owner's directory changed: %v", err)
			}
		})
	}
}

func TestPiNativeLockReclaimsOnlyStaleEmptyDirectories(t *testing.T) {
	for _, kind := range []string{"stale", "fresh", "future", "nonempty", "file", "symlink"} {
		t.Run(kind, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "settings.json.lock")
			if kind == "file" {
				if err := os.WriteFile(path, []byte("keep"), 0o600); err != nil {
					t.Fatal(err)
				}
			} else if kind == "symlink" {
				dest := t.TempDir()
				if err := os.Symlink(dest, path); err != nil {
					t.Skipf("symlinks unavailable: %v", err)
				}
			} else if err := os.Mkdir(path, 0o755); err != nil {
				t.Fatal(err)
			}
			if kind == "nonempty" {
				if err := os.WriteFile(filepath.Join(path, "keep"), []byte("keep"), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			stamp := time.Now().Add(-piLockStale - time.Minute)
			if kind == "fresh" {
				stamp = time.Now()
			}
			if kind == "future" {
				stamp = time.Now().Add(time.Hour)
			}
			if kind != "symlink" {
				if err := os.Chtimes(path, stamp, stamp); err != nil {
					t.Fatal(err)
				}
			}
			before, err := os.Lstat(path)
			if err != nil {
				t.Fatal(err)
			}
			// Hold the old inode so removal cannot immediately recycle its identity.
			if kind == "stale" {
				f, err := os.OpenRoot(path)
				if err != nil {
					t.Fatal(err)
				}
				defer f.Close()
			}
			lock, err := acquirePiNativeLock(path)
			if kind == "stale" {
				if err != nil {
					t.Fatalf("stale directory not reclaimed: %v", err)
				}
				defer lock.release()
				if os.SameFile(before, lock.ours) {
					t.Fatal("stale directory was reused")
				}
				if err := lock.verify(); err != nil {
					t.Fatal(err)
				}
				return
			}
			if !errors.Is(err, ErrPiExtensionsBusy) {
				t.Fatalf("err=%v", err)
			}
			after, err := os.Lstat(path)
			if err != nil || !os.SameFile(before, after) || !before.ModTime().Equal(after.ModTime()) {
				t.Fatalf("foreign path changed: %v", err)
			}
		})
	}
}
