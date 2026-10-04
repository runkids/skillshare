//go:build !online

package integration

import (
	"encoding/json"
	"os"
	"path/filepath"
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
