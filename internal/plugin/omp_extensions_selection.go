package plugin

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"unicode/utf8"
)

// ompNativePackage identifies npm/Bun/source installs through the selected
// launcher, not arbitrary package.json files in HOME. Standalone binaries and
// wrappers without provable package ownership remain read-only; never run them.
func (s *Service) ompNativePackage(target string) (string, string) {
	// Account CLI is plugin-command routing only, not an extension runtime.
	bin := "omp"
	path, err := exec.LookPath(bin)
	if err != nil {
		return "", "Cannot identify an installed OMP 18.6.1 package without executing the CLI."
	}
	path, err = filepath.EvalSymlinks(path)
	if err != nil {
		return "", "Cannot resolve the OMP launcher."
	}
	path, _ = filepath.Abs(path)
	for dir := filepath.Dir(path); ; dir = filepath.Dir(dir) {
		raw, err := ompRead(filepath.Join(dir, "package.json"))
		if err == nil {
			var pkg struct {
				Name, Version string
				Bin           map[string]string
			}
			if json.Unmarshal(raw, &pkg) != nil || pkg.Name != "@oh-my-pi/pi-coding-agent" || pkg.Version != "18.6.1" {
				return hash(raw), "Only the verified OMP 18.6.1 package is selectable."
			}
			entry, err := filepath.EvalSymlinks(filepath.Join(dir, pkg.Bin["omp"]))
			if pkg.Bin["omp"] == "" || err != nil || entry != path {
				return hash(raw), "OMP launcher ownership cannot be established; wrappers and standalone binaries are read-only."
			}
			return hash(raw) + "\x00" + path, ""
		}
		if !os.IsNotExist(err) {
			return "", "OMP package metadata is unreadable."
		}
		if filepath.Dir(dir) == dir {
			break
		}
	}
	return "", "OMP launcher has no verified package metadata; standalone binaries are read-only."
}

// Reject links rather than following user-controlled configuration/source aliases.
// Writes also anchor the settings parent with os.OpenRoot and revalidate before rename.
func ompSafePath(path string) error {
	if !filepath.IsAbs(path) || !utf8.ValidString(path) || strings.ContainsAny(path, "\x00\r\n") {
		return fmt.Errorf("unsafe OMP path")
	}
	for p := filepath.Clean(path); ; p = filepath.Dir(p) {
		st, err := os.Lstat(p)
		if err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("unreadable OMP path")
		}
		if err == nil && (st.Mode()&os.ModeSymlink != 0 || p != path && !st.IsDir()) {
			return fmt.Errorf("linked or non-directory OMP path component")
		}
		if filepath.Dir(p) == p {
			break
		}
	}
	return nil
}

