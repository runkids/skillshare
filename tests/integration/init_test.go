//go:build !online

package integration

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"skillshare/internal/testutil"
)

func TestInit_Fresh_CreatesConfigAndSource(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	// Remove config file to simulate fresh state
	os.Remove(sb.ConfigPath)

	// Run init with input to skip interactive prompts
	// Input: "2" to start fresh, "n" to skip adding other targets, "n" to skip git, "n" to skip skill
	result := sb.RunCLIWithInput("2\nn\nn\nn\n", "init")

	result.AssertSuccess(t)
	result.AssertOutputContains(t, "Initialized successfully")

	// Verify config was created
	if !sb.FileExists(sb.ConfigPath) {
		t.Error("config file should be created")
	}

	// Verify source directory was created
	if !sb.FileExists(sb.SourcePath) {
		t.Error("source directory should be created")
	}
}

func TestInit_WithSourceFlag_UsesCustomPath(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	// Remove config file
	os.Remove(sb.ConfigPath)

	customSource := filepath.Join(sb.Home, "my-skills")

	// Input: "1" to start fresh (no skills detected), "n" to skip git, "n" to skip skill
	result := sb.RunCLIWithInput("1\nn\nn\n", "init", "--source", customSource)

	result.AssertSuccess(t)
	result.AssertOutputContains(t, customSource)

	// Verify custom source was created
	if !sb.FileExists(customSource) {
		t.Error("custom source directory should be created")
	}
}

func TestInit_DryRun_DoesNotWrite(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	os.Remove(sb.ConfigPath)
	os.RemoveAll(sb.SourcePath)

	result := sb.RunCLIWithInput("n\nn\n", "init", "--dry-run")

	result.AssertSuccess(t)
	result.AssertOutputContains(t, "Dry run")

	if sb.FileExists(sb.ConfigPath) {
		t.Error("dry-run should not create config")
	}

	if sb.FileExists(sb.SourcePath) {
		t.Error("dry-run should not create source directory")
	}

	defaultSkillPath := filepath.Join(sb.SourcePath, "skillshare", "SKILL.md")
	if sb.FileExists(defaultSkillPath) {
		t.Error("dry-run should not create default skill")
	}
}

func TestInit_AlreadyInitialized_ReturnsError(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	// Create config to simulate already initialized
	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)

	result := sb.RunCLI("init")

	result.AssertFailure(t)
	result.AssertAnyOutputContains(t, "already initialized")
}

func TestInit_CreatesDefaultSkill(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	// Remove config file
	os.Remove(sb.ConfigPath)

	// Use flags for reliability (survey MultiSelect doesn't work well in non-TTY stdin)
	result := sb.RunCLI("init", "--no-copy", "--no-targets", "--no-git", "--skill")

	result.AssertSuccess(t)

	// Verify default skillshare skill was created
	defaultSkillPath := filepath.Join(sb.SourcePath, "skillshare", "SKILL.md")
	if !sb.FileExists(defaultSkillPath) {
		t.Error("default skillshare skill should be created")
	}
}

func TestInit_DetectsCLI_OffersImport(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	// Remove config file
	os.Remove(sb.ConfigPath)

	// Create existing claude skills directory with a skill
	claudeSkillsPath := filepath.Join(sb.Home, ".claude", "skills")
	os.MkdirAll(claudeSkillsPath, 0755)
	testSkillPath := filepath.Join(claudeSkillsPath, "test-skill")
	os.MkdirAll(testSkillPath, 0755)
	os.WriteFile(filepath.Join(testSkillPath, "SKILL.md"), []byte("# Test"), 0644)

	// Input: "2" to start fresh (not copy), "y" to add claude as target, "n" to skip git, "n" to skip skill
	result := sb.RunCLIWithInput("2\ny\nn\nn\n", "init")

	result.AssertSuccess(t)
	result.AssertOutputContains(t, "claude")
}

