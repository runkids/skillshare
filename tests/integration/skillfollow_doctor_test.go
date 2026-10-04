//go:build !online

package integration

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"skillshare/internal/install"
	"skillshare/internal/testutil"
)

func TestSkillfollowDoctorTrackedRepoNotMissing(t *testing.T) {
	for _, project := range []bool{false, true} {
		t.Run(map[bool]string{false: "global", true: "project"}[project], func(t *testing.T) {
			sb := testutil.NewSandbox(t)
			defer sb.Cleanup()
			source, projectRoot := sb.SourcePath, ""
			if project {
				projectRoot = sb.SetupProjectDir("claude")
				source = filepath.Join(projectRoot, ".skillshare", "skills")
			} else {
				sb.WriteConfig("source: " + source + "\ntargets: {}\n")
			}
			external := filepath.Join(sb.Root, "external")
			sb.WriteFile(filepath.Join(external, "safe", "SKILL.md"), "---\nname: safe\ndescription: Safe skill\n---\n# Safe\n")
			testutil.RunGit(t, external, "init")
			if err := os.Symlink(external, filepath.Join(source, "_repo")); err != nil {
				t.Fatal(err)
			}
			sb.WriteFile(filepath.Join(source, ".skillfollow"), "_repo\n")
			store := install.NewMetadataStore()
			store.Set("_repo", &install.MetadataEntry{Source: "github.com/example/repo", Tracked: true})
			if err := store.Save(source); err != nil {
				t.Fatal(err)
			}
			run := func() *testutil.Result {
				if project {
					return sb.RunCLIInDir(projectRoot, "doctor", "--json", "-p")
				}
				return sb.RunCLI("doctor", "--json", "-g")
			}
			missing := func(result *testutil.Result) bool {
				t.Helper()
				var output struct {
					Checks []struct {
						Name string `json:"name"`
					} `json:"checks"`
				}
				if err := json.Unmarshal([]byte(result.Stdout), &output); err != nil {
					t.Fatal(err, result.Stdout)
				}
				if len(output.Checks) == 0 {
					t.Fatal("doctor returned no checks")
				}
				for _, check := range output.Checks {
					if check.Name == "tracked_repos" {
						return true
					}
				}
				return false
			}
			if result := run(); missing(result) {
				t.Errorf("live followed repository reported missing: %s", result.Stdout)
			}
			// Truly missing tracked repositories must still receive the recovery diagnostic.
			if err := os.Remove(filepath.Join(source, "_repo")); err != nil {
				t.Fatal(err)
			}
			if result := run(); !missing(result) {
				t.Errorf("missing repository was not reported: %s", result.Stdout)
			}
		})
	}
}

// An unreadable declaration makes discovery fail; doctor must skip the checks
// that need the discovered skills instead of reading the failed scan as empty.
// A directory named .skillfollow fails to read for every user, including root.
func TestSkillfollowDoctorUnreadableDeclarationSkipsDiscoveryChecks(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	target := sb.CreateTarget("claude")
	sb.WriteConfig("source: " + sb.SourcePath + "\ntargets:\n  claude:\n    path: " + target + "\n")
	sb.WriteFile(filepath.Join(sb.SourcePath, "group", "a", "SKILL.md"), "---\nname: a\ndescription: A\n---\n# A\n")
	sb.RunCLI("sync", "-g").AssertSuccess(t)
	if err := os.Mkdir(filepath.Join(sb.SourcePath, ".skillfollow"), 0755); err != nil {
		t.Fatal(err)
	}

	text := sb.RunCLI("doctor", "-g")
	text.AssertAnyOutputContains(t, "skill discovery failed")
	if strings.Contains(text.Stdout+text.Stderr, "without SKILL.md") {
		t.Errorf("failed discovery reported as a group without SKILL.md:\n%s%s", text.Stdout, text.Stderr)
	}

	result := sb.RunCLI("doctor", "--json", "-g")
	var output struct {
		Checks []struct {
			Name    string `json:"name"`
			Status  string `json:"status"`
			Message string `json:"message"`
		} `json:"checks"`
	}
	if err := json.Unmarshal([]byte(result.Stdout), &output); err != nil {
		t.Fatal(err, result.Stdout)
	}
	skipped := map[string]bool{"skills_validity": false, "skill_integrity": false, "skill_targets_field": false, "sync_drift": false}
	declarationReported := false
	for _, check := range output.Checks {
		if check.Name == "skillfollow" && check.Status == "warning" && strings.Contains(check.Message, ".skillfollow") {
			declarationReported = true
		}
		if _, ok := skipped[check.Name]; ok {
			if check.Status != "info" || !strings.Contains(check.Message, "skill discovery failed") {
				t.Errorf("%s: want skipped info, got %s %q", check.Name, check.Status, check.Message)
			}
			skipped[check.Name] = true
		}
	}
	if !declarationReported {
		t.Errorf("declaration read error not reported: %s", result.Stdout)
	}
	for name, seen := range skipped {
		if !seen {
			t.Errorf("%s: skipped check not reported", name)
		}
	}
}
