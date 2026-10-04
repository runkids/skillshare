//go:build !online

package integration

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"skillshare/internal/testutil"
)

func TestSkillfollowDiscoverySurfaces(t *testing.T) {
	for _, project := range []bool{false, true} {
		name := "global"
		if project {
			name = "project"
		}
		t.Run(name, func(t *testing.T) {
			sb := testutil.NewSandbox(t)
			defer sb.Cleanup()
			projectRoot := filepath.Join(sb.Root, "project")
			source := sb.SourcePath
			if project {
				source = filepath.Join(projectRoot, ".skillshare", "skills")
			}
			mkdir := func(path string) {
				t.Helper()
				if err := os.MkdirAll(path, 0755); err != nil {
					t.Fatal(err)
				}
			}
			write := func(path, body string) {
				t.Helper()
				mkdir(filepath.Dir(path))
				if err := os.WriteFile(path, []byte(body), 0644); err != nil {
					t.Fatal(err)
				}
			}
			mkdir(source)
			external := filepath.Join(sb.Root, "external")
			for _, rel := range []string{"repo/a", "repo/b", "group/c", "hidden/secret"} {
				write(filepath.Join(external, rel, "SKILL.md"), "---\nname: "+filepath.Base(rel)+"\ndescription: Test fixture\n---\n# Test\n")
			}
			cmd := exec.Command("git", "init", filepath.Join(external, "repo"))
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("git init: %s %v", out, err)
			}
			for link, target := range map[string]string{"_repo": "repo", "group": "group", "undeclared": "hidden"} {
				if err := os.Symlink(filepath.Join(external, target), filepath.Join(source, link)); err != nil {
					t.Fatal(err)
				}
			}
			write(filepath.Join(source, "invalid"), "file")
			if project {
				write(filepath.Join(projectRoot, ".skillshare", "config.yaml"), "targets: []\n")
			} else {
				sb.WriteConfig("source: " + source + "\ntargets: {}\n")
			}
			run := func(command string) *testutil.Result {
				if project {
					return sb.RunCLIInDir(projectRoot, command, "-p", "--json")
				}
				return sb.RunCLI(command, "-g", "--json")
			}
			// The default remains invisible and does not add the optional JSON surface.
			before := run("list")
			before.AssertSuccess(t)
			if strings.Contains(before.Stdout, "_repo") || strings.Contains(before.Stdout, "group") {
				t.Fatal(before.Stdout)
			}
			status := run("status")
			status.AssertSuccess(t)
			if strings.Contains(status.Stdout, "skillfollow") {
				t.Fatal(status.Stdout)
			}
			write(filepath.Join(source, ".skillfollow"), "_repo\ngroup\nmissing\ninvalid\n")
			write(filepath.Join(source, ".skillfollow.local"), "group\n")
			snapshot := func(root string) map[string]string {
				result := map[string]string{}
				err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
					if err != nil {
						return err
					}
					if info.IsDir() {
						return nil
					}
					if info.Mode()&os.ModeSymlink != 0 {
						dest, err := os.Readlink(path)
						result[path] = "link:" + dest
						return err
					}
					data, err := os.ReadFile(path)
					result[path] = string(data)
					return err
				})
				if err != nil {
					t.Fatal(err)
				}
				return result
			}
			sourceBefore, externalBefore := snapshot(source), snapshot(external)
			listed := run("list")
			listed.AssertSuccess(t)
			var skills []struct {
				Name     string `json:"name"`
				RelPath  string `json:"rel_path"`
				RepoName string `json:"repo_name"`
			}
			if err := json.Unmarshal([]byte(listed.Stdout), &skills); err != nil {
				t.Fatal(err, listed.Stdout)
			}
			if len(skills) != 3 {
				t.Fatal(listed.Stdout)
			}
			for _, rel := range []string{"_repo/a", "_repo/b", "group/c"} {
				if !strings.Contains(listed.Stdout, rel) {
					t.Fatalf("missing %s: %s", rel, listed.Stdout)
				}
			}
			if strings.Contains(listed.Stdout, "secret") {
				t.Fatal(listed.Stdout)
			}
			status = run("status")
			status.AssertSuccess(t)
			var output struct {
				Source struct {
					Follow struct {
						Entries  int  `json:"entry_count"`
						Followed int  `json:"followed_count"`
						Skipped  int  `json:"skipped_count"`
						Local    bool `json:"local_active"`
					} `json:"skillfollow"`
				} `json:"source"`
				SkillCount int `json:"skill_count"`
				Repos      []struct {
					Name  string `json:"name"`
					Count int    `json:"skill_count"`
				} `json:"tracked_repos"`
			}
			if err := json.Unmarshal([]byte(status.Stdout), &output); err != nil {
				t.Fatal(err)
			}
			if output.Source.Follow.Entries != 4 || output.Source.Follow.Followed != 2 || output.Source.Follow.Skipped != 2 || !output.Source.Follow.Local || output.SkillCount != 3 || len(output.Repos) != 1 || output.Repos[0].Name != "_repo" || output.Repos[0].Count != 2 {
				t.Fatal(status.Stdout)
			}
			doctor := run("doctor")
			var diagnostic struct {
				Checks []struct {
					Name    string `json:"name"`
					Message string `json:"message"`
				} `json:"checks"`
			}
			if err := json.Unmarshal([]byte(doctor.Stdout), &diagnostic); err != nil {
				t.Fatal(err, doctor.Stdout)
			}
			for _, message := range []string{"_repo: followed", "group: followed", "missing: missing", "invalid: invalid-target", "undeclared: not followed"} {
				if !strings.Contains(doctor.Stdout, message) {
					t.Fatalf("missing %s: %s", message, doctor.Stdout)
				}
			}
			for _, check := range diagnostic.Checks {
				if check.Name == "undeclared_source_links" && !strings.Contains(check.Message, "undeclared:") {
					t.Fatal(check.Message)
				}
			}
			if !reflect.DeepEqual(sourceBefore, snapshot(source)) || !reflect.DeepEqual(externalBefore, snapshot(external)) {
				t.Fatal("discovery commands changed source content")
			}
		})
	}
}
