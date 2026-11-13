package main

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"skillshare/internal/profile"
)

func TestSyncProfileUnmatchedWarningOnce(t *testing.T) {
	for _, project := range []bool{false, true} {
		for _, jsonMode := range []bool{false, true} {
			t.Run(fmt.Sprintf("project=%t/json=%t", project, jsonMode), func(t *testing.T) {
				_, _, root := profileDiagnosticFixture(t, project, "profiles:\n  work:\n    skills:\n      include: [gone, absent]\n", profile.State{Version: 1, Active: "work"})
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
				if strings.Count(output, "Profile work: unmatched patterns") != 1 || !strings.Contains(output, "gone, absent") {
					t.Fatal("missing or duplicated warning for empty desired selection", output)
				}
			})
		}
	}
}
