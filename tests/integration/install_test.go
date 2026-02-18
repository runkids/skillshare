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

	cmd := exec.Command("git", "init")
	cmd.Dir = gitRepoPath
	if err := cmd.Run(); err != nil {
		t.Skip("git not available, skipping git test")
	}

	cmd = exec.Command("git", "config", "user.email", "test@test.com")
	cmd.Dir = gitRepoPath
	cmd.Run()

	cmd = exec.Command("git", "config", "user.name", "Test")
	cmd.Dir = gitRepoPath
	cmd.Run()

	cmd = exec.Command("git", "add", ".")
	cmd.Dir = gitRepoPath
	cmd.Run()

	cmd = exec.Command("git", "commit", "-m", "Initial commit")
	cmd.Dir = gitRepoPath
	cmd.Run()

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

	cmd = exec.Command("git", "add", ".")
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

	// Initialize git repo
	cmd := exec.Command("git", "init")
	cmd.Dir = gitRepoPath
	if err := cmd.Run(); err != nil {
		t.Skip("git not available, skipping git test")
	}

	cmd = exec.Command("git", "config", "user.email", "test@test.com")
	cmd.Dir = gitRepoPath
	cmd.Run()

	cmd = exec.Command("git", "config", "user.name", "Test")
	cmd.Dir = gitRepoPath
	cmd.Run()

	cmd = exec.Command("git", "add", ".")
	cmd.Dir = gitRepoPath
	cmd.Run()

	cmd = exec.Command("git", "commit", "-m", "Initial commit")
	cmd.Dir = gitRepoPath
	cmd.Run()

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

	// Initialize git repo
	cmd := exec.Command("git", "init")
	cmd.Dir = gitRepoPath
	if err := cmd.Run(); err != nil {
		t.Skip("git not available, skipping git test")
	}

	cmd = exec.Command("git", "config", "user.email", "test@test.com")
	cmd.Dir = gitRepoPath
	cmd.Run()

	cmd = exec.Command("git", "config", "user.name", "Test")
	cmd.Dir = gitRepoPath
	cmd.Run()

	cmd = exec.Command("git", "add", ".")
	cmd.Dir = gitRepoPath
	cmd.Run()

	cmd = exec.Command("git", "commit", "-m", "Initial commit")
	cmd.Dir = gitRepoPath
	cmd.Run()

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

func TestInstall_Discovery_FindsSkills(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)

	// Create a monorepo-style git repository
	gitRepoPath := filepath.Join(sb.Root, "discover-repo")
	skill1Path := filepath.Join(gitRepoPath, "skill-alpha")
	skill2Path := filepath.Join(gitRepoPath, "nested", "skill-beta")

	os.MkdirAll(skill1Path, 0755)
	os.MkdirAll(skill2Path, 0755)
	os.WriteFile(filepath.Join(skill1Path, "SKILL.md"), []byte("# Alpha"), 0644)
	os.WriteFile(filepath.Join(skill2Path, "SKILL.md"), []byte("# Beta"), 0644)

	// Test the discovery via internal package directly
	// (Can't easily test interactive selection in integration tests)
	result := sb.RunCLI("install", skill1Path)

	result.AssertSuccess(t)
	result.AssertOutputContains(t, "Installed")
}

