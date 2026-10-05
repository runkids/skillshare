package plugin

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// Oh My Pi (omp) v18.6.1 installs marketplace plugins the way Claude does: a marketplace is a
// folder with .omp-plugin/marketplace.json or .claude-plugin/marketplace.json, a plugin is
// name@marketplace, and `omp plugin install` copies the plugin into OMP's cache and links it
// into the scope's node_modules without running it. Skillshare therefore registers a snapshot
// as a marketplace and drives fresh-cache installs through the omp CLI. Scoped removal
// uses a non-destructive adapter that retains cache; native upgrade stays blocked. Its npm, Git
// and linked plugins are installed by bun, which runs install scripts, so they are listed but
// never managed. OMP accounts are not plugin targets: another config directory does not move
// OMP's plugin roots (see accountEnv).

// marketplaceAgent reports whether an Agent installs plugins as name@marketplace from a
// marketplace Skillshare registers for its snapshot.
func marketplaceAgent(agent string) bool {
	return agent == "claude" || agent == "codex" || agent == "omp"
}

// ompName is OMP's rule for plugin and marketplace names (marketplace/types.ts).
var ompName = regexp.MustCompile(`^[a-zA-Z0-9]([a-zA-Z0-9.-]*[a-zA-Z0-9])?$`)

func validOMPName(name string) bool { return len(name) <= 64 && ompName.MatchString(name) }

// ompHostProblem is why OMP plugins cannot be managed here, "" when they can. A profile or
// config-dir override moves OMP's roots, and a project is anchored at the nearest .omp folder,
// else the nearest Git root: a command run elsewhere would write into another root.
func (s *Service) ompHostProblem() (key, message string) {
	if os.Getenv("PI_CONFIG_DIR") != "" || os.Getenv("OMP_PROFILE") != "" || os.Getenv("PI_PROFILE") != "" || os.Getenv("PI_CODING_AGENT_DIR") != "" {
		return "plugins.error.ompRootEnv", "PI_CONFIG_DIR, PI_CODING_AGENT_DIR, OMP_PROFILE or PI_PROFILE is set, so Skillshare cannot verify OMP's plugin roots; plugin writes are blocked."
	}
	if s.ProjectRoot == "" {
		return "", ""
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "plugins.error.commandFailed", err.Error()
	}
	anchor := ompProjectAnchor(s.ProjectRoot, home)
	if anchor == "" || filepath.Clean(anchor) == filepath.Clean(s.ProjectRoot) {
		return "", ""
	}
	return "plugins.error.ompProjectAnchor", fmt.Sprintf("OMP keeps this project's plugins under %s, not here; manage plugins from that folder, or add a .omp folder to this project.", anchor)
}

// ompProjectAnchor is where OMP resolves cwd's project plugins (discovery/helpers.ts
// resolveActiveProjectRegistryPath): the nearest ancestor with a .omp folder, else the nearest
// one with .git, never home or above it. "" means OMP would create .omp/plugins in cwd itself.
func ompProjectAnchor(cwd, home string) string {
	for _, marker := range []string{".omp", ".git"} {
		for dir := filepath.Clean(cwd); dir != filepath.Clean(home); dir = filepath.Dir(dir) {
			if info, err := os.Stat(filepath.Join(dir, marker)); err == nil && (marker == ".git" || info.IsDir()) {
				return dir
			}
			if filepath.Dir(dir) == dir {
				break
			}
		}
	}
	return ""
}

// ompPackageName accepts the npm package names bun installs for OMP, scoped or not.
var ompPackageName = regexp.MustCompile(`^(@[a-z0-9][a-z0-9._-]*/)?[a-z0-9][a-z0-9._-]*$`)

// parseOMPInventory reads `omp plugin list --json`: marketplace plugins per scope, and the
// npm, Git and linked plugins of the user scope, which are listed by their package name.
func parseOMPInventory(data []byte, project string) ([]Installed, error) {
	var list struct {
		Npm []struct {
			Name    string `json:"name"`
			Version string `json:"version"`
			Enabled bool   `json:"enabled"`
		} `json:"npm"`
		Marketplace []struct {
			ID      string `json:"id"`
			Scope   string `json:"scope"`
			Entries []struct {
				Version string `json:"version"`
				Enabled *bool  `json:"enabled"`
			} `json:"entries"`
		} `json:"marketplace"`
	}
	if json.Unmarshal(data, &list) != nil || list.Npm == nil || list.Marketplace == nil {
		return nil, fmt.Errorf("unrecognized OMP plugin list schema")
	}
	scope := "user"
	if project != "" {
		scope = "project"
	}
	result := []Installed{}
	for _, item := range list.Marketplace {
		if !validID(item.ID) || len(item.Entries) == 0 {
			return nil, fmt.Errorf("native plugin list contains an unsupported plugin identifier")
		}
		name, market, _ := strings.Cut(item.ID, "@")
		entry := item.Entries[0]
		if item.Scope != scope {
			continue
		}
		result = append(result, Installed{ID: item.ID, Name: name, Marketplace: market, Version: entry.Version, Scope: scope, Installed: true, EnabledKnown: true, Enabled: entry.Enabled == nil || *entry.Enabled})
	}
	if project != "" {
		return result, nil
	}
	for _, item := range list.Npm {
		if !ompPackageName.MatchString(item.Name) {
			return nil, fmt.Errorf("native plugin list contains an unsupported plugin identifier")
		}
		result = append(result, Installed{ID: item.Name, Name: item.Name, Version: item.Version, Scope: "user", Installed: true, EnabledKnown: true, Enabled: item.Enabled})
	}
	return result, nil
}

