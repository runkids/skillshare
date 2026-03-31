//go:build !online

package integration

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"skillshare/internal/testutil"
)

func TestInstallBranch_TrackedRepo(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	sb.WriteConfig("source: " + sb.SourcePath + "\ntargets: {}\n")

	// Create bare repo with two branches
	remoteRepo := filepath.Join(sb.Root, "remote-repo.git")
	gitInit(t, remoteRepo, true)

	// Clone, create content on main, push
	workDir := filepath.Join(sb.Root, "work")
	gitClone(t, remoteRepo, workDir)
	if err := os.MkdirAll(filepath.Join(workDir, "main-skill"), 0755); err != nil {
		t.Fatalf("mkdir main-skill: %v", err)
	}
	if err := os.WriteFile(filepath.Join(workDir, "main-skill", "SKILL.md"), []byte("---\nname: main-skill\n---\n# Main branch skill"), 0644); err != nil {
		t.Fatalf("write main-skill: %v", err)
	}
	gitAddCommit(t, workDir, "add main-skill")
	gitPush(t, workDir)

	// Create 'dev' branch with different skill
	cmd := exec.Command("git", "checkout", "-b", "dev")
	cmd.Dir = workDir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git checkout -b dev: %s %v", out, err)
	}
	if err := os.MkdirAll(filepath.Join(workDir, "dev-skill"), 0755); err != nil {
		t.Fatalf("mkdir dev-skill: %v", err)
	}
	if err := os.WriteFile(filepath.Join(workDir, "dev-skill", "SKILL.md"), []byte("---\nname: dev-skill\n---\n# Dev branch skill"), 0644); err != nil {
		t.Fatalf("write dev-skill: %v", err)
	}
	gitAddCommit(t, workDir, "add dev-skill")
	pushCmd := exec.Command("git", "push", "origin", "dev")
	pushCmd.Dir = workDir
	if out, err := pushCmd.CombinedOutput(); err != nil {
		t.Fatalf("git push origin dev: %s %v", out, err)
	}

	// Install tracked repo from dev branch with explicit name
	result := sb.RunCLI("install", "file://"+remoteRepo, "--track", "--branch", "dev", "--name", "test-repo", "--skip-audit")
	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, "dev-skill")

	// Verify dev-skill exists
	devSkillPath := filepath.Join(sb.SourcePath, "_test-repo", "dev-skill", "SKILL.md")
	if _, err := os.Stat(devSkillPath); err != nil {
		t.Errorf("dev-skill should exist at %s: %v", devSkillPath, err)
	}

	// Verify main-skill also exists (it's on dev branch too since dev branched from main)
	mainSkillPath := filepath.Join(sb.SourcePath, "_test-repo", "main-skill", "SKILL.md")
	if _, err := os.Stat(mainSkillPath); err != nil {
		t.Errorf("main-skill should exist at %s: %v", mainSkillPath, err)
	}
}

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