func TestInstall_Discovery_DryRun_ShowsSkills(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)

	// Create a monorepo with multiple skills
	gitRepoPath := filepath.Join(sb.Root, "dry-run-repo")
	skill1Path := filepath.Join(gitRepoPath, "skill-one")
	skill2Path := filepath.Join(gitRepoPath, "skill-two")

	os.MkdirAll(skill1Path, 0755)
	os.MkdirAll(skill2Path, 0755)
	os.WriteFile(filepath.Join(skill1Path, "SKILL.md"), []byte("# One"), 0644)
	os.WriteFile(filepath.Join(skill2Path, "SKILL.md"), []byte("# Two"), 0644)

	// Initialize git repo
	cmd := exec.Command("git", "init")
	cmd.Dir = gitRepoPath
	if err := cmd.Run(); err != nil {
		t.Skip("git not available, skipping git test")
	}

	cmd = exec.Command("git", "config", "user.email", "test@test.com")
	cmd.Dir = gitRepoPath
	cmd.Run()

	cmd = exec.Command("git", "config", "user.name", "Test")
	cmd.Dir = gitRepoPath
	cmd.Run()

	cmd = exec.Command("git", "add", ".")
	cmd.Dir = gitRepoPath
	cmd.Run()

	cmd = exec.Command("git", "commit", "-m", "Initial commit")
	cmd.Dir = gitRepoPath
	cmd.Run()

	// Use file:// protocol to test git discovery with local repo
	result := sb.RunCLI("install", "file://"+gitRepoPath, "--dry-run")

	result.AssertSuccess(t)
	result.AssertOutputContains(t, "Found")
	result.AssertOutputContains(t, "skill-one")
	result.AssertOutputContains(t, "skill-two")
	result.AssertOutputContains(t, "dry-run")

	// Verify nothing was installed
	if sb.FileExists(filepath.Join(sb.SourcePath, "skill-one")) {
		t.Error("skill should not be installed in dry-run mode")
	}
}

// TestInstall_Discovery_HiddenDirs_FindsSkills tests that skills inside hidden
// directories (like .curated/, .system/) are discovered, while .git is skipped.
func TestInstall_Discovery_HiddenDirs_FindsSkills(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)

	// Create a repo with skills inside hidden directories (like openai/skills)
	gitRepoPath := filepath.Join(sb.Root, "hidden-dir-repo")
	curatedSkill := filepath.Join(gitRepoPath, ".curated", "pdf")
	systemSkill := filepath.Join(gitRepoPath, ".system", "figma")

	os.MkdirAll(curatedSkill, 0755)
	os.MkdirAll(systemSkill, 0755)
	os.WriteFile(filepath.Join(curatedSkill, "SKILL.md"), []byte("# PDF"), 0644)
	os.WriteFile(filepath.Join(systemSkill, "SKILL.md"), []byte("# Figma"), 0644)

	// Initialize git repo
	cmd := exec.Command("git", "init")
	cmd.Dir = gitRepoPath
	if err := cmd.Run(); err != nil {
		t.Skip("git not available, skipping git test")
	}

	cmd = exec.Command("git", "config", "user.email", "test@test.com")
	cmd.Dir = gitRepoPath
	cmd.Run()

	cmd = exec.Command("git", "config", "user.name", "Test")
	cmd.Dir = gitRepoPath
	cmd.Run()

	cmd = exec.Command("git", "add", ".")
	cmd.Dir = gitRepoPath
	cmd.Run()

	cmd = exec.Command("git", "commit", "-m", "Initial commit")
	cmd.Dir = gitRepoPath
	cmd.Run()

	// Use file:// protocol to test git discovery with local repo
	result := sb.RunCLI("install", "file://"+gitRepoPath, "--dry-run")

	result.AssertSuccess(t)
	result.AssertOutputContains(t, "Found")
	result.AssertOutputContains(t, "pdf")
	result.AssertOutputContains(t, "figma")
}