func TestInit_WithSkills_CopiesOnConfirm(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	// Remove config file
	os.Remove(sb.ConfigPath)

	// Create existing claude skills directory with a skill
	claudeSkillsPath := filepath.Join(sb.Home, ".claude", "skills")
	os.MkdirAll(claudeSkillsPath, 0755)
	testSkillPath := filepath.Join(claudeSkillsPath, "my-test-skill")
	os.MkdirAll(testSkillPath, 0755)
	os.WriteFile(filepath.Join(testSkillPath, "SKILL.md"), []byte("# My Test Skill"), 0644)

	// Input: "1" to copy from claude, "n" to skip git, "n" to skip skill
	result := sb.RunCLIWithInput("1\nn\nn\n", "init")

	result.AssertSuccess(t)
	result.AssertOutputContains(t, "Copy")

	// Check if skill was copied to source
	copiedSkillPath := filepath.Join(sb.SourcePath, "my-test-skill", "SKILL.md")
	if !sb.FileExists(copiedSkillPath) {
		t.Error("skill should be copied to source")
	}
}

func TestInit_AlreadyInitialized_RemoteFlag_AddsRemote(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	// Create config to simulate already initialized
	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)

	// Initialize git in source directory
	cmd := exec.Command("git", "init")
	cmd.Dir = sb.SourcePath
	if err := cmd.Run(); err != nil {
		t.Skip("git not available")
	}

	// Run init with --remote on already initialized setup
	result := sb.RunCLI("init", "--remote", "git@github.com:test/skills.git")

	result.AssertSuccess(t)
	result.AssertOutputContains(t, "Git remote configured")

	// Verify remote was added
	cmd = exec.Command("git", "remote", "-v")
	cmd.Dir = sb.SourcePath
	output, err := cmd.Output()
	if err != nil {
		t.Errorf("failed to check git remote: %v", err)
	}
	if !strings.Contains(string(output), "git@github.com:test/skills.git") {
		t.Errorf("remote should be configured, got: %s", output)
	}
}

func TestInit_AlreadyInitialized_RemoteFlag_DryRun(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)

	// Initialize git in source directory
	cmd := exec.Command("git", "init")
	cmd.Dir = sb.SourcePath
	if err := cmd.Run(); err != nil {
		t.Skip("git not available")
	}

	result := sb.RunCLI("init", "--remote", "git@github.com:test/skills.git", "--dry-run")

	result.AssertSuccess(t)
	result.AssertOutputContains(t, "Would add git remote")

	// Verify remote was NOT added
	cmd = exec.Command("git", "remote", "-v")
	cmd.Dir = sb.SourcePath
	output, _ := cmd.Output()
	if strings.Contains(string(output), "github.com") {
		t.Error("dry-run should not add remote")
	}
}

func TestInit_AlreadyInitialized_RemoteFlag_AlreadyExists(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)

	// Initialize git with existing remote
	cmd := exec.Command("git", "init")
	cmd.Dir = sb.SourcePath
	if err := cmd.Run(); err != nil {
		t.Skip("git not available")
	}

	cmd = exec.Command("git", "remote", "add", "origin", "git@github.com:existing/repo.git")
	cmd.Dir = sb.SourcePath
	cmd.Run()

	// Try to add different remote
	result := sb.RunCLI("init", "--remote", "git@github.com:new/repo.git")

	result.AssertSuccess(t)
	result.AssertOutputContains(t, "already exists")
	result.AssertOutputContains(t, "git remote set-url")
}

func TestInit_AlreadyInitialized_RemoteFlag_SameRemote(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)

	// Initialize git with existing remote
	cmd := exec.Command("git", "init")
	cmd.Dir = sb.SourcePath
	if err := cmd.Run(); err != nil {
		t.Skip("git not available")
	}

	cmd = exec.Command("git", "remote", "add", "origin", "git@github.com:test/skills.git")
	cmd.Dir = sb.SourcePath
	cmd.Run()

	// Try to add same remote
	result := sb.RunCLI("init", "--remote", "git@github.com:test/skills.git")

	result.AssertSuccess(t)
	result.AssertOutputContains(t, "already configured")
}

