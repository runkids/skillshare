//go:build !online

package integration

import (
	"os"
	"path/filepath"
	"testing"

	"skillshare/internal/install"
	"skillshare/internal/testutil"
)

func TestInstall_SkillIgnore_ExcludesMatchedSkills(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.WriteConfig(`source: ` + sb.SourcePath + "\ntargets: {}\n")

	// Create a git repo with multiple skills and a .skillignore
	gitRepoPath := filepath.Join(sb.Root, "ignore-repo")
	for _, name := range []string{"skill-a", "skill-b", "internal-tool"} {
		p := filepath.Join(gitRepoPath, name)
		os.MkdirAll(p, 0755)
		os.WriteFile(filepath.Join(p, "SKILL.md"), []byte("# "+name), 0644)
	}
	os.WriteFile(filepath.Join(gitRepoPath, ".skillignore"), []byte("# Exclude internal tools\ninternal-tool\n"), 0644)

	initGitRepo(t, gitRepoPath)

	// Discover — internal-tool should be excluded
	source, _ := install.ParseSource("file://" + gitRepoPath)
	discovery, err := install.DiscoverFromGit(source)
	if err != nil {
		t.Fatalf("DiscoverFromGit() error = %v", err)
	}
	defer install.CleanupDiscovery(discovery)

	for _, sk := range discovery.Skills {
		if sk.Name == "internal-tool" {
			t.Error("internal-tool should be excluded by .skillignore")
		}
	}
	if len(discovery.Skills) != 2 {
		t.Errorf("expected 2 skills, got %d", len(discovery.Skills))
	}
}

func TestInstall_SkillIgnore_WildcardPattern(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.WriteConfig(`source: ` + sb.SourcePath + "\ntargets: {}\n")

	gitRepoPath := filepath.Join(sb.Root, "wildcard-repo")
	for _, name := range []string{"prod-skill", "test-one", "test-two"} {
		p := filepath.Join(gitRepoPath, name)
		os.MkdirAll(p, 0755)
		os.WriteFile(filepath.Join(p, "SKILL.md"), []byte("# "+name), 0644)
	}
	os.WriteFile(filepath.Join(gitRepoPath, ".skillignore"), []byte("test-*\n"), 0644)

	initGitRepo(t, gitRepoPath)

	source, _ := install.ParseSource("file://" + gitRepoPath)
	discovery, err := install.DiscoverFromGit(source)
	if err != nil {
		t.Fatalf("DiscoverFromGit() error = %v", err)
	}
	defer install.CleanupDiscovery(discovery)

	if len(discovery.Skills) != 1 {
		t.Errorf("expected 1 skill (prod-skill), got %d: %+v", len(discovery.Skills), discovery.Skills)
	}
	if len(discovery.Skills) == 1 && discovery.Skills[0].Name != "prod-skill" {
		t.Errorf("expected prod-skill, got %s", discovery.Skills[0].Name)
	}
}

// --- Feature #18: --exclude ---

func TestInstall_ExcludeFlag_SkipsExcluded(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.WriteConfig(`source: ` + sb.SourcePath + "\ntargets: {}\n")

	gitRepoPath := createMultiSkillGitRepo(t, sb, "exclude-repo", []string{"keep-a", "keep-b", "skip-me"})

	result := sb.RunCLI("install", "file://"+gitRepoPath, "--all", "--exclude", "skip-me")
	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, "Excluded 1 skill")

	if !sb.FileExists(filepath.Join(sb.SourcePath, "keep-a", "SKILL.md")) {
		t.Error("keep-a should be installed")
	}
	if !sb.FileExists(filepath.Join(sb.SourcePath, "keep-b", "SKILL.md")) {
		t.Error("keep-b should be installed")
	}
	if sb.FileExists(filepath.Join(sb.SourcePath, "skip-me")) {
		t.Error("skip-me should NOT be installed (--exclude)")
	}
}

