package install

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInstallRemoteExtra_OfflineRepairRestoresPreviousBeforeFetch(t *testing.T) {
	for _, project := range []bool{false, true} {
		t.Run(fmt.Sprintf("project=%t", project), func(t *testing.T) {
			req := newRemoteExtraRequest(t)
			if project {
				req.ProjectRoot = t.TempDir()
				req.LockDir = filepath.Join(req.ProjectRoot, ".skillshare")
				req.ExtrasRoot = filepath.Join(req.LockDir, "extras")
			}
			if _, err := InstallRemoteExtra(req); err != nil {
				t.Fatal(err)
			}
			dest := filepath.Join(req.ExtrasRoot, req.Name)
			before, err := fingerprintExtraTree(dest)
			if err != nil {
				t.Fatal(err)
			}
			req.Branch, req.Opts.Update = "feature", true
			old := extraBoundary
			t.Cleanup(func() { extraBoundary = old })
			extraBoundary = func(step string) error {
				if step == "previous" {
					return errSimulatedDeath
				}
				return nil
			}
			_, err = InstallRemoteExtra(req)
			extraBoundary = old
			if !errors.Is(err, errSimulatedDeath) {
				t.Fatalf("interruption: %v", err)
			}
			if _, err := os.Lstat(dest); !os.IsNotExist(err) {
				t.Fatalf("fixture must interrupt between live renames: %v", err)
			}
			markerPath := extraJournalPath(req.ExtrasRoot, req.Name)
			marker, err := os.ReadFile(markerPath)
			if err != nil {
				t.Fatal(err)
			}
			configBefore, err := os.ReadFile(req.ConfigPath)
			if err != nil {
				t.Fatal(err)
			}
			// The only remote is a disposable fixture. Move it offline after the
			// actual publication boundary, without changing the journal's identity.
			remote := strings.TrimPrefix(req.RawSource, "file://")
			if err := os.Rename(remote, remote+".offline"); err != nil {
				t.Fatal(err)
			}
			if _, err := InstallRemoteExtra(req); err == nil {
				t.Fatal("offline candidate unexpectedly published")
			}
			after, err := fingerprintExtraTree(dest)
			if err != nil || !sameExtraFingerprint(before, after) {
				t.Fatalf("known previous content unavailable after failed fetch: %v, tree=%v", err, after)
			}
			if data, err := os.ReadFile(markerPath); err != nil || string(data) != string(marker) {
				t.Fatalf("failed repair changed marker: %s %v", data, err)
			}
			if data, err := os.ReadFile(req.ConfigPath); err != nil || string(data) != string(configBefore) {
				t.Fatalf("offline repair changed declaration: %s %v", data, err)
			}
			if state := InspectRemoteExtra(req.ExtrasRoot, req.Name); !state.Interrupted {
				t.Fatalf("unfinished repair no longer reported: %+v", state)
			}
		})
	}
}
