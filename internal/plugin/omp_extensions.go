package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"

	"gopkg.in/yaml.v3"
)

// OMPExtensionsView is a static inventory of OMP v18.6.1 inputs.
// Selected means selected by the inspected files, not imported or running.
// CLI flags, overlays, foreign providers, factory exports and trust are not evaluated.
type OMPExtensionsView struct {
	Target       string            `json:"target"`
	Scope        string            `json:"scope"`
	Root         string            `json:"root"`
	SettingsPath string            `json:"settingsPath"`
	ReadOnly     bool              `json:"readOnly"`
	Reasons      []string          `json:"reasons"`
	Warnings     []string          `json:"warnings"`
	Rows         []OMPExtensionRow `json:"rows"`
	Revision     string            `json:"revision"`
	disabled     []string
}

type OMPExtensionRow struct {
	Key            string   `json:"key"`
	Selectable     bool     `json:"selectable"`
	ReadOnlyReason string   `json:"readOnlyReason"`
	Enabled        *bool    `json:"enabled"`
	Path           string   `json:"path"`
	Name           string   `json:"name"`
	DerivedID      string   `json:"derivedId"`
	Source         string   `json:"source"`    // native, configured, hook, plugin
	Scope          string   `json:"scope"`     // global, project, account
	Selection      string   `json:"selection"` // selected, disabled, shadowed, unknown
	Owner          string   `json:"owner"`     // native (unmanaged), hooks, plugin, unknown
	Notes          []string `json:"notes"`
	HooksTarget    string   `json:"hooksTarget,omitempty"`
	// Native package identity and manifest version, not inferred from extension filenames.
	PluginName    string `json:"pluginName,omitempty"`
	PluginRoot    string `json:"pluginRoot,omitempty"`
	PluginVersion string `json:"pluginVersion,omitempty"`
}

type ompInventory struct {
	view               *OMPExtensionsView
	home, cwd          string
	uncertain          bool
	disabled           []string
	metadata           []string
	selectionUncertain bool
}

type ompRoot struct{ path, scope string }

