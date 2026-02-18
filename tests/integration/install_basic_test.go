//go:build !online

package integration

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"skillshare/internal/install"
	"skillshare/internal/testutil"
)

func TestInstall_LocalPath_CopiesToSource(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)

	// Create a local skill directory to install from
	localSkillPath := filepath.Join(sb.Root, "external-skill")
	os.MkdirAll(localSkillPath, 0755)
	os.WriteFile(filepath.Join(localSkillPath, "SKILL.md"), []byte("# External Skill"), 0644)

	result := sb.RunCLI("install", localSkillPath)

	result.AssertSuccess(t)
	result.AssertOutputContains(t, "Installed")
	result.AssertOutputContains(t, "external-skill")

	// Verify skill was copied to source
	installedPath := filepath.Join(sb.SourcePath, "external-skill", "SKILL.md")
	if !sb.FileExists(installedPath) {
		t.Error("skill should be installed to source directory")
	}

	// Verify metadata was created
	metaPath := filepath.Join(sb.SourcePath, "external-skill", ".skillshare-meta.json")
	if !sb.FileExists(metaPath) {
		t.Error("metadata file should be created")
	}
}

func TestInstall_CustomName_UsesName(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)

	// Create a local skill directory
	localSkillPath := filepath.Join(sb.Root, "original-name")
	os.MkdirAll(localSkillPath, 0755)
	os.WriteFile(filepath.Join(localSkillPath, "SKILL.md"), []byte("# Skill"), 0644)

	result := sb.RunCLI("install", localSkillPath, "--name", "custom-name")

	result.AssertSuccess(t)
	result.AssertOutputContains(t, "custom-name")

	// Verify skill was installed with custom name
	installedPath := filepath.Join(sb.SourcePath, "custom-name", "SKILL.md")
	if !sb.FileExists(installedPath) {
		t.Error("skill should be installed with custom name")
	}

	// Original name should not exist
	originalPath := filepath.Join(sb.SourcePath, "original-name")
	if sb.FileExists(originalPath) {
		t.Error("skill should not be installed with original name")
	}
}

func TestInstall_ExistsWithoutForce_Errors(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	// Create existing skill in source
	sb.CreateSkill("existing-skill", map[string]string{"SKILL.md": "# Existing"})

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)

	// Create local skill to install
	localSkillPath := filepath.Join(sb.Root, "existing-skill")
	os.MkdirAll(localSkillPath, 0755)
	os.WriteFile(filepath.Join(localSkillPath, "SKILL.md"), []byte("# New Version"), 0644)

	result := sb.RunCLI("install", localSkillPath)

	result.AssertFailure(t)
	result.AssertAnyOutputContains(t, "already exists")
}

func TestInstall_Force_Overwrites(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.CreateSkill("existing-skill", map[string]string{"SKILL.md": "# Old Version"})

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)

	localSkillPath := filepath.Join(sb.Root, "existing-skill")
	os.MkdirAll(localSkillPath, 0755)
	os.WriteFile(filepath.Join(localSkillPath, "SKILL.md"), []byte("# New Version"), 0644)

	result := sb.RunCLI("install", localSkillPath, "--force")

	result.AssertSuccess(t)
	result.AssertOutputContains(t, "Installed")

	content := sb.ReadFile(filepath.Join(sb.SourcePath, "existing-skill", "SKILL.md"))
	if !strings.Contains(content, "New Version") {
		t.Error("skill content should be updated")
	}
}

func TestInstall_Force_ByName_UsesMetadata(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)

	localSkillPath := filepath.Join(sb.Root, "local-source")
	os.MkdirAll(localSkillPath, 0755)
	os.WriteFile(filepath.Join(localSkillPath, "SKILL.md"), []byte("# Version 1"), 0644)

	result := sb.RunCLI("install", localSkillPath, "--name", "reinstall-skill")
	result.AssertSuccess(t)

	os.WriteFile(filepath.Join(localSkillPath, "SKILL.md"), []byte("# Version 2"), 0644)

	result = sb.RunCLI("install", "reinstall-skill", "--force")
	result.AssertSuccess(t)

	content := sb.ReadFile(filepath.Join(sb.SourcePath, "reinstall-skill", "SKILL.md"))
	if !strings.Contains(content, "Version 2") {
		t.Error("skill should be reinstalled from stored source")
	}
}

