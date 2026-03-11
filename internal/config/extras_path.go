package config

import "path/filepath"

// ExtrasSourceDir returns the source directory for a named extra in global mode.
func ExtrasSourceDir(skillsSource, name string) string {
	return filepath.Join(filepath.Dir(skillsSource), "extras", name)
}

// ExtrasSourceDirProject returns the source directory for a named extra in project mode.
func ExtrasSourceDirProject(projectRoot, name string) string {
	return filepath.Join(projectRoot, ".skillshare", "extras", name)
}

// ExtrasParentDir returns the extras parent directory (for migration/init).
func ExtrasParentDir(skillsSource string) string {
	return filepath.Join(filepath.Dir(skillsSource), "extras")
}