// OMPExtensions never calls the native CLI, imports code, migrates settings, or writes.
func (s *Service) OMPExtensions(ctx context.Context, target string) (*OMPExtensionsView, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	base := filepath.Join(home, ".omp")
	agent := filepath.Join(base, "agent")
	if override := os.Getenv("PI_CODING_AGENT_DIR"); target == "omp" && override != "" {
		if !filepath.IsAbs(override) {
			return nil, fmt.Errorf("PI_CODING_AGENT_DIR must be absolute")
		}
		agent = filepath.Clean(override)
	}
	scope := "global"
	if target != "omp" {
		account, ok := s.Accounts[target]
		if !ok || account.Agent != "omp" || account.Dir == "" || s.ProjectRoot != "" {
			return nil, fmt.Errorf("not an OMP target")
		}
		if !filepath.IsAbs(account.Dir) {
			return nil, fmt.Errorf("OMP config_dir must be absolute")
		}
		agent = filepath.Clean(account.Dir)
		scope = "account"
	}
	v := &OMPExtensionsView{Target: target, Scope: scope, Root: agent, ReadOnly: true, Reasons: []string{"Static OMP v18.6.1 filesystem inventory; selection does not prove a running extension.", "CLI flags, runtime overrides, foreign providers and config overlays are not evaluated."}, Warnings: []string{}, Rows: []OMPExtensionRow{}}
	inv := &ompInventory{view: v, home: home, cwd: s.ProjectRoot}
	if ompUnsupportedRootEnv() {
		inv.uncertain = true
		inv.warn("Native config-root/profile overrides are not resolved; explicit config_dir targets remain pinned, but runtime selection is unknown.")
	}
	roots := []ompRoot{{agent, scope}}
	if s.ProjectRoot != "" {
		v.Scope, v.Root = "project", filepath.Join(s.ProjectRoot, ".omp")
		roots = append([]ompRoot{{v.Root, "project"}}, roots...)
	}
	// YAML arrays replace lower settings layers; legacy JSON still contributes native paths.
	globalPath := filepath.Join(agent, "config.yml")
	if _, err := os.Stat(globalPath); os.IsNotExist(err) {
		if _, err = os.Stat(filepath.Join(agent, "config.yaml")); err == nil {
			globalPath = filepath.Join(agent, "config.yaml")
		}
	}
	v.SettingsPath = globalPath
	effective := inv.settings(globalPath)
	if s.ProjectRoot != "" {
		v.SettingsPath = filepath.Join(v.Root, "config.yml")
		for _, file := range []string{filepath.Join(v.Root, "settings.json"), v.SettingsPath} {
			for key, value := range inv.settings(file) {
				if value != nil {
					effective[key] = value
				}
			}
		}
	}
	inv.disabled = inv.strings(effective, "disabledExtensions", v.SettingsPath)
	configured := inv.strings(effective, "extensions", v.SettingsPath)
	if os.Getenv("PI_CONFIG_FILES") != "" {
		inv.uncertain = true
		inv.warn("Config overlays are present; effective selection is unknown.")
	}
	if _, err := os.Stat(globalPath); os.IsNotExist(err) {
		if _, err := os.Stat(filepath.Join(agent, "settings.json")); err == nil {
			inv.warn("Legacy user settings may require OMP migration; this inventory does not migrate or read agent.db.")
		}
	}
	// Foreign project settings can replace native arrays. Do not pretend they are native.
	if s.ProjectRoot != "" {
		for _, dir := range []string{".claude", ".codex", ".gemini"} {
			for _, file := range []string{"settings.json", "config.yml"} {
				if _, err := os.Stat(filepath.Join(s.ProjectRoot, dir, file)); err == nil {
					inv.uncertain = true
					inv.warn("Foreign project settings are present; effective selection is unknown.")
				}
			}
		}
	}
	// Native capability deduplicates by derived name, project first. Path dedup happens later.
	for _, root := range roots {
		for _, p := range inv.scan(filepath.Join(root.path, "extensions"), "native") {
			inv.add(p, "native", root.scope, false)
		}
	}
	for _, root := range roots {
		file := filepath.Join(root.path, "settings.json")
		for _, raw := range inv.strings(inv.settings(file), "extensions", file) {
			p, ok := inv.resolve(raw, root.scope)
			if !ok {
				continue
			}
			if st, err := os.Stat(p); err == nil && st.IsDir() {
				for _, entry := range inv.scan(p, "native") {
					inv.add(entry, "native", root.scope, false)
				}
			} else {
				inv.add(p, "native", root.scope, false)
			}
		}
	}
	// Capability precedence is project first even when the project entry came
	// from legacy settings rather than the native extensions directory.
	slices.SortStableFunc(v.Rows, func(a, b OMPExtensionRow) int {
		if a.Scope == b.Scope {
			return 0
		}
		if a.Scope == "project" {
			return -1
		}
		if b.Scope == "project" {
			return 1
		}
		return 0
	})
	nativeNames := map[string]bool{}
	for i := range v.Rows {
		row := &v.Rows[i]
		if row.Source != "native" || row.Selection == "disabled" {
			continue
		}
		if st, err := os.Stat(row.Path); err != nil || !st.Mode().IsRegular() {
			continue
		}
		if nativeNames[row.DerivedID] {
			row.Selection = "shadowed"
			row.Notes = append(row.Notes, "Earlier native entry with the same derived name wins.")
		} else {
			nativeNames[row.DerivedID] = true
		}
	}
	// Hook-specific IDs must not be filtered as extension-module IDs.
	hookIDs := map[string]bool{}
	for _, root := range roots {
		for _, kind := range []string{"pre", "post"} {
			dir := filepath.Join(root.path, "hooks", kind)
			for _, entry := range inv.entries(dir) {
				if strings.HasPrefix(entry.Name(), ".") || !ompScanFile(entry.Name(), "configured") {
					continue
				}
				p := filepath.Join(dir, entry.Name())
				st, err := os.Stat(p)
				if err != nil || !st.Mode().IsRegular() {
					continue
				}
				row := inv.row(p, "hook", root.scope)
				row.DerivedID = "hook:" + kind + ":" + strings.TrimSuffix(entry.Name(), filepath.Ext(entry.Name())) + ":" + entry.Name()
				if slices.Contains(inv.disabled, row.DerivedID) {
					row.Selection = "disabled"
				} else if hookIDs[row.DerivedID] {
					row.Selection = "shadowed"
				} else {
					hookIDs[row.DerivedID] = true
				}
				v.Rows = append(v.Rows, row)
			}
		}
	}
	inv.plugins(base, agent, scope)
	for _, raw := range configured {
		p, ok := inv.resolve(raw, v.Scope)
		if !ok {
			continue
		}
		if st, err := os.Stat(p); err == nil && st.IsDir() {
			for _, entry := range inv.scan(p, "configured") {
				inv.add(entry, "configured", v.Scope, false)
			}
		} else {
			inv.add(p, "configured", v.Scope, true)
		}
	}
	// The runtime's final dedup uses absolute spelling, not realpath or module name.
	seen := map[string]bool{}
	for i := range v.Rows {
		row := &v.Rows[i]
		if inv.uncertain && row.Selection != "shadowed" {
			row.Selection = "unknown"
			row.Notes = append(row.Notes, "Effective settings could not be established.")
		}
		if row.Path == "" || row.Selection == "disabled" || row.Selection == "shadowed" {
			continue
		}
		if seen[row.Path] {
			row.Selection = "shadowed"
			row.Notes = append(row.Notes, "Earlier absolute path wins; symlink aliases are not collapsed.")
		} else {
			seen[row.Path] = true
		}
	}
	inv.hooksOwners(s, target)
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	inv.selection(s, target)
	return v, nil
}

