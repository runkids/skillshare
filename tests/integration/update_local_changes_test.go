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

// check --diff shows the edit against the version the skill was installed at,
// even after upstream has moved on.
func TestCheckDiff_ShowsChangesSinceInstall(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	setupGlobalConfig(sb)
	_, work := setupGroupedSkills(t, sb, "alpha", "beta")
	writeGroupedSkill(t, work, "alpha", "v2")
	gitAddCommit(t, work, "upstream v2")
	gitPush(t, work)

	edited := "---\nname: alpha\n---\n# my edit\n"
	if err := os.WriteFile(filepath.Join(sb.SourcePath, "alpha", "SKILL.md"), []byte(edited), 0644); err != nil {
		t.Fatal(err)
	}

	result := sb.RunCLI("check", "alpha", "beta", "--diff")
	result.AssertSuccess(t)
	for _, want := range []string{"1 file changed since install", "modified SKILL.md", "- # alpha v1", "+ # my edit", "no local changes since install"} {
		if !strings.Contains(result.Stdout, want) {
			t.Errorf("check --diff output lacks %q:\n%s", want, result.Stdout)
		}
	}
	if strings.Contains(result.Stdout, "v2") {
		t.Errorf("check --diff compared with upstream HEAD instead of the installed version:\n%s", result.Stdout)
	}
}

func TestCheckDiffProject_ShowsChangesSinceInstall(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	projectRoot := sb.SetupProjectDir("claude")
	remote := filepath.Join(sb.Root, "project-grouped.git")
	work := filepath.Join(sb.Root, "project-grouped-work")
	gitInit(t, remote, true)
	gitClone(t, remote, work)
	writeGroupedSkill(t, work, "alpha", "v1")
	gitAddCommit(t, work, "add skills")
	gitPush(t, work)
	sb.RunCLIInDir(projectRoot, "install", "file://"+remote+"//skills/alpha", "-p", "--skip-audit").AssertSuccess(t)

	skillFile := filepath.Join(projectRoot, ".skillshare", "skills", "alpha", "SKILL.md")
	if err := os.WriteFile(skillFile, []byte("---\nname: alpha\n---\n# my edit\n"), 0644); err != nil {
		t.Fatal(err)
	}

	result := sb.RunCLIInDir(projectRoot, "check", "alpha", "--diff", "-p")
	result.AssertSuccess(t)
	result.AssertOutputContains(t, "- # alpha v1")
	result.AssertOutputContains(t, "+ # my edit")
}

func TestCheckDiff_RejectsUnsupportedCombinations(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	setupGlobalConfig(sb)

	for want, args := range map[string][]string{
		"cannot be combined with --json": {"check", "alpha", "--diff", "--json"},
		"needs a skill name or --group":  {"check", "--diff"},
		"compares skills only":           {"check", "agents", "alpha", "--diff"},
	} {
		result := sb.RunCLI(args...)
		result.AssertFailure(t)
		result.AssertAnyOutputContains(t, want)
	}
}
