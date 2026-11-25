//go:build !windows

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	versionpkg "skillshare/internal/version"
)

func TestUpgradeCLIBinary_PyPIKeepsManagedBinary(t *testing.T) {
	prefix := filepath.Join(t.TempDir(), "tools", "skillshare-cli")
	binary := filepath.Join(prefix, "bin", "skillshare")
	for _, dir := range []string{filepath.Dir(binary), filepath.Join(prefix, "lib", "python3.12", "site-packages", "skillshare_cli-1.0.0.dist-info")} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for path, content := range map[string]string{binary: "wheel binary", filepath.Join(prefix, "uv-receipt.toml"): "[tool]\n"} {
		if err := os.WriteFile(path, []byte(content), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	tests := []struct {
		name   string
		latest string
		force  bool
		want   string
	}{
		{name: "newer release", latest: "1.1.0", want: "installed from PyPI, run: uv tool upgrade skillshare-cli"},
		{name: "force at the same version", latest: "1.0.0", force: true, want: "already up to date"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			oldVersion, oldPath, oldFetch := version, upgradeExecPath, fetchLatestRelease
			t.Cleanup(func() { version, upgradeExecPath, fetchLatestRelease = oldVersion, oldPath, oldFetch })
			version = "1.0.0"
			upgradeExecPath = func() (string, error) { return binary, nil }
			fetchLatestRelease = func() (*versionpkg.Release, error) { return &versionpkg.Release{Version: tt.latest}, nil }

			var got string
			var err error
			out := captureStdout(t, func() { got, err = upgradeCLIBinary(false, tt.force) })
			if err != nil {
				t.Fatalf("upgradeCLIBinary: %v", err)
			}
			if got != "" || !strings.Contains(out, tt.want) {
				t.Fatalf("upgraded to %q, output %q; want no upgrade and %q", got, out, tt.want)
			}
			if data, _ := os.ReadFile(binary); string(data) != "wheel binary" {
				t.Fatalf("binary was replaced: %q", data)
			}
		})
	}
}