func ompExtensionName(p string) string {
	// Native normalizes backslashes even on POSIX before deriving the ID.
	p = strings.ReplaceAll(p, "\\", "/")
	base := filepath.Base(p)
	if base == "index.ts" || base == "index.js" {
		return filepath.Base(filepath.Dir(p))
	}
	if dot := strings.LastIndex(base, "."); dot > 0 {
		return base[:dot]
	}
	return base
}

func (inv *ompInventory) warn(message string) {
	if !slices.Contains(inv.view.Warnings, message) {
		inv.view.Warnings = append(inv.view.Warnings, message)
	}
}

// Read only regular bounded metadata files: no FIFO/device reads and no code imports.
func ompRead(p string) ([]byte, error) {
	st, err := os.Stat(p)
	if err != nil {
		return nil, err
	}
	if !st.Mode().IsRegular() || st.Size() > 1024*1024 {
		return nil, fmt.Errorf("unsupported metadata file")
	}
	f, err := os.Open(p)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, 1024*1024+1))
	if len(data) > 1024*1024 {
		return nil, fmt.Errorf("metadata file too large")
	}
	return data, err
}

func (inv *ompInventory) document(p string, yamlFile bool) (map[string]any, bool) {
	data, err := ompRead(p)
	inv.metadata = append(inv.metadata, p+"\x00"+hash(data)+"\x00"+fmt.Sprint(err))
	if os.IsNotExist(err) {
		return map[string]any{}, true
	}
	if err != nil {
		inv.warn("Cannot read metadata: " + p)
		return nil, false
	}
	var doc map[string]any
	if yamlFile {
		dec := yaml.NewDecoder(strings.NewReader(string(data)))
		err = dec.Decode(&doc)
		if err == io.EOF {
			return map[string]any{}, true
		}
		if err == nil {
			var extra any
			if dec.Decode(&extra) != io.EOF {
				err = fmt.Errorf("multiple YAML documents")
			}
		}
	} else {
		err = json.Unmarshal(data, &doc)
	}
	if err != nil || doc == nil {
		inv.warn("Malformed metadata: " + p)
		return nil, false
	}
	return doc, true
}

func (inv *ompInventory) settings(p string) map[string]any {
	doc, ok := inv.document(p, filepath.Ext(p) != ".json")
	if !ok {
		inv.uncertain = true
	}
	if doc == nil {
		return map[string]any{}
	}
	return doc
}

func (inv *ompInventory) strings(doc map[string]any, key, p string) []string {
	value := doc[key]
	if value == nil {
		return nil
	}
	items, ok := value.([]any)
	if !ok {
		inv.uncertain = true
		inv.warn("Unsupported " + key + " list in " + p)
		return nil
	}
	var out []string
	for _, item := range items {
		if value, ok := item.(string); ok {
			out = append(out, value)
		} else {
			inv.uncertain = true
			inv.warn("Unsupported " + key + " entry in " + p)
		}
	}
	return out
}

func (inv *ompInventory) resolve(raw, scope string) (string, bool) {
	// OMP also supports aliases/protocols/Unicode shorthand. This milestone does not.
	if raw == "" || strings.Contains(raw, "://") || strings.HasPrefix(raw, "@") || strings.HasPrefix(raw, ":") || strings.ContainsAny(raw, "\r\n\x00") {
		inv.warn("Unsupported configured path syntax; source value omitted.")
		inv.omitted(scope, "unsupported", "Unsupported path or protocol; not resolved.")
		return "", false
	}
	if raw == "~" {
		raw = inv.home
	} else if strings.HasPrefix(raw, "~/") {
		raw = filepath.Join(inv.home, raw[2:])
	}
	if !filepath.IsAbs(raw) {
		if inv.cwd == "" {
			inv.warn("Relative configured paths require a known session cwd.")
			inv.omitted(scope, "relative", "Relative path omitted: session cwd is unknown.")
			return "", false
		}
		raw = filepath.Join(inv.cwd, raw)
	}
	return filepath.Clean(raw), true
}

