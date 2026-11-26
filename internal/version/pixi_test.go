package version

import (
	"path/filepath"
	"testing"
)

// The conda-meta/pixi files pixi 0.85 writes, trimmed to the field detection reads.
const (
	pixiGlobalMeta  = `{"manifest_path": "/home/me/.pixi/manifests/pixi-global.toml", "environment_name": "skillshare"}`
	pixiProjectMeta = `{"manifest_path": "/home/me/proj/pixi.toml", "environment_name": "default"}`
)

func TestDetectInstallMethodPixi(t *testing.T) {
	tests := []struct {
		name  string
		exec  string
		files map[string]string
		want  InstallMethod
	}{
		{name: "pixi global, conda package", exec: ".pixi/envs/skillshare/bin/skillshare", files: map[string]string{
			".pixi/envs/skillshare/bin/skillshare":                         "binary",
			".pixi/envs/skillshare/conda-meta/pixi":                        pixiGlobalMeta,
			".pixi/envs/skillshare/conda-meta/skillshare-0.31.0-h0_0.json": "{}",
		}, want: InstallPixiGlobal},
		{name: "pixi project, conda package", exec: "proj/.pixi/envs/default/bin/skillshare", files: map[string]string{
			"proj/.pixi/envs/default/bin/skillshare":                         "binary",
			"proj/.pixi/envs/default/conda-meta/pixi":                        pixiProjectMeta,
			"proj/.pixi/envs/default/conda-meta/skillshare-0.31.0-h0_0.json": "{}",
		}, want: InstallPixi},
		// pixi installs PyPI dependencies with its own uv, which signs the wheel uv-pixi.
		{name: "pixi project, PyPI wheel", exec: "proj/.pixi/envs/default/bin/skillshare", files: map[string]string{
			"proj/.pixi/envs/default/bin/skillshare":                                         "binary",
			"proj/.pixi/envs/default/conda-meta/pixi":                                        pixiProjectMeta,
			"proj/.pixi/envs/default/lib/python3.14/site-packages/" + distInfo + "INSTALLER": "uv-pixi\n",
		}, want: InstallPixiPyPI},
		// A wheel pip put into a pixi environment is not in pixi's manifest.
		{name: "pip in a pixi environment", exec: "proj/.pixi/envs/default/bin/skillshare", files: map[string]string{
			"proj/.pixi/envs/default/bin/skillshare":                                         "binary",
			"proj/.pixi/envs/default/bin/pip":                                                "pip",
			"proj/.pixi/envs/default/conda-meta/pixi":                                        pixiProjectMeta,
			"proj/.pixi/envs/default/lib/python3.14/site-packages/" + distInfo + "INSTALLER": "pip\n",
		}, want: InstallPip},
		{name: "pip in a pixi global environment", exec: ".pixi/envs/python/bin/skillshare", files: map[string]string{
			".pixi/envs/python/bin/skillshare":                                         "binary",
			".pixi/envs/python/conda-meta/pixi":                                        pixiGlobalMeta,
			".pixi/envs/python/lib/python3.14/site-packages/" + distInfo + "INSTALLER": "pip\n",
		}, want: InstallPip},
		{name: "a conda package for another tool", exec: "proj/.pixi/envs/default/bin/skillshare", files: map[string]string{
			"proj/.pixi/envs/default/bin/skillshare":                            "binary",
			"proj/.pixi/envs/default/conda-meta/pixi":                           pixiProjectMeta,
			"proj/.pixi/envs/default/conda-meta/skillshare-cli-0.1.0-h0_0.json": "{}",
		}, want: InstallDirect},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := pythonTree(t, tt.files)
			execPath := filepath.Join(root, filepath.FromSlash(tt.exec))
			if got := DetectInstallMethod(execPath); got != tt.want {
				t.Fatalf("DetectInstallMethod(%q) = %v, want %v", tt.exec, got, tt.want)
			}
		})
	}
}

func TestPixiUpgradeCommands(t *testing.T) {
	commands := map[InstallMethod]string{
		InstallPixiGlobal: "pixi global update skillshare",
		InstallPixi:       "pixi upgrade skillshare",
		InstallPixiPyPI:   "pixi upgrade skillshare-cli",
	}
	for method, want := range commands {
		if got := method.UpgradeCommand(); got != want {
			t.Errorf("%v.UpgradeCommand() = %q, want %q", method, got, want)
		}
		if !method.IsPackageManaged() {
			t.Errorf("%v.IsPackageManaged() = false, want true", method)
		}
	}
	// The wheel updates from PyPI; the conda package comes from a channel that repackages GitHub releases.
	if !InstallPixiPyPI.IsPyPI() || InstallPixiPyPI.IsPixi() {
		t.Errorf("InstallPixiPyPI: IsPyPI() = %v, IsPixi() = %v; want true, false", InstallPixiPyPI.IsPyPI(), InstallPixiPyPI.IsPixi())
	}
	for _, method := range []InstallMethod{InstallPixiGlobal, InstallPixi} {
		if !method.IsPixi() || method.IsPyPI() {
			t.Errorf("%v: IsPixi() = %v, IsPyPI() = %v; want true, false", method, method.IsPixi(), method.IsPyPI())
		}
	}
}
