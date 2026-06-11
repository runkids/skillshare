package utils

import "strings"

// NestedSeparator is the separator used in flat names for nested paths.
// Example: "_team/frontend/ui" becomes "_team__frontend__ui"
const NestedSeparator = "__"

// TrackedRepoPrefix marks directories as tracked repositories.
// Directories starting with _ are cloned repos that preserve .git
const TrackedRepoPrefix = "_"

// IsHidden checks if a file/directory name starts with a dot.
// Returns false for empty strings to prevent panic on empty names.
func IsHidden(name string) bool {
	return len(name) > 0 && name[0] == '.'
}

// HasTildePrefix checks if a path starts with ~.
// Returns false for empty strings to prevent panic on empty paths.
func HasTildePrefix(path string) bool {
	return len(path) > 0 && path[0] == '~'
}

// PathToFlatName converts a relative path to a flat name using __ as separator.
// Example: "_team/frontend/ui" -> "_team__frontend__ui"
// Example: "my-skill" -> "my-skill" (no change for flat paths)
func PathToFlatName(relPath string) string {
	// Normalize path separators and remove leading/trailing slashes
	relPath = strings.ReplaceAll(relPath, "\\", "/")
	relPath = strings.Trim(relPath, "/")

	if relPath == "" || relPath == "." {
		return ""
	}

	// Replace / with __
	return strings.ReplaceAll(relPath, "/", NestedSeparator)
}

// FlatNameToPath converts a flat name back to a relative path.
// Example: "_team__frontend__ui" -> "_team/frontend/ui"
// Example: "my-skill" -> "my-skill" (no change for flat names)
func FlatNameToPath(flatName string) string {
	if flatName == "" {
		return ""
	}

	// Replace __ with /
	return strings.ReplaceAll(flatName, NestedSeparator, "/")
}

// IsTrackedRepoDir checks if a directory name indicates a tracked repository.
// Tracked repos start with _ and contain .git directory.
func IsTrackedRepoDir(name string) bool {
	return strings.HasPrefix(name, TrackedRepoPrefix)
}

// HasNestedSeparator checks if a name contains the nested separator (__).
// This indicates the name came from a nested path structure.
func HasNestedSeparator(name string) bool {
	return strings.Contains(name, NestedSeparator)
}
