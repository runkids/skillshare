package version

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

// InstallMethod indicates how skillshare was installed.
type InstallMethod int

const (
	InstallDirect InstallMethod = iota // GitHub release / go install / manual
	InstallBrew                        // Homebrew (brew install)
)

// UpgradeCommand returns the CLI command the user should run to upgrade.
func (m InstallMethod) UpgradeCommand() string {
	if m == InstallBrew {
		return "brew upgrade skillshare"
	}
	return "skillshare upgrade"
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
