//go:build !online

package integration

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"skillshare/internal/testutil"
)

func TestInitProject_Fresh_CreatesStructure(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	projectRoot := filepath.Join(sb.Root, "project")
	os.MkdirAll(projectRoot, 0755)

	result := sb.RunCLIInDir(projectRoot, "init", "-p", "--targets", "claude,cursor")
	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, "Initialized successfully")

	// Verify structure
	if !sb.FileExists(filepath.Join(projectRoot, ".skillshare", "config.yaml")) {
		t.Error(".skillshare/config.yaml should exist")
	}
	if !sb.FileExists(filepath.Join(projectRoot, ".skillshare", "skills")) {
		t.Error(".skillshare/skills/ should exist")
	}
	if !sb.FileExists(filepath.Join(projectRoot, ".skillshare", ".gitignore")) {
		t.Error(".skillshare/.gitignore should exist")
	}
	// Target dirs created
	if !sb.FileExists(filepath.Join(projectRoot, ".claude", "skills")) {
		t.Error(".claude/skills/ should exist")
	}
	if !sb.FileExists(filepath.Join(projectRoot, ".cursor", "skills")) {
		t.Error(".cursor/skills/ should exist")
	}
}

func TestInitProject_AlreadyInitialized_Error(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	projectRoot := sb.SetupProjectDir("claude")
	result := sb.RunCLIInDir(projectRoot, "init", "-p", "--targets", "claude")
	result.AssertFailure(t)
	result.AssertAnyOutputContains(t, "already initialized")
}

func TestInitProject_DryRun_NoFiles(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	projectRoot := filepath.Join(sb.Root, "project")
	os.MkdirAll(projectRoot, 0755)

	result := sb.RunCLIInDir(projectRoot, "init", "-p", "--targets", "claude", "--dry-run")
	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, "Dry run")

	if sb.FileExists(filepath.Join(projectRoot, ".skillshare", "config.yaml")) {
		t.Error("dry-run should not create config")
	}
}

func TestInitProject_ConfigContainsTargets(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	projectRoot := filepath.Join(sb.Root, "project")
	os.MkdirAll(projectRoot, 0755)

	result := sb.RunCLIInDir(projectRoot, "init", "-p", "--targets", "claude,cursor")
	result.AssertSuccess(t)

	cfg := sb.ReadFile(filepath.Join(projectRoot, ".skillshare", "config.yaml"))
	if !strings.Contains(cfg, "claude") {
		t.Error("config should contain claude target")
	}
	if !strings.Contains(cfg, "cursor") {
		t.Error("config should contain cursor target")
	}
}

func TestInitProject_ModeFlag_SetsTargetsMode(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	projectRoot := filepath.Join(sb.Root, "project")
	os.MkdirAll(projectRoot, 0755)

	result := sb.RunCLIInDir(projectRoot, "init", "-p", "--targets", "claude,cursor", "--mode", "symlink")
	result.AssertSuccess(t)

	cfg := sb.ReadFile(filepath.Join(projectRoot, ".skillshare", "config.yaml"))
	if !strings.Contains(cfg, "mode: symlink") {
		t.Errorf("project config should include symlink mode override, got:\n%s", cfg)
	}
}

func TestInitProject_ModeFlag_Invalid(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	projectRoot := filepath.Join(sb.Root, "project")
	os.MkdirAll(projectRoot, 0755)

	result := sb.RunCLIInDir(projectRoot, "init", "-p", "--targets", "claude", "--mode", "invalid")
	result.AssertFailure(t)
	result.AssertAnyOutputContains(t, "invalid --mode value")
}

func TestInitProject_Discover_WithMode_AddsTargetMode(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	projectRoot := sb.SetupProjectDir("claude")
	os.MkdirAll(filepath.Join(projectRoot, ".cursor"), 0755)

	result := sb.RunCLIInDir(projectRoot, "init", "-p", "--discover", "--select", "cursor", "--mode", "copy")
	result.AssertSuccess(t)
	result.AssertOutputContains(t, "Added 1 target")

	cfg := sb.ReadFile(filepath.Join(projectRoot, ".skillshare", "config.yaml"))
	if !strings.Contains(cfg, "mode: copy") {
		t.Errorf("newly discovered target should use copy mode, got:\n%s", cfg)
	}
	if !strings.Contains(cfg, "- claude") {
		t.Errorf("existing target should remain unchanged, got:\n%s", cfg)
	}
}

func TestInitProject_ConfigHasSchemaComment(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	projectRoot := filepath.Join(sb.Root, "project")
	os.MkdirAll(projectRoot, 0755)

	result := sb.RunCLIInDir(projectRoot, "init", "-p", "--targets", "claude")
	result.AssertSuccess(t)

	cfg := sb.ReadFile(filepath.Join(projectRoot, ".skillshare", "config.yaml"))
	firstLine := strings.SplitN(cfg, "\n", 2)[0]
	if !strings.HasPrefix(firstLine, "# yaml-language-server: $schema=") {
		t.Errorf("project config should start with schema comment, got first line: %q", firstLine)
	}
}

