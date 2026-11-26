//go:build !windows

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	versionpkg "skillshare/internal/version"
)

// pixi global links the conda package's binary into its environment; replacing
// it would leave pixi recording the old version.
func TestUpgradeCLIBinary_PixiKeepsManagedBinary(t *testing.T) {
	prefix := filepath.Join(t.TempDir(), ".pixi", "envs", "skillshare")
	binary := filepath.Join(prefix, "bin", "skillshare")
	for _, dir := range []string{filepath.Dir(binary), filepath.Join(prefix, "conda-meta")} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for path, content := range map[string]string{
		binary: "conda binary",
		filepath.Join(prefix, "conda-meta", "pixi"):                       `{"manifest_path": "/home/me/.pixi/manifests/pixi-global.toml"}`,
		filepath.Join(prefix, "conda-meta", "skillshare-1.0.0-h0_0.json"): "{}",
	} {
		if err := os.WriteFile(path, []byte(content), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	oldVersion, oldPath, oldFetch := version, upgradeExecPath, fetchLatestVersion
	t.Cleanup(func() { version, upgradeExecPath, fetchLatestVersion = oldVersion, oldPath, oldFetch })
	version = "1.0.0"
	upgradeExecPath = func() (string, error) { return binary, nil }
	fetchLatestVersion = func(versionpkg.InstallMethod) (string, error) { return "1.1.0", nil }

	var got string
	var err error
	out := captureStdout(t, func() { got, err = upgradeCLIBinary(false, false) })
	if err != nil {
		t.Fatalf("upgradeCLIBinary: %v", err)
	}
	if want := "installed with pixi, run: pixi global update skillshare"; got != "" || !strings.Contains(out, want) {
		t.Fatalf("upgraded to %q, output %q; want no upgrade and %q", got, out, want)
	}
	if data, _ := os.ReadFile(binary); string(data) != "conda binary" {
		t.Fatalf("binary was replaced: %q", data)
	}
}
