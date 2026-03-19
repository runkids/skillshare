package config

import "path/filepath"

// ResolveExtrasSourceDir resolves the source directory for an extra using
// three-level priority: per-extra source > extras_source > default.
// All paths are expected to be already expanded (no ~ tildes).
func ResolveExtrasSourceDir(extra ExtraConfig, extrasSource, skillsSource string) string {
	if extra.Source != "" {
		return extra.Source
	}
	if extrasSource != "" {
		return filepath.Join(extrasSource, extra.Name)
	}
	return filepath.Join(filepath.Dir(skillsSource), "extras", extra.Name)
}

// ExtrasSourceDirProject returns the source directory for a named extra in project mode.
// Project mode does not support extras_source or per-extra Source — path is always fixed.
func ExtrasSourceDirProject(projectRoot, name string) string {
	return filepath.Join(projectRoot, ".skillshare", "extras", name)
}

// ExtrasParentDir returns the extras parent directory (for migration/init).
func ExtrasParentDir(skillsSource string) string {
	return filepath.Join(filepath.Dir(skillsSource), "extras")
}

// ExtrasParentDirProject returns the extras parent directory in project mode.
func ExtrasParentDirProject(projectRoot string) string {
	return filepath.Join(projectRoot, ".skillshare", "extras")
}

// ResolveExtrasSourceType returns which level resolved the source path.
// IMPORTANT: priority logic must mirror ResolveExtrasSourceDir above.
func ResolveExtrasSourceType(extra ExtraConfig, extrasSource string) string {
	if extra.Source != "" {
		return "per-extra"
	}
	if extrasSource != "" {
		return "extras_source"
	}
	return "default"
}