func (inv *ompInventory) selection(s *Service, target string) {
	v := inv.view
	v.disabled = slices.Clone(inv.disabled)
	gate, reason := s.ompNativePackage(target)
	if reason == "" && !ompLockSupported() {
		reason = "Native OMP locking is not implemented on this platform."
	}
	if reason == "" && (inv.uncertain || inv.selectionUncertain) {
		reason = "Effective OMP settings are unknown; resolve the inventory warnings first."
	}
	if reason == "" {
		if err := ompSafePath(v.SettingsPath); err != nil {
			reason = err.Error()
		}
	}
	if reason == "" {
		if _, _, err := ompYAML(v.SettingsPath); err != nil {
			reason = err.Error()
		}
	}
	if reason == "" {
		base := filepath.Join(inv.home, ".omp")
		defaultAgent := filepath.Join(base, "agent")
		agent := filepath.Dir(v.SettingsPath)
		if s.ProjectRoot != "" {
			agent = defaultAgent
			if override := os.Getenv("PI_CODING_AGENT_DIR"); override != "" {
				agent = override
			}
		}
		if ompPluginRoot(base, agent) == "" {
			root := ompPluginRoot(base, defaultAgent)
			if _, err := os.Lstat(root); err == nil || !os.IsNotExist(err) {
				reason = "The explicit agent directory does not establish ownership of the native user plugin root; resolve plugin name collisions in OMP first."
			}
		}
	}
	if reason == "" && s.ProjectRoot != "" {
		home, _ := os.UserHomeDir()
		if filepath.Clean(s.ProjectRoot) == filepath.Clean(home) {
			reason = "OMP does not discover project settings at HOME."
		}
	}
	// Constructing native settings when only legacy user JSON exists migrates it.
	// Do not create YAML that prevents that migration from ever running.
	if reason == "" {
		_, err := os.Stat(v.SettingsPath)
		if os.IsNotExist(err) {
			if _, err := os.Lstat(filepath.Join(filepath.Dir(v.SettingsPath), "settings.json")); err == nil {
				reason = "Legacy settings require native migration before YAML selection can be edited."
			}
		}
	}
	// Native startup loads .env inputs; this static editor cannot establish their overrides.
	for _, root := range []string{filepath.Dir(v.SettingsPath), filepath.Dir(filepath.Dir(v.SettingsPath)), s.ProjectRoot} {
		if root == "" {
			continue
		}
		if _, err := os.Lstat(filepath.Join(root, ".env")); err == nil {
			reason = "Native .env overrides are not evaluated; selection is read-only."
		}
	}
	counts := map[string]int{}
	bypasses := map[string]bool{}
	for _, r := range v.Rows {
		if r.Source == "hook" || slices.Contains(r.Notes, "Explicit file bypasses the extension-module disabled-name filter.") {
			bypasses[r.Path] = true
		}
		if strings.HasPrefix(r.DerivedID, "extension-module:") {
			counts[r.DerivedID]++
		}
	}
	fingerprints := []string{gate}
	selectable := false
	for i := range v.Rows {
		r := &v.Rows[i]
		r.Key = hash([]byte(r.Source + "\x00" + r.Scope + "\x00" + r.Path + "\x00" + r.DerivedID))
		if r.Selection != "unknown" {
			enabled := !slices.Contains(inv.disabled, r.DerivedID)
			r.Enabled = &enabled
		}
		switch {
		case r.Owner == "hooks":
			r.ReadOnlyReason = "Managed by Hooks; change it on the Hooks page."
		case slices.Contains(r.Notes, "Hooks ownership record exists but the file has drifted; resolve it in Hooks before changing extension selection."):
			r.ReadOnlyReason = "Hooks-owned file has drifted; resolve it on the Hooks page."
		case r.Owner == "plugin" || r.Source == "plugin":
			r.ReadOnlyReason = "Managed by its plugin; change it on the Plugins page."
		case r.Source == "hook":
			r.ReadOnlyReason = "Ambient hooks use hook IDs and bypass the extension-module filter."
		case reason != "":
			r.ReadOnlyReason = reason
		case bypasses[r.Path]:
			r.ReadOnlyReason = "This file also loads through an explicit file or ambient hook that bypasses extension-module filtering."
		case counts[r.DerivedID] > 1:
			r.ReadOnlyReason = "Duplicate derived-name group: changing this ID affects multiple entries; rename or resolve the collision first."
		case r.Selection == "unknown" || r.Path == "":
			r.ReadOnlyReason = "Effective selection could not be established."
		case slices.Contains(r.Notes, "Explicit file bypasses the extension-module disabled-name filter."):
			r.ReadOnlyReason = "Explicit configured files bypass disabledExtensions; edit their configuration in OMP."
		case !ompScanFile(filepath.Base(r.Path), "plugin"):
			r.ReadOnlyReason = "Unsupported extension entry-point type; selection is read-only."
		case r.Source == "native" && ompHasGitignore(r.Path):
			r.ReadOnlyReason = "Native gitignore filtering is not reproduced."
		default:
			if err := ompSafePath(r.Path); err != nil {
				r.ReadOnlyReason = err.Error()
			}
		}
		raw, err := ompRead(r.Path)
		fingerprints = append(fingerprints, r.Path+"\x00"+hash(raw)+"\x00"+fmt.Sprint(err))
		if r.ReadOnlyReason == "" && err != nil {
			r.ReadOnlyReason = "Extension file is unreadable or exceeds the static inventory limit."
		}
		r.Selectable = r.ReadOnlyReason == ""
		selectable = selectable || r.Selectable
	}
	v.ReadOnly = !selectable
	if reason != "" {
		v.Reasons = append(v.Reasons, reason)
	}
	slices.Sort(inv.metadata)
	raw, _ := json.Marshal([]any{target, s.ProjectRoot, v.SettingsPath, v.Rows, inv.metadata, fingerprints})
	v.Revision = hash(raw)
}
