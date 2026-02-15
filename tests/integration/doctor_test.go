//go:build !online

package integration

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"skillshare/internal/testutil"
)

func TestDoctor_AllGood_PassesAll(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.CreateSkill("skill1", map[string]string{"SKILL.md": "# Skill 1"})
	targetPath := sb.CreateTarget("claude")

	// Initialize git and commit to avoid warnings
	cmd := exec.Command("git", "init")
	cmd.Dir = sb.SourcePath
	if err := cmd.Run(); err != nil {
		t.Skip("git not available")
	}

	cmd = exec.Command("git", "config", "user.email", "test@test.com")
	cmd.Dir = sb.SourcePath
	cmd.Run()

	cmd = exec.Command("git", "config", "user.name", "Test")
	cmd.Dir = sb.SourcePath
	cmd.Run()

	cmd = exec.Command("git", "add", "-A")
	cmd.Dir = sb.SourcePath
	cmd.Run()

	cmd = exec.Command("git", "commit", "-m", "initial")
	cmd.Dir = sb.SourcePath
	cmd.Run()

	// Create synced state
	os.Symlink(filepath.Join(sb.SourcePath, "skill1"), filepath.Join(targetPath, "skill1"))

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets:
  claude:
    path: ` + targetPath + `
`)

	result := sb.RunCLI("doctor")

	result.AssertSuccess(t)
	result.AssertOutputContains(t, "All checks passed")
}

func TestDoctor_NoConfig_ShowsError(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	os.Remove(sb.ConfigPath)

	result := sb.RunCLI("doctor")

	result.AssertSuccess(t) // doctor doesn't return error, it reports issues
	result.AssertOutputContains(t, "not found")
}

func TestDoctor_NoSource_ShowsError(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	// Remove source directory
	os.RemoveAll(sb.SourcePath)

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)

	result := sb.RunCLI("doctor")

	result.AssertSuccess(t)
	result.AssertOutputContains(t, "Source not found")
}

func TestDoctor_ChecksSymlinkSupport(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)

	result := sb.RunCLI("doctor")

	result.AssertSuccess(t)
	result.AssertOutputContains(t, "Symlink")
}

func TestDoctor_TargetIssues_ShowsProblems(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	// Point to non-existent directory with non-existent parent
	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets:
  broken:
    path: /nonexistent/path/skills
`)

	result := sb.RunCLI("doctor")

	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, "parent directory not found")
}

func TestDoctor_WrongSymlink_ShowsWarning(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	// Create wrong symlink
	wrongSource := filepath.Join(sb.Home, "wrong-source")
	os.MkdirAll(wrongSource, 0755)

	targetPath := filepath.Join(sb.Home, ".claude", "skills")
	os.MkdirAll(filepath.Dir(targetPath), 0755)
	os.Symlink(wrongSource, targetPath)

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets:
  claude:
    path: ` + targetPath + `
    mode: symlink
`)

	result := sb.RunCLI("doctor")

	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, "wrong location")
}

func TestDoctor_ShowsSkillCount(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.CreateSkill("skill1", map[string]string{"SKILL.md": "# Skill 1"})
	sb.CreateSkill("skill2", map[string]string{"SKILL.md": "# Skill 2"})
	sb.CreateSkill("skill3", map[string]string{"SKILL.md": "# Skill 3"})

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)

	result := sb.RunCLI("doctor")

	result.AssertSuccess(t)
	result.AssertOutputContains(t, "3 skills")
}

func TestDoctor_GitNotInitialized_ShowsWarning(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)

	result := sb.RunCLI("doctor")

	result.AssertSuccess(t)
	result.AssertOutputContains(t, "Git")
	result.AssertOutputContains(t, "not initialized")
}

func TestDoctor_GitInitialized_ShowsSuccess(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	// Initialize git in source
	cmd := exec.Command("git", "init")
	cmd.Dir = sb.SourcePath
	if err := cmd.Run(); err != nil {
		t.Skip("git not available")
	}

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)

	result := sb.RunCLI("doctor")

	result.AssertSuccess(t)
	result.AssertOutputContains(t, "Git")
	result.AssertOutputContains(t, "initialized")
}

func TestDoctor_GitUncommittedChanges_ShowsWarning(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	// Initialize git
	cmd := exec.Command("git", "init")
	cmd.Dir = sb.SourcePath
	if err := cmd.Run(); err != nil {
		t.Skip("git not available")
	}

	cmd = exec.Command("git", "config", "user.email", "test@test.com")
	cmd.Dir = sb.SourcePath
	cmd.Run()

	cmd = exec.Command("git", "config", "user.name", "Test")
	cmd.Dir = sb.SourcePath
	cmd.Run()

	// Create a skill (uncommitted)
	sb.CreateSkill("uncommitted", map[string]string{"SKILL.md": "# Uncommitted"})

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)

	result := sb.RunCLI("doctor")

	result.AssertSuccess(t)
	result.AssertOutputContains(t, "uncommitted")
}

func TestDoctor_SkillWithoutSKILLmd_ShowsWarning(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	// Create a skill with SKILL.md
	sb.CreateSkill("valid-skill", map[string]string{"SKILL.md": "# Valid"})

	// Create a directory without SKILL.md
	invalidSkill := filepath.Join(sb.SourcePath, "invalid-skill")
	os.MkdirAll(invalidSkill, 0755)
	os.WriteFile(filepath.Join(invalidSkill, "README.md"), []byte("# No SKILL.md"), 0644)

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)

	result := sb.RunCLI("doctor")

	result.AssertSuccess(t)
	result.AssertOutputContains(t, "without SKILL.md")
	result.AssertOutputContains(t, "invalid-skill")
}

func TestDoctor_BrokenSymlink_ShowsError(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	targetPath := sb.CreateTarget("claude")

	// Create a broken symlink
	brokenLink := filepath.Join(targetPath, "broken-skill")
	os.Symlink("/nonexistent/path", brokenLink)

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets:
  claude:
    path: ` + targetPath + `
`)

	result := sb.RunCLI("doctor")

	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, "broken symlink")
}