func TestInstall_ExcludeFlag_CommaSeparated(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.WriteConfig(`source: ` + sb.SourcePath + "\ntargets: {}\n")

	gitRepoPath := createMultiSkillGitRepo(t, sb, "exclude-multi-repo", []string{"keep", "drop-a", "drop-b"})

	result := sb.RunCLI("install", "file://"+gitRepoPath, "--all", "--exclude", "drop-a,drop-b")
	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, "Excluded 2 skill")

	if !sb.FileExists(filepath.Join(sb.SourcePath, "keep", "SKILL.md")) {
		t.Error("keep should be installed")
	}
	if sb.FileExists(filepath.Join(sb.SourcePath, "drop-a")) {
		t.Error("drop-a should NOT be installed")
	}
	if sb.FileExists(filepath.Join(sb.SourcePath, "drop-b")) {
		t.Error("drop-b should NOT be installed")
	}
}

// --- Feature #8: License display ---

func TestInstall_LicenseDisplay_ShowsInOutput(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.WriteConfig(`source: ` + sb.SourcePath + "\ntargets: {}\n")

	// Create a git repo with a skill that has a license field
	gitRepoPath := filepath.Join(sb.Root, "license-repo")
	skillPath := filepath.Join(gitRepoPath, "licensed-skill")
	os.MkdirAll(skillPath, 0755)
	os.WriteFile(filepath.Join(skillPath, "SKILL.md"), []byte("---\nname: licensed-skill\nlicense: MIT\n---\n# Licensed Skill"), 0644)

	initGitRepo(t, gitRepoPath)

	// Single skill with license — should show license info
	result := sb.RunCLI("install", "file://"+gitRepoPath)
	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, "MIT")
}

func TestInstall_LicenseDiscovery_PopulatesField(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.WriteConfig(`source: ` + sb.SourcePath + "\ntargets: {}\n")

	gitRepoPath := filepath.Join(sb.Root, "license-discover-repo")
	for _, s := range []struct{ name, license string }{
		{"mit-skill", "MIT"},
		{"apache-skill", "Apache-2.0"},
		{"no-license", ""},
	} {
		p := filepath.Join(gitRepoPath, s.name)
		os.MkdirAll(p, 0755)
		content := "---\nname: " + s.name + "\n"
		if s.license != "" {
			content += "license: " + s.license + "\n"
		}
		content += "---\n# " + s.name
		os.WriteFile(filepath.Join(p, "SKILL.md"), []byte(content), 0644)
	}

	initGitRepo(t, gitRepoPath)

	source, _ := install.ParseSource("file://" + gitRepoPath)
	discovery, err := install.DiscoverFromGit(source)
	if err != nil {
		t.Fatalf("DiscoverFromGit() error = %v", err)
	}
	defer install.CleanupDiscovery(discovery)

	licenseMap := map[string]string{}
	for _, sk := range discovery.Skills {
		licenseMap[sk.Name] = sk.License
	}

	if licenseMap["mit-skill"] != "MIT" {
		t.Errorf("mit-skill license = %q, want MIT", licenseMap["mit-skill"])
	}
	if licenseMap["apache-skill"] != "Apache-2.0" {
		t.Errorf("apache-skill license = %q, want Apache-2.0", licenseMap["apache-skill"])
	}
	if licenseMap["no-license"] != "" {
		t.Errorf("no-license should have empty license, got %q", licenseMap["no-license"])
	}
}

// --- --exclude warning on direct install ---

func TestInstall_ExcludeFlag_WarnsOnDirectInstall(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.WriteConfig(`source: ` + sb.SourcePath + "\ntargets: {}\n")

	// Create a local skill directory (not a git repo)
	localSkill := filepath.Join(sb.Root, "local-skill")
	os.MkdirAll(localSkill, 0755)
	os.WriteFile(filepath.Join(localSkill, "SKILL.md"), []byte("---\nname: local-skill\n---\n# Local"), 0644)

	result := sb.RunCLI("install", localSkill, "--exclude", "something")
	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, "only supported for multi-skill repos")
}