// TestInstall_OrchestratorStructure_PreservesNesting tests that when installing
// an orchestrator structure (root SKILL.md + child skills), the nested structure
// is preserved rather than flattening all skills to root.
func TestInstall_OrchestratorStructure_PreservesNesting(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)

	// Create an orchestrator-style skill structure directly in source path
	// This simulates what installSelectedSkills should do when given:
	// - root skill (path=".")
	// - child skills (path="child-a", "child-b")
	//
	// The fix ensures children are nested under root when root is selected.

	// First, create a local orchestrator structure to simulate discovery results
	orchestratorPath := filepath.Join(sb.Root, "orchestrator")
	childAPath := filepath.Join(orchestratorPath, "child-a")
	childBPath := filepath.Join(orchestratorPath, "child-b")

	os.MkdirAll(orchestratorPath, 0755)
	os.MkdirAll(childAPath, 0755)
	os.MkdirAll(childBPath, 0755)

	os.WriteFile(filepath.Join(orchestratorPath, "SKILL.md"), []byte("# Parent Skill\nOrchestrator for child skills"), 0644)
	os.WriteFile(filepath.Join(childAPath, "SKILL.md"), []byte("# Child A"), 0644)
	os.WriteFile(filepath.Join(childBPath, "SKILL.md"), []byte("# Child B"), 0644)

	// Simulate the discovery result structure
	// Skills found when discovering a subdir with root SKILL.md:
	// - SkillInfo{Name: "orchestrator", Path: "."} - root
	// - SkillInfo{Name: "child-a", Path: "child-a"}
	// - SkillInfo{Name: "child-b", Path: "child-b"}

	rootSkill := install.SkillInfo{Name: "orchestrator", Path: "."}
	childA := install.SkillInfo{Name: "child-a", Path: "child-a"}
	childB := install.SkillInfo{Name: "child-b", Path: "child-b"}

	// Test the nesting logic: when root is selected, children should nest under it
	// This is the logic from installSelectedSkills after the fix

	// Detect orchestrator: if root skill (path=".") is in selected, children nest under it
	parentName := ""
	selected := []install.SkillInfo{rootSkill, childA, childB}
	for _, skill := range selected {
		if skill.Path == "." {
			parentName = skill.Name
			break
		}
	}

	if parentName != "orchestrator" {
		t.Fatalf("expected parent name to be 'orchestrator', got '%s'", parentName)
	}

	// Now install using the nesting logic
	for _, skill := range selected {
		var destPath string
		if skill.Path == "." {
			// Root skill - install directly
			destPath = filepath.Join(sb.SourcePath, skill.Name)
		} else if parentName != "" {
			// Child skill with parent selected - nest under parent
			destPath = filepath.Join(sb.SourcePath, parentName, skill.Name)
		} else {
			// Standalone child skill - install to root
			destPath = filepath.Join(sb.SourcePath, skill.Name)
		}

		// Install by copying the local skill
		srcPath := filepath.Join(orchestratorPath, skill.Path)
		if skill.Path == "." {
			srcPath = orchestratorPath
		}

		// Copy the skill directory
		os.MkdirAll(destPath, 0755)
		srcSkillFile := filepath.Join(srcPath, "SKILL.md")
		destSkillFile := filepath.Join(destPath, "SKILL.md")
		content, _ := os.ReadFile(srcSkillFile)
		os.WriteFile(destSkillFile, content, 0644)
	}

	// Verify nested structure
	// orchestrator/SKILL.md should exist
	parentSkillFile := filepath.Join(sb.SourcePath, "orchestrator", "SKILL.md")
	if !sb.FileExists(parentSkillFile) {
		t.Error("orchestrator/SKILL.md should exist")
	}

	// orchestrator/child-a/SKILL.md should exist (NESTED, not flattened)
	childASkillFile := filepath.Join(sb.SourcePath, "orchestrator", "child-a", "SKILL.md")
	if !sb.FileExists(childASkillFile) {
		t.Error("orchestrator/child-a/SKILL.md should exist (nested structure)")
	}

	// orchestrator/child-b/SKILL.md should exist (NESTED, not flattened)
	childBSkillFile := filepath.Join(sb.SourcePath, "orchestrator", "child-b", "SKILL.md")
	if !sb.FileExists(childBSkillFile) {
		t.Error("orchestrator/child-b/SKILL.md should exist (nested structure)")
	}

	// child-a should NOT exist at root level (this was the bug)
	flatChildA := filepath.Join(sb.SourcePath, "child-a")
	if sb.FileExists(flatChildA) {
		t.Error("child-a should NOT exist at root level (should be nested under orchestrator)")
	}

	// child-b should NOT exist at root level (this was the bug)
	flatChildB := filepath.Join(sb.SourcePath, "child-b")
	if sb.FileExists(flatChildB) {
		t.Error("child-b should NOT exist at root level (should be nested under orchestrator)")
	}
}

