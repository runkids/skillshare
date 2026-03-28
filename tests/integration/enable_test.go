//go:build !online

package integration

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

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
	result.AssertAnyOutputContains(t, "Disabled: my-skill")
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
	result.AssertAnyOutputContains(t, "Enabled: my-skill")

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

func TestDisable_GlobPattern(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.CreateSkill("draft-a", map[string]string{"SKILL.md": "# A"})
	sb.CreateSkill("draft-b", map[string]string{"SKILL.md": "# B"})
	sb.CreateSkill("keep-me", map[string]string{"SKILL.md": "# Keep"})
	sb.WriteConfig("source: " + sb.SourcePath + "\ntargets: {}\n")

	result := sb.RunCLI("disable", "draft-*")
	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, "Disabled: draft-*")

	ignorePath := filepath.Join(sb.SourcePath, ".skillignore")
	data, _ := os.ReadFile(ignorePath)
	if !strings.Contains(string(data), "draft-*") {
		t.Errorf(".skillignore should contain draft-*, got: %q", string(data))
	}
}

func TestDisable_DryRun(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.CreateSkill("my-skill", map[string]string{"SKILL.md": "# Skill"})
	sb.WriteConfig("source: " + sb.SourcePath + "\ntargets: {}\n")

	result := sb.RunCLI("disable", "my-skill", "--dry-run")
	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, "Would add")

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