func TestInit_AlreadyInitialized_RemoteFlag_NoGit_AutoInitsGit(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	// Config exists but git is NOT initialized (simulates second machine setup)
	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)

	result := sb.RunCLI("init", "--remote", "git@github.com:test/skills.git")

	result.AssertSuccess(t)
	result.AssertOutputContains(t, "Git remote configured")

	// Verify git was initialized
	gitDir := filepath.Join(sb.SourcePath, ".git")
	if _, err := os.Stat(gitDir); os.IsNotExist(err) {
		t.Error("git should have been auto-initialized")
	}

	// Verify remote was added
	cmd := exec.Command("git", "remote", "-v")
	cmd.Dir = sb.SourcePath
	output, err := cmd.Output()
	if err != nil {
		t.Errorf("failed to check git remote: %v", err)
	}
	if !strings.Contains(string(output), "git@github.com:test/skills.git") {
		t.Errorf("remote should be configured, got: %s", output)
	}
}

// ============================================
// Non-interactive flag tests
// ============================================

func TestInit_NoCopy_StartsEmpty(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	os.Remove(sb.ConfigPath)

	// Create existing skills directory
	claudeSkillsPath := filepath.Join(sb.Home, ".claude", "skills")
	os.MkdirAll(claudeSkillsPath, 0755)
	testSkillPath := filepath.Join(claudeSkillsPath, "my-skill")
	os.MkdirAll(testSkillPath, 0755)
	os.WriteFile(filepath.Join(testSkillPath, "SKILL.md"), []byte("# Test"), 0644)

	// Run init with --no-copy and --no-targets to skip prompts
	result := sb.RunCLI("init", "--no-copy", "--no-targets", "--no-git", "--no-skill")

	result.AssertSuccess(t)
	result.AssertOutputContains(t, "--no-copy")

	// Verify skill was NOT copied (only skillshare skill should exist)
	copiedSkillPath := filepath.Join(sb.SourcePath, "my-skill")
	if sb.FileExists(copiedSkillPath) {
		t.Error("skill should NOT be copied when using --no-copy")
	}
}

func TestInit_CopyFromByName(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	os.Remove(sb.ConfigPath)

	// Create existing claude skills directory with a skill
	claudeSkillsPath := filepath.Join(sb.Home, ".claude", "skills")
	os.MkdirAll(claudeSkillsPath, 0755)
	testSkillPath := filepath.Join(claudeSkillsPath, "copy-test-skill")
	os.MkdirAll(testSkillPath, 0755)
	os.WriteFile(filepath.Join(testSkillPath, "SKILL.md"), []byte("# Copy Test"), 0644)

	// Run init with --copy-from claude
	result := sb.RunCLI("init", "--copy-from", "claude", "--no-targets", "--no-git", "--no-skill")

	result.AssertSuccess(t)
	result.AssertOutputContains(t, "matched by name")

	// Verify skill was copied
	copiedSkillPath := filepath.Join(sb.SourcePath, "copy-test-skill", "SKILL.md")
	if !sb.FileExists(copiedSkillPath) {
		t.Error("skill should be copied when using --copy-from claude")
	}
}

func TestInit_CopyFromByPath(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	os.Remove(sb.ConfigPath)

	// Create custom skills directory
	customPath := filepath.Join(sb.Home, "custom-skills")
	os.MkdirAll(customPath, 0755)
	testSkillPath := filepath.Join(customPath, "path-test-skill")
	os.MkdirAll(testSkillPath, 0755)
	os.WriteFile(filepath.Join(testSkillPath, "SKILL.md"), []byte("# Path Test"), 0644)

	// Run init with --copy-from as a path
	result := sb.RunCLI("init", "--copy-from", customPath, "--no-targets", "--no-git", "--no-skill")

	result.AssertSuccess(t)

	// Verify skill was copied
	copiedSkillPath := filepath.Join(sb.SourcePath, "path-test-skill", "SKILL.md")
	if !sb.FileExists(copiedSkillPath) {
		t.Error("skill should be copied when using --copy-from with path")
	}
}

