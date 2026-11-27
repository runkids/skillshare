//go:build !online

package integration

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"skillshare/internal/testutil"
)

func TestSync_FollowSourceLinks_UnavailableLinkKeepsTargets(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	checkout := filepath.Join(sb.Root, "code", "dev-skills")
	for _, name := range []string{"foo", "bar"} {
		sb.WriteFile(filepath.Join(checkout, name, "SKILL.md"), "---\nname: "+name+"\n---\n# "+name)
	}
	if err := os.MkdirAll(filepath.Join(checkout, ".git"), 0755); err != nil {
		t.Fatal(err)
	}
	sb.CreateSymlink(checkout, filepath.Join(sb.SourcePath, "_dev-skills"))
	mergeTarget := sb.CreateTarget("claude")
	copyTarget := sb.CreateTarget("cursor")
	sb.WriteConfig(`source: ` + sb.SourcePath + `
follow_source_links: true
targets:
  claude:
    path: ` + mergeTarget + `
  cursor:
    path: ` + copyTarget + `
    mode: copy
`)

	sb.RunCLI("sync").AssertSuccess(t)
	wantMerge, wantCopy := sb.ListDir(mergeTarget), sb.ListDir(copyTarget)
	if !sb.IsSymlink(filepath.Join(mergeTarget, "_dev-skills__foo")) || !sb.FileExists(filepath.Join(copyTarget, "_dev-skills__foo", "SKILL.md")) {
		t.Fatalf("followed skills not synced: merge %v, copy %v", wantMerge, wantCopy)
	}

	// The checkout goes away (unmounted drive, re-clone in progress).
	if err := os.Rename(checkout, checkout+".moved"); err != nil {
		t.Fatal(err)
	}
	result := sb.RunCLI("sync")
	result.AssertSuccess(t)
	warning := "source link _dev-skills not followed: target is missing; kept existing target entries, nothing pruned this run"
	if got := strings.Count(result.Output(), warning); got != 1 {
		t.Fatalf("want the warning once, got %d:\n%s", got, result.Output())
	}
	if got := sb.ListDir(mergeTarget); !reflect.DeepEqual(got, wantMerge) {
		t.Errorf("merge target changed: %v, want %v", got, wantMerge)
	}
	if got := sb.ListDir(copyTarget); !reflect.DeepEqual(got, wantCopy) {
		t.Errorf("copy target changed: %v, want %v", got, wantCopy)
	}
	if !sb.FileExists(filepath.Join(copyTarget, "_dev-skills__foo", "SKILL.md")) {
		t.Error("copy of an unavailable skill was deleted")
	}
}

// Automation reading --json must see why nothing was pruned.
func TestSync_FollowSourceLinks_JSONReportsUnavailableLink(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	sb.CreateSkill("local", map[string]string{"SKILL.md": "# local"})
	sb.CreateSymlink(filepath.Join(sb.Root, "gone"), filepath.Join(sb.SourcePath, "_dev-skills"))
	target := sb.CreateTarget("claude")
	sb.WriteConfig("source: " + sb.SourcePath + "\nfollow_source_links: true\ntargets:\n  claude:\n    path: " + target + "\n")

	result := sb.RunCLI("sync", "--json")
	result.AssertSuccess(t)
	var out struct {
		Warnings []string `json:"warnings"`
	}
	if err := json.Unmarshal([]byte(result.Stdout), &out); err != nil {
		t.Fatalf("parse json: %v\n%s", err, result.Stdout)
	}
	want := "source link _dev-skills not followed: target is missing; kept existing target entries, nothing pruned this run"
	if !reflect.DeepEqual(out.Warnings, []string{want}) {
		t.Fatalf("warnings = %q", out.Warnings)
	}
}

// Sync prunes nothing while a followed link is unavailable, so diff must not
// list that link's entries as orphans to prune.
func TestDiff_FollowSourceLinks_UnavailableLinkListsNoOrphans(t *testing.T) {
	for _, mode := range []string{"merge", "copy"} {
		t.Run(mode, func(t *testing.T) {
			sb := testutil.NewSandbox(t)
			defer sb.Cleanup()

			checkout := filepath.Join(sb.Root, "code", "dev-skills")
			sb.WriteFile(filepath.Join(checkout, "foo", "SKILL.md"), "---\nname: foo\n---\n# foo")
			sb.CreateSymlink(checkout, filepath.Join(sb.SourcePath, "_dev-skills"))
			target := sb.CreateTarget("claude")
			sb.WriteConfig("source: " + sb.SourcePath + "\nfollow_source_links: true\ntargets:\n  claude:\n    path: " + target + "\n    mode: " + mode + "\n")
			sb.RunCLI("sync").AssertSuccess(t)

			if err := os.Rename(checkout, checkout+".moved"); err != nil {
				t.Fatal(err)
			}
			result := sb.RunCLI("diff")
			result.AssertSuccess(t)
			out := result.Output()
			if strings.Contains(out, "_dev-skills__foo") {
				t.Fatalf("diff lists the unavailable link's entry:\n%s", out)
			}
			if !strings.Contains(out, "source link _dev-skills not followed") || !strings.Contains(out, "nothing pruned") {
				t.Fatalf("diff does not say the link was not followed:\n%s", out)
			}
		})
	}
}
