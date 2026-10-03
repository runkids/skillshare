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

	result := sb.RunCLI("init", "--no-skill")

	result.AssertSuccess(t)
	result.AssertOutputContains(t, "✓ Config")

	// Verify config was created
	if !sb.FileExists(sb.ConfigPath) {
		t.Error("config file should be created")
	}

	// Verify source directory was created
	if !sb.FileExists(sb.SourcePath) {
		t.Error("source directory should be created")
	}

	cfg := sb.ReadFile(sb.ConfigPath)
	if !strings.Contains(cfg, "__pycache__/") {
		t.Errorf("config should include default __pycache__/ ignore, got:\n%s", cfg)
	}
}

func TestInit_WithSourceFlag_UsesCustomPath(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	// Remove config file
	os.Remove(sb.ConfigPath)

	customSource := filepath.Join(sb.Home, "my-skills")

	result := sb.RunCLI("init", "--source", customSource, "--no-skill")

	result.AssertSuccess(t)
	result.AssertOutputContains(t, "~/my-skills")

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

	result := sb.RunCLI("init", "--dry-run")

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

	result := sb.RunCLI("init", "--no-skill")

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

	result := sb.RunCLI("init", "--no-skill")

	result.AssertSuccess(t)
	result.AssertOutputContains(t, "1 imported")

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

func TestInit_AlreadyInitialized_RemoteFlag_WithNoGit_DoesNotCreateCommit(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets: {}
`)

	// Initialize git repo and create initial commit.
	cmd := exec.Command("git", "init")
	cmd.Dir = sb.SourcePath
	if err := cmd.Run(); err != nil {
		t.Skip("git not available")
	}
	testutil.RunGit(t, sb.SourcePath, "config", "user.email", "test@example.com")
	testutil.RunGit(t, sb.SourcePath, "config", "user.name", "Test User")

	seedDir := filepath.Join(sb.SourcePath, "seed-skill")
	if err := os.MkdirAll(seedDir, 0o755); err != nil {
		t.Fatalf("failed to create seed skill dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(seedDir, "SKILL.md"), []byte("# Seed"), 0o644); err != nil {
		t.Fatalf("failed to write seed SKILL.md: %v", err)
	}
	testutil.RunGit(t, sb.SourcePath, "add", "-A")
	testutil.RunGit(t, sb.SourcePath, "commit", "-m", "initial")
	beforeHead := testutil.RunGit(t, sb.SourcePath, "rev-parse", "HEAD")

	// Add dirty file that should remain uncommitted when --no-git is used.
	dirtyDir := filepath.Join(sb.SourcePath, "dirty-skill")
	if err := os.MkdirAll(dirtyDir, 0o755); err != nil {
		t.Fatalf("failed to create dirty skill dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dirtyDir, "SKILL.md"), []byte("# Dirty"), 0o644); err != nil {
		t.Fatalf("failed to write dirty SKILL.md: %v", err)
	}

	result := sb.RunCLI("init", "--remote", "git@github.com:test/skills.git", "--no-git")
	result.AssertSuccess(t)
	result.AssertOutputContains(t, "--no-git")

	afterHead := testutil.RunGit(t, sb.SourcePath, "rev-parse", "HEAD")
	if beforeHead != afterHead {
		t.Fatalf("expected no new commit with --no-git, before=%s after=%s", beforeHead, afterHead)
	}

	status := testutil.RunGit(t, sb.SourcePath, "status", "--porcelain")
	if !strings.Contains(status, "dirty-skill") {
		t.Fatalf("expected dirty file to remain uncommitted, got status: %s", status)
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
	result.AssertOutputContains(t, "1 from claude")

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
	result.AssertOutputContains(t, "Targets  claude, cursor")

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
	result.AssertOutputContains(t, "universal")

	// Verify config has targets (including auto-detected universal)
	configContent := sb.ReadFile(sb.ConfigPath)
	if !strings.Contains(configContent, "claude:") || !strings.Contains(configContent, "cursor:") {
		t.Errorf("config should contain all detected targets, got: %s", configContent)
	}
	if !strings.Contains(configContent, "universal:") {
		t.Errorf("config should auto-include universal target when any CLI is detected, got: %s", configContent)
	}
}

func TestInit_UniversalAutoDetected(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	os.Remove(sb.ConfigPath)

	// Create only a claude skills directory — universal path (~/.agents/) does
	// NOT exist on disk, yet init should auto-include it as a recommended target.
	claudeSkillsPath := filepath.Join(sb.Home, ".claude", "skills")
	os.MkdirAll(claudeSkillsPath, 0755)

	// Remove other sandbox defaults so only claude is detected
	os.RemoveAll(filepath.Join(sb.Home, ".codex"))
	os.RemoveAll(filepath.Join(sb.Home, ".cursor"))

	result := sb.RunCLI("init", "--no-copy", "--all-targets", "--no-git", "--no-skill")

	result.AssertSuccess(t)

	// Verify config includes universal pointing to ~/.agents/skills
	configContent := sb.ReadFile(sb.ConfigPath)
	if !strings.Contains(configContent, "universal:") {
		t.Errorf("config should contain universal target, got:\n%s", configContent)
	}

	agentsPath := filepath.Join(sb.Home, ".agents", "skills")
	if !strings.Contains(configContent, agentsPath) {
		t.Errorf("universal target should point to %s, got:\n%s", agentsPath, configContent)
	}
}

func TestInit_Discover_UniversalAutoAdded(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	// Config has claude only, no universal
	claudeSkillsPath := filepath.Join(sb.Home, ".claude", "skills")
	os.MkdirAll(claudeSkillsPath, 0755)

	// Create a new agent directory so detectNewAgents finds something
	cursorSkillsPath := filepath.Join(sb.Home, ".cursor", "skills")
	os.MkdirAll(cursorSkillsPath, 0755)

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets:
  claude:
    path: ` + claudeSkillsPath + `
`)

	// Run discover with --select cursor — universal should also appear as candidate
	result := sb.RunCLI("init", "--discover", "--select", "cursor,universal")

	result.AssertSuccess(t)

	// Verify config now has both cursor and universal
	configContent := sb.ReadFile(sb.ConfigPath)
	if !strings.Contains(configContent, "cursor:") {
		t.Errorf("config should contain cursor target, got:\n%s", configContent)
	}
	if !strings.Contains(configContent, "universal:") {
		t.Errorf("config should contain universal target (auto-added candidate), got:\n%s", configContent)
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
	result.AssertOutputContains(t, "✓ Config")

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
	result.AssertOutputContains(t, "No new AI tools found")
}

func TestInit_Discover_DoesNotExpandSharedDirectoryTargets(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sharedParent := filepath.Join(sb.Home, ".agents")
	os.MkdirAll(sharedParent, 0755)

	claudeSkillsPath := filepath.Join(sb.Home, ".claude", "skills")
	codexSkillsPath := filepath.Join(sb.Home, ".codex", "skills")
	cursorSkillsPath := filepath.Join(sb.Home, ".cursor", "skills")
	copilotSkillsPath := filepath.Join(sb.Home, ".copilot", "skills")
	universalSkillsPath := filepath.Join(sharedParent, "skills")
	os.MkdirAll(claudeSkillsPath, 0755)
	os.MkdirAll(codexSkillsPath, 0755)
	os.MkdirAll(cursorSkillsPath, 0755)
	os.MkdirAll(copilotSkillsPath, 0755)
	// gemini has a path of its own, so discover has at least one genuinely new
	// agent to offer — codex resolves to the already-configured universal target.
	os.MkdirAll(filepath.Join(sb.Home, ".gemini", "skills"), 0755)

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets:
  claude:
    path: ` + claudeSkillsPath + `
  cursor:
    path: ` + cursorSkillsPath + `
  copilot:
    path: ` + copilotSkillsPath + `
  universal:
    path: ` + universalSkillsPath + `
`)

	result := sb.RunCLI("init", "--discover", "--select", "zed")

	result.AssertSuccess(t)
	result.AssertOutputContains(t, "zed was not found on this machine")
	result.AssertOutputNotContains(t, "✓ Targets")

	configContent := sb.ReadFile(sb.ConfigPath)
	if strings.Contains(configContent, "zed:") {
		t.Errorf("config should not contain zed target, got:\n%s", configContent)
	}
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
	result.AssertOutputContains(t, "Targets  cursor")

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

func TestInit_Discover_WithMode_OnlyAppliesToNewTargets(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	claudeSkillsPath := filepath.Join(sb.Home, ".claude", "skills")
	cursorSkillsPath := filepath.Join(sb.Home, ".cursor", "skills")
	os.MkdirAll(claudeSkillsPath, 0755)
	os.MkdirAll(cursorSkillsPath, 0755)

	sb.WriteConfig(`source: ` + sb.SourcePath + `
mode: merge
targets:
  claude:
    path: ` + claudeSkillsPath + `
    mode: symlink
`)

	result := sb.RunCLI("init", "--discover", "--select", "cursor", "--mode", "copy")

	result.AssertSuccess(t)
	result.AssertOutputContains(t, "Targets  cursor")

	configContent := sb.ReadFile(sb.ConfigPath)
	if !strings.Contains(configContent, "mode: symlink") {
		t.Errorf("existing target mode should remain unchanged, got:\n%s", configContent)
	}
	if !strings.Contains(configContent, "cursor:") || !strings.Contains(configContent, "mode: copy") {
		t.Errorf("new target should be added with mode copy, got:\n%s", configContent)
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
	result.AssertOutputContains(t, "Targets  claude, cursor")

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
	result.AssertOutputContains(t, "claude is already set up")
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
	result.AssertOutputContains(t, "Unknown AI tool: unknownagent")
}

func TestInit_ModeFlag_SetsDefaultMode(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	os.Remove(sb.ConfigPath)

	result := sb.RunCLI("init", "--mode", "copy", "--no-copy", "--no-targets", "--no-git", "--no-skill")

	result.AssertSuccess(t)

	configContent := sb.ReadFile(sb.ConfigPath)
	if !strings.Contains(configContent, "mode: copy") {
		t.Errorf("config should contain mode: copy, got:\n%s", configContent)
	}
}

func TestInit_ModeFlag_Invalid(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	os.Remove(sb.ConfigPath)

	result := sb.RunCLI("init", "--mode", "invalid", "--no-copy", "--no-targets", "--no-git", "--no-skill")

	result.AssertFailure(t)
	result.AssertAnyOutputContains(t, "invalid --mode value")
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

func TestInit_SuccessMessage_ShowsNextSteps(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	os.Remove(sb.ConfigPath)

	result := sb.RunCLI("init", "--no-copy", "--no-targets", "--no-git", "--no-skill")

	result.AssertSuccess(t)
	result.AssertOutputContains(t, "skillshare install <repo>")
}

// ============================================
// --subdir flag tests (global mode only)
// ============================================

func TestInit_Subdir_SetsSourcePath(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	os.Remove(sb.ConfigPath)

	result := sb.RunCLI("init", "--subdir", "skills", "--no-copy", "--no-targets", "--no-git", "--no-skill")

	result.AssertSuccess(t)
	result.AssertOutputContains(t, "✓ Config")

	// Verify config source ends with /skills (the subdir)
	configContent := sb.ReadFile(sb.ConfigPath)
	expectedSuffix := filepath.Join("skillshare", "skills", "skills")
	if !strings.Contains(configContent, expectedSuffix) {
		t.Errorf("config source should end with skills/skills (subdir appended), got:\n%s", configContent)
	}

	// Verify subdirectory was created
	subdirPath := filepath.Join(sb.SourcePath, "skills")
	if !sb.FileExists(subdirPath) {
		t.Error("subdir should be created inside source path")
	}
}

func TestInit_Subdir_ListFindsSkillsInSubdir(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	os.Remove(sb.ConfigPath)

	// Init with --subdir
	result := sb.RunCLI("init", "--subdir", "skills", "--no-copy", "--no-targets", "--no-git", "--no-skill")
	result.AssertSuccess(t)

	// Create a skill inside the subdir
	subdirSkillPath := filepath.Join(sb.SourcePath, "skills", "subdir-skill")
	os.MkdirAll(subdirSkillPath, 0755)
	os.WriteFile(filepath.Join(subdirSkillPath, "SKILL.md"), []byte("---\nname: subdir-skill\ndescription: test\n---\n# Test"), 0644)

	// List should find the skill
	listResult := sb.RunCLI("list", "--no-tui")
	listResult.AssertSuccess(t)
	listResult.AssertOutputContains(t, "subdir-skill")
}

func TestInit_Subdir_SyncWorksWithSubdir(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	os.Remove(sb.ConfigPath)

	// Create a claude target directory
	claudeSkillsPath := filepath.Join(sb.Home, ".claude", "skills")
	os.MkdirAll(claudeSkillsPath, 0755)

	// Init with --subdir and a target
	result := sb.RunCLI("init", "--subdir", "myskills", "--no-copy", "--targets", "claude", "--no-git", "--no-skill")
	result.AssertSuccess(t)

	// Create a skill inside the subdir
	subdirSkillPath := filepath.Join(sb.SourcePath, "myskills", "sync-test")
	os.MkdirAll(subdirSkillPath, 0755)
	os.WriteFile(filepath.Join(subdirSkillPath, "SKILL.md"), []byte("---\nname: sync-test\ndescription: test\n---\n# Sync Test"), 0644)

	// Sync should work
	syncResult := sb.RunCLI("sync", "--no-tui")
	syncResult.AssertSuccess(t)
	syncResult.AssertOutputContains(t, "Sync complete")

	// Verify symlink was created in target
	symlinkPath := filepath.Join(claudeSkillsPath, "sync-test")
	if _, err := os.Lstat(symlinkPath); os.IsNotExist(err) {
		t.Error("skill symlink should be created in target after sync")
	}
}

func TestInit_Subdir_ForcesGlobalModeInProjectDir(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	// Simulate a project directory with .skillshare/config.yaml
	projectDir := filepath.Join(sb.Home, "myproject")
	os.MkdirAll(filepath.Join(projectDir, ".skillshare"), 0755)
	os.WriteFile(filepath.Join(projectDir, ".skillshare", "config.yaml"), []byte("skills: []\ntargets: {}\n"), 0644)

	// Remove global config so init can run fresh
	os.Remove(sb.ConfigPath)

	// Run init --subdir from inside the project directory
	// Without the fix, this would route to project mode and fail with "unknown option"
	result := sb.RunCLIInDir(projectDir, "init", "--subdir", "skills", "--no-copy", "--no-targets", "--no-git", "--no-skill")

	result.AssertSuccess(t)
	result.AssertOutputContains(t, "✓ Config")
}

func TestInit_Subdir_DryRunDoesNotCreate(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	os.Remove(sb.ConfigPath)
	os.RemoveAll(sb.SourcePath)

	result := sb.RunCLI("init", "--subdir", "skills", "--no-copy", "--no-targets", "--no-git", "--no-skill", "--dry-run")

	result.AssertSuccess(t)
	result.AssertOutputContains(t, "Dry run")

	// Config should not exist
	if sb.FileExists(sb.ConfigPath) {
		t.Error("dry-run should not create config")
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

// ============================================
// Codex shares the universal ~/.agents/skills path
// ============================================

func TestInit_CodexOnly_AddsUniversalInsteadOfCodex(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	os.Remove(sb.ConfigPath)

	// Only ~/.codex exists (created by the sandbox); Codex reads ~/.agents/skills,
	// so it must be credited to the universal target, not a codex target.
	os.RemoveAll(filepath.Join(sb.Home, ".claude"))
	os.RemoveAll(filepath.Join(sb.Home, ".cursor"))

	result := sb.RunCLI("init", "--no-copy", "--all-targets", "--no-git", "--no-skill")

	result.AssertSuccess(t)

	configContent := sb.ReadFile(sb.ConfigPath)
	if !strings.Contains(configContent, "universal:") {
		t.Errorf("config should contain universal target, got:\n%s", configContent)
	}
	if strings.Contains(configContent, "codex:") {
		t.Errorf("config should not contain a separate codex target, got:\n%s", configContent)
	}
}

func TestInit_CodexAndClaude_AddsClaudeAndUniversal(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	os.Remove(sb.ConfigPath)
	os.RemoveAll(filepath.Join(sb.Home, ".cursor"))
	os.MkdirAll(filepath.Join(sb.Home, ".claude", "skills"), 0755)

	result := sb.RunCLI("init", "--no-copy", "--all-targets", "--no-git", "--no-skill")

	result.AssertSuccess(t)

	configContent := sb.ReadFile(sb.ConfigPath)
	for _, want := range []string{"claude:", "universal:"} {
		if !strings.Contains(configContent, want) {
			t.Errorf("config should contain %s, got:\n%s", want, configContent)
		}
	}
	if strings.Contains(configContent, "codex:") {
		t.Errorf("config should not contain a separate codex target, got:\n%s", configContent)
	}
}

func TestInit_TargetsCodex_UsesAgentsPath(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	os.Remove(sb.ConfigPath)

	result := sb.RunCLI("init", "--no-copy", "--targets", "codex", "--no-git", "--no-skill")

	result.AssertSuccess(t)

	configContent := sb.ReadFile(sb.ConfigPath)
	agentsPath := filepath.Join(sb.Home, ".agents", "skills")
	if !strings.Contains(configContent, agentsPath) {
		t.Errorf("codex target should point to %s, got:\n%s", agentsPath, configContent)
	}
}

func TestSync_CodexTarget_WritesToAgentsPath(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	os.Remove(sb.ConfigPath)
	sb.RunCLI("init", "--no-copy", "--targets", "codex", "--no-git", "--no-skill").AssertSuccess(t)
	sb.CreateSkill("codex-skill", map[string]string{"SKILL.md": "# Codex Skill"})

	sb.RunCLI("sync").AssertSuccess(t)

	if !sb.IsSymlink(filepath.Join(sb.Home, ".agents", "skills", "codex-skill")) {
		t.Error("skill should be linked into ~/.agents/skills")
	}
	if sb.FileExists(filepath.Join(sb.Home, ".codex", "skills", "codex-skill")) {
		t.Error("skill should not be written to the deprecated ~/.codex/skills")
	}
}

func TestInit_GooseOnly_AddsUniversalInsteadOfGoose(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	os.Remove(sb.ConfigPath)

	// Goose now reads ~/.agents/skills, so only ~/.config/goose identifies it
	// and the detection must be credited to the universal target.
	for _, dir := range []string{".claude", ".codex", ".cursor"} {
		os.RemoveAll(filepath.Join(sb.Home, dir))
	}
	os.MkdirAll(filepath.Join(sb.Home, ".config", "goose"), 0755)

	result := sb.RunCLI("init", "--no-copy", "--all-targets", "--no-git", "--no-skill")

	result.AssertSuccess(t)

	configContent := sb.ReadFile(sb.ConfigPath)
	if !strings.Contains(configContent, "universal:") {
		t.Errorf("config should contain universal target, got:\n%s", configContent)
	}
	if strings.Contains(configContent, "goose:") {
		t.Errorf("config should not contain a separate goose target, got:\n%s", configContent)
	}
}

// ============================================
// Headless defaults (no terminal: every question takes its default)
// ============================================

func TestInit_Headless_NoFlags_AppliesDefaults(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	os.Remove(sb.ConfigPath)
	claudeSkill := filepath.Join(sb.Home, ".claude", "skills", "my-skill")
	os.MkdirAll(claudeSkill, 0755)
	os.WriteFile(filepath.Join(claudeSkill, "SKILL.md"), []byte("---\nname: my-skill\n---\n# Mine\n"), 0644)

	result := sb.RunCLI("init")
	result.AssertSuccess(t)

	cfg := sb.ReadFile(sb.ConfigPath)
	if !strings.Contains(cfg, "claude:") {
		t.Errorf("detected tools should become targets, got:\n%s", cfg)
	}
	if !sb.FileExists(filepath.Join(sb.SourcePath, "my-skill", "SKILL.md")) {
		t.Error("existing skills should be imported")
	}
	if !sb.FileExists(filepath.Join(sb.SourcePath, ".git")) {
		t.Error("git should be initialized")
	}
	if !sb.FileExists(filepath.Join(sb.SourcePath, "skillshare", "SKILL.md")) {
		t.Error("built-in skill should be installed")
	}
	if !sb.IsSymlink(claudeSkill) {
		t.Error("the tool's copy matches the source, so sync should replace it with a link")
	}
}

func TestInit_Headless_RemoteWithSameNameSkill_UsesRepoVersion(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	os.Remove(sb.ConfigPath)
	remote := createSkillsRemote(t, sb.Root, map[string]string{"pdf-tools": "repo version", "tdd": "repo tdd"})
	local := filepath.Join(sb.Home, ".claude", "skills", "pdf-tools")
	os.MkdirAll(local, 0755)
	os.WriteFile(filepath.Join(local, "SKILL.md"), []byte("local version"), 0644)

	result := sb.RunCLI("init", "--remote", remote, "--no-skill")
	result.AssertSuccess(t)
	result.AssertOutputContains(t, "repo version used for pdf-tools")

	if got := sb.ReadFile(filepath.Join(sb.SourcePath, "pdf-tools", "SKILL.md")); got != "repo version" {
		t.Errorf("same-name skill should keep the repo version, got %q", got)
	}
	if !sb.FileExists(filepath.Join(sb.SourcePath, "tdd", "SKILL.md")) {
		t.Error("the repo's other skills should be pulled")
	}
}

func TestInit_DryRun_DoesNotCreateToolFolders(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	os.Remove(sb.ConfigPath)
	os.RemoveAll(filepath.Join(sb.Home, ".cursor"))
	os.MkdirAll(filepath.Join(sb.Home, ".cursor"), 0755)

	result := sb.RunCLI("init", "--dry-run")
	result.AssertSuccess(t)

	if sb.FileExists(filepath.Join(sb.Home, ".cursor", "skills")) {
		t.Error("detection must not create a tool's skills folder")
	}
}

// createSkillsRemote makes a bare repo whose top level holds the given
// skills (name → SKILL.md content) and returns its file:// URL.
func createSkillsRemote(t *testing.T, root string, skills map[string]string) string {
	t.Helper()
	work := filepath.Join(root, "remote-work")
	bare := filepath.Join(root, "remote.git")
	for name, content := range skills {
		os.MkdirAll(filepath.Join(work, name), 0755)
		os.WriteFile(filepath.Join(work, name, "SKILL.md"), []byte(content), 0644)
	}
	for _, args := range [][]string{
		{"init", "-q", "-b", "main"},
		{"add", "."},
		{"-c", "user.name=t", "-c", "user.email=t@t", "commit", "-qm", "skills"},
		{"clone", "-q", "--bare", ".", bare},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = work
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v (%s)", args, err, out)
		}
	}
	return "file://" + bare
}
