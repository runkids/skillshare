package install

import (
	"fmt"
	"path/filepath"
	"slices"
)

// BuiltinExtension is one transform extension bundled in the skillshare repo
// under extensions/, with a human-readable description so the dashboard can
// explain what it does before it is installed.
type BuiltinExtension struct {
	Name        string
	Description string
}

// BuiltinExtensions is the catalog of bundled transform extensions. The
// dashboard offers these for one-click install so users don't have to copy the
// reference scripts by hand.
var BuiltinExtensions = []BuiltinExtension{
	{Name: "codex-agents", Description: "Convert Markdown subagents (Claude format) into Codex CLI TOML agents"},
	{Name: "gemini-commands", Description: "Convert Markdown slash-commands (Claude/Cursor format) into Gemini CLI TOML commands"},
}

// IsBuiltinExtension reports whether name is a known bundled extension. It
// doubles as a whitelist guard against path traversal in download requests.
func IsBuiltinExtension(name string) bool {
	return slices.ContainsFunc(BuiltinExtensions, func(b BuiltinExtension) bool { return b.Name == name })
}

// BuiltinExtensionDescription returns the catalog description for name, or an
// empty string if name is not a bundled extension.
func BuiltinExtensionDescription(name string) string {
	for _, b := range BuiltinExtensions {
		if b.Name == name {
			return b.Description
		}
	}
	return ""
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
