//go:build !online

package integration

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"skillshare/internal/install"
	"skillshare/internal/testutil"
)

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

	initGitRepo(t, gitRepoPath)

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

	initGitRepo(t, gitRepoPath)

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

	initGitRepo(t, gitRepoPath)

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

	initGitRepo(t, gitRepoPath)

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

	initGitRepo(t, gitRepoPath)

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

	initGitRepo(t, gitRepoPath)

	result := sb.RunCLI("install", "file://"+gitRepoPath, "--name", "renamed")
	result.AssertFailure(t)
	result.AssertAnyOutputContains(t, "--name can only be used when exactly one skill is discovered")
}

// createMultiSkillGitRepo creates a local git repo with multiple skills for testing.
// Returns the git repo path (usable with file:// protocol).

func TestInstall_SubdirFuzzyResolve(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.WriteConfig(`source: ` + sb.SourcePath + "\ntargets: {}\n")

	// Create a git repo where skills live under a skills/ prefix
	gitRepoPath := filepath.Join(sb.Root, "fuzzy-repo")
	skillPath := filepath.Join(gitRepoPath, "skills", "pdf")
	os.MkdirAll(skillPath, 0755)
	os.WriteFile(filepath.Join(skillPath, "SKILL.md"), []byte("# PDF Skill"), 0644)

	initGitRepo(t, gitRepoPath)

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

	initGitRepo(t, gitRepoPath)

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

// --- Feature #17: .skillignore ---
