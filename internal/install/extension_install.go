package install

import (
	"fmt"
	"path/filepath"
	"slices"
)

// BuiltinExtensions lists the transform extensions bundled in the skillshare
// repo under extensions/. The dashboard offers these for one-click install so
// users don't have to copy the reference scripts by hand.
var BuiltinExtensions = []string{"md2codex", "md2gemini"}

// IsBuiltinExtension reports whether name is a known bundled extension. It
// doubles as a whitelist guard against path traversal in download requests.
func IsBuiltinExtension(name string) bool {
	return slices.Contains(BuiltinExtensions, name)
}

// InstallBuiltinExtension downloads the bundled extension `name` from the
// skillshare repo (extensions/<name>) into destRoot/<name>. destRoot is the
// target extensions directory for the current mode (global or project).
func InstallBuiltinExtension(name, destRoot string) error {
	if !IsBuiltinExtension(name) {
		return fmt.Errorf("unknown built-in extension: %q", name)
	}
	dest := filepath.Join(destRoot, name)
	_, err := downloadGitHubDirWithAPIBase("runkids", "skillshare", "extensions/"+name, dest, "https://api.github.com", nil, nil)
	return err
}
