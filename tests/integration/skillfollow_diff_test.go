//go:build !online

package integration

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"skillshare/internal/testutil"
)

// Diff previews sync with the same follow set: while a declared entry is
// unavailable nothing is reported as removed, the standard-naming copy sync
// would keep is reported as kept, and the output names the prune pause.
func TestSkillfollowDiffPreviewsPausedPrune(t *testing.T) {
	for _, project := range []bool{false, true} {
		name := map[bool]string{false: "global", true: "project"}[project]
		t.Run(name, func(t *testing.T) {
			sb := testutil.NewSandbox(t)
			defer sb.Cleanup()
			source := sb.SourcePath
			merge, copyTarget := sb.CreateTarget("claude"), filepath.Join(sb.Root, "copyt")
			projectRoot := ""
			if project {
				projectRoot = sb.SetupProjectDir("claude")
				source = filepath.Join(projectRoot, ".skillshare", "skills")
				merge = filepath.Join(projectRoot, ".claude", "skills")
				copyTarget = filepath.Join(projectRoot, ".copyt", "skills")
				sb.WriteProjectConfig(projectRoot, "targets:\n  - claude\n  - name: copyt\n    skills:\n      path: "+copyTarget+"\n      mode: copy\n      target_naming: standard\n")
			} else {
				sb.WriteConfig("source: " + source + "\nmode: merge\ntargets:\n  claude:\n    path: " + merge + "\n  copyt:\n    skills:\n      path: " + copyTarget + "\n      mode: copy\n      target_naming: standard\n")
			}
			if err := os.MkdirAll(copyTarget, 0755); err != nil {
				t.Fatal(err)
			}
			run := func(args ...string) *testutil.Result {
				if project {
					return sb.RunCLIInDir(projectRoot, append(args, "-p")...)
				}
				return sb.RunCLI(append(args, "-g")...)
			}
			type diffItem struct{ Action, Name, Reason string }
			diff := func() (map[string][]string, map[string]map[string]diffItem) {
				t.Helper()
				result := run("diff", "--json")
				result.AssertSuccess(t)
				var out struct {
					Targets []struct {
						Name        string     `json:"name"`
						PrunePaused []string   `json:"prune_paused"`
						Items       []diffItem `json:"items"`
					} `json:"targets"`
				}
				if err := json.Unmarshal([]byte(result.Stdout), &out); err != nil {
					t.Fatal(err, result.Stdout)
				}
				paused, items := map[string][]string{}, map[string]map[string]diffItem{}
				for _, target := range out.Targets {
					paused[target.Name] = target.PrunePaused
					items[target.Name] = map[string]diffItem{}
					for _, item := range target.Items {
						items[target.Name][item.Name] = item
					}
				}
				return paused, items
			}

			external := filepath.Join(sb.Root, "external")
			sb.WriteFile(filepath.Join(external, "group", "c", "SKILL.md"), "---\nname: c\n---\n# c\n")
			sb.WriteFile(filepath.Join(external, "missing", "x", "SKILL.md"), "---\nname: x\n---\n# x\n")
			for _, entry := range []string{"group", "missing"} {
				if err := os.Symlink(filepath.Join(external, entry), filepath.Join(source, entry)); err != nil {
					t.Fatal(err)
				}
			}
			sb.WriteFile(filepath.Join(source, ".skillfollow"), "group\nmissing\n")
			run("sync").AssertSuccess(t)

			// The merge target's managed link text becomes the resolved followed path.
			resolved, err := filepath.EvalSymlinks(filepath.Join(external, "group", "c"))
			if err != nil {
				t.Fatal(err)
			}
			if err := os.Remove(filepath.Join(merge, "group__c")); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(resolved, filepath.Join(merge, "group__c")); err != nil {
				t.Fatal(err)
			}
			// The entry goes missing, and the followed group changes under the copy.
			offline := filepath.Join(sb.Root, "offline")
			if err := os.Rename(filepath.Join(external, "missing"), offline); err != nil {
				t.Fatal(err)
			}
			sb.WriteFile(filepath.Join(external, "group", "c", "SKILL.md"), "---\nname: c\n---\n# c changed\n")

			paused, items := diff()
			for _, target := range []string{"claude", "copyt"} {
				if len(paused[target]) != 1 || paused[target][0] != "missing (missing)" {
					t.Errorf("%s prune_paused = %v", target, paused[target])
				}
				for _, item := range items[target] {
					if item.Action == "remove" {
						t.Errorf("%s reports %s %s (%s) while prune is paused", target, item.Action, item.Name, item.Reason)
					}
				}
			}
			if got := items["copyt"]["c"]; got.Action != "keep" {
				t.Errorf("copyt c = %+v, want keep", got)
			}
			if got, ok := items["claude"]["group__c"]; ok {
				t.Errorf("claude group__c = %+v, want in sync", got)
			}
			text := run("diff", "--no-tui")
			text.AssertSuccess(t)
			text.AssertOutputContains(t, "prune paused; unavailable .skillfollow entry: missing (missing)")

			// Restored without x: diff reports what sync then does.
			if err := os.RemoveAll(filepath.Join(offline, "x")); err != nil {
				t.Fatal(err)
			}
			sb.WriteFile(filepath.Join(offline, "y", "SKILL.md"), "---\nname: y\n---\n# y\n")
			if err := os.Rename(offline, filepath.Join(external, "missing")); err != nil {
				t.Fatal(err)
			}
			paused, items = diff()
			if paused["copyt"] != nil || paused["claude"] != nil {
				t.Errorf("prune_paused after restore = %v", paused)
			}
			for name, want := range map[string]string{"x": "remove", "y": "add", "c": "modify"} {
				if got := items["copyt"][name]; got.Action != want {
					t.Errorf("copyt %s = %+v, want %s", name, got, want)
				}
			}
			run("sync").AssertSuccess(t)
			if _, err := os.Stat(filepath.Join(copyTarget, "x")); !os.IsNotExist(err) {
				t.Errorf("sync kept the orphan copy diff reported as removed: %v", err)
			}
			if body := sb.ReadFile(filepath.Join(copyTarget, "c", "SKILL.md")); body != "---\nname: c\n---\n# c changed\n" {
				t.Errorf("sync did not update c: %q", body)
			}
		})
	}
}