func TestInit_TargetsCSV(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	os.Remove(sb.ConfigPath)

	// Create both claude and cursor directories
	claudeSkillsPath := filepath.Join(sb.Home, ".claude", "skills")
	os.MkdirAll(claudeSkillsPath, 0755)
	cursorSkillsPath := filepath.Join(sb.Home, ".cursor", "skills")
	os.MkdirAll(cursorSkillsPath, 0755)

	// Run init with --targets specifying both
	result := sb.RunCLI("init", "--no-copy", "--targets", "claude,cursor", "--no-git", "--no-skill")

	result.AssertSuccess(t)
	result.AssertOutputContains(t, "Added 2 targets")

	// Verify config has both targets
	configContent := sb.ReadFile(sb.ConfigPath)
	if !strings.Contains(configContent, "claude:") {
		t.Error("config should contain claude target")
	}
	if !strings.Contains(configContent, "cursor:") {
		t.Error("config should contain cursor target")
	}
}

func TestInit_AllTargets(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	os.Remove(sb.ConfigPath)

	// Create multiple CLI directories
	claudeSkillsPath := filepath.Join(sb.Home, ".claude", "skills")
	os.MkdirAll(claudeSkillsPath, 0755)
	cursorSkillsPath := filepath.Join(sb.Home, ".cursor", "skills")
	os.MkdirAll(cursorSkillsPath, 0755)

	// Run init with --all-targets
	result := sb.RunCLI("init", "--no-copy", "--all-targets", "--no-git", "--no-skill")

	result.AssertSuccess(t)
	result.AssertOutputContains(t, "--all-targets")

	// Verify config has targets
	configContent := sb.ReadFile(sb.ConfigPath)
	if !strings.Contains(configContent, "claude:") || !strings.Contains(configContent, "cursor:") {
		t.Errorf("config should contain all detected targets, got: %s", configContent)
	}
}

func TestInit_NoTargets(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	os.Remove(sb.ConfigPath)

	// Create a CLI directory
	claudeSkillsPath := filepath.Join(sb.Home, ".claude", "skills")
	os.MkdirAll(claudeSkillsPath, 0755)

	// Run init with --no-targets
	result := sb.RunCLI("init", "--no-copy", "--no-targets", "--no-git", "--no-skill")

	result.AssertSuccess(t)
	result.AssertOutputContains(t, "--no-targets")

	// Verify config has empty targets
	configContent := sb.ReadFile(sb.ConfigPath)
	if strings.Contains(configContent, "claude:") {
		t.Error("config should NOT contain any targets when using --no-targets")
	}
}

func TestInit_NoGit_SkipsGitInit(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	os.Remove(sb.ConfigPath)

	// Run init with --no-git
	result := sb.RunCLI("init", "--no-copy", "--no-targets", "--no-git", "--no-skill")

	result.AssertSuccess(t)
	result.AssertOutputContains(t, "--no-git")

	// Verify .git was NOT created
	gitDir := filepath.Join(sb.SourcePath, ".git")
	if sb.FileExists(gitDir) {
		t.Error(".git directory should NOT exist when using --no-git")
	}
}

func TestInit_GitFlag_InitializesGit(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	os.Remove(sb.ConfigPath)

	// Run init with --git
	result := sb.RunCLI("init", "--no-copy", "--no-targets", "--git", "--no-skill")

	result.AssertSuccess(t)

	// Verify .git was created
	gitDir := filepath.Join(sb.SourcePath, ".git")
	if !sb.FileExists(gitDir) {
		t.Error(".git directory should exist when using --git")
	}
}

func TestInit_MutualExclusion_CopyFromAndNoCopy(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	os.Remove(sb.ConfigPath)

	// Run init with both --copy-from and --no-copy
	result := sb.RunCLI("init", "--copy-from", "claude", "--no-copy")

	result.AssertFailure(t)
	result.AssertAnyOutputContains(t, "mutually exclusive")
}

func TestInit_MutualExclusion_TargetsFlags(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	os.Remove(sb.ConfigPath)

	// Run init with both --targets and --all-targets
	result := sb.RunCLI("init", "--no-copy", "--targets", "claude", "--all-targets", "--no-git")

	result.AssertFailure(t)
	result.AssertAnyOutputContains(t, "mutually exclusive")
}

func TestInit_MutualExclusion_GitFlags(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	os.Remove(sb.ConfigPath)

	// Run init with both --git and --no-git
	result := sb.RunCLI("init", "--no-copy", "--no-targets", "--git", "--no-git", "--no-skill")

	result.AssertFailure(t)
	result.AssertAnyOutputContains(t, "mutually exclusive")
}