func TestInstall_Update_ByName_ReinstallsNonGit(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)

	localSkillPath := filepath.Join(sb.Root, "update-source")
	os.MkdirAll(localSkillPath, 0755)
	os.WriteFile(filepath.Join(localSkillPath, "SKILL.md"), []byte("# Version 1"), 0644)

	result := sb.RunCLI("install", localSkillPath, "--name", "update-skill")
	result.AssertSuccess(t)

	// Update source content
	os.WriteFile(filepath.Join(localSkillPath, "SKILL.md"), []byte("# Version 2"), 0644)

	// --update should reinstall for non-git sources
	result = sb.RunCLI("install", "update-skill", "--update")
	result.AssertSuccess(t)

	// Verify updated content
	content := sb.ReadFile(filepath.Join(sb.SourcePath, "update-skill", "SKILL.md"))
	if !strings.Contains(content, "Version 2") {
		t.Error("skill should be reinstalled with updated content")
	}
}

func TestInstall_Update_ByName_GitPull(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)

	gitRepoPath := filepath.Join(sb.Root, "git-skill")
	os.MkdirAll(gitRepoPath, 0755)
	sb.WriteFile(filepath.Join(gitRepoPath, "SKILL.md"), "# Version 1")

	initGitRepo(t, gitRepoPath)

	source, err := install.ParseSource("file://" + gitRepoPath)
	if err != nil {
		t.Fatalf("failed to parse source: %v", err)
	}

	result, err := install.Install(source, filepath.Join(sb.SourcePath, "git-skill"), install.InstallOptions{Force: true})
	if err != nil {
		t.Fatalf("failed to install from git source: %v", err)
	}
	if result.Action != "cloned" {
		t.Fatalf("expected install action cloned, got %s", result.Action)
	}

	metaPath := filepath.Join(sb.SourcePath, "git-skill", ".skillshare-meta.json")
	metaContent := sb.ReadFile(metaPath)
	if !strings.Contains(metaContent, "\"source\": \"file://") {
		t.Fatalf("expected metadata source to use file:// clone URL")
	}

	content := sb.ReadFile(filepath.Join(sb.SourcePath, "git-skill", "SKILL.md"))
	if !strings.Contains(content, "Version 1") {
		t.Fatalf("expected installed skill to contain Version 1")
	}

	sb.WriteFile(filepath.Join(gitRepoPath, "SKILL.md"), "# Version 2")

	cmd := exec.Command("git", "add", ".")
	cmd.Dir = gitRepoPath
	cmd.Run()

	cmd = exec.Command("git", "commit", "-m", "Update skill")
	cmd.Dir = gitRepoPath
	cmd.Run()

	updateResult := sb.RunCLI("install", "git-skill", "--update")
	updateResult.AssertSuccess(t)
	updateResult.AssertAnyOutputContains(t, "Installed")

	metaContent = sb.ReadFile(metaPath)
	if !strings.Contains(metaContent, "\"source\": \"file://") {
		t.Fatalf("expected metadata source to use file:// clone URL")
	}

	content = sb.ReadFile(filepath.Join(sb.SourcePath, "git-skill", "SKILL.md"))
	if !strings.Contains(content, "Version 2") {
		t.Fatalf("skill should be updated via git pull")
	}
}

func TestInstall_Update_ByName_NotInstalled_Errors(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)

	result := sb.RunCLI("install", "missing-skill", "--update")

	result.AssertFailure(t)
	result.AssertAnyOutputContains(t, "invalid source")
}

func TestInstall_DryRun_NoChanges(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)

	// Create local skill to install
	localSkillPath := filepath.Join(sb.Root, "dry-run-skill")
	os.MkdirAll(localSkillPath, 0755)
	os.WriteFile(filepath.Join(localSkillPath, "SKILL.md"), []byte("# Dry Run"), 0644)

	result := sb.RunCLI("install", localSkillPath, "--dry-run")

	result.AssertSuccess(t)
	result.AssertOutputContains(t, "dry-run")
	result.AssertOutputContains(t, "would copy")

	// Verify skill was NOT installed
	installedPath := filepath.Join(sb.SourcePath, "dry-run-skill")
	if sb.FileExists(installedPath) {
		t.Error("skill should not be installed in dry-run mode")
	}
}

func TestInstall_InvalidName_Errors(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)

	// Create local skill
	localSkillPath := filepath.Join(sb.Root, "valid-skill")
	os.MkdirAll(localSkillPath, 0755)
	os.WriteFile(filepath.Join(localSkillPath, "SKILL.md"), []byte("# Skill"), 0644)

	// Try to install with invalid name (starts with -)
	result := sb.RunCLI("install", localSkillPath, "--name", "-invalid")

	result.AssertFailure(t)
	result.AssertAnyOutputContains(t, "invalid skill name")
}

