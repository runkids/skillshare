//go:build !online

package integration

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"skillshare/internal/install"
	"skillshare/internal/testutil"
)

func TestInstallExtra_EmptyRepositoryLifecycle(t *testing.T) {
	for _, project := range []bool{false, true} {
		for _, edited := range []bool{false, true} {
			t.Run(fmt.Sprintf("project=%t/edited=%t", project, edited), func(t *testing.T) {
				sb := testutil.NewSandbox(t)
				defer sb.Cleanup()
				sb.WriteConfig("source: " + sb.SourcePath + "\ntargets: {}\n")
				repo := filepath.Join(sb.Root, "empty-remote")
				if err := os.Mkdir(repo, 0755); err != nil {
					t.Fatal(err)
				}
				testutil.RunGit(t, repo, "init")
				testutil.RunGit(t, repo, "config", "user.name", "Fixture")
				testutil.RunGit(t, repo, "config", "user.email", "fixture@example.invalid")
				testutil.RunGit(t, repo, "commit", "--allow-empty", "-m", "empty tree")
				commit := strings.TrimSpace(testutil.RunGit(t, repo, "rev-parse", "HEAD"))
				cwd, mode := sb.Root, "-g"
				root := filepath.Join(filepath.Dir(sb.SourcePath), "extras")
				if project {
					cwd, mode = sb.SetupProjectDir("claude"), "-p"
					root = filepath.Join(cwd, ".skillshare", "extras")
				}
				args := []string{"install", "file://" + filepath.ToSlash(repo), "--kind", "extra", "--name", "empty", "--skip-audit", "--json", mode}
				sb.RunCLIInDir(cwd, args...).AssertSuccess(t)
				store, _, err := install.ReadMetadata(root)
				if err != nil {
					t.Fatal(err)
				}
				entry := store.GetExact("empty")
				if entry == nil || entry.Commit != commit || entry.FileHashes == nil || len(entry.FileHashes) != 0 {
					t.Errorf("known empty provenance not recorded: %+v", entry)
				}
				check := sb.RunCLIInDir(cwd, "check", "extras", "empty", "--json", mode)
				check.AssertSuccess(t)
				var states []struct{ State string }
				if err := json.Unmarshal([]byte(check.Stdout), &states); err != nil || len(states) != 1 || states[0].State != "intact" {
					t.Errorf("empty installed tree not intact: %s %v", check.Stdout, err)
				}
				// A known empty tree must be replaceable without explicit force.
				sb.RunCLIInDir(cwd, args...).AssertSuccess(t)
				source := filepath.Join(root, "empty")
				if edited {
					if err := os.WriteFile(filepath.Join(source, "note.md"), []byte("keep user note"), 0644); err != nil {
						t.Fatal(err)
					}
					sb.RunCLIInDir(cwd, args...).AssertFailure(t)
				}
				sb.RunCLIInDir(cwd, "extras", "remove", "empty", "--force", mode).AssertSuccess(t)
				if edited {
					data, err := os.ReadFile(filepath.Join(source, "note.md"))
					if err != nil || string(data) != "keep user note" {
						t.Fatalf("user note removed: %q %v", data, err)
					}
				} else if _, err := os.Stat(source); !os.IsNotExist(err) {
					t.Fatalf("unedited empty source retained: %v", err)
				}
			})
		}
	}
}
