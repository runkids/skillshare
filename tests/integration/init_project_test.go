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
