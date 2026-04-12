package testutil

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestResolveTestBinaryPath_UsesEnvOverride(t *testing.T) {
	t.Helper()

	binaryName := "skillshare"
	script := "#!/bin/sh\nexit 0\n"
	if runtime.GOOS == "windows" {
		binaryName += ".exe"
		script = "@echo off\r\nexit /b 0\r\n"
	}

	path := filepath.Join(t.TempDir(), binaryName)
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake binary: %v", err)
	}

	t.Setenv("SKILLSHARE_TEST_BINARY", path)

	got, err := resolveTestBinaryPath()
	if err != nil {
		t.Fatalf("resolveTestBinaryPath returned error: %v", err)
	}
	if got != path {
		t.Fatalf("resolveTestBinaryPath = %q, want %q", got, path)
	}
}

func TestBuildTestBinary_BuildsCLIFromRepoRoot(t *testing.T) {
	t.Helper()

	repoRoot, err := testRepoRoot()
	if err != nil {
		t.Fatalf("testRepoRoot returned error: %v", err)
	}

	binaryPath, err := buildTestBinary(repoRoot, t.TempDir())
	if err != nil {
		t.Fatalf("buildTestBinary returned error: %v", err)
	}

	info, err := os.Stat(binaryPath)
	if err != nil {
		t.Fatalf("built binary missing: %v", err)
	}
	if info.IsDir() {
		t.Fatalf("expected binary file, got directory: %s", binaryPath)
	}
}
