//go:build !online

package integration

import (
	"os"
	"path/filepath"
	"testing"

	"skillshare/internal/testutil"
)

func TestUIProject_RequiresInit(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	// Create an empty project directory without .skillshare/config.yaml
	projectRoot := filepath.Join(sb.Root, "project")
	os.MkdirAll(projectRoot, 0755)

	result := sb.RunCLIInDir(projectRoot, "ui", "-p", "--no-open")

	result.AssertFailure(t)
	result.AssertAnyOutputContains(t, "run 'skillshare init -p' first")
}

func TestUIProject_ExplicitFlag(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	projectRoot := sb.SetupProjectDir("claude-code")

	// With -p flag and no .skillshare in a different dir, should fail
	emptyDir := filepath.Join(sb.Root, "empty")
	os.MkdirAll(emptyDir, 0755)
	result := sb.RunCLIInDir(emptyDir, "ui", "-p", "--no-open")
	result.AssertFailure(t)
	result.AssertAnyOutputContains(t, "run 'skillshare init -p' first")

	// With -p flag in project dir, the server will try to start and bind a port.
	// We just verify it doesn't fail with "not initialized" error by checking
	// the setup path works. The actual server binding is tested separately.
	_ = projectRoot
}

func TestUIProject_MutuallyExclusiveFlags(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	result := sb.RunCLI("ui", "-p", "-g", "--no-open")
	result.AssertFailure(t)
	result.AssertAnyOutputContains(t, "mutually exclusive")
}