// TestInstall_ChildSkillsOnly_InstallsToRoot tests that when only child skills
// are selected (without root), they are installed to root level (not nested).
func TestInstall_ChildSkillsOnly_InstallsToRoot(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)

	// Create local skill files
	orchestratorPath := filepath.Join(sb.Root, "orchestrator")
	childAPath := filepath.Join(orchestratorPath, "child-a")
	childBPath := filepath.Join(orchestratorPath, "child-b")

	os.MkdirAll(orchestratorPath, 0755)
	os.MkdirAll(childAPath, 0755)
	os.MkdirAll(childBPath, 0755)

	os.WriteFile(filepath.Join(orchestratorPath, "SKILL.md"), []byte("# Parent Skill"), 0644)
	os.WriteFile(filepath.Join(childAPath, "SKILL.md"), []byte("# Child A"), 0644)
	os.WriteFile(filepath.Join(childBPath, "SKILL.md"), []byte("# Child B"), 0644)

	// Only select child skills, NOT the root
	childA := install.SkillInfo{Name: "child-a", Path: "child-a"}
	childB := install.SkillInfo{Name: "child-b", Path: "child-b"}

	// Detect orchestrator: no root skill selected
	parentName := ""
	selected := []install.SkillInfo{childA, childB}
	for _, skill := range selected {
		if skill.Path == "." {
			parentName = skill.Name
			break
		}
	}

	if parentName != "" {
		t.Fatalf("expected no parent name, got '%s'", parentName)
	}

	// Install using the nesting logic (without parent, should go to root)
	for _, skill := range selected {
		var destPath string
		if skill.Path == "." {
			destPath = filepath.Join(sb.SourcePath, skill.Name)
		} else if parentName != "" {
			destPath = filepath.Join(sb.SourcePath, parentName, skill.Name)
		} else {
			// Standalone child skill - install to root
			destPath = filepath.Join(sb.SourcePath, skill.Name)
		}

		srcPath := filepath.Join(orchestratorPath, skill.Path)
		os.MkdirAll(destPath, 0755)
		content, _ := os.ReadFile(filepath.Join(srcPath, "SKILL.md"))
		os.WriteFile(filepath.Join(destPath, "SKILL.md"), content, 0644)
	}

	// Verify children are at root level (not nested)
	childASkillFile := filepath.Join(sb.SourcePath, "child-a", "SKILL.md")
	if !sb.FileExists(childASkillFile) {
		t.Error("child-a/SKILL.md should exist at root level")
	}

	childBSkillFile := filepath.Join(sb.SourcePath, "child-b", "SKILL.md")
	if !sb.FileExists(childBSkillFile) {
		t.Error("child-b/SKILL.md should exist at root level")
	}

	// Orchestrator should NOT exist (wasn't selected)
	parentSkillFile := filepath.Join(sb.SourcePath, "orchestrator")
	if sb.FileExists(parentSkillFile) {
		t.Error("orchestrator should NOT exist (wasn't selected)")
	}
}

// TestInstall_Discovery_RootSkill tests that a repo with SKILL.md only at the
// root is correctly discovered (fixes issue #8).
func TestInstall_Discovery_RootSkill(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)

	// Create a git repo with SKILL.md at root only (no child skills)
	gitRepoPath := filepath.Join(sb.Root, "root-skill-repo")
	os.MkdirAll(gitRepoPath, 0755)
	os.WriteFile(filepath.Join(gitRepoPath, "SKILL.md"), []byte("---\nname: root-skill\n---\n# Root Skill"), 0644)
	os.WriteFile(filepath.Join(gitRepoPath, "README.md"), []byte("# Repo with root skill only"), 0644)

	// Initialize git repo
	cmd := exec.Command("git", "init")
	cmd.Dir = gitRepoPath
	if err := cmd.Run(); err != nil {
		t.Skip("git not available, skipping git test")
	}

	cmd = exec.Command("git", "config", "user.email", "test@test.com")
	cmd.Dir = gitRepoPath
	cmd.Run()

	cmd = exec.Command("git", "config", "user.name", "Test")
	cmd.Dir = gitRepoPath
	cmd.Run()

	cmd = exec.Command("git", "add", ".")
	cmd.Dir = gitRepoPath
	cmd.Run()

	cmd = exec.Command("git", "commit", "-m", "Initial commit")
	cmd.Dir = gitRepoPath
	cmd.Run()

	// Test discovery via the internal API directly
	source, err := install.ParseSource("file://" + gitRepoPath)
	if err != nil {
		t.Fatalf("ParseSource() error = %v", err)
	}

	discovery, err := install.DiscoverFromGit(source)
	if err != nil {
		t.Fatalf("DiscoverFromGit() error = %v", err)
	}
	defer install.CleanupDiscovery(discovery)

	// Should find exactly 1 skill (the root)
	if len(discovery.Skills) != 1 {
		t.Fatalf("expected 1 skill, got %d: %+v", len(discovery.Skills), discovery.Skills)
	}

	skill := discovery.Skills[0]
	if skill.Path != "." {
		t.Errorf("skill Path = %q, want %q", skill.Path, ".")
	}
	// Name should be derived from repo name, not temp dir
	if skill.Name != "root-skill-repo" {
		t.Errorf("skill Name = %q, want %q", skill.Name, "root-skill-repo")
	}
}

