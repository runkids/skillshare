package plugin

import (
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
)

type ompInstalledPlugin struct {
	name, scope, selection string
	rows                   []OMPExtensionRow
}

func (inv *ompInventory) plugins(base, agent, scope string) {
	roots := []ompRoot{}
	if root := ompPluginRoot(base, agent); root != "" {
		roots = append(roots, ompRoot{root, scope})
	} else {
		inv.warn("The agent config_dir does not establish a user plugin root; default-user plugins were not inspected.")
	}
	if inv.cwd != "" {
		if root := inv.projectPluginRoot(base); root != "" && (len(roots) == 0 || root != roots[0].path) {
			roots = append(roots, ompRoot{root, "project"})
		}
	}
	overrides := map[string]any{}
	overridesKnown := true
	if inv.cwd != "" {
		for _, dir := range []string{".omp", ".pi"} {
			file := filepath.Join(inv.cwd, dir, "plugin-overrides.json")
			if _, err := os.Stat(file); os.IsNotExist(err) {
				continue
			}
			overrides, overridesKnown = inv.document(file, false)
			break
		}
	}
	var all []ompInstalledPlugin
	for _, root := range roots {
		all = append(all, inv.collectPlugins(root, overrides, overridesKnown)...)
	}
	projectNames := map[string]bool{}
	for _, p := range all {
		if p.scope == "project" && p.selection != "disabled" {
			projectNames[p.name] = true
		}
	}
	for _, p := range all {
		for _, row := range p.rows {
			if p.scope != "project" && projectNames[p.name] && p.selection != "disabled" {
				row.Selection = "shadowed"
				row.Notes = append(row.Notes, "Project plugin with the same package name wins.")
			}
			inv.view.Rows = append(inv.view.Rows, row)
		}
	}
}

// Plugin anchors may walk ancestors; native extensions deliberately do not.
func (inv *ompInventory) projectPluginRoot(base string) string {
	configDir := os.Getenv("PI_CONFIG_DIR")
	if configDir == "" {
		configDir = ".omp"
	}
	for _, anchor := range []string{configDir, ".git"} {
		for dir := inv.cwd; dir != inv.home; dir = filepath.Dir(dir) {
			if dir != filepath.Dir(base) {
				if st, err := os.Stat(filepath.Join(dir, anchor)); err == nil && (anchor == ".git" || st.IsDir()) {
					return filepath.Join(dir, configDir, "plugins")
				}
			}
			if filepath.Dir(dir) == dir {
				break
			}
		}
	}
	return ""
}