func TestInit_FullNonInteractive(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	os.Remove(sb.ConfigPath)

	// Create existing skills
	claudeSkillsPath := filepath.Join(sb.Home, ".claude", "skills")
	os.MkdirAll(claudeSkillsPath, 0755)
	testSkillPath := filepath.Join(claudeSkillsPath, "full-test")
	os.MkdirAll(testSkillPath, 0755)
	os.WriteFile(filepath.Join(testSkillPath, "SKILL.md"), []byte("# Full Test"), 0644)

	// Full non-interactive: copy from claude, all targets, with git, no skill
	result := sb.RunCLI("init", "--copy-from", "claude", "--all-targets", "--git", "--no-skill")

	result.AssertSuccess(t)
	result.AssertOutputContains(t, "Initialized successfully")

	// Verify skill was copied
	if !sb.FileExists(filepath.Join(sb.SourcePath, "full-test", "SKILL.md")) {
		t.Error("skill should be copied")
	}

	// Verify git was initialized
	if !sb.FileExists(filepath.Join(sb.SourcePath, ".git")) {
		t.Error(".git should exist")
	}

	// Verify config has target
	configContent := sb.ReadFile(sb.ConfigPath)
	if !strings.Contains(configContent, "claude:") {
		t.Error("config should contain claude target")
	}
}

// ============================================
// Discover mode tests
// ============================================

func TestInit_AlreadyInitialized_ErrorMentionsDiscover(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	// Create config to simulate already initialized
	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)

	result := sb.RunCLI("init")

	result.AssertFailure(t)
	result.AssertAnyOutputContains(t, "--discover")
}

func TestInit_Discover_NoNewAgents(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	// Remove extra agent directories that sandbox creates by default
	os.RemoveAll(filepath.Join(sb.Home, ".codex"))
	os.RemoveAll(filepath.Join(sb.Home, ".cursor"))

	// Create config with claude target (the only agent)
	claudeSkillsPath := filepath.Join(sb.Home, ".claude", "skills")
	os.MkdirAll(claudeSkillsPath, 0755)

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets:
  claude:
    path: ` + claudeSkillsPath + `
`)

	// Run discover - no new agents should be found since only claude exists and is already configured
	result := sb.RunCLI("init", "--discover")

	result.AssertSuccess(t)
	result.AssertOutputContains(t, "No new agents")
}

func TestInit_Discover_WithSelect_AddsNewAgent(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	// Create initial config with claude only
	claudeSkillsPath := filepath.Join(sb.Home, ".claude", "skills")
	os.MkdirAll(claudeSkillsPath, 0755)

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets:
  claude:
    path: ` + claudeSkillsPath + `
`)

	// Create cursor directory (new agent)
	cursorSkillsPath := filepath.Join(sb.Home, ".cursor", "skills")
	os.MkdirAll(cursorSkillsPath, 0755)

	// Run discover with --select
	result := sb.RunCLI("init", "--discover", "--select", "cursor")

	result.AssertSuccess(t)
	result.AssertOutputContains(t, "Added 1 agent")

	// Verify config now has cursor
	configContent := sb.ReadFile(sb.ConfigPath)
	if !strings.Contains(configContent, "cursor:") {
		t.Errorf("config should contain cursor target, got: %s", configContent)
	}
	// Should still have claude
	if !strings.Contains(configContent, "claude:") {
		t.Errorf("config should still contain claude target, got: %s", configContent)
	}
}

func TestInit_Discover_WithSelect_MultipleAgents(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	// Create initial config with no targets
	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)

	// Create multiple agent directories
	os.MkdirAll(filepath.Join(sb.Home, ".claude", "skills"), 0755)
	os.MkdirAll(filepath.Join(sb.Home, ".cursor", "skills"), 0755)
	os.MkdirAll(filepath.Join(sb.Home, ".codex", "skills"), 0755)

	// Run discover with --select for multiple agents
	result := sb.RunCLI("init", "--discover", "--select", "claude,cursor")

	result.AssertSuccess(t)
	result.AssertOutputContains(t, "Added 2 agent")

	// Verify config has both
	configContent := sb.ReadFile(sb.ConfigPath)
	if !strings.Contains(configContent, "claude:") {
		t.Error("config should contain claude target")
	}
	if !strings.Contains(configContent, "cursor:") {
		t.Error("config should contain cursor target")
	}
	// Should NOT have codex (not selected)
	if strings.Contains(configContent, "codex:") {
		t.Error("config should NOT contain codex target")
	}
}

