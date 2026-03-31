//go:build !online

package integration

import (
	"testing"

	"skillshare/internal/testutil"
)

func TestInstallBranch_FlagParsing(t *testing.T) {
	t.Run("rejects --branch without value", func(t *testing.T) {
		sb := testutil.NewSandbox(t)
		defer sb.Cleanup()
		sb.WriteConfig("source: " + sb.SourcePath + "\ntargets: {}\n")

		result := sb.RunCLI("install", "github.com/owner/repo", "--branch")
		result.AssertFailure(t)
		result.AssertAnyOutputContains(t, "--branch requires a value")
	})

	t.Run("rejects --branch with local path", func(t *testing.T) {
		sb := testutil.NewSandbox(t)
		defer sb.Cleanup()
		sb.WriteConfig("source: " + sb.SourcePath + "\ntargets: {}\n")

		localSkill := sb.CreateSkill("my-skill", map[string]string{
			"SKILL.md": "---\nname: my-skill\n---\n# Content",
		})

		result := sb.RunCLI("install", localSkill, "--branch", "dev")
		result.AssertFailure(t)
		result.AssertAnyOutputContains(t, "--branch can only be used with git")
	})

	t.Run("rejects empty --branch value", func(t *testing.T) {
		sb := testutil.NewSandbox(t)
		defer sb.Cleanup()
		sb.WriteConfig("source: " + sb.SourcePath + "\ntargets: {}\n")

		result := sb.RunCLI("install", "github.com/owner/repo", "--branch", "  ")
		result.AssertFailure(t)
		result.AssertAnyOutputContains(t, "--branch requires a non-empty value")
	})
}
