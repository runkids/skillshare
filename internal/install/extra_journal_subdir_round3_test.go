package install

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// JSON fixtures keep these tests compilable before the optional field exists.
func TestExtraJournal_ResolvedSubdirValidationAndLegacy(t *testing.T) {
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
}
