package install

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

const round2ExtrasLockName = ".skillshare-extras.lock"

func TestLockExtrasRoot_InternalPersistentAndNoFollow(t *testing.T) {
	t.Run("baseline", func(t *testing.T) {
		for _, kind := range []string{"regular", "symlink", "directory"} {
			t.Run(kind, func(t *testing.T) {
				base := t.TempDir()
				t.Setenv("XDG_STATE_HOME", filepath.Join(base, "state"))
				root := filepath.Join(base, "extras")
				if err := os.Mkdir(root, 0755); err != nil {
					t.Fatal(err)
				}
				file := filepath.Join(root, round2ExtrasLockName)
				sentinel := filepath.Join(base, "sentinel")
				if err := os.WriteFile(sentinel, []byte("untouched"), 0644); err != nil {
					t.Fatal(err)
				}
				switch kind {
				case "symlink":
					if runtime.GOOS == "windows" {
						t.Skip("requires symlink privilege")
					}
					if err := os.Symlink(sentinel, file); err != nil {
						t.Fatal(err)
					}
				case "directory":
					if err := os.Mkdir(file, 0755); err != nil {
						t.Fatal(err)
					}
				}
				unlock, err := LockExtrasRoot(root)
				if kind != "regular" {
					if err == nil {
						unlock()
						t.Error("accepted nonregular internal lock path")
					}
				} else {
					if err != nil {
						t.Fatal(err)
					}
					before, err := os.Lstat(file)
					unlock()
					if err != nil || !before.Mode().IsRegular() {
						t.Fatalf("no regular root-owned lock: %v %v", before, err)
					}
					after, err := os.Lstat(file)
					if err != nil || !os.SameFile(before, after) {
						t.Errorf("unlock removed/replaced permanent inode: %v", err)
					}
				}
				if data, err := os.ReadFile(sentinel); err != nil || string(data) != "untouched" {
					t.Errorf("followed lock target: %q %v", data, err)
				}
			})
		}
	})
	t.Run("TestLockExtrasRoot_WritableRootWithoutWritableParent", func(t *testing.T) {
		base := t.TempDir()
		t.Setenv("XDG_STATE_HOME", filepath.Join(base, "state"))
		parent := filepath.Join(base, "mount-parent")
		root := filepath.Join(parent, "mounted-extras")
		if err := os.MkdirAll(root, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.Chmod(parent, 0555); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.Chmod(parent, 0755) })
		// Topology and lock placement are asserted even under root. Permission
		// denial itself is exercised only when the runner is a nonroot Unix user.
		if os.Geteuid() == 0 || runtime.GOOS == "windows" {
			t.Log("containment proof only: chmod does not establish parent denial for this runner")
		} else {
			probe := filepath.Join(parent, "denied-sibling")
			if err := os.WriteFile(probe, []byte("probe"), 0600); !os.IsPermission(err) {
				t.Fatalf("fixture must actually deny sibling creation: %v", err)
			}
		}
		unlock, err := LockExtrasRoot(root)
		if err != nil {
			t.Fatal(err)
		}
		unlock()
		if info, err := os.Lstat(filepath.Join(root, round2ExtrasLockName)); err != nil || !info.Mode().IsRegular() {
			t.Fatalf("lock is not inside writable mounted root: %v %v", info, err)
		}
		entries, err := os.ReadDir(parent)
		if err != nil || len(entries) != 1 || entries[0].Name() != "mounted-extras" {
			t.Errorf("parent sidecar created: %+v %v", entries, err)
		}
	})
}
