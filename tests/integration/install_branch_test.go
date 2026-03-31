//go:build !online

package integration

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"skillshare/internal/install"
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

func TestInstallBranch_RegularInstall(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	sb.WriteConfig("source: " + sb.SourcePath + "\ntargets: {}\n")

	// Create bare repo with two branches
	remoteRepo := filepath.Join(sb.Root, "regular-repo.git")
	gitInit(t, remoteRepo, true)

	workDir := filepath.Join(sb.Root, "work-regular")
	gitClone(t, remoteRepo, workDir)

	// main branch: one skill
	os.MkdirAll(filepath.Join(workDir, "alpha"), 0755)
	os.WriteFile(filepath.Join(workDir, "alpha", "SKILL.md"), []byte("---\nname: alpha\n---\n# Alpha"), 0644)
	gitAddCommit(t, workDir, "add alpha")
	gitPush(t, workDir)

	// feature branch: additional skill
	cmd := exec.Command("git", "checkout", "-b", "feature")
	cmd.Dir = workDir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git checkout -b feature: %s %v", out, err)
	}
	os.MkdirAll(filepath.Join(workDir, "beta"), 0755)
	os.WriteFile(filepath.Join(workDir, "beta", "SKILL.md"), []byte("---\nname: beta\n---\n# Beta"), 0644)
	gitAddCommit(t, workDir, "add beta")
	pushCmd := exec.Command("git", "push", "origin", "feature")
	pushCmd.Dir = workDir
	if out, err := pushCmd.CombinedOutput(); err != nil {
		t.Fatalf("git push origin feature: %s %v", out, err)
	}

	// Install from feature branch with --all
	result := sb.RunCLI("install", "file://"+remoteRepo, "--branch", "feature", "--all", "--skip-audit")
	result.AssertSuccess(t)

	// Both alpha and beta should be installed (feature branch has both)
	if _, err := os.Stat(filepath.Join(sb.SourcePath, "alpha", "SKILL.md")); err != nil {
		t.Error("alpha skill should be installed")
	}
	if _, err := os.Stat(filepath.Join(sb.SourcePath, "beta", "SKILL.md")); err != nil {
		t.Error("beta skill should be installed")
	}
}

func TestInstallBranch_MetadataPersistence(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	sb.WriteConfig("source: " + sb.SourcePath + "\ntargets: {}\n")

	// Create bare repo with staging branch
	remoteRepo := filepath.Join(sb.Root, "meta-repo.git")
	gitInit(t, remoteRepo, true)

	workDir := filepath.Join(sb.Root, "work-meta")
	gitClone(t, remoteRepo, workDir)
	os.MkdirAll(filepath.Join(workDir, "my-skill"), 0755)
	os.WriteFile(filepath.Join(workDir, "my-skill", "SKILL.md"), []byte("---\nname: my-skill\n---\n# Skill"), 0644)
	gitAddCommit(t, workDir, "add skill")
	gitPush(t, workDir)

	cmd := exec.Command("git", "checkout", "-b", "staging")
	cmd.Dir = workDir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git checkout -b staging: %s %v", out, err)
	}
	// Need a commit on staging so it differs from main
	os.WriteFile(filepath.Join(workDir, "my-skill", "SKILL.md"), []byte("---\nname: my-skill\n---\n# Skill staging"), 0644)
	gitAddCommit(t, workDir, "staging commit")
	pushCmd := exec.Command("git", "push", "origin", "staging")
	pushCmd.Dir = workDir
	if out, err := pushCmd.CombinedOutput(); err != nil {
		t.Fatalf("git push: %s %v", out, err)
	}

	// Install from staging branch
	result := sb.RunCLI("install", "file://"+remoteRepo, "--branch", "staging", "--all", "--skip-audit")
	result.AssertSuccess(t)

	// Check .skillshare-meta.json has branch field
	metaPath := filepath.Join(sb.SourcePath, "my-skill", ".skillshare-meta.json")
	data, err := os.ReadFile(metaPath)
	if err != nil {
		t.Fatalf("read meta: %v", err)
	}

	var meta install.SkillMeta
	if err := json.Unmarshal(data, &meta); err != nil {
		t.Fatalf("unmarshal meta: %v", err)
	}
	if meta.Branch != "staging" {
		t.Errorf("meta.Branch = %q, want %q", meta.Branch, "staging")
	}
}

func TestInstallBranch_UpdatePreservesBranch(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	sb.WriteConfig("source: " + sb.SourcePath + "\ntargets: {}\n")

	// Create bare repo
	remoteRepo := filepath.Join(sb.Root, "update-repo.git")
	gitInit(t, remoteRepo, true)

	workDir := filepath.Join(sb.Root, "work-update")
	gitClone(t, remoteRepo, workDir)
	os.MkdirAll(filepath.Join(workDir, "updatable"), 0755)
	os.WriteFile(filepath.Join(workDir, "updatable", "SKILL.md"), []byte("---\nname: updatable\n---\n# V1"), 0644)
	gitAddCommit(t, workDir, "v1")
	gitPush(t, workDir)

	// Create dev branch
	cmd := exec.Command("git", "checkout", "-b", "dev")
	cmd.Dir = workDir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("checkout -b dev: %s %v", out, err)
	}
	pushCmd := exec.Command("git", "push", "origin", "dev")
	pushCmd.Dir = workDir
	if out, err := pushCmd.CombinedOutput(); err != nil {
		t.Fatalf("push dev: %s %v", out, err)
	}

	// Install from dev branch using explicit subdir notation (file:///repo//updatable)
	// so meta.Source is a valid re-installable URL.
	result := sb.RunCLI("install", "file://"+remoteRepo+"//updatable", "--branch", "dev", "--skip-audit")
	result.AssertSuccess(t)

	// Verify branch is persisted in metadata
	metaPath := filepath.Join(sb.SourcePath, "updatable", ".skillshare-meta.json")
	data, err := os.ReadFile(metaPath)
	if err != nil {
		t.Fatalf("read meta: %v", err)
	}
	var meta install.SkillMeta
	if err := json.Unmarshal(data, &meta); err != nil {
		t.Fatalf("unmarshal meta: %v", err)
	}
	if meta.Branch != "dev" {
		t.Errorf("meta.Branch = %q, want %q", meta.Branch, "dev")
	}

	// Push update on dev branch only
	os.WriteFile(filepath.Join(workDir, "updatable", "SKILL.md"), []byte("---\nname: updatable\n---\n# V2 dev"), 0644)
	gitAddCommit(t, workDir, "v2 on dev")
	pushCmd = exec.Command("git", "push", "origin", "dev")
	pushCmd.Dir = workDir
	if out, err := pushCmd.CombinedOutput(); err != nil {
		t.Fatalf("push dev v2: %s %v", out, err)
	}

	// Update should pull from dev branch (not main)
	updateResult := sb.RunCLI("update", "updatable", "--skip-audit")
	updateResult.AssertSuccess(t)

	// Verify content is V2 dev
	content, err := os.ReadFile(filepath.Join(sb.SourcePath, "updatable", "SKILL.md"))
	if err != nil {
		t.Fatalf("read skill: %v", err)
	}
	if !strings.Contains(string(content), "V2 dev") {
		t.Errorf("expected V2 dev content, got: %s", content)
	}
}
