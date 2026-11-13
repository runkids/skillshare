package install

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRepairRemoteExtra_JournalSubdirMustMatchRawSelection(t *testing.T) {
	for _, web := range []bool{false, true} {
		t.Run(fmt.Sprintf("web=%t", web), func(t *testing.T) {
			req := newRemoteExtraRequest(t)
			if web {
				req, _ = pinnedWebTreeFixture(t, false, false)
			}
			req.Opts.Update = true
			oldBoundary := extraBoundary
			t.Cleanup(func() { extraBoundary = oldBoundary })
			extraBoundary = func(step string) error {
				if step == "marker" {
					return errSimulatedDeath
				}
				return nil
			}
			_, err := InstallRemoteExtra(req)
			extraBoundary = oldBoundary
			if !errors.Is(err, errSimulatedDeath) {
				t.Fatalf("actual producer interruption: %v", err)
			}
			markerPath := extraJournalPath(req.ExtrasRoot, req.Name)
			data, err := os.ReadFile(markerPath)
			if err != nil {
				t.Fatal(err)
			}
			var marker map[string]any
			if err := json.Unmarshal(data, &marker); err != nil {
				t.Fatal(err)
			}
			// Traversal-safe but not ANY allowed path selection of the raw origin.
			marker["subdir"] = "unrelated"
			data, err = json.Marshal(marker)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(markerPath, data, 0600); err != nil {
				t.Fatal(err)
			}
			before, err := fingerprintExtraTree(req.ExtrasRoot)
			if err != nil {
				t.Fatal(err)
			}
			configBefore, err := os.ReadFile(req.ConfigPath)
			if err != nil {
				t.Fatal(err)
			}
			req.Branch = "missing-after-repair"
			_, err = InstallRemoteExtra(req)
			if err == nil || !strings.Contains(strings.ToLower(err.Error()), "selection") {
				t.Errorf("journal path outside raw selection accepted: %v", err)
			}
			after, scanErr := fingerprintExtraTree(req.ExtrasRoot)
			if scanErr != nil || !sameExtraFingerprint(before, after) {
				t.Errorf("invalid marker changed managed participants: %v", scanErr)
			}
			if after, err := os.ReadFile(req.ConfigPath); err != nil || string(after) != string(configBefore) {
				t.Errorf("invalid marker changed config: %q %v", after, err)
			}
			if after, err := os.ReadFile(filepath.Join(req.ExtrasRoot, extraStagingDir, "publishing-"+req.Name+".json")); err != nil || string(after) != string(data) {
				t.Errorf("invalid marker changed recovery evidence: %q %v", after, err)
			}
		})
	}
}