// ompCachePath is where omp would cache a plugin of this identity (marketplace/cache.ts):
// <cacheRoot>/<marketplace>___<name>___<version>, 0.0.0 when no manifest names a version.
func ompCachePath(cacheRoot, id, version string) string {
	name, market, _ := strings.Cut(id, "@")
	if version == "" {
		version = "0.0.0"
	}
	return filepath.Join(cacheRoot, market+"___"+name+"___"+version)
}

// ompGuard blocks the omp changes whose native command would delete or replace a cache folder.
// omp deletes a plugin's cache folder on uninstall and replaces it on upgrade unless the user
// registry or the one project visible from the working directory still refers to it; every
// other project is invisible, so nothing proves the folder is unused. Installs choose
// an absent cache identity; scoped removals use a separate adapter.
func ompGuard(h Host, c *Change) {
	switch c.Action {
	case "install":
		if h.ompCacheRoot == "" || strings.TrimPrefix(h.Version, "omp/") != "18.6.1" || c.Binding.Source == "" {
			c.Action = "blocked"
			c.Message, c.MessageKey, c.MessageArgs = "OMP installation requires version 18.6.1, a reviewed source and a verified cache root; no native changes were made.", "plugins.error.ompInstallSafety", nil
			break
		}
		if err := ompFreshCache(h, c); err != nil {
			c.Action = "blocked"
			c.Message, c.MessageKey, c.MessageArgs = "OMP's cache destination cannot be safely allocated; installing could replace files another project uses. Installation is blocked.", "plugins.error.ompCacheInUse", nil
		}
	case "update":
		c.Action = "blocked"
		c.Message, c.MessageKey, c.MessageArgs = "OMP upgrades can replace cache files used by projects absent from its inventory. Skillshare leaves the installation unchanged; updating requires a non-destructive upgrade contract.", "plugins.error.ompNativeDestructive", nil
	}
}

// ompDefaultCacheRoot is omp's plugin cache when no plugin is installed yet to show it.
func ompDefaultCacheRoot() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	if xdg := os.Getenv("XDG_DATA_HOME"); xdg != "" && !filepath.IsAbs(xdg) {
		return ""
	}
	base := filepath.Join(home, ".omp")
	if root := ompPluginRoot(base, filepath.Join(base, "agent")); root != "" && ompSafePath(root) == nil {
		return filepath.Join(root, "cache", "plugins")
	}
	return ""
}

// ompInstallVersion is the version omp records for a marketplace plugin whose catalog entry has
// none (marketplace/manager.ts #resolvePluginVersion): .claude-plugin/plugin.json, then root
// plugin.json, then package.json. "" means omp would record a Git SHA or 0.0.0.
func ompInstallVersion(root string) string {
	for _, name := range []string{".claude-plugin/plugin.json", "plugin.json", "package.json"} {
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(name)))
		if err != nil {
			continue
		}
		var m struct {
			Version string `json:"version"`
		}
		if json.Unmarshal(data, &m) == nil && m.Version != "" {
			return m.Version
		}
	}
	return ""
}

var ansiSequence = regexp.MustCompile("\x1b\\[[0-9;]*m")

// ompMarketplaceLine is one row of `omp plugin marketplace list`: the name, two spaces, its source.
var ompMarketplaceLine = regexp.MustCompile(`^\s*(\S+)\s{2,}(.+?)\s*$`)

// parseOMPMarketplaces reads `omp plugin marketplace list`, which has no --json in v18.6.1. A
// local marketplace's source is its resolved absolute path, so it serves as the root.
func parseOMPMarketplaces(data []byte) (map[string]string, error) {
	text := ansiSequence.ReplaceAllString(string(data), "")
	markets := map[string]string{}
	for _, line := range strings.Split(text, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || trimmed == "No marketplaces configured" || strings.HasPrefix(trimmed, "Configured Marketplaces") || strings.HasPrefix(trimmed, "Add one with") {
			continue
		}
		m := ompMarketplaceLine.FindStringSubmatch(line)
		if m == nil || !validOMPName(m[1]) {
			return nil, fmt.Errorf("unrecognized native marketplace list")
		}
		markets[m[1]] = m[2]
	}
	return markets, nil
}

// ompExtensions reports whether package.json declares OMP extension modules, which OMP imports
// in-process when it starts; a Pi manifest counts, as OMP falls back to it.
func ompExtensions(root string) bool {
	data, err := os.ReadFile(filepath.Join(root, "package.json"))
	if err != nil {
		return false
	}
	var m struct {
		OMP struct {
			Extensions json.RawMessage `json:"extensions"`
		} `json:"omp"`
		Pi struct {
			Extensions json.RawMessage `json:"extensions"`
		} `json:"pi"`
	}
	if json.Unmarshal(data, &m) != nil {
		return false
	}
	return len(m.OMP.Extensions) > 0 || len(m.Pi.Extensions) > 0
}
