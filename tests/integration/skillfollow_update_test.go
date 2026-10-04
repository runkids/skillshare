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

// A metadata-backed skill below a followed non-_ group belongs to the user:
// every update route refuses it before fetching, and other skills still update.
// group/g is a git checkout, which install's direct pull path would mutate;
// group/plain is a subdir reinstall. Routes that classify a git checkout as a
// repository update it through the followed-repository policy instead.
func TestSkillfollowUpdateRefusesFollowedGroupSkill(t *testing.T) {
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
			run := func(args ...string) *testutil.Result {
				if project {
					return sb.RunCLIInDir(projectRoot, append(args, "-p")...)
				}
				return sb.RunCLI(append(args, "-g")...)
			}
			base := t.TempDir()
			remote := testutil.SetupBareRemoteRepo(t, base)
			testutil.SeedRemoteBranch(t, base, remote, "main", map[string]string{"SKILL.md": "---\nname: g\ndescription: Fixture\n---\n# G\n"})
			external := filepath.Join(sb.Root, "external")
			checkout, ordinary := filepath.Join(external, "g"), filepath.Join(source, "ordinary")
			for _, path := range []string{checkout, ordinary} {
				testutil.RunGit(t, "", "clone", remote, path)
			}
			sb.WriteFile(filepath.Join(external, "plain", "SKILL.md"), "# Plain\n")
			if err := os.Symlink(external, filepath.Join(source, "group")); err != nil {
				t.Fatal(err)
			}
			sb.WriteFile(filepath.Join(source, ".skillfollow"), "group\n")
			url := "file://" + filepath.ToSlash(remote)
			store := install.NewMetadataStore()
			store.Set("group/g", &install.MetadataEntry{Source: url})
			store.Set("group/plain", &install.MetadataEntry{Source: url + "//plain", RepoURL: url, Subdir: "plain"})
			store.Set("ordinary", &install.MetadataEntry{Source: url})
			if err := store.Save(source); err != nil {
				t.Fatal(err)
			}
			seed := filepath.Join(base, "seed-main")
			sb.WriteFile(filepath.Join(seed, "remote.txt"), "new\n")
			testutil.RunGit(t, seed, "add", ".")
			testutil.RunGit(t, seed, "commit", "-m", "advance")
			testutil.RunGit(t, seed, "push", "origin", "HEAD:main")
			before := testutil.RunGit(t, checkout, "rev-parse", "HEAD")
			expected := testutil.RunGit(t, seed, "rev-parse", "HEAD")

			refused := func(result *testutil.Result, args []string) {
				t.Helper()
				result.AssertFailure(t)
				result.AssertAnyOutputContains(t, "followed repository update refused")
				if got := testutil.RunGit(t, checkout, "rev-parse", "HEAD"); got != before {
					t.Fatalf("%v pulled the user's checkout", args)
				}
			}
			// Global names resolve by basename and glob; project names are exact paths.
			rows := [][]string{{"update", "--all", "--dry-run"}, {"update", "plain"}, {"update", "plain", "--dry-run"}, {"update", "pla*"}}
			if project {
				rows = [][]string{{"update", "--all", "--dry-run"}, {"update", "group/plain"}, {"update", "group/plain", "--dry-run"}}
			}
			for _, args := range rows {
				refused(run(args...), args)
			}

			result := run("update", "--all", "--json")
			var output struct {
				Updated int `json:"updated"`
				Items   []struct {
					Name, Type, Status, Error string
				} `json:"items"`
			}
			if err := json.Unmarshal([]byte(result.Stdout), &output); err != nil {
				t.Fatalf("invalid JSON: %v\n%s\n%s", err, result.Stdout, result.Stderr)
			}
			refused(result, []string{"--all", "--json"})
			if output.Updated != 1 || len(output.Items) != 2 {
				t.Fatalf("unexpected JSON: %s", result.Stdout)
			}
			for i, name := range []string{"group/g", "group/plain"} {
				item := output.Items[i]
				if item.Name != name || item.Type != "skill" || item.Status != "failed" || !strings.Contains(item.Error, "followed repository update refused") {
					t.Fatalf("unexpected JSON item %d: %s", i, result.Stdout)
				}
			}
			if got := testutil.RunGit(t, ordinary, "rev-parse", "HEAD"); got != expected {
				t.Fatal("ordinary skill did not update")
			}

			// Group expansion also refuses the regular skill, beside the checkout's policy update.
			for _, args := range [][]string{{"update", "--group", "group"}, {"update", "group"}} {
				result := run(args...)
				result.AssertFailure(t)
				result.AssertAnyOutputContains(t, "skill group/plain is inside followed entry group")
			}
		})
	}
}