func TestInit_Discover_DryRun(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	// Create initial config
	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)

	// Create new agent directory
	os.MkdirAll(filepath.Join(sb.Home, ".cursor", "skills"), 0755)

	// Run discover with --dry-run
	result := sb.RunCLI("init", "--discover", "--select", "cursor", "--dry-run")

	result.AssertSuccess(t)
	result.AssertOutputContains(t, "Dry run")

	// Verify config was NOT modified
	configContent := sb.ReadFile(sb.ConfigPath)
	if strings.Contains(configContent, "cursor:") {
		t.Error("dry-run should NOT add cursor to config")
	}
}

func TestInit_Select_RequiresDiscover(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	os.Remove(sb.ConfigPath)

	// Run init with --select but without --discover
	result := sb.RunCLI("init", "--select", "cursor")

	result.AssertFailure(t)
	result.AssertAnyOutputContains(t, "--select requires --discover")
}

func TestInit_Discover_SkipsAlreadyConfigured(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	// Create config with claude already
	claudeSkillsPath := filepath.Join(sb.Home, ".claude", "skills")
	os.MkdirAll(claudeSkillsPath, 0755)

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets:
  claude:
    path: ` + claudeSkillsPath + `
`)

	// Try to add claude again via --select
	result := sb.RunCLI("init", "--discover", "--select", "claude")

	result.AssertSuccess(t)
	result.AssertOutputContains(t, "already in config")
}

func TestInit_Discover_UnknownAgent(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	// Create initial config
	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)

	// Try to add unknown agent
	result := sb.RunCLI("init", "--discover", "--select", "unknownagent")

	result.AssertSuccess(t)
	result.AssertOutputContains(t, "Unknown agent")
}

// ============================================
// Skill flag tests
// ============================================

func TestInit_NoSkill_SkipsInstall(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	os.Remove(sb.ConfigPath)

	result := sb.RunCLI("init", "--no-copy", "--no-targets", "--no-git", "--no-skill")

	result.AssertSuccess(t)
	result.AssertOutputContains(t, "--no-skill")

	// Verify skill was NOT installed
	skillPath := filepath.Join(sb.SourcePath, "skillshare", "SKILL.md")
	if sb.FileExists(skillPath) {
		t.Error("skill should NOT be installed when using --no-skill")
	}
}

func TestInit_SkillFlag_InstallsSkill(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	os.Remove(sb.ConfigPath)

	result := sb.RunCLI("init", "--no-copy", "--no-targets", "--no-git", "--skill")

	result.AssertSuccess(t)

	// Verify skill was installed (either downloaded or fallback)
	skillPath := filepath.Join(sb.SourcePath, "skillshare", "SKILL.md")
	if !sb.FileExists(skillPath) {
		t.Error("skill should be installed when using --skill")
	}
}

func TestInit_Fresh_ConfigHasSchemaComment(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	os.Remove(sb.ConfigPath)

	result := sb.RunCLI("init", "--no-copy", "--no-targets", "--no-git", "--no-skill")
	result.AssertSuccess(t)

	configContent := sb.ReadFile(sb.ConfigPath)
	firstLine := strings.SplitN(configContent, "\n", 2)[0]
	if !strings.HasPrefix(firstLine, "# yaml-language-server: $schema=") {
		t.Errorf("config should start with schema comment, got first line: %q", firstLine)
	}
}

func TestInit_MutualExclusion_SkillFlags(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	os.Remove(sb.ConfigPath)

	result := sb.RunCLI("init", "--no-copy", "--no-targets", "--no-git", "--skill", "--no-skill")

	result.AssertFailure(t)
	result.AssertAnyOutputContains(t, "mutually exclusive")
}