func (inv *ompInventory) omitted(scope, kind, note string) {
	inv.selectionUncertain = true
	id := "configured:" + kind
	for _, row := range inv.view.Rows {
		if row.Scope == scope && row.DerivedID == id {
			return
		}
	}
	inv.view.Rows = append(inv.view.Rows, OMPExtensionRow{Name: "Unresolved configured path", DerivedID: id, Source: "configured", Scope: scope, Selection: "unknown", Owner: "unknown", Notes: []string{note}})
}

func (inv *ompInventory) row(p, source, scope string) OMPExtensionRow {
	name := ompExtensionName(p)
	row := OMPExtensionRow{Path: p, Name: name, DerivedID: "extension-module:" + name, Source: source, Scope: scope, Selection: "selected", Owner: "native", Notes: []string{}}
	if st, err := os.Stat(p); err != nil || !st.Mode().IsRegular() {
		row.Selection = "unknown"
		row.Notes = append(row.Notes, "Entry is missing, unreadable, or not a regular file.")
	}
	return row
}

func (inv *ompInventory) add(p, source, scope string, explicit bool) {
	row := inv.row(p, source, scope)
	if slices.Contains(inv.disabled, row.DerivedID) && !explicit {
		row.Selection = "disabled"
	}
	if explicit {
		row.Notes = append(row.Notes, "Explicit file bypasses the extension-module disabled-name filter.")
	}
	if source == "native" && ompHasGitignore(p) && row.Selection == "selected" {
		row.Selection = "unknown"
		row.Notes = append(row.Notes, "Native gitignore filtering is not reproduced.")
		inv.warn("Native candidates with gitignore files have unknown selection.")
	}
	inv.view.Rows = append(inv.view.Rows, row)
}

func ompHasGitignore(p string) bool {
	for dir := filepath.Dir(p); ; dir = filepath.Dir(dir) {
		if _, err := os.Stat(filepath.Join(dir, ".gitignore")); err == nil {
			return true
		}
		if filepath.Dir(dir) == dir {
			return false
		}
	}
}

func (inv *ompInventory) entries(dir string) []os.DirEntry {
	entries, err := os.ReadDir(dir)
	if err != nil && !os.IsNotExist(err) {
		inv.warn("Cannot scan directory: " + dir)
	}
	return entries
}

func ompScanFile(name, mode string) bool {
	ext := filepath.Ext(name)
	if mode == "plugin" {
		return slices.Contains([]string{".ts", ".js", ".mjs", ".cjs"}, ext) && !strings.HasSuffix(name, ".d.ts") && !strings.HasSuffix(name, ".d.mts") && !strings.HasSuffix(name, ".d.cts")
	}
	return ext == ".ts" || ext == ".js"
}

func ompIndexes(mode string) []string {
	if mode == "plugin" {
		return []string{"index.ts", "index.js", "index.mjs", "index.cjs"}
	}
	return []string{"index.ts", "index.js"}
}

func ompIndex(dir, mode string) string {
	for _, name := range ompIndexes(mode) {
		p := filepath.Join(dir, name)
		if st, err := os.Stat(p); err == nil && st.Mode().IsRegular() {
			return p
		}
	}
	return ""
}

func (inv *ompInventory) manifest(dir, mode string) ([]string, bool) {
	file := filepath.Join(dir, "package.json")
	doc, _ := inv.document(file, false)
	value, exists := doc["omp"]
	if !exists || value == nil {
		value = doc["pi"]
	}
	manifest, _ := value.(map[string]any)
	entries, ok := manifest["extensions"].([]any)
	if !ok || len(entries) == 0 {
		return nil, false
	}
	out := []string{}
	declared := mode != "native"
	for _, entry := range entries {
		raw, ok := entry.(string)
		if !ok {
			inv.warn("Unsupported manifest extension entry in " + file)
			continue
		}
		declared = true
		if strings.Contains(raw, "://") || strings.ContainsAny(raw, "\r\n\x00") {
			inv.warn("Unsupported manifest path syntax in " + file + "; source value omitted.")
			continue
		}
		p := raw
		if !filepath.IsAbs(p) {
			p = filepath.Join(dir, raw)
		}
		st, err := os.Stat(p)
		if err != nil {
			inv.warn("Missing declared extension: " + p)
			continue
		}
		if st.IsDir() {
			if mode == "native" {
				_, ts := os.Stat(filepath.Join(p, "index.ts"))
				_, js := os.Stat(filepath.Join(p, "index.js"))
				if ts == nil && js == nil {
					inv.uncertain = true
					inv.warn("Declared native directory has ambiguous index ordering: " + p)
				}
			}
			p = ompIndex(p, mode)
			if p == "" {
				continue
			}
		}
		out = append(out, p)
	}
	return out, declared
}

