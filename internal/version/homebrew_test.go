package version

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDetectInstallMethod(t *testing.T) {
	tests := []struct {
		name     string
		execPath string
		want     InstallMethod
	}{
		{name: "homebrew arm", execPath: "/opt/homebrew/Cellar/skillshare/0.26.3/bin/skillshare", want: InstallBrew},
		{name: "linuxbrew", execPath: "/home/linuxbrew/.linuxbrew/Cellar/skillshare/0.26.3/bin/skillshare", want: InstallBrew},
		{name: "install script", execPath: "/home/me/.local/bin/skillshare", want: InstallDirect},
		{name: "nix-like name outside the store", execPath: "/home/me/nix/store/skillshare", want: InstallDirect},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := DetectInstallMethod(tt.execPath); got != tt.want {
				t.Fatalf("DetectInstallMethod(%q) = %v, want %v", tt.execPath, got, tt.want)
			}
		})
	}
}

func TestDetectInstallMethodNixStore(t *testing.T) {
	if got := DetectInstallMethod("/nix/store/abc123-skillshare-0.26.3/bin/skillshare"); !got.IsNix() {
		t.Fatalf("DetectInstallMethod(nix store) = %v, want a Nix method", got)
	}
}

func TestDetectNixMethod(t *testing.T) {
	const execPath = "/nix/store/abc123-skillshare-0.26.3/bin/skillshare"
	writeManifest := func(t *testing.T, body string) string {
		t.Helper()
		path := filepath.Join(t.TempDir(), "manifest.json")
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		return path
	}
	tests := []struct {
		name     string
		manifest string
		want     InstallMethod
	}{
		{
			name:     "nix profile add",
			manifest: `{"version":3,"elements":{"skillshare":{"storePaths":["/nix/store/abc123-skillshare-0.26.3"]}}}`,
			want:     InstallNixProfile,
		},
		{
			name:     "home manager combined package",
			manifest: `{"version":3,"elements":{"home-manager-path":{"storePaths":["/nix/store/def456-home-manager-path"]}}}`,
			want:     InstallNix,
		},
		{
			name:     "older skillshare in the profile",
			manifest: `{"version":3,"elements":{"skillshare":{"storePaths":["/nix/store/old999-skillshare-0.26.2"]}}}`,
			want:     InstallNix,
		},
		{
			name:     "older manifest without element names",
			manifest: `{"version":2,"elements":[{"storePaths":["/nix/store/abc123-skillshare-0.26.3"]}]}`,
			want:     InstallNix,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manifests := []string{filepath.Join(t.TempDir(), "missing.json"), writeManifest(t, tt.manifest)}
			if got := detectNixMethod(execPath, manifests); got != tt.want {
				t.Fatalf("detectNixMethod = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestUpgradeInstruction(t *testing.T) {
	tests := []struct {
		method InstallMethod
		want   string
	}{
		{method: InstallDirect, want: "run: skillshare upgrade"},
		{method: InstallBrew, want: "run: brew upgrade skillshare"},
		{method: InstallNixProfile, want: "run: nix profile upgrade skillshare"},
		{method: InstallNix, want: NixConfigUpgradeHint},
	}
	for _, tt := range tests {
		if got := tt.method.UpgradeInstruction(); got != tt.want {
			t.Fatalf("UpgradeInstruction(%v) = %q, want %q", tt.method, got, tt.want)
		}
	}
}
