package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"skillshare/internal/profile"
)

func TestSyncProfileUnmatchedWarningOnce(t *testing.T) {
	for _, scenario := range []string{"global", "project", "global-incomplete", "project-incomplete"} {
		project, incomplete := strings.HasPrefix(scenario, "project"), strings.HasSuffix(scenario, "incomplete")
		for _, jsonMode := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/json=%t", scenario, jsonMode), func(t *testing.T) {
				cfg, _, root := profileDiagnosticFixture(t, project, "follow_source_links: true\nprofiles:\n  work:\n    skills:\n      include: [gone, absent]\n", profile.State{Version: 1, Active: "work"})
				if incomplete {
					if err := os.Symlink(filepath.Join(root, "absent"), filepath.Join(cfg.Source, "broken")); err != nil {
						t.Skipf("symlink unavailable: %v", err)
					}
				}
				args := []string{"-g", "--dry-run"}
				if project {
					t.Chdir(root)
					args[0] = "-p"
				}
				if jsonMode {
					args = append(args, "--json")
				}
				var commandErr error
				output := captureStdout(t, func() { commandErr = cmdSync(args) })
				if commandErr != nil {
					t.Fatal(output, commandErr)
				}
				if jsonMode {
					var out struct {
						Warnings []string `json:"warnings"`
					}
					if err := json.Unmarshal([]byte(output), &out); err != nil {
						t.Fatal(output, err)
					}
					output = strings.Join(out.Warnings, "\n")
				}
				if incomplete {
					if strings.Contains(output, "Profile work: unmatched patterns") {
						t.Fatal("incomplete discovery emitted a false unmatched warning", output)
					}
					return
				}
				if strings.Count(output, "Profile work: unmatched patterns") != 1 || !strings.Contains(output, "gone, absent") {
					t.Fatal("missing or duplicated warning for empty desired selection", output)
				}
			})
		}
	}
}