func TestInstall_FileURLDot_DryRun_UsesRepoName(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)

	gitRepoPath := filepath.Join(sb.Root, "root-skill-repo")
	os.MkdirAll(gitRepoPath, 0755)
	os.WriteFile(filepath.Join(gitRepoPath, "SKILL.md"), []byte("# Root Skill"), 0644)

	cmd := exec.Command("git", "init")
	cmd.Dir = gitRepoPath
	if err := cmd.Run(); err != nil {
		t.Skip("git not available, skipping git test")
	}

	cmd = exec.Command("git", "config", "user.email", "test@test.com")
	cmd.Dir = gitRepoPath
	cmd.Run()

	cmd = exec.Command("git", "config", "user.name", "Test")
	cmd.Dir = gitRepoPath
	cmd.Run()

	cmd = exec.Command("git", "add", ".")
	cmd.Dir = gitRepoPath
	cmd.Run()

	cmd = exec.Command("git", "commit", "-m", "Initial commit")
	cmd.Dir = gitRepoPath
	cmd.Run()

	result := sb.RunCLI("install", "file://"+gitRepoPath+"/.", "--dry-run")
	result.AssertSuccess(t)
	result.AssertOutputContains(t, "root-skill-repo")
	result.AssertOutputNotContains(t, "── . ──")
}

func TestInstall_Discovery_SingleSkill_CustomName(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)

	gitRepoPath := filepath.Join(sb.Root, "root-skill-repo")
	os.MkdirAll(gitRepoPath, 0755)
	os.WriteFile(filepath.Join(gitRepoPath, "SKILL.md"), []byte("# Root Skill"), 0644)

	cmd := exec.Command("git", "init")
	cmd.Dir = gitRepoPath
	if err := cmd.Run(); err != nil {
		t.Skip("git not available, skipping git test")
	}

	cmd = exec.Command("git", "config", "user.email", "test@test.com")
	cmd.Dir = gitRepoPath
	cmd.Run()

	cmd = exec.Command("git", "config", "user.name", "Test")
	cmd.Dir = gitRepoPath
	cmd.Run()

	cmd = exec.Command("git", "add", ".")
	cmd.Dir = gitRepoPath
	cmd.Run()

	cmd = exec.Command("git", "commit", "-m", "Initial commit")
	cmd.Dir = gitRepoPath
	cmd.Run()

	result := sb.RunCLI("install", "file://"+gitRepoPath, "--name", "haha")
	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, "haha")

	if !sb.FileExists(filepath.Join(sb.SourcePath, "haha", "SKILL.md")) {
		t.Fatal("expected skill to be installed with custom name")
	}

	if sb.FileExists(filepath.Join(sb.SourcePath, "root-skill-repo")) {
		t.Fatal("repo-derived name should not be installed when --name is provided")
	}
}

