package version

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// nixProfileElement is the name `nix profile add github:runkids/skillshare`
// gives the flake's default package, and the one the upgrade command names.
const nixProfileElement = "skillshare"

// nixProfileManifests returns the manifests of the user's `nix profile`
// profiles: ~/.nix-profile, and its XDG location when use-xdg-base-directories
// is set.
func nixProfileManifests() []string {
	var paths []string
	if home, err := os.UserHomeDir(); err == nil {
		paths = append(paths, filepath.Join(home, ".nix-profile", "manifest.json"))
		state := os.Getenv("XDG_STATE_HOME")
		if state == "" {
			state = filepath.Join(home, ".local", "state")
		}
		paths = append(paths, filepath.Join(state, "nix", "profile", "manifest.json"))
	}
	return paths
}

// nixProfileManifest mirrors the subset of a version 3 `nix profile` manifest
// we need. nix-env profiles have no manifest.json, and older versions list
// elements without names, so neither parses into this.
type nixProfileManifest struct {
	Elements map[string]struct {
		StorePaths []string `json:"storePaths"`
	} `json:"elements"`
}

// detectNixMethod tells a `nix profile` install, which one command upgrades,
// from a binary a NixOS, Home Manager or nix-env setup provides. Home Manager
// adds one combined package to the profile, so the skillshare store path is
// only an element's own when the user added skillshare directly.
func detectNixMethod(execPath string, manifests []string) InstallMethod {
	parts := strings.SplitN(execPath, "/", 5) // "", "nix", "store", "<hash>-<name>", rest
	if len(parts) < 4 {
		return InstallNix
	}
	storePath := "/nix/store/" + parts[3]
	for _, manifest := range manifests {
		data, err := os.ReadFile(manifest)
		if err != nil {
			continue
		}
		var m nixProfileManifest
		if json.Unmarshal(data, &m) != nil {
			continue
		}
		if slices.Contains(m.Elements[nixProfileElement].StorePaths, storePath) {
			return InstallNixProfile
		}
	}
	return InstallNix
}
