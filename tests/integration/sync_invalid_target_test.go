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

// writeConfigWithFileTarget configures a valid claude target plus a "broken"
// target whose skills path is a regular file.
func writeConfigWithFileTarget(t *testing.T, sb *testutil.Sandbox) (claudeSkills string) {
	t.Helper()
	sb.CreateSkill("test-skill", map[string]string{"SKILL.md": "# Test\n\nTest."})
	claudeSkills = sb.CreateTarget("claude")
	file := filepath.Join(sb.Home, "skills-file")
	sb.WriteFile(file, "x")
	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets:
  claude:
    skills:
      path: ` + claudeSkills + `
  broken:
    skills:
      path: ` + file + `
`)
	return claudeSkills
}

func TestSync_InvalidTargetConfigFailsOnlyThatTarget(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	claudeSkills := writeConfigWithFileTarget(t, sb)

	result := sb.RunCLI("sync")
	result.AssertFailure(t)
	result.AssertRowContains(t, "broken", "invalid config: path is not a directory")

	if !sb.IsSymlink(filepath.Join(claudeSkills, "test-skill")) {
		t.Fatal("valid target should still be synced")
	}
	status, args := lastSyncLogEntry(t, sb)
	got, _ := json.Marshal(args["failed_targets"])
	if status != "partial" || args["targets_failed"] != float64(1) || string(got) != `["broken"]` {
		t.Fatalf("expected partial sync with broken failed, got status %q args %v", status, args)
	}
}

func TestSync_InvalidTargetConfigReportedInJSON(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	writeConfigWithFileTarget(t, sb)

	result := sb.RunCLI("sync", "--json")
	result.AssertFailure(t)

	var out struct {
		Details []struct {
			Name   string `json:"name"`
			Linked int    `json:"linked"`
			Error  string `json:"error"`
		} `json:"details"`
	}
	if err := json.Unmarshal([]byte(result.Stdout), &out); err != nil {
		t.Fatalf("parse sync JSON: %v\n%s", err, result.Stdout)
	}
	byName := map[string]string{}
	linked := map[string]int{}
	for _, d := range out.Details {
		byName[d.Name], linked[d.Name] = d.Error, d.Linked
	}
	if byName["claude"] != "" || linked["claude"] != 1 {
		t.Fatalf("expected claude to sync 1 skill, got %+v", out.Details)
	}
	if want := "invalid config: path is not a directory: " + filepath.Join(sb.Home, "skills-file"); byName["broken"] != want {
		t.Fatalf("broken error = %q, want %q", byName["broken"], want)
	}
}

func TestSyncProject_InvalidTargetConfigFailsOnlyThatTarget(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	projectRoot := sb.SetupProjectDir("claude")
	sb.CreateProjectSkill(projectRoot, "my-skill", map[string]string{"SKILL.md": "# My Skill"})
	if err := os.WriteFile(filepath.Join(projectRoot, "skills-file"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	sb.WriteProjectConfig(projectRoot, `targets:
  - claude
  - name: broken
    skills:
      path: skills-file
`)

	result := sb.RunCLIInDir(projectRoot, "sync", "-p")
	result.AssertFailure(t)
	result.AssertRowContains(t, "broken", "invalid config: path is not a directory")

	if !sb.IsSymlink(filepath.Join(projectRoot, ".claude", "skills", "my-skill")) {
		t.Fatal("valid project target should still be synced")
	}
}

func TestSyncProject_TargetWithoutPathFailsOnlyThatTarget(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	projectRoot := sb.SetupProjectDir("claude")
	sb.CreateProjectSkill(projectRoot, "my-skill", map[string]string{"SKILL.md": "# My Skill"})
	sb.WriteProjectConfig(projectRoot, `targets:
  - claude
  - name: custom
`)

	result := sb.RunCLIInDir(projectRoot, "sync", "-p")
	result.AssertFailure(t)
	result.AssertRowContains(t, "custom", "invalid config: missing path")

	if !sb.IsSymlink(filepath.Join(projectRoot, ".claude", "skills", "my-skill")) {
		t.Fatal("valid project target should still be synced")
	}
}

func TestSyncProject_AgentsOnlySkipsInvalidTargetAndLogsIt(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	projectRoot := sb.SetupProjectDir("claude")
	sb.WriteFile(filepath.Join(projectRoot, ".skillshare", "agents", "tutor.md"), "# Tutor")
	sb.WriteFile(filepath.Join(projectRoot, "skills-file"), "x")
	sb.WriteProjectConfig(projectRoot, `targets:
  - claude
  - name: broken
    skills:
      path: skills-file
    agents:
      path: broken-agents
`)

	result := sb.RunCLIInDir(projectRoot, "sync", "-p", "agents")
	result.AssertFailure(t)
	result.AssertOutputContains(t, "broken: invalid config: path is not a directory")

	if !sb.IsSymlink(filepath.Join(projectRoot, ".claude", "agents", "tutor.md")) {
		t.Fatal("valid project target should still get its agents")
	}
	if _, err := os.Lstat(filepath.Join(projectRoot, "broken-agents", "tutor.md")); !os.IsNotExist(err) {
		t.Fatalf("expected no agent sync into the invalid target, lstat err = %v", err)
	}
	logResult := sb.RunCLIInDir(projectRoot, "log", "-p", "--json", "--cmd", "sync", "--tail", "1")
	logResult.AssertSuccess(t)
	var entry struct {
		Status string         `json:"status"`
		Args   map[string]any `json:"args"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(logResult.Stdout)), &entry); err != nil {
		t.Fatalf("parse sync log entry: %v\n%s", err, logResult.Stdout)
	}
	got, _ := json.Marshal(entry.Args["failed_targets"])
	if entry.Status != "partial" || string(got) != `["broken"]` {
		t.Fatalf("expected partial sync with broken failed, got status %q args %v", entry.Status, entry.Args)
	}
}
