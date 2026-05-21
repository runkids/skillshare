package config

import (
	"os"
	"path/filepath"
)

// PluginsSourceDirProject returns the plugin source directory in project mode.
func PluginsSourceDirProject(projectRoot string) string {
	return filepath.Join(projectRoot, ".skillshare", "plugins")
}

// HooksSourceDirProject returns the hook source directory in project mode.
func HooksSourceDirProject(projectRoot string) string {
	return filepath.Join(projectRoot, ".skillshare", "hooks")
}

// ClaudeMarketplaceRoot returns the rendered Claude marketplace root.
func ClaudeMarketplaceRoot(projectRoot string) string {
	if projectRoot != "" {
		return filepath.Join(projectRoot, ".skillshare", "rendered", "claude-marketplace")
	}
	return filepath.Join(BaseDir(), "rendered", "claude-marketplace")
}

// CodexMarketplaceRoot returns the Codex marketplace root.
func CodexMarketplaceRoot(projectRoot string) string {
	if projectRoot != "" {
		return filepath.Join(projectRoot, ".agents", "plugins")
	}
	home, _ := osUserHomeDir()
	return filepath.Join(home, ".agents", "plugins")
}

// CodexPluginCacheBase returns the native Codex plugin cache base.
func CodexPluginCacheBase() string {
	home, _ := osUserHomeDir()
	return filepath.Join(home, ".codex", "plugins", "cache")
}

// CodexPluginCacheRoot returns the Skillshare-managed Codex plugin cache root.
func CodexPluginCacheRoot() string {
	return filepath.Join(CodexPluginCacheBase(), "skillshare")
}

// CodexConfigPath returns the path to the Codex config TOML.
func CodexConfigPath() string {
	home, _ := osUserHomeDir()
	return filepath.Join(home, ".codex", "config.toml")
}

// CodexHooksConfigPath returns the path to Codex hooks.json for the given scope.
func CodexHooksConfigPath(projectRoot string) string {
	if projectRoot != "" {
		return filepath.Join(projectRoot, ".codex", "hooks.json")
	}
	home, _ := osUserHomeDir()
	return filepath.Join(home, ".codex", "hooks.json")
}

// CodexHooksRoot returns the Skillshare-managed Codex hooks root for the given scope.
func CodexHooksRoot(projectRoot string) string {
	if projectRoot != "" {
		return filepath.Join(projectRoot, ".codex", "hooks", "skillshare")
	}
	home, _ := osUserHomeDir()
	return filepath.Join(home, ".codex", "hooks", "skillshare")
}

// ClaudeSettingsPath returns the Claude settings.json path for the given scope.
func ClaudeSettingsPath(projectRoot string) string {
	if projectRoot != "" {
		return filepath.Join(projectRoot, ".claude", "settings.json")
	}
	home, _ := osUserHomeDir()
	return filepath.Join(home, ".claude", "settings.json")
}

// ClaudeInstalledPluginsPath returns the Claude installed_plugins.json metadata path.
func ClaudeInstalledPluginsPath() string {
	home, _ := osUserHomeDir()
	return filepath.Join(home, ".claude", "plugins", "installed_plugins.json")
}

// ClaudeHooksRoot returns the Skillshare-managed Claude hooks root for the given scope.
func ClaudeHooksRoot(projectRoot string) string {
	if projectRoot != "" {
		return filepath.Join(projectRoot, ".claude", "hooks", "skillshare")
	}
	home, _ := osUserHomeDir()
	return filepath.Join(home, ".claude", "hooks", "skillshare")
}

// osUserHomeDir exists to keep path helpers testable.
var osUserHomeDir = func() (string, error) {
	return os.UserHomeDir()
}
