package version

import (
	"os"
	"path/filepath"
	"strings"
)

// pypiPackage is the PyPI distribution that ships the skillshare binary as a
// wheel, and the name the upgrade commands use.
const pypiPackage = "skillshare-cli"

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
	if !hasPyPIDistInfo(prefix) {
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
	return InstallPip, true
}

// ponytail: Glob reads [, * and ? in the prefix as pattern syntax; list the
// directories with os.ReadDir if such install paths show up.
func hasPyPIDistInfo(prefix string) bool {
	distInfo := strings.ReplaceAll(pypiPackage, "-", "_") + "-*.dist-info"
	for _, dir := range pypiSitePackages {
		if matches, _ := filepath.Glob(filepath.Join(prefix, filepath.FromSlash(dir), distInfo)); len(matches) > 0 {
			return true
		}
	}
	return false
}