func TestDoctor_DuplicateSkills_ShowsWarning(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	// Create skill in source
	sb.CreateSkill("duplicate-skill", map[string]string{"SKILL.md": "# Source"})

	// Create target with local skill of same name (not symlink)
	// Use symlink mode - merge mode allows local skills by design
	targetPath := sb.CreateTarget("claude")
	localSkill := filepath.Join(targetPath, "duplicate-skill")
	os.MkdirAll(localSkill, 0755)
	os.WriteFile(filepath.Join(localSkill, "SKILL.md"), []byte("# Local"), 0644)

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets:
  claude:
    path: ` + targetPath + `
    mode: symlink
`)

	result := sb.RunCLI("doctor")

	result.AssertSuccess(t)
	result.AssertOutputContains(t, "Duplicate")
	result.AssertOutputContains(t, "duplicate-skill")
}

func TestDoctor_BackupExists_ShowsInfo(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	// Create backup directory
	backupDir := filepath.Join(filepath.Dir(sb.ConfigPath), "backups", "2026-01-16_12-00-00")
	os.MkdirAll(backupDir, 0755)
	os.WriteFile(filepath.Join(backupDir, "test"), []byte("backup"), 0644)

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)

	result := sb.RunCLI("doctor")

	result.AssertSuccess(t)
	result.AssertOutputContains(t, "Backup")
}

func TestDoctor_NoBackups_ShowsNone(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)

	result := sb.RunCLI("doctor")

	result.AssertSuccess(t)
	result.AssertOutputContains(t, "Backup")
	result.AssertOutputContains(t, "none")
}

func TestDoctor_ProjectMode_AutoDetectsProjectConfig(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	projectRoot := sb.SetupProjectDir("claude")
	sb.CreateProjectSkill(projectRoot, "project-skill", map[string]string{"SKILL.md": "# Project Skill"})

	result := sb.RunCLIInDir(projectRoot, "doctor")

	result.AssertSuccess(t)
	result.AssertOutputContains(t, "(project)")
	result.AssertOutputContains(t, ".skillshare/config.yaml")
	result.AssertOutputContains(t, ".skillshare/skills")
	result.AssertOutputContains(t, "Backups: not used in project mode")
}

func TestDoctor_ProjectMode_WithFlagUsesProjectConfig(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	projectRoot := sb.SetupProjectDir("claude")
	sb.CreateProjectSkill(projectRoot, "project-skill", map[string]string{"SKILL.md": "# Project Skill"})

	result := sb.RunCLIInDir(projectRoot, "doctor", "-p")

	result.AssertSuccess(t)
	result.AssertOutputContains(t, "(project)")
	result.AssertOutputContains(t, ".skillshare/config.yaml")
	result.AssertOutputContains(t, ".skillshare/skills")
}