func (inv *ompInventory) collectPlugins(root ompRoot, overrides map[string]any, overridesKnown bool) []ompInstalledPlugin {
	if st, err := os.Stat(filepath.Join(root.path, "node_modules")); err != nil || !st.IsDir() {
		return nil
	}
	packagePath := filepath.Join(root.path, "package.json")
	pkg, pkgOK := inv.document(packagePath, false)
	_, pkgErr := os.Stat(packagePath)
	deps, depsOK := pkg["dependencies"].(map[string]any)
	if pkg["dependencies"] == nil {
		depsOK = true
	}
	lockPath := filepath.Join(root.path, "omp-plugins.lock.json")
	lock, lockOK := inv.document(lockPath, false)
	_, lockErr := os.Stat(lockPath)
	states, statesOK := lock["plugins"].(map[string]any)
	if lock["plugins"] == nil {
		statesOK = true
	}
	// Marketplace installs can have only a runtime lock and node_modules links,
	// with no root package.json. OMP treats its missing dependencies as empty.
	pkgKnown := pkgOK && depsOK && (pkgErr == nil || os.IsNotExist(pkgErr))
	known := pkgKnown && lockOK && statesOK && lockErr == nil && overridesKnown
	if !known {
		inv.selectionUncertain = true
		inv.warn("Plugin metadata is missing, malformed, or unsupported; selection is unknown at " + root.path)
	}
	names := map[string]bool{}
	for name := range deps {
		names[name] = true
	}
	for name := range states {
		names[name] = true
	}
	// Without usable metadata, inspect only direct installed package manifests, never
	// invent an enabled state. Scoped npm packages remain under their @scope directory.
	if !pkgKnown || !lockOK || lockErr != nil {
		for _, entry := range inv.entries(filepath.Join(root.path, "node_modules")) {
			if strings.HasPrefix(entry.Name(), ".") {
				continue
			}
			if strings.HasPrefix(entry.Name(), "@") {
				for _, child := range inv.entries(filepath.Join(root.path, "node_modules", entry.Name())) {
					names[entry.Name()+"/"+child.Name()] = true
				}
			} else {
				names[entry.Name()] = true
			}
		}
	}
	keys := make([]string, 0, len(names))
	for name := range names {
		keys = append(keys, name)
	}
	sort.Strings(keys)
	var out []ompInstalledPlugin
	for _, name := range keys {
		// Metadata-controlled paths must remain a single npm package beneath node_modules.
		parts := strings.Split(name, "/")
		valid := len(parts) == 1 && namePattern.MatchString(parts[0])
		if len(parts) == 2 && strings.HasPrefix(parts[0], "@") {
			valid = namePattern.MatchString(strings.TrimPrefix(parts[0], "@")) && namePattern.MatchString(parts[1])
		}
		if !valid {
			inv.warn("Unsupported installed package identity; name omitted.")
			continue
		}
		dir := filepath.Join(root.path, "node_modules", name)
		manifestPath := filepath.Join(dir, "package.json")
		manifestDoc, manifestOK := inv.document(manifestPath, false)
		if _, err := os.Stat(manifestPath); err != nil {
			manifestOK = false
			inv.warn("Missing or unreadable installed plugin manifest: " + manifestPath)
		}
		value, hasOMP := manifestDoc["omp"]
		if !hasOMP || value == nil {
			value = manifestDoc["pi"]
		}
		manifest, isPlugin := value.(map[string]any)
		if manifestOK && !isPlugin {
			continue
		}
		state, stateOK := states[name].(map[string]any)
		selection := "selected"
		if !known || !manifestOK || (states[name] != nil && !stateOK) {
			selection = "unknown"
		}
		if stateOK {
			enabled, ok := state["enabled"].(bool)
			if !ok {
				selection = "unknown"
			} else if !enabled && selection != "unknown" {
				selection = "disabled"
			}
		}
		disabled, validDisabled := ompStringArray(overrides["disabled"])
		if !validDisabled {
			selection = "unknown"
		}
		if slices.Contains(disabled, name) && selection != "unknown" {
			selection = "disabled"
		}
		version, _ := manifestDoc["version"].(string)
		plugin := ompInstalledPlugin{name: name, scope: root.scope, selection: selection}
		entries, validEntries := ompManifestEntries(manifest["extensions"])
		features, validFeatures := manifest["features"].(map[string]any)
		if manifest["features"] == nil {
			validFeatures = true
		}
		enabledFeatures, validEnabled := ompStringArray(state["enabledFeatures"])
		useDefaults := state["enabledFeatures"] == nil
		if overrides["features"] != nil {
			featureOverrides, ok := overrides["features"].(map[string]any)
			if !ok {
				validEnabled = false
			} else if value, exists := featureOverrides[name]; exists {
				enabledFeatures, validEnabled = ompStringArray(value)
				useDefaults = value == nil
			}
		}
		if !validEntries || !validFeatures || !validEnabled {
			plugin.selection = "unknown"
		}
		featureNames := make([]string, 0, len(features))
		for name := range features {
			featureNames = append(featureNames, name)
		}
		sort.Strings(featureNames)
		for _, featureName := range featureNames {
			feature, ok := features[featureName].(map[string]any)
			if !ok {
				plugin.selection = "unknown"
				continue
			}
			enabled := slices.Contains(enabledFeatures, featureName) || (useDefaults && feature["default"] == true)
			if !enabled {
				continue
			}
			more, ok := ompStringArray(feature["extensions"])
			if !ok {
				plugin.selection = "unknown"
				continue
			}
			entries = append(entries, more...)
		}
		unsupportedSource := false
		for _, entry := range entries {
			if strings.Contains(entry, "://") || strings.ContainsAny(entry, "\r\n\x00") {
				inv.warn("Unsupported plugin extension path syntax in " + filepath.Join(dir, "package.json") + "; source value omitted.")
				unsupportedSource = true
				continue
			}
			p := filepath.Join(dir, entry)
			st, err := os.Stat(p)
			paths := []string{p}
			if err == nil && st.IsDir() {
				paths = inv.scan(p, "plugin")
			}
			for _, p := range paths {
				row := inv.row(p, "plugin", root.scope)
				row.Owner = "plugin"
				row.PluginName, row.PluginRoot, row.PluginVersion = name, dir, version
				row.Notes = append(row.Notes, "Installed plugin: "+name)
				if row.Selection == "selected" {
					row.Selection = plugin.selection
				}
				if slices.Contains(inv.disabled, row.DerivedID) && row.Selection != "unknown" {
					row.Selection = "disabled"
				}
				plugin.rows = append(plugin.rows, row)
			}
		}
		if unsupportedSource || !manifestOK || !validEntries || !validFeatures || !validEnabled {
			inv.selectionUncertain = true
			plugin.rows = append(plugin.rows, OMPExtensionRow{Path: filepath.Join(dir, "package.json"), Name: name, DerivedID: "plugin-manifest:" + name, Source: "plugin", Scope: root.scope, Selection: "unknown", Owner: "plugin", PluginName: name, PluginRoot: dir, PluginVersion: version, Notes: []string{"Plugin manifest, source or feature selection could not be established."}})
		}
		out = append(out, plugin)
	}
	return out
}

func ompStringArray(value any) ([]string, bool) {
	if value == nil {
		return nil, true
	}
	list, ok := value.([]any)
	if !ok {
		return nil, false
	}
	var out []string
	for _, entry := range list {
		text, ok := entry.(string)
		if !ok {
			return out, false
		}
		out = append(out, text)
	}
	return out, true
}

func ompManifestEntries(value any) ([]string, bool) {
	if text, ok := value.(string); ok {
		return []string{text}, true
	}
	return ompStringArray(value)
}
