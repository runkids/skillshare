//go:build !online

package integration

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"skillshare/internal/oplog"
	"skillshare/internal/testutil"
)

func TestDisable_AddsToSkillignore(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.CreateSkill("my-skill", map[string]string{
		"SKILL.md": "---\nname: my-skill\n---\n# My Skill",
	})
	sb.WriteConfig("source: " + sb.SourcePath + "\ntargets: {}\n")

	result := sb.RunCLI("disable", "my-skill")
	result.AssertSuccess(t)
	result.AssertRowContains(t, "my-skill", "added to .skillignore")
	result.AssertAnyOutputContains(t, "skillshare sync")

	ignorePath := filepath.Join(sb.SourcePath, ".skillignore")
	data, err := os.ReadFile(ignorePath)
	if err != nil {
		t.Fatalf("expected .skillignore to exist: %v", err)
	}
	if !strings.Contains(string(data), "my-skill") {
		t.Errorf(".skillignore should contain my-skill, got: %q", string(data))
	}
}

func TestDisable_AlreadyDisabled(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.CreateSkill("my-skill", map[string]string{
		"SKILL.md": "---\nname: my-skill\n---\n# My Skill",
	})
	sb.WriteConfig("source: " + sb.SourcePath + "\ntargets: {}\n")

	sb.RunCLI("disable", "my-skill")

	result := sb.RunCLI("disable", "my-skill")
	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, "already disabled")
}

func TestEnable_RemovesFromSkillignore(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.CreateSkill("my-skill", map[string]string{
		"SKILL.md": "---\nname: my-skill\n---\n# My Skill",
	})
	sb.WriteConfig("source: " + sb.SourcePath + "\ntargets: {}\n")

	sb.RunCLI("disable", "my-skill")

	result := sb.RunCLI("enable", "my-skill")
	result.AssertSuccess(t)
	result.AssertRowContains(t, "my-skill", "removed from .skillignore")

	ignorePath := filepath.Join(sb.SourcePath, ".skillignore")
	data, err := os.ReadFile(ignorePath)
	if err != nil && !os.IsNotExist(err) {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(string(data), "my-skill") {
		t.Errorf(".skillignore should not contain my-skill after enable, got: %q", string(data))
	}
}

func TestEnable_NotDisabled(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.CreateSkill("my-skill", map[string]string{
		"SKILL.md": "---\nname: my-skill\n---\n# My Skill",
	})
	sb.WriteConfig("source: " + sb.SourcePath + "\ntargets: {}\n")

	result := sb.RunCLI("enable", "my-skill")
	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, "not disabled")
}

// toggleSandbox has the given skills and agents, plus files written relative
// to the source root that "skills/" or "agents/" starts.
func toggleSandbox(t *testing.T, skills []string, files map[string]string) *testutil.Sandbox {
	t.Helper()
	sb := testutil.NewSandbox(t)
	t.Cleanup(sb.Cleanup)
	for _, name := range skills {
		sb.CreateSkill(name, map[string]string{"SKILL.md": "---\nname: " + filepath.Base(name) + "\n---\n# Skill"})
	}
	for rel, content := range files {
		sb.WriteFile(filepath.Join(filepath.Dir(sb.SourcePath), rel), content)
	}
	agents := filepath.Join(filepath.Dir(sb.SourcePath), "agents")
	sb.WriteConfig("source: " + sb.SourcePath + "\nagents_source: " + agents + "\ntargets: {}\n")
	return sb
}

func lastOp(t *testing.T, sb *testutil.Sandbox) oplog.Entry {
	t.Helper()
	entries, err := oplog.Read(sb.ConfigPath, oplog.OpsFile, 1)
	if err != nil || len(entries) != 1 {
		t.Fatalf("oplog: %+v %v", entries, err)
	}
	return entries[0]
}

func TestEnable_BlockedByAnotherRule(t *testing.T) {
	sb := toggleSandbox(t, []string{"draft-one"}, map[string]string{"skills/.skillignore": "draft-*\n"})

	result := sb.RunCLI("enable", "draft-one")
	result.AssertExitCode(t, 1)
	result.AssertOutputContains(t, "could not enable draft-one: still disabled: another rule in .skillignore")
	if n := strings.Count(result.Stdout, "still disabled"); n != 1 {
		t.Errorf("the reason is printed %d times, want once:\n%s", n, result.Stdout)
	}
}

func TestEnable_BlockedByAgentFolderRule(t *testing.T) {
	sb := toggleSandbox(t, nil, map[string]string{
		"agents/archive/reviewer.md": "---\nname: reviewer\n---\n# Agent",
		"agents/.agentignore":        "archive/\n",
	})

	result := sb.RunCLI("enable", "-g", "--kind", "agent", "archive/reviewer")
	result.AssertExitCode(t, 1)
	result.AssertOutputContains(t, "could not enable archive/reviewer.md: still disabled: another rule in .agentignore")
}

