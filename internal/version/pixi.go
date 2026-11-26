package version

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// detectPixiMethod tells a binary that pixi installed by the conda-meta/pixi
// file pixi writes into each environment it creates. The environment root is
// the binary's folder or one of the two above it, so the bin folder's name is
// not assumed. A wheel that pip put into a pixi environment is not pixi's, and
// falls through to the PyPI checks.
func detectPixiMethod(execPath string) (InstallMethod, bool) {
	prefix := filepath.Dir(execPath)
	for range 3 {
		if data, err := os.ReadFile(filepath.Join(prefix, "conda-meta", "pixi")); err == nil {
			return pixiEnvMethod(prefix, data)
		}
		prefix = filepath.Dir(prefix)
	}
	return InstallDirect, false
}

func pixiEnvMethod(prefix string, meta []byte) (InstallMethod, bool) {
	// conda records each package it links as <name>-<version>-<build>.json.
	if matches, _ := filepath.Glob(filepath.Join(prefix, "conda-meta", "skillshare-[0-9]*.json")); len(matches) > 0 {
		// pixi global keeps every global environment in one pixi-global.toml.
		var env struct {
			ManifestPath string `json:"manifest_path"`
		}
		if json.Unmarshal(meta, &env) == nil && filepath.Base(env.ManifestPath) == "pixi-global.toml" {
			return InstallPixiGlobal, true
		}
		return InstallPixi, true
	}
	// pixi installs PyPI dependencies with its own uv, which records itself as uv-pixi.
	if distInfo := pypiDistInfo(prefix); distInfo != "" {
		if installer, err := os.ReadFile(filepath.Join(distInfo, "INSTALLER")); err == nil && strings.TrimSpace(string(installer)) == "uv-pixi" {
			return InstallPixiPyPI, true
		}
	}
	return InstallDirect, false
}