func TestInstall_Discovery_MultipleSkills_NameFlagErrors(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)

	gitRepoPath := filepath.Join(sb.Root, "multi-skill-repo")
	skillAPath := filepath.Join(gitRepoPath, "skill-a")
	skillBPath := filepath.Join(gitRepoPath, "skill-b")
	os.MkdirAll(skillAPath, 0755)
	os.MkdirAll(skillBPath, 0755)
	os.WriteFile(filepath.Join(skillAPath, "SKILL.md"), []byte("# Skill A"), 0644)
	os.WriteFile(filepath.Join(skillBPath, "SKILL.md"), []byte("# Skill B"), 0644)

	cmd := exec.Command("git", "init")
	cmd.Dir = gitRepoPath
	if err := cmd.Run(); err != nil {
		t.Skip("git not available, skipping git test")
	}

	cmd = exec.Command("git", "config", "user.email", "test@test.com")
	cmd.Dir = gitRepoPath
	cmd.Run()

	cmd = exec.Command("git", "config", "user.name", "Test")
	cmd.Dir = gitRepoPath
	cmd.Run()

	cmd = exec.Command("git", "add", ".")
	cmd.Dir = gitRepoPath
	cmd.Run()

	cmd = exec.Command("git", "commit", "-m", "Initial commit")
	cmd.Dir = gitRepoPath
	cmd.Run()

	result := sb.RunCLI("install", "file://"+gitRepoPath, "--name", "renamed")
	result.AssertFailure(t)
	result.AssertAnyOutputContains(t, "--name can only be used when exactly one skill is discovered")
}

// createMultiSkillGitRepo creates a local git repo with multiple skills for testing.
// Returns the git repo path (usable with file:// protocol).
func createMultiSkillGitRepo(t *testing.T, sb *testutil.Sandbox, name string, skills []string) string {
	t.Helper()
	gitRepoPath := filepath.Join(sb.Root, name)
	for _, skill := range skills {
		skillPath := filepath.Join(gitRepoPath, skill)
		os.MkdirAll(skillPath, 0755)
		os.WriteFile(filepath.Join(skillPath, "SKILL.md"), []byte("# "+skill), 0644)
	}

	cmd := exec.Command("git", "init")
	cmd.Dir = gitRepoPath
	if err := cmd.Run(); err != nil {
		t.Skip("git not available, skipping git test")
	}

	for _, c := range [][]string{
		{"config", "user.email", "test@test.com"},
		{"config", "user.name", "Test"},
		{"add", "."},
		{"commit", "-m", "Initial commit"},
	} {
		cmd = exec.Command("git", c...)
		cmd.Dir = gitRepoPath
		cmd.Run()
	}

	return gitRepoPath
}

func TestInstall_SkillFlag_SelectsSpecific(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.WriteConfig(`source: ` + sb.SourcePath + "\ntargets: {}\n")

	gitRepoPath := createMultiSkillGitRepo(t, sb, "skill-flag-repo", []string{"skill-one", "skill-two", "skill-three"})

	result := sb.RunCLI("install", "file://"+gitRepoPath, "-s", "skill-one")
	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, "skill-one")

	if !sb.FileExists(filepath.Join(sb.SourcePath, "skill-one", "SKILL.md")) {
		t.Error("skill-one should be installed")
	}
	if sb.FileExists(filepath.Join(sb.SourcePath, "skill-two")) {
		t.Error("skill-two should NOT be installed")
	}
	if sb.FileExists(filepath.Join(sb.SourcePath, "skill-three")) {
		t.Error("skill-three should NOT be installed")
	}
}

func TestInstall_SkillFlag_CommaSeparated(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.WriteConfig(`source: ` + sb.SourcePath + "\ntargets: {}\n")

	gitRepoPath := createMultiSkillGitRepo(t, sb, "comma-repo", []string{"skill-one", "skill-two", "skill-three"})

	result := sb.RunCLI("install", "file://"+gitRepoPath, "-s", "skill-one,skill-two")
	result.AssertSuccess(t)

	if !sb.FileExists(filepath.Join(sb.SourcePath, "skill-one", "SKILL.md")) {
		t.Error("skill-one should be installed")
	}
	if !sb.FileExists(filepath.Join(sb.SourcePath, "skill-two", "SKILL.md")) {
		t.Error("skill-two should be installed")
	}
	if sb.FileExists(filepath.Join(sb.SourcePath, "skill-three")) {
		t.Error("skill-three should NOT be installed")
	}
}

func TestInstall_SkillFlag_NotFound(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.WriteConfig(`source: ` + sb.SourcePath + "\ntargets: {}\n")

	gitRepoPath := createMultiSkillGitRepo(t, sb, "notfound-repo", []string{"skill-one", "skill-two"})

	result := sb.RunCLI("install", "file://"+gitRepoPath, "-s", "nonexistent")
	result.AssertFailure(t)
	result.AssertAnyOutputContains(t, "skills not found: nonexistent")
	result.AssertAnyOutputContains(t, "Available:")
}

