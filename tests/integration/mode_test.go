//go:build !online

package integration

import (
	"os"
	"path/filepath"
	"testing"

	"skillshare/internal/testutil"
)

func TestMode_MutuallyExclusive(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	result := sb.RunCLI("sync", "-p", "-g")
	result.AssertFailure(t)
	result.AssertAnyOutputContains(t, "mutually exclusive")
}

func TestMode_GlobalFlag_ForcesGlobal(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	// Create project dir with config
	projectRoot := sb.SetupProjectDir("claude")

	// Write global config
	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets:
  claude:
    path: ` + filepath.Join(sb.Home, ".claude", "skills") + `
`)

	// -g flag forces global even in project dir
	result := sb.RunCLIInDir(projectRoot, "status", "-g")
	result.AssertSuccess(t)
	// Should show global source, not project source
	result.AssertOutputContains(t, sb.SourcePath)
}

func TestMode_AutoDetect_ProjectWhenConfigExists(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	projectRoot := sb.SetupProjectDir("claude")
	sb.CreateProjectSkill(projectRoot, "auto-detect", map[string]string{"SKILL.md": "# A"})

	// No flag → auto-detect project mode
	result := sb.RunCLIInDir(projectRoot, "list")
	result.AssertSuccess(t)
	result.AssertOutputContains(t, "Installed skills (project)")
}

func TestMode_AutoDetect_GlobalWhenNoConfig(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	sb.WriteConfig(`source: ` + sb.SourcePath + `
targets:
  claude:
    path: ` + filepath.Join(sb.Home, ".claude", "skills") + `
`)
	sb.CreateSkill("global-skill", map[string]string{"SKILL.md": "# G"})

	// Run from a dir without .skillshare/ → global mode
	emptyDir := filepath.Join(sb.Root, "empty")
	os.MkdirAll(emptyDir, 0755)
	result := sb.RunCLIInDir(emptyDir, "list")
	result.AssertSuccess(t)
	result.AssertOutputContains(t, "global-skill")
}

func TestMode_InstallTrackRequiresGitInProject(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()
	projectRoot := sb.SetupProjectDir("claude")

	// --track now allowed in project mode, but requires a git source
	result := sb.RunCLIInDir(projectRoot, "install", "/some/path", "--track", "-p")
	result.AssertFailure(t)
	result.AssertAnyOutputContains(t, "git repository source")
}
