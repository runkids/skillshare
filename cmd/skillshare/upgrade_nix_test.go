//go:build !windows

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	versionpkg "skillshare/internal/version"
)

const nixStoreBinary = "/nix/store/abc123-skillshare-1.0.0/bin/skillshare"

// fakeNixUpgrade runs the CLI step as a Nix-store binary at v1.0.0 against a
// latest release of latest. With inProfile, the user's nix profile lists it.
func fakeNixUpgrade(t *testing.T, latest string, inProfile, dryRun, force bool) (string, string) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_STATE_HOME", filepath.Join(home, "state"))
	if inProfile {
		dir := filepath.Join(home, ".nix-profile")
		manifest := `{"version":3,"elements":{"skillshare":{"storePaths":["/nix/store/abc123-skillshare-1.0.0"]}}}`
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "manifest.json"), []byte(manifest), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	oldVersion, oldPath, oldFetch := version, upgradeExecPath, fetchLatestRelease
	t.Cleanup(func() { version, upgradeExecPath, fetchLatestRelease = oldVersion, oldPath, oldFetch })
	version = "1.0.0"
	upgradeExecPath = func() (string, error) { return nixStoreBinary, nil }
	fetchLatestRelease = func() (*versionpkg.Release, error) { return &versionpkg.Release{Version: latest}, nil }

	var got string
	var err error
	out := captureStdout(t, func() { got, err = upgradeCLIBinary(dryRun, force) })
	if err != nil {
		t.Fatalf("upgradeCLIBinary: %v", err)
	}
	return got, out
}

func TestUpgradeCLIBinary_NixKeepsStoreBinary(t *testing.T) {
	tests := []struct {
		name      string
		latest    string
		inProfile bool
		dryRun    bool
		force     bool
		want      string
	}{
		{name: "nix profile, newer release", latest: "1.1.0", inProfile: true, want: "run: nix profile upgrade skillshare"},
		{name: "nix configuration, newer release", latest: "1.1.0", want: versionpkg.NixConfigUpgradeHint},
		{name: "dry run, newer release", latest: "1.1.0", inProfile: true, dryRun: true, want: "run: nix profile upgrade skillshare"},
		{name: "force at the same version", latest: "1.0.0", force: true, want: "already up to date"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, out := fakeNixUpgrade(t, tt.latest, tt.inProfile, tt.dryRun, tt.force)
			if got != "" || !strings.Contains(out, tt.want) {
				t.Fatalf("upgraded to %q, output %q; want no upgrade and %q", got, out, tt.want)
			}
		})
	}
}

// The dashboard turns this row into a 409 upgrade.nix by matching the note.
func TestUpgradeCLIBinary_NixRowCarriesDashboardNote(t *testing.T) {
	_, out := fakeNixUpgrade(t, "1.1.0", false, false, false)
	if !strings.Contains(out, versionpkg.NixUpgradeNote) {
		t.Fatalf("output %q lacks %q", out, versionpkg.NixUpgradeNote)
	}
}