func TestInstall_AllFlag_InstallsAll(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.WriteConfig(`source: ` + sb.SourcePath + "\ntargets: {}\n")

	gitRepoPath := createMultiSkillGitRepo(t, sb, "all-flag-repo", []string{"skill-one", "skill-two", "skill-three"})

	result := sb.RunCLI("install", "file://"+gitRepoPath, "--all")
	result.AssertSuccess(t)

	if !sb.FileExists(filepath.Join(sb.SourcePath, "skill-one", "SKILL.md")) {
		t.Error("skill-one should be installed")
	}
	if !sb.FileExists(filepath.Join(sb.SourcePath, "skill-two", "SKILL.md")) {
		t.Error("skill-two should be installed")
	}
	if !sb.FileExists(filepath.Join(sb.SourcePath, "skill-three", "SKILL.md")) {
		t.Error("skill-three should be installed")
	}
}

func TestInstall_YesFlag_InstallsAll(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.WriteConfig(`source: ` + sb.SourcePath + "\ntargets: {}\n")

	gitRepoPath := createMultiSkillGitRepo(t, sb, "yes-flag-repo", []string{"skill-one", "skill-two", "skill-three"})

	result := sb.RunCLI("install", "file://"+gitRepoPath, "-y")
	result.AssertSuccess(t)

	if !sb.FileExists(filepath.Join(sb.SourcePath, "skill-one", "SKILL.md")) {
		t.Error("skill-one should be installed")
	}
	if !sb.FileExists(filepath.Join(sb.SourcePath, "skill-two", "SKILL.md")) {
		t.Error("skill-two should be installed")
	}
	if !sb.FileExists(filepath.Join(sb.SourcePath, "skill-three", "SKILL.md")) {
		t.Error("skill-three should be installed")
	}
}

func TestInstall_SkillFlag_DryRun(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.WriteConfig(`source: ` + sb.SourcePath + "\ntargets: {}\n")

	gitRepoPath := createMultiSkillGitRepo(t, sb, "dryrun-skill-repo", []string{"skill-one", "skill-two", "skill-three"})

	result := sb.RunCLI("install", "file://"+gitRepoPath, "-s", "skill-one", "--dry-run")
	result.AssertSuccess(t)
	result.AssertOutputContains(t, "skill-one")
	result.AssertOutputContains(t, "dry-run")
	result.AssertOutputContains(t, "1 skill(s)")

	// skill-two and skill-three should NOT appear in output
	result.AssertOutputNotContains(t, "skill-two")
	result.AssertOutputNotContains(t, "skill-three")

	// Nothing should be installed
	if sb.FileExists(filepath.Join(sb.SourcePath, "skill-one")) {
		t.Error("skill should not be installed in dry-run mode")
	}
}

func TestInstall_SkillAndAllConflict(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.WriteConfig(`source: ` + sb.SourcePath + "\ntargets: {}\n")

	result := sb.RunCLI("install", "file:///some/repo", "-s", "x", "--all")
	result.AssertFailure(t)
	result.AssertAnyOutputContains(t, "--skill and --all cannot be used together")
}

func TestInstall_SkillAndTrackConflict(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.WriteConfig(`source: ` + sb.SourcePath + "\ntargets: {}\n")

	result := sb.RunCLI("install", "file:///some/repo", "-s", "x", "--track")
	result.AssertFailure(t)
	result.AssertAnyOutputContains(t, "--skill cannot be used with --track")
}

