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

// A skill installed from a repo and then edited by hand is reported by check
// and left alone by update until --force discards the edit (issue #621).
func TestUpdate_LocallyModifiedSkillNeedsForce(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	setupGlobalConfig(sb)
	setupGroupedSkills(t, sb, "alpha", "beta")

	skillFile := filepath.Join(sb.SourcePath, "alpha", "SKILL.md")
	const edited = "---\nname: alpha\n---\n# my edit\n"
	if err := os.WriteFile(skillFile, []byte(edited), 0644); err != nil {
		t.Fatal(err)
	}
	assertContent := func(want string) {
		t.Helper()
		got, err := os.ReadFile(skillFile)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(got), "my edit") != strings.Contains(want, "my edit") {
			t.Fatalf("SKILL.md = %q", got)
		}
	}

	check := sb.RunCLI("check", "--json")
	check.AssertSuccess(t)
	var out struct {
		Skills []struct{ Name, Status string } `json:"skills"`
	}
	if err := json.Unmarshal([]byte(check.Stdout), &out); err != nil {
		t.Fatalf("parse check JSON: %v\n%s", err, check.Stdout)
	}
	statuses := map[string]string{}
	for _, s := range out.Skills {
		statuses[s.Name] = s.Status
	}
	if statuses["alpha"] != "modified" || statuses["beta"] != "up_to_date" {
		t.Fatalf("check statuses = %v, want alpha modified and beta up_to_date", statuses)
	}

	if got := runUpdateAllJSON(t, sb); got.Skipped != 1 || got.UpToDate != 1 {
		t.Fatalf("update --all: got %+v, want 1 skipped and 1 up to date", got)
	}
	assertContent(edited)

	sb.RunCLI("update", "alpha", "--skip-audit").AssertFailure(t)
	assertContent(edited)

	sb.RunCLI("update", "alpha", "--skip-audit", "--force").AssertSuccess(t)
	assertContent("v1")
}

// A skipped modified skill is counted once even when its repository then
// fails to fetch, and a dry run previews instead of failing.
func TestUpdate_LocallyModifiedSkillCountedOnceOnFetchFailure(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	setupGlobalConfig(sb)
	remote, _ := setupGroupedSkills(t, sb, "alpha", "beta")

	if err := os.WriteFile(filepath.Join(sb.SourcePath, "alpha", "SKILL.md"), []byte("# my edit\n"), 0644); err != nil {
		t.Fatal(err)
	}
	sb.RunCLI("update", "alpha", "--dry-run").AssertSuccess(t)

	if err := os.RemoveAll(remote); err != nil {
		t.Fatal(err)
	}
	if got := runUpdateAllJSON(t, sb); got.Skipped != 2 {
		t.Fatalf("update --all: got %+v, want 2 skipped", got)
	}
}

// update --all with a single modified skill takes the single-target path; it
// skips the skill like the batch path instead of failing the run.
func TestUpdateAll_SingleModifiedSkillIsSkipped(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	setupGlobalConfig(sb)
	setupGroupedSkills(t, sb, "alpha")

	if err := os.WriteFile(filepath.Join(sb.SourcePath, "alpha", "SKILL.md"), []byte("# my edit\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if got := runUpdateAllJSON(t, sb); got.Skipped != 1 {
		t.Fatalf("update --all: got %+v, want 1 skipped", got)
	}
}
