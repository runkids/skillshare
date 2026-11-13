package install

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// A mode-only update is not a local edit, but its delivered mode still belongs
// to the journaled commit. Stop the request after repair so a later publication
// cannot conceal an incorrect repair.
func TestInstallRemoteExtra_RepairDeliversModeOnlyRevision(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX executable bit; not a native Windows assertion")
	}
	for _, project := range []bool{false, true} {
		for _, boundary := range []string{"marker", "incoming", "previous", "live"} {
			t.Run(fmt.Sprintf("project=%t/%s", project, boundary), func(t *testing.T) {
				req := newRemoteExtraRequest(t)
				if project {
					req.ProjectRoot = t.TempDir()
					req.LockDir = filepath.Join(req.ProjectRoot, ".skillshare")
					req.ExtrasRoot = filepath.Join(req.LockDir, "extras")
				}
				req.Branch = "main"
				req.PinBranch = func() string { return "main" }
				initial, err := InstallRemoteExtra(req)
				if err != nil {
					t.Fatal(err)
				}
				live := filepath.Join(req.ExtrasRoot, req.Name, "SKILL.md")
				before, err := os.ReadFile(live)
				if err != nil {
					t.Fatal(err)
				}
				seed := filepath.Join(t.TempDir(), "seed")
				runGit(t, "", "clone", "--branch", "main", req.RawSource, seed)
				runGit(t, seed, "config", "user.name", "Fixture")
				runGit(t, seed, "config", "user.email", "fixture@example.invalid")
				runGit(t, seed, "update-index", "--chmod=+x", "SKILL.md")
				runGit(t, seed, "commit", "-m", "mode only")
				runGit(t, seed, "push", "origin", "main")
				want, err := getGitFullHash(seed)
				if err != nil || want == initial.Commit {
					t.Fatalf("mode fixture did not advance: %s %v", want, err)
				}
				req.Opts.Update = true
				old := extraBoundary
				t.Cleanup(func() { extraBoundary = old })
				extraBoundary = func(step string) error {
					if step == boundary {
						return errSimulatedDeath
					}
					return nil
				}
				_, err = InstallRemoteExtra(req)
				extraBoundary = old
				if !errors.Is(err, errSimulatedDeath) {
					t.Fatalf("did not interrupt at %s: %v", boundary, err)
				}
				// Repair must finish the journaled mode-only commit first. The new
				// request then fails on its deliberately nonexistent branch.
				req.Branch = "fixture-missing-after-repair"
				if _, err := InstallRemoteExtra(req); err == nil || !strings.Contains(err.Error(), req.Branch) {
					t.Fatalf("expected subsequent fetch refusal: %v", err)
				}
				info, err := os.Stat(live)
				if err != nil {
					t.Fatal(err)
				}
				if info.Mode().Perm()&0111 == 0 {
					t.Errorf("journaled executable mode not delivered: %v", info.Mode())
				}
				if data, err := os.ReadFile(live); err != nil || string(data) != string(before) {
					t.Errorf("mode-only repair changed bytes: %q %v", data, err)
				}
				state := InspectRemoteExtra(req.ExtrasRoot, req.Name)
				if state.Err != nil || state.Commit != want || state.Modified || state.Interrupted {
					t.Errorf("repaired provenance: %+v want=%s", state, want)
				}
				if project {
					lock, err := LoadLock(req.LockDir)
					if err != nil || lock.ExtraCommitFor(req.Name, req.RawSource) != want {
						t.Errorf("repaired pin: %+v %v", lock, err)
					}
				}
			})
		}
	}
}
