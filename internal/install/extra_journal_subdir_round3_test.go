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

// JSON fixtures keep these tests compilable before the optional field exists.
func TestExtraJournal_ResolvedSubdirValidationAndLegacy(t *testing.T) {
	t.Run("baseline", func(t *testing.T) {
		for _, tc := range []struct {
			name, subdir string
			present, bad bool
		}{
			{name: "absent-legacy"},
			{name: "known-empty-root", present: true},
			{name: "known-directory", present: true, subdir: "rules"},
			{name: "parent-escape", present: true, subdir: "../outside", bad: true},
			{name: "absolute", present: true, subdir: "/outside", bad: true},
			{name: "encoded-escape", present: true, subdir: "%2e%2e/outside", bad: true},
		} {
			t.Run(tc.name, func(t *testing.T) {
				root := t.TempDir()
				stage := filepath.Join(root, extraStagingDir)
				if err := os.MkdirAll(stage, 0700); err != nil {
					t.Fatal(err)
				}
				marker := map[string]any{"version": 1, "name": "rules", "remote": "example-org/hub", "branch": "main", "commit": strings.Repeat("a", 40), "had_previous": false, "old": nil}
				if tc.present {
					marker["subdir"] = tc.subdir
				}
				data, err := json.Marshal(marker)
				if err != nil {
					t.Fatal(err)
				}
				path := extraJournalPath(root, "rules")
				if err := os.WriteFile(path, data, 0600); err != nil {
					t.Fatal(err)
				}
				journal, err := readExtraJournal(root, "rules")
				if tc.bad {
					if err == nil {
						t.Error("unsafe recorded directory accepted")
					}
				} else {
					if err != nil {
						t.Fatal(err)
					}
					encoded, err := json.Marshal(journal)
					if err != nil {
						t.Fatal(err)
					}
					var got map[string]any
					if err := json.Unmarshal(encoded, &got); err != nil {
						t.Fatal(err)
					}
					subdir, present := got["subdir"]
					if present != tc.present || present && subdir != tc.subdir || got["version"] != float64(1) || got["remote"] != marker["remote"] {
						t.Errorf("resolved/legacy identity not preserved: %s", encoded)
					}
				}
				if after, err := os.ReadFile(path); err != nil || string(after) != string(data) {
					t.Errorf("inspection changed marker: %q %v", after, err)
				}
			})
		}
	})
	t.Run("TestRepairRemoteExtra_JournalSubdirMustMatchRawSelection", func(t *testing.T) {
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
				// Defensive validation/selection, not a supported publisher identity.
				journal, err := readExtraJournal(req.ExtrasRoot, req.Name)
				if err != nil {
					t.Fatal(err)
				}
				if _, err := journalExtraSource(journal, req.Opts.ParseOptions); err == nil || !strings.Contains(strings.ToLower(err.Error()), "selection") {
					t.Fatalf("selection owner admitted mismatched raw member: %v", err)
				}
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
	})
}
