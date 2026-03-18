//go:build !online

package integration

import (
	"os"
	"path/filepath"
	"testing"

	"skillshare/internal/testutil"
)

func TestUI_NoConfig_ReturnsInitError(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	result := sb.RunCLI("ui", "--no-open")

	result.AssertFailure(t)
	result.AssertAnyOutputContains(t, "run 'skillshare init' first")
}

func TestUI_SourceMissing_ReturnsInitError(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	missingSource := filepath.Join(sb.Root, "missing-source")
	sb.WriteConfig(`source: ` + missingSource + `
targets: {}
`)

	result := sb.RunCLI("ui", "--no-open")

	result.AssertFailure(t)
	result.AssertAnyOutputContains(t, "run 'skillshare init' first")
}

func TestUI_ClearCache(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	// Create a fake UI cache to clear
	cacheDir := filepath.Join(sb.Root, ".cache", "skillshare", "ui", "0.99.0")
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(cacheDir, "index.html"), []byte("<html>"), 0644)

	// Set XDG_CACHE_HOME so the binary uses our sandbox cache
	sb.SetEnv("XDG_CACHE_HOME", filepath.Join(sb.Root, ".cache"))

	result := sb.RunCLI("ui", "--clear-cache")

	result.AssertSuccess(t)
	result.AssertAnyOutputContains(t, "UI cache cleared")

	// Verify cache directory was removed
	if _, err := os.Stat(filepath.Join(sb.Root, ".cache", "skillshare", "ui")); !os.IsNotExist(err) {
		t.Error("expected UI cache directory to be removed after --clear-cache")
	}
}

func TestUI_BasePathFlag_MissingValue(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	result := sb.RunCLI("ui", "--base-path")
	result.AssertFailure(t)
	result.AssertAnyOutputContains(t, "--base-path requires a value")
}

func TestUI_BasePathShortFlag_MissingValue(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	result := sb.RunCLI("ui", "-b")
	result.AssertFailure(t)
	result.AssertAnyOutputContains(t, "--base-path requires a value")
}

func TestUI_BasePathEnvVar(t *testing.T) {
	sb := testutil.NewSandbox(t)
	defer sb.Cleanup()

	// With env var but no config — should fail on missing config, not on unknown flag
	sb.SetEnv("SKILLSHARE_UI_BASE_PATH", "/myapp")
	result := sb.RunCLI("ui", "--no-open")
	result.AssertFailure(t)
	result.AssertAnyOutputContains(t, "run 'skillshare init' first")
	result.AssertOutputNotContains(t, "unknown flag")
}
