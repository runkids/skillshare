package install

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestInstallRemoteExtra_AuditFailureMountedRootNonwritableParent(t *testing.T) {
	t.Run("baseline", func(t *testing.T) {
		for _, project := range []bool{false, true} {
			t.Run(fmt.Sprintf("project=%t", project), func(t *testing.T) {
				req := newRemoteExtraRequest(t)
				parent := filepath.Join(t.TempDir(), "mount-parent")
				req.ExtrasRoot = filepath.Join(parent, "mounted-extras")
				if err := os.MkdirAll(req.ExtrasRoot, 0755); err != nil {
					t.Fatal(err)
				}
				if project {
					req.ProjectRoot = t.TempDir()
					req.LockDir = filepath.Join(req.ProjectRoot, ".skillshare")
				}
				if err := os.Chmod(parent, 0555); err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() { _ = os.Chmod(parent, 0755) })
				probe := filepath.Join(parent, "denied-sibling")
				probeErr := os.WriteFile(probe, []byte("probe"), 0600)
				if probeErr == nil {
					if err := os.Remove(probe); err != nil {
						t.Fatal(err)
					}
				}
				if os.Geteuid() != 0 && runtime.GOOS != "windows" {
					if !os.IsPermission(probeErr) {
						t.Fatalf("fixture did not deny sibling creation: %v", probeErr)
					}
				} else {
					t.Log("topology-only: root/Windows runner does not establish chmod denial")
				}
				before, err := os.ReadFile(req.ConfigPath)
				if err != nil {
					t.Fatal(err)
				}
				req.Opts.SkipAudit = false
				req.Opts.AuditProjectRoot = t.TempDir()
				old := scanExtraCandidate
				audited := false
				scanExtraCandidate = func(AuditGate) (*AuditGateResult, error) {
					audited = true
					return nil, errors.New("synthetic mounted audit refusal")
				}
				t.Cleanup(func() { scanExtraCandidate = old })
				if _, err := InstallRemoteExtra(req); err == nil {
					t.Fatal("audit refusal ignored")
				}
				if !audited {
					t.Fatal("writable extras root blocked by nonwritable parent before audit")
				}
				entries, err := os.ReadDir(req.ExtrasRoot)
				if err != nil || len(entries) != 1 || entries[0].Name() != ".skillshare-extras.lock" || !entries[0].Type().IsRegular() {
					t.Errorf("expected only permanent administrative lock: %+v %v", entries, err)
				}
				after, err := os.ReadFile(req.ConfigPath)
				if err != nil || string(after) != string(before) {
					t.Error("refusal changed config")
				}
				entries, err = os.ReadDir(parent)
				if err != nil || len(entries) != 1 || entries[0].Name() != "mounted-extras" {
					t.Errorf("parent side effects: %+v %v", entries, err)
				}
				if project {
					for _, file := range []string{filepath.Join(req.LockDir, LockFileName), filepath.Join(req.ProjectRoot, ".gitignore")} {
						if _, err := os.Lstat(file); !os.IsNotExist(err) {
							t.Errorf("refusal changed participant %s: %v", file, err)
						}
					}
				}
			})
		}
	})
	t.Run("TestInstallRemoteExtra_AuditFailureLeavesOnlyAdministrativeLock", func(t *testing.T) {
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
	})
}
