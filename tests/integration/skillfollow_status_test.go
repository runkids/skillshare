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

// A live followed repository with tracked metadata is one healthy row, never
// also a missing row: status reads missing repositories with the same snapshot.
func TestSkillfollowStatusTrackedRepoNotDuplicated(t *testing.T) {
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
			repoRows := func() []string {
				t.Helper()
				var result *testutil.Result
				if project {
					result = sb.RunCLIInDir(projectRoot, "status", "--json", "-p")
				} else {
					result = sb.RunCLI("status", "--json", "-g")
				}
				result.AssertSuccess(t)
				var output struct {
					TrackedRepos []struct {
						Name   string `json:"name"`
						Status string `json:"status"`
					} `json:"tracked_repos"`
				}
				if err := json.Unmarshal([]byte(result.Stdout), &output); err != nil {
					t.Fatal(err, result.Stdout)
				}
				var rows []string
				for _, repo := range output.TrackedRepos {
					if repo.Name == "_repo" {
						rows = append(rows, repo.Status)
					}
				}
				return rows
			}
			if rows := repoRows(); len(rows) != 1 || rows[0] == "missing" {
				t.Fatalf("live followed repository rows = %q", rows)
			}
			if err := os.Remove(filepath.Join(source, "_repo")); err != nil {
				t.Fatal(err)
			}
			if rows := repoRows(); len(rows) != 1 || rows[0] != "missing" {
				t.Fatalf("missing repository rows = %q", rows)
			}
		})
	}
}