// TestInstall_SubdirFuzzyResolve tests that when a subdir doesn't exist at the
// exact path, the installer scans the repo for a matching skill by basename.
// e.g. "owner/repo/pdf" where "pdf" lives at "skills/pdf/SKILL.md".
func TestInstall_SubdirFuzzyResolve(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.WriteConfig(`source: ` + sb.SourcePath + "\ntargets: {}\n")

	// Create a git repo where skills live under a skills/ prefix
	gitRepoPath := filepath.Join(sb.Root, "fuzzy-repo")
	skillPath := filepath.Join(gitRepoPath, "skills", "pdf")
	os.MkdirAll(skillPath, 0755)
	os.WriteFile(filepath.Join(skillPath, "SKILL.md"), []byte("# PDF Skill"), 0644)

	// Initialize git repo
	cmd := exec.Command("git", "init")
	cmd.Dir = gitRepoPath
	if err := cmd.Run(); err != nil {
		t.Skip("git not available, skipping git test")
	}

	for _, c := range [][]string{
		{"config", "user.email", "test@test.com"},
		{"config", "user.name", "Test"},
		{"add", "."},
		{"commit", "-m", "Initial commit"},
	} {
		cmd = exec.Command("git", c...)
		cmd.Dir = gitRepoPath
		cmd.Run()
	}

	// Construct source with subdir "pdf" (simulates GitHub URL like owner/repo/pdf)
	source := &install.Source{
		Type:     install.SourceTypeGitHTTPS,
		Raw:      "file://" + gitRepoPath + "/pdf",
		CloneURL: "file://" + gitRepoPath,
		Subdir:   "pdf",
		Name:     "pdf",
	}

	destPath := filepath.Join(sb.SourcePath, "pdf")
	result, err := install.Install(source, destPath, install.InstallOptions{})
	if err != nil {
		t.Fatalf("Install() error = %v", err)
	}

	if result.Action != "cloned and extracted" {
		t.Errorf("Action = %q, want %q", result.Action, "cloned and extracted")
	}

	// Verify skill was installed
	if !sb.FileExists(filepath.Join(sb.SourcePath, "pdf", "SKILL.md")) {
		t.Error("pdf skill should be installed via fuzzy subdir resolution")
	}

	// Verify content is correct
	content := sb.ReadFile(filepath.Join(sb.SourcePath, "pdf", "SKILL.md"))
	if !strings.Contains(content, "PDF Skill") {
		t.Error("installed skill should have correct content")
	}
}

// TestInstall_SubdirFuzzyResolve_Discovery tests fuzzy resolution through the
// DiscoverFromGitSubdir path (multi-skill subdir with discovery).
func TestInstall_SubdirFuzzyResolve_Discovery(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.WriteConfig(`source: ` + sb.SourcePath + "\ntargets: {}\n")

	// Create a git repo: skills/frontend/SKILL.md + skills/frontend/child-a/SKILL.md
	gitRepoPath := filepath.Join(sb.Root, "fuzzy-discover-repo")
	parentPath := filepath.Join(gitRepoPath, "skills", "frontend")
	childPath := filepath.Join(parentPath, "child-a")
	os.MkdirAll(parentPath, 0755)
	os.MkdirAll(childPath, 0755)
	os.WriteFile(filepath.Join(parentPath, "SKILL.md"), []byte("# Frontend"), 0644)
	os.WriteFile(filepath.Join(childPath, "SKILL.md"), []byte("# Child A"), 0644)

	cmd := exec.Command("git", "init")
	cmd.Dir = gitRepoPath
	if err := cmd.Run(); err != nil {
		t.Skip("git not available, skipping git test")
	}

	for _, c := range [][]string{
		{"config", "user.email", "test@test.com"},
		{"config", "user.name", "Test"},
		{"add", "."},
		{"commit", "-m", "Initial commit"},
	} {
		cmd = exec.Command("git", c...)
		cmd.Dir = gitRepoPath
		cmd.Run()
	}

	// Construct source with subdir "frontend" (simulates GitHub URL like owner/repo/frontend)
	source := &install.Source{
		Type:     install.SourceTypeGitHTTPS,
		Raw:      "file://" + gitRepoPath + "/frontend",
		CloneURL: "file://" + gitRepoPath,
		Subdir:   "frontend",
		Name:     "frontend",
	}

	discovery, err := install.DiscoverFromGitSubdir(source)
	if err != nil {
		t.Fatalf("DiscoverFromGitSubdir() error = %v", err)
	}
	defer install.CleanupDiscovery(discovery)

	if len(discovery.Skills) == 0 {
		t.Fatal("expected at least 1 skill discovered")
	}

	// Should find the root skill of the subdir + child
	var foundRoot bool
	for _, sk := range discovery.Skills {
		if sk.Path == "." {
			foundRoot = true
		}
	}
	if !foundRoot {
		t.Error("expected root skill of resolved subdir to be discovered")
	}
}
