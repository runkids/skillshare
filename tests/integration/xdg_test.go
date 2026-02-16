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

// runWithXDG executes the CLI binary with XDG_CONFIG_HOME set and
// SKILLSHARE_CONFIG intentionally excluded, so the XDG path is used.
func runWithXDG(sb *testutil.Sandbox, xdgDir string, args ...string) *testutil.Result {
	cmd := exec.Command(sb.BinaryPath, args...)
	// Build env WITHOUT SKILLSHARE_CONFIG but WITH XDG_CONFIG_HOME
	cmd.Env = []string{
		"HOME=" + sb.Home,
		"XDG_CONFIG_HOME=" + xdgDir,
		"PATH=" + os.Getenv("PATH"),
	}
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = -1
		}
	}
	return &testutil.Result{
		ExitCode: exitCode,
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
	}
}

func TestXDG_ConfigHome_OverridesDefaultPath(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	xdgDir := filepath.Join(sb.Root, "xdg-config")

	// Init with XDG_CONFIG_HOME set (no SKILLSHARE_CONFIG)
	result := runWithXDG(sb, xdgDir, "init", "--no-targets", "--no-git", "--no-skill", "--no-copy")
	if result.ExitCode != 0 {
		t.Fatalf("init failed (exit %d): %s", result.ExitCode, result.Output())
	}

	// Verify config was created under XDG path
	configPath := filepath.Join(xdgDir, "skillshare", "config.yaml")
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Errorf("config not created at XDG path: %s", configPath)
	}

	// Verify source dir was created under XDG path
	skillsDir := filepath.Join(xdgDir, "skillshare", "skills")
	if _, err := os.Stat(skillsDir); os.IsNotExist(err) {
		t.Errorf("skills dir not created at XDG path: %s", skillsDir)
	}

	// Verify default path was NOT used
	defaultConfig := filepath.Join(sb.Home, ".config", "skillshare", "config.yaml")
	if _, err := os.Stat(defaultConfig); err == nil {
		t.Error("config should NOT be at default path when XDG_CONFIG_HOME is set")
	}
}

func TestXDG_Doctor_ShowsBaseDirectory(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	// First init with standard config (using sandbox's SKILLSHARE_CONFIG)
	sb.WriteConfig("source: " + sb.SourcePath + "\ntargets: {}\n")

	result := sb.RunCLI("doctor")
	if result.ExitCode != 0 {
		t.Fatalf("doctor failed: %s", result.Output())
	}

	if !strings.Contains(result.Output(), "Config directory") {
		t.Error("doctor output should contain 'Config directory'")
	}
}

func TestXDG_StatusWorksWithXDGPath(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	xdgDir := filepath.Join(sb.Root, "xdg-config")

	// Init under XDG path
	result := runWithXDG(sb, xdgDir, "init", "--no-targets", "--no-git", "--no-skill", "--no-copy")
	if result.ExitCode != 0 {
		t.Fatalf("init failed: %s", result.Output())
	}

	// Status should work with XDG path
	result = runWithXDG(sb, xdgDir, "status")
	if result.ExitCode != 0 {
		t.Fatalf("status failed (exit %d): %s", result.ExitCode, result.Output())
	}
}
