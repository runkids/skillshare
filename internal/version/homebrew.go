package version

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// InstallMethod indicates how skillshare was installed.
type InstallMethod int

const (
	InstallDirect     InstallMethod = iota // GitHub release / go install / manual
	InstallBrew                            // Homebrew (brew install)
	InstallNix                             // Nix store, from a NixOS, Home Manager or overlay configuration
	InstallNixProfile                      // Nix store, from nix profile add
)

// NixConfigUpgradeHint tells a user whose Nix configuration provides
// skillshare how to update it. The steps differ per setup, so no single
// command fits.
const NixConfigUpgradeHint = "update skillshare in your Nix configuration and rebuild"

// UpgradeCommand returns the CLI command the user should run to upgrade, or ""
// when no single command does it.
func (m InstallMethod) UpgradeCommand() string {
	switch m {
	case InstallBrew:
		return "brew upgrade skillshare"
	case InstallNix:
		return ""
	case InstallNixProfile:
		return "nix profile upgrade skillshare"
	}
	return "skillshare upgrade"
}

// UpgradeInstruction returns how to upgrade as a lowercase instruction:
// "run: <command>", or NixConfigUpgradeHint when there is no command.
func (m InstallMethod) UpgradeInstruction() string {
	if cmd := m.UpgradeCommand(); cmd != "" {
		return "run: " + cmd
	}
	return NixConfigUpgradeHint
}

// IsNix reports whether Nix, not skillshare, must replace the binary.
func (m InstallMethod) IsNix() bool {
	return m == InstallNix || m == InstallNixProfile
}

// DetectCurrentInstallMethod determines how the running binary was installed,
// falling back to InstallDirect when its path cannot be resolved.
func DetectCurrentInstallMethod() InstallMethod {
	execPath, err := os.Executable()
	if err != nil {
		return InstallDirect
	}
	if execPath, err = filepath.EvalSymlinks(execPath); err != nil {
		return InstallDirect
	}
	return DetectInstallMethod(execPath)
}

// DetectInstallMethod determines the installation method from the resolved
// executable path. The caller should resolve symlinks before calling this.
func DetectInstallMethod(execPath string) InstallMethod {
	homebrewPrefixes := []string{
		"/usr/local/Cellar/skillshare",
		"/opt/homebrew/Cellar/skillshare",
		"/home/linuxbrew/.linuxbrew/Cellar/skillshare",
	}
	for _, prefix := range homebrewPrefixes {
		if strings.HasPrefix(execPath, prefix) {
			return InstallBrew
		}
	}
	// The Nix store is read-only to its users; Nix itself must replace the binary.
	if strings.HasPrefix(execPath, "/nix/store/") {
		return detectNixMethod(execPath, nixProfileManifests())
	}
	return InstallDirect
}

// brewInfoResult mirrors the subset of `brew info --json=v2` we need.
type brewInfoResult struct {
	Formulae []struct {
		Versions struct {
			Stable string `json:"stable"`
		} `json:"versions"`
	} `json:"formulae"`
}

// FetchBrewLatestVersion returns the latest version available in the
// Homebrew formula by running `brew info --json=v2 skillshare`.
func FetchBrewLatestVersion() (string, error) {
	out, err := exec.Command("brew", "info", "--json=v2", "skillshare").Output()
	if err != nil {
		return "", fmt.Errorf("brew info failed: %w", err)
	}

	var info brewInfoResult
	if err := json.Unmarshal(out, &info); err != nil {
		return "", fmt.Errorf("failed to parse brew info: %w", err)
	}

	if len(info.Formulae) == 0 || info.Formulae[0].Versions.Stable == "" {
		return "", fmt.Errorf("no stable version found in brew info")
	}

	return info.Formulae[0].Versions.Stable, nil
}

// SudoUpgradeHint is the command shown when an upgrade needs a password but
// has no terminal to ask on. The dashboard matches on it to explain the failure.
// It has no sudo prefix: the CLI elevates only the binary replacement, while a
// fully root upgrade leaves root-owned skill files that later break git pull.
const SudoUpgradeHint = "skillshare upgrade"

// SudoUpgradeHintPrefix precedes SudoUpgradeHint in the CLI error.
const SudoUpgradeHintPrefix = "run in a terminal: "

// NixUpgradeNote marks the upgrade row that a Nix install prints when a newer
// CLI exists. The dashboard matches on it to explain why the CLI was kept.
const NixUpgradeNote = "installed with Nix"
