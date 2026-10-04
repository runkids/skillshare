//go:build !online

package integration

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"skillshare/internal/testutil"
)

// A missing .skillfollow entry pauses prune: sync still links followed skills,
// removes nothing it cannot attribute, and status and doctor name the entry.
func TestSkillfollowSyncPausesPruneForMissingEntry(t *testing.T) {
	for _, project := range []bool{false, true} {
		name := map[bool]string{false: "global", true: "project"}[project]
		t.Run(name, func(t *testing.T) {
			sb := testutil.NewSandbox(t)
			defer sb.Cleanup()
			source, target := sb.SourcePath, sb.CreateTarget("claude")
			projectRoot := ""
			if project {
				projectRoot = sb.SetupProjectDir("claude")
				source = filepath.Join(projectRoot, ".skillshare", "skills")
				target = filepath.Join(projectRoot, ".claude", "skills")
			} else {
				sb.WriteConfig("source: " + source + "\nmode: merge\ntargets:\n  claude:\n    path: " + target + "\n")
			}
			run := func(args ...string) *testutil.Result {
				if project {
					return sb.RunCLIInDir(projectRoot, append(args, "-p")...)
				}
				return sb.RunCLI(append(args, "-g")...)
			}

			external := filepath.Join(sb.Root, "external")
			for _, skill := range []string{"a", "b"} {
				sb.WriteFile(filepath.Join(external, "group", skill, "SKILL.md"), "---\nname: "+skill+"\n---\n# "+skill+"\n")
			}
			if err := os.Symlink(filepath.Join(external, "group"), filepath.Join(source, "group")); err != nil {
				t.Fatal(err)
			}
			offline := filepath.Join(external, "offline")
			if err := os.Symlink(offline, filepath.Join(source, "_off")); err != nil {
				t.Fatal(err)
			}
			sb.WriteFile(filepath.Join(source, ".skillfollow"), "group\n_off\n")
			// A broken link into the source would normally be pruned.
			stale := filepath.Join(target, "_off__c")
			if err := os.Symlink(filepath.Join(source, "_off", "c"), stale); err != nil {
				t.Fatal(err)
			}

			synced := run("sync")
			synced.AssertSuccess(t)
			synced.AssertOutputContains(t, "prune paused")
			for _, link := range []string{"group__a", "group__b"} {
				if dest, err := os.Readlink(filepath.Join(target, link)); err != nil || !strings.Contains(dest, filepath.Join("group", link[len("group__"):])) {
					t.Fatalf("%s -> %q (%v)", link, dest, err)
				}
			}
			if _, err := os.Lstat(stale); err != nil {
				t.Fatalf("prune removed an unattributable link while paused: %v", err)
			}

			status := run("status", "--json")
			status.AssertSuccess(t)
			var output struct {
				Source struct {
					Follow struct {
						PrunePaused []string `json:"prune_paused"`
					} `json:"skillfollow"`
				} `json:"source"`
			}
			if err := json.Unmarshal([]byte(status.Stdout), &output); err != nil {
				t.Fatal(err, status.Stdout)
			}
			want := "prune paused: _off is missing; restore or fix " + filepath.Join(source, "_off") + ", or remove _off from .skillfollow[.local], to resume cleanup"
			if len(output.Source.Follow.PrunePaused) != 1 || output.Source.Follow.PrunePaused[0] != want {
				t.Fatalf("status prune_paused = %v", output.Source.Follow.PrunePaused)
			}

			doctor := run("doctor", "--json")
			var diagnostic struct {
				Checks []struct {
					Name    string `json:"name"`
					Message string `json:"message"`
				} `json:"checks"`
			}
			if err := json.Unmarshal([]byte(doctor.Stdout), &diagnostic); err != nil {
				t.Fatal(err, doctor.Stdout)
			}
			named := false
			for _, check := range diagnostic.Checks {
				named = named || (check.Name == "skillfollow_prune" && check.Message == want)
			}
			if !named {
				t.Fatalf("doctor does not name the blocking entry: %s", doctor.Stdout)
			}

			// Once the entry is restored, prune resumes.
			sb.WriteFile(filepath.Join(offline, "d", "SKILL.md"), "---\nname: d\n---\n# d\n")
			resumed := run("sync")
			resumed.AssertSuccess(t)
			resumed.AssertOutputNotContains(t, "prune paused")
			if _, err := os.Lstat(stale); !os.IsNotExist(err) {
				t.Fatalf("prune did not resume: %v", err)
			}
			if _, err := os.Lstat(filepath.Join(target, "_off__d")); err != nil {
				t.Fatalf("restored entry was not linked: %v", err)
			}
		})
	}
}