func TestDisable_OverriddenByNegationRule(t *testing.T) {
	for _, tc := range []struct {
		name   string
		args   []string
		ignore string
	}{
		{"skill", []string{"disable", "my-skill"}, "skills/.skillignore"},
		{"agent", []string{"disable", "-g", "--kind", "agent", "mine"}, "agents/.agentignore"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			sb := toggleSandbox(t, []string{"my-skill"}, map[string]string{
				"agents/mine.md":            "# Agent",
				"skills/.skillignore.local": "!my-skill\n",
				"agents/.agentignore.local": "!mine.md\n",
			})

			result := sb.RunCLI(tc.args...)
			result.AssertExitCode(t, 1)
			result.AssertOutputContains(t, "still enabled")
			result.AssertOutputNotContains(t, "skillshare sync")
			if op := lastOp(t, sb); op.Command != "disable" || op.Status != "error" {
				t.Errorf("oplog = %+v, want a disable error", op)
			}
			// The line it wrote disables nothing, so it is removed again.
			if data, _ := os.ReadFile(filepath.Join(filepath.Dir(sb.SourcePath), tc.ignore)); strings.TrimSpace(string(data)) != "" {
				t.Errorf("%s = %q, want no line left", tc.ignore, data)
			}
		})
	}
}

// A name can refer to several skills; the ones another rule keeps disabled fail alone.
func TestEnable_PartialFailure(t *testing.T) {
	sb := toggleSandbox(t, []string{"group/inner", "other/inner"}, map[string]string{"skills/.skillignore": "group/inner\nother/\n"})

	result := sb.RunCLI("enable", "inner")
	result.AssertExitCode(t, 1)
	result.AssertRowContains(t, "group/inner", "removed from .skillignore")
	result.AssertOutputContains(t, "skillshare sync")
	result.AssertOutputContains(t, "could not enable other/inner: still disabled")
	if op := lastOp(t, sb); op.Status != "partial" || !strings.Contains(op.Message, "other/inner") {
		t.Errorf("oplog = %+v, want partial naming other/inner", op)
	}
}

func TestToggle_BlockedInProjectMode(t *testing.T) {
	for _, tc := range []struct {
		name string
		args []string
		file string
		want string
	}{
		{"skill", []string{"enable", "draft-one", "-p"}, "skills/.skillignore", "another rule in .skillignore"},
		{"agent", []string{"enable", "--kind", "agent", "draft-helper", "-p"}, "agents/.agentignore", "another rule in .agentignore"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			sb := testutil.NewSandbox(t)
			defer sb.Cleanup()
			root := sb.SetupProjectDir()
			sb.CreateProjectSkill(root, "draft-one", map[string]string{"SKILL.md": "---\nname: draft-one\n---\n# Skill"})
			sb.WriteFile(filepath.Join(root, ".skillshare", "agents", "draft-helper.md"), "# Agent")
			sb.WriteFile(filepath.Join(root, ".skillshare", tc.file), "draft-*\n")

			result := sb.RunCLIInDir(root, tc.args...)
			result.AssertExitCode(t, 1)
			result.AssertOutputContains(t, tc.want)
		})
	}
}

// A bare name also removes the line the list TUI or dashboard wrote for its relPath.
func TestEnable_BareNameRemovesRelPathLine(t *testing.T) {
	sb := toggleSandbox(t, []string{"group/inner"}, map[string]string{"skills/.skillignore": "group/inner\n"})

	result := sb.RunCLI("enable", "inner")
	result.AssertSuccess(t)
	result.AssertRowContains(t, "group/inner", "removed from .skillignore")
}

func TestEnable_DryRunListsRelPathLine(t *testing.T) {
	sb := toggleSandbox(t, []string{"group/inner"}, map[string]string{"skills/.skillignore": "group/inner\n"})

	result := sb.RunCLI("enable", "inner", "--dry-run")
	result.AssertSuccess(t)
	result.AssertRowContains(t, "group/inner", "would be removed from")
}

func TestDisable_AgentFrontmatterNameTakesEffect(t *testing.T) {
	sb := toggleSandbox(t, nil, map[string]string{"agents/rev.md": "---\nname: critic\n---\n# Agent"})

	result := sb.RunCLI("disable", "-g", "--kind", "agent", "critic")
	result.AssertSuccess(t)
	result.AssertRowContains(t, "rev.md", "added to .agentignore")
}

// An exact relPath names that skill alone, not others in folders of that name.
func TestEnable_ExactRelPathWins(t *testing.T) {
	sb := toggleSandbox(t, []string{"react-ui", "archive/react-ui"}, map[string]string{"skills/.skillignore": "react-ui\narchive/\n"})

	result := sb.RunCLI("enable", "react-ui")
	result.AssertSuccess(t)
	result.AssertRowContains(t, "react-ui", "removed from .skillignore")
}

func TestDisable_DryRun(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.CreateSkill("my-skill", map[string]string{"SKILL.md": "# Skill"})
	sb.WriteConfig("source: " + sb.SourcePath + "\ntargets: {}\n")

	result := sb.RunCLI("disable", "my-skill", "--dry-run")
	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, "would be added to")

	ignorePath := filepath.Join(sb.SourcePath, ".skillignore")
	if _, err := os.Stat(ignorePath); err == nil {
		t.Error(".skillignore should not exist after dry-run")
	}
}

func TestDisable_NoArgs(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	result := sb.RunCLI("disable")
	result.AssertFailure(t)
}