func TestInstall_SourceNotExist_Errors(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)

	result := sb.RunCLI("install", "/nonexistent/path/to/skill")

	result.AssertFailure(t)
	result.AssertAnyOutputContains(t, "does not exist")
}

func TestInstall_NoSKILLmd_ShowsWarning(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)

	// Create local skill without SKILL.md
	localSkillPath := filepath.Join(sb.Root, "no-skillmd")
	os.MkdirAll(localSkillPath, 0755)
	os.WriteFile(filepath.Join(localSkillPath, "README.md"), []byte("# Readme"), 0644)

	result := sb.RunCLI("install", localSkillPath)

	result.AssertSuccess(t)
	result.AssertOutputContains(t, "no SKILL.md")
}

func TestInstall_Help_ShowsUsage(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	result := sb.RunCLI("install", "--help")

	result.AssertSuccess(t)
	result.AssertOutputContains(t, "Usage:")
	result.AssertOutputContains(t, "--force")
	result.AssertOutputContains(t, "--dry-run")
	result.AssertOutputContains(t, "--name")
}

func TestInstall_NoArgs_InstallsFromConfig(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)

	result := sb.RunCLI("install")

	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, "No remote skills defined")
}

func TestInstall_LocalGitRepo_ClonesSuccessfully(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)

	// Create a local git repository to test git clone
	gitRepoPath := filepath.Join(sb.Root, "git-skill-repo")
	os.MkdirAll(gitRepoPath, 0755)
	os.WriteFile(filepath.Join(gitRepoPath, "SKILL.md"), []byte("# Git Skill"), 0644)

	initGitRepo(t, gitRepoPath)

	// Install from local git repo path (should detect it's a local path, not git URL)
	result := sb.RunCLI("install", gitRepoPath)

	result.AssertSuccess(t)
	result.AssertOutputContains(t, "Installed")

	// Verify skill was installed (should have copied, not cloned since it's a local path)
	installedPath := filepath.Join(sb.SourcePath, "git-skill-repo", "SKILL.md")
	if !sb.FileExists(installedPath) {
		t.Error("skill should be installed from local git repo")
	}
}

func TestInstall_MetadataContainsSource(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)

	// Create local skill to install
	localSkillPath := filepath.Join(sb.Root, "meta-test-skill")
	os.MkdirAll(localSkillPath, 0755)
	os.WriteFile(filepath.Join(localSkillPath, "SKILL.md"), []byte("# Meta Test"), 0644)

	result := sb.RunCLI("install", localSkillPath)
	result.AssertSuccess(t)

	// Read and verify metadata
	metaContent := sb.ReadFile(filepath.Join(sb.SourcePath, "meta-test-skill", ".skillshare-meta.json"))

	if !strings.Contains(metaContent, `"type": "local"`) {
		t.Error("metadata should contain type: local")
	}
	if !strings.Contains(metaContent, "meta-test-skill") {
		t.Error("metadata should contain source path")
	}
	if !strings.Contains(metaContent, "installed_at") {
		t.Error("metadata should contain installed_at timestamp")
	}
}

func TestInstall_GitSubdir_DirectInstall(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)

	// Create a monorepo-style git repository with multiple skills
	gitRepoPath := filepath.Join(sb.Root, "monorepo")
	skill1Path := filepath.Join(gitRepoPath, "skills", "skill-one")
	skill2Path := filepath.Join(gitRepoPath, "skills", "skill-two")

	os.MkdirAll(skill1Path, 0755)
	os.MkdirAll(skill2Path, 0755)
	os.WriteFile(filepath.Join(skill1Path, "SKILL.md"), []byte("# Skill One"), 0644)
	os.WriteFile(filepath.Join(skill2Path, "SKILL.md"), []byte("# Skill Two"), 0644)

	initGitRepo(t, gitRepoPath)

	// Install specific skill from subdir (using local path with subdir pattern)
	// This tests the direct install path when subdir is specified
	result := sb.RunCLI("install", filepath.Join(gitRepoPath, "skills", "skill-one"))

	result.AssertSuccess(t)
	result.AssertOutputContains(t, "Installed")

	// Verify only skill-one was installed
	if !sb.FileExists(filepath.Join(sb.SourcePath, "skill-one", "SKILL.md")) {
		t.Error("skill-one should be installed")
	}
}