func TestInitProject_ConfigLocal(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	projectRoot := filepath.Join(sb.Root, "project")
	os.MkdirAll(projectRoot, 0755)

	result := sb.RunCLIInDir(projectRoot, "init", "-p", "--config", "local", "--targets", "claude")
	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, "Initialized successfully")

	// .gitignore should contain config.yaml (without trailing slash)
	gitignore := sb.ReadFile(filepath.Join(projectRoot, ".skillshare", ".gitignore"))
	if !strings.Contains(gitignore, "config.yaml") {
		t.Errorf(".gitignore should contain config.yaml, got:\n%s", gitignore)
	}
	// config.yaml should NOT be listed as config.yaml/ (directory form)
	if strings.Contains(gitignore, "config.yaml/") {
		t.Errorf(".gitignore should not add trailing slash to config.yaml, got:\n%s", gitignore)
	}

	// config.yaml should still be generated
	if !sb.FileExists(filepath.Join(projectRoot, ".skillshare", "config.yaml")) {
		t.Error(".skillshare/config.yaml should exist")
	}

	// Output should mention config gitignored
	result.AssertAnyOutputContains(t, "Config gitignored")
}

func TestInitProject_SharedRepoClone(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	projectRoot := filepath.Join(sb.Root, "project")

	// Simulate a git clone: .skillshare/skills/ exists + .gitignore has config.yaml, but no config.yaml
	skillshareDir := filepath.Join(projectRoot, ".skillshare")
	os.MkdirAll(filepath.Join(skillshareDir, "skills", "my-skill"), 0755)
	os.WriteFile(filepath.Join(skillshareDir, "skills", "my-skill", "SKILL.md"), []byte("# test"), 0644)
	os.WriteFile(filepath.Join(skillshareDir, ".gitignore"), []byte(
		"# BEGIN SKILLSHARE MANAGED - DO NOT EDIT\nlogs/\ntrash/\nconfig.yaml\n# END SKILLSHARE MANAGED\n",
	), 0644)

	// Run init -p WITHOUT --config local (simulating teammate clone)
	result := sb.RunCLIInDir(projectRoot, "init", "-p", "--targets", "claude")
	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, "shared skills directory")

	// config.yaml should be generated with empty targets
	cfg := sb.ReadFile(filepath.Join(projectRoot, ".skillshare", "config.yaml"))
	if strings.Contains(cfg, "claude") {
		t.Errorf("shared repo clone should have empty targets, got:\n%s", cfg)
	}
}

func TestInitProject_PartialRepair_WithTargets(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	projectRoot := filepath.Join(sb.Root, "project")

	// Setup partial init: .skillshare/skills/ exists + .gitignore WITHOUT config.yaml
	skillshareDir := filepath.Join(projectRoot, ".skillshare")
	os.MkdirAll(filepath.Join(skillshareDir, "skills"), 0755)
	os.WriteFile(filepath.Join(skillshareDir, ".gitignore"), []byte(
		"# BEGIN SKILLSHARE MANAGED - DO NOT EDIT\nlogs/\ntrash/\n# END SKILLSHARE MANAGED\n",
	), 0644)

	// Run init -p --targets claude (non-interactive partial repair)
	result := sb.RunCLIInDir(projectRoot, "init", "-p", "--targets", "claude")
	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, "repairing missing config")

	// config.yaml should contain claude target
	cfg := sb.ReadFile(filepath.Join(projectRoot, ".skillshare", "config.yaml"))
	if !strings.Contains(cfg, "claude") {
		t.Errorf("repaired config should contain claude target, got:\n%s", cfg)
	}
}

func TestInitProject_ConfigFlag_InvalidValue(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	projectRoot := filepath.Join(sb.Root, "project")
	os.MkdirAll(projectRoot, 0755)

	result := sb.RunCLIInDir(projectRoot, "init", "-p", "--config", "remote")
	result.AssertFailure(t)
	result.AssertAnyOutputContains(t, "--config only supports 'local'")
}

func TestInitProject_ConfigFlag_MissingValue(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	projectRoot := filepath.Join(sb.Root, "project")
	os.MkdirAll(projectRoot, 0755)

	result := sb.RunCLIInDir(projectRoot, "init", "-p", "--config")
	result.AssertFailure(t)
	result.AssertAnyOutputContains(t, "--config requires a value")
}

func TestInitProject_GitignoreIncludesLogsDirectory(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	projectRoot := filepath.Join(sb.Root, "project")
	os.MkdirAll(projectRoot, 0755)

	result := sb.RunCLIInDir(projectRoot, "init", "-p", "--targets", "claude")
	result.AssertSuccess(t)

	gitignore := sb.ReadFile(filepath.Join(projectRoot, ".skillshare", ".gitignore"))
	if !strings.Contains(gitignore, "logs/") {
		t.Errorf("project .gitignore should include logs/, got:\n%s", gitignore)
	}
}
