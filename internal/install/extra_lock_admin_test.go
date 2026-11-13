package install

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

const round2ExtrasLockName = ".skillshare-extras.lock"

func TestLockExtrasRoot_InternalPersistentAndNoFollow(t *testing.T) {
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
}

func TestLockExtrasRoot_WritableRootWithoutWritableParent(t *testing.T) {
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
}

func TestInstallRemoteExtra_AuditFailureLeavesOnlyAdministrativeLock(t *testing.T) {
	for _, project := range []bool{false, true} {
		t.Run(fmt.Sprintf("project=%t", project), func(t *testing.T) {
			req := newRemoteExtraRequest(t)
			if project {
				req.ProjectRoot = t.TempDir()
				req.LockDir = filepath.Join(req.ProjectRoot, ".skillshare")
				req.ExtrasRoot = filepath.Join(req.ProjectRoot, "custom-mounted-extras")
			}
			before, err := os.ReadFile(req.ConfigPath)
			if err != nil {
				t.Fatal(err)
			}
			req.Opts.SkipAudit = false
			req.Opts.AuditProjectRoot = t.TempDir()
			old := scanExtraCandidate
			scanExtraCandidate = func(AuditGate) (*AuditGateResult, error) { return nil, errors.New("synthetic audit refusal") }
			t.Cleanup(func() { scanExtraCandidate = old })
			if _, err := InstallRemoteExtra(req); err == nil {
				t.Fatal("audit refusal ignored")
			}
			entries, err := os.ReadDir(req.ExtrasRoot)
			if err != nil || len(entries) != 1 || entries[0].Name() != round2ExtrasLockName || !entries[0].Type().IsRegular() {
				t.Errorf("refused install must leave only administrative regular lock: %+v %v", entries, err)
			}
			after, err := os.ReadFile(req.ConfigPath)
			if err != nil || string(after) != string(before) {
				t.Error("audit refusal changed config")
			}
			if project {
				for _, file := range []string{filepath.Join(req.LockDir, LockFileName), filepath.Join(req.ProjectRoot, ".gitignore")} {
					if _, err := os.Lstat(file); !os.IsNotExist(err) {
						t.Errorf("audit refusal changed participant %s: %v", file, err)
					}
				}
			}
		})
	}
}
