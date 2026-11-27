package version

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// pypiPackage is the PyPI distribution that ships the skillshare binary as a
// wheel, and the name the upgrade commands use.
const pypiPackage = "skillshare-cli"

// pypiJSONURL is PyPI's JSON API for the package; tests point it elsewhere.
var pypiJSONURL = "https://pypi.org/pypi/" + pypiPackage + "/json"

// pypiSitePackages are where an installer puts a wheel's metadata, relative to
// the prefix whose bin (Scripts on Windows) holds the binary: POSIX venvs and
// --user, Debian's dist-packages, Fedora's lib64, Windows venvs and system
// Python, and the Windows --user scheme.
var pypiSitePackages = []string{
	"lib/python*/site-packages",
	"lib/python*/dist-packages",
	"lib64/python*/site-packages",
	"Lib/site-packages",
	"site-packages",
}

// detectPyPIMethod tells a binary installed from the skillshare-cli wheel by
// the package's own dist-info next to its prefix. That keeps a binary
// install.sh put in ~/.local/bin a direct install, although pip --user uses
// the same folder.
func detectPyPIMethod(execPath string) (InstallMethod, bool) {
	bin := filepath.Dir(execPath)
	if name := filepath.Base(bin); !strings.EqualFold(name, "bin") && !strings.EqualFold(name, "Scripts") {
		return InstallDirect, false
	}
	prefix := filepath.Dir(bin)
	if pypiDistInfo(prefix) == "" {
		return InstallDirect, false
	}
	// uv tool install leaves a receipt in the tool's environment. pyvenv.cfg
	// cannot tell it apart: uv venv writes the same file.
	if _, err := os.Stat(filepath.Join(prefix, "uv-receipt.toml")); err == nil {
		return InstallUV, true
	}
	// pipx records each app it installs in that app's environment, wherever
	// PIPX_HOME points.
	if _, err := os.Stat(filepath.Join(prefix, "pipx_metadata.json")); err == nil {
		return InstallPipx, true
	}
	// uvx runs the wheel from an environment in uv's archive cache, which a
	// later run with @latest replaces rather than upgrades.
	if strings.HasPrefix(filepath.Base(filepath.Dir(prefix)), "archive-v") {
		return InstallUVX, true
	}
	// uv venv records uv in pyvenv.cfg and, unless seeded, installs no pip,
	// so pip's command would reach another Python.
	if pyvenvHasUV(filepath.Join(prefix, "pyvenv.cfg")) && !hasPip(bin) {
		return InstallUVPip, true
	}
	return InstallPip, true
}

func hasPip(bin string) bool {
	for _, name := range []string{"pip", "pip.exe"} {
		if _, err := os.Stat(filepath.Join(bin, name)); err == nil {
			return true
		}
	}
	return false
}

func pyvenvHasUV(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		if key, _, ok := strings.Cut(scanner.Text(), "="); ok && strings.TrimSpace(key) == "uv" {
			return true
		}
	}
	return false
}

// FetchPyPILatestVersion returns the newest skillshare-cli version on PyPI.
// PyPI installs upgrade from there, and its upload follows the GitHub release,
// so GitHub can name a version their installer cannot fetch yet.
func FetchPyPILatestVersion() (string, error) {
	client := &http.Client{Timeout: 15 * time.Second} // GitHub's limit, since Check runs after commands
	resp, err := client.Get(pypiJSONURL)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("PyPI returned HTTP %d", resp.StatusCode)
	}
	var project struct {
		Info struct {
			Version string `json:"version"`
		} `json:"info"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&project); err != nil {
		return "", fmt.Errorf("failed to parse PyPI response: %w", err)
	}
	if project.Info.Version == "" {
		return "", fmt.Errorf("no version in PyPI response")
	}
	return project.Info.Version, nil
}

// ponytail: Glob reads [, * and ? in the prefix as pattern syntax; list the
// directories with os.ReadDir if such install paths show up.
// pypiDistInfo returns the skillshare-cli dist-info directory under prefix, or "".
func pypiDistInfo(prefix string) string {
	distInfo := strings.ReplaceAll(pypiPackage, "-", "_") + "-*.dist-info"
	for _, dir := range pypiSitePackages {
		if matches, _ := filepath.Glob(filepath.Join(prefix, filepath.FromSlash(dir), distInfo)); len(matches) > 0 {
			return matches[0]
		}
	}
	return ""
}