// Native scans direct files plus child manifest/index, never the root's own index
// or manifest. Configured/plugin directories use manifest, index, then one-level scan.
func (inv *ompInventory) scan(dir, mode string) []string {
	if mode != "native" {
		if files, declared := inv.manifest(dir, mode); declared {
			return files
		}
		if p := ompIndex(dir, mode); p != "" {
			return []string{p}
		}
	}
	var out, declaredFiles, indexes []string
	for _, entry := range inv.entries(dir) {
		if mode == "native" && strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		p := filepath.Join(dir, entry.Name())
		st, err := os.Stat(p)
		if err != nil {
			continue
		}
		if st.IsDir() {
			if files, declared := inv.manifest(p, mode); declared {
				if mode == "native" {
					declaredFiles = append(declaredFiles, files...)
				} else {
					out = append(out, files...)
				}
				continue
			}
			if index := ompIndex(p, mode); index != "" {
				if mode == "native" {
					indexes = append(indexes, index)
				} else {
					out = append(out, index)
				}
			}
		} else if st.Mode().IsRegular() && ompScanFile(entry.Name(), mode) {
			out = append(out, p)
		}
	}
	return append(append(out, declaredFiles...), indexes...)
}

// Ownership comes from the hooks ledger plus the current file hash, never its name.
func (inv *ompInventory) hooksOwners(s *Service, target string) {
	file := filepath.Join(s.StateDir, "hooks", "state.json")
	data, err := ompRead(file)
	inv.metadata = append(inv.metadata, file+"\x00"+hash(data)+"\x00"+fmt.Sprint(err))
	if os.IsNotExist(err) {
		return
	}
	var ledger struct {
		Version int                                                         `json:"version"`
		Records map[string]struct{ Owner, Target, Path, Root, Hash string } `json:"records"`
	}
	if err != nil || json.Unmarshal(data, &ledger) != nil || ledger.Version != 1 {
		inv.uncertain = true
		inv.warn("Hooks ownership ledger is unreadable or unsupported.")
		return
	}
	for i := range inv.view.Rows {
		row := &inv.view.Rows[i]
		rec, ok := ledger.Records[hash([]byte("file\x00"+row.Path))]
		ownerMatches := rec.Owner == s.ConfigPath || row.Scope == "global" && s.GlobalConfigPath != "" && rec.Owner == s.GlobalConfigPath
		if !ok || rec.Path != row.Path || rec.Target != target || !ownerMatches || (rec.Root != "" && rec.Root != s.ProjectRoot) {
			continue
		}
		content, err := ompRead(row.Path)
		if err == nil && hash(content) == rec.Hash {
			row.Owner = "hooks"
			row.HooksTarget = target
			row.Notes = append(row.Notes, "Managed by Hooks; review changes in the Hooks page.")
		} else {
			row.Notes = append(row.Notes, "Hooks ownership record exists but the file has drifted; resolve it in Hooks before changing extension selection.")
		}
	}
}

// ompPluginRoot is intentionally separate from agent/config_dir. An arbitrary
// account directory does not prove that the default user's plugins belong to it.
func ompUnsupportedRootEnv() bool {
	return os.Getenv("PI_CONFIG_DIR") != "" || os.Getenv("OMP_PROFILE") != "" || os.Getenv("PI_PROFILE") != ""
}

func ompPluginRoot(base, agent string) string {
	if ompUnsupportedRootEnv() || agent != filepath.Join(base, "agent") {
		return ""
	}
	if runtime.GOOS == "linux" || runtime.GOOS == "darwin" {
		if xdg := os.Getenv("XDG_DATA_HOME"); xdg != "" {
			root := filepath.Join(xdg, "omp")
			if st, err := os.Stat(root); err == nil && st.IsDir() {
				return filepath.Join(root, "plugins")
			}
		}
	}
	return filepath.Join(base, "plugins")
}
