package validate

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var validTargetNameRegex = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_-]*$`)

// Reserved names that cannot be used as target names
var reservedNames = []string{"add", "remove", "rm", "list", "ls", "help", "all"}

// validSkillNameRegex allows letters, numbers, underscores, and hyphens
// More permissive than target names - can start with number
var validSkillNameRegex = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_-]*$`)

// validFlatSkillNameRegex allows flat skill names with nested separators (__)
// Also allows _ prefix for tracked repository skills
// Examples: "my-skill", "_team__frontend__ui", "personal__email"
var validFlatSkillNameRegex = regexp.MustCompile(`^_?[a-zA-Z0-9][a-zA-Z0-9_-]*$`)

// validTrackedRepoNameRegex validates tracked repository names (start with _)
// Examples: "_team-repo", "_company-skills"
var validTrackedRepoNameRegex = regexp.MustCompile(`^_[a-zA-Z0-9][a-zA-Z0-9_-]*$`)

// TargetName validates a target name.
// Rules:
//   - Must start with a letter
//   - Can contain letters, numbers, underscores, and hyphens
//   - Length 1-64 characters
//   - Cannot be a reserved word
func TargetName(name string) error {
	if name == "" {
		return fmt.Errorf("target name cannot be empty")
	}

	if len(name) > 64 {
		return fmt.Errorf("target name too long (max 64 characters)")
	}

	if !validTargetNameRegex.MatchString(name) {
		return fmt.Errorf("target name must start with a letter and contain only letters, numbers, underscores, and hyphens")
	}

	for _, r := range reservedNames {
		if strings.EqualFold(name, r) {
			return fmt.Errorf("'%s' is a reserved name", name)
		}
	}

	return nil
}

// SkillName validates a skill name.
// Rules:
//   - Must start with a letter or number
//   - Can contain letters, numbers, underscores, and hyphens
//   - Length 1-64 characters
func SkillName(name string) error {
	if name == "" {
		return fmt.Errorf("skill name cannot be empty")
	}

	if len(name) > 64 {
		return fmt.Errorf("skill name too long (max 64 characters)")
	}

	if !validSkillNameRegex.MatchString(name) {
		return fmt.Errorf("skill name must start with a letter or number and contain only letters, numbers, underscores, and hyphens")
	}

	return nil
}

// Path validates a file system path.
// Rules:
//   - Cannot be empty
//   - Cannot contain null bytes (security)
//   - Length limit 4096 characters
func Path(path string) error {
	if path == "" {
		return fmt.Errorf("path cannot be empty")
	}

	if strings.ContainsRune(path, '\x00') {
		return fmt.Errorf("path contains invalid characters")
	}

	if len(path) > 4096 {
		return fmt.Errorf("path too long (max 4096 characters)")
	}

	return nil
}

// TargetPath validates a target path for adding as a skill sync target.
// Returns warnings (non-fatal) and errors (fatal).
// Errors are returned when:
//   - The path does not exist
//   - The path is not a directory
//
// Warnings are returned when:
//   - The path doesn't end with "skills" or "skill"
func TargetPath(path string) (warnings []string, err error) {
	// Basic path validation
	if err := Path(path); err != nil {
		return nil, err
	}

	// Check if path looks like a skills directory
	if !IsLikelySkillsPath(path) {
		base := filepath.Base(path)
		warnings = append(warnings, fmt.Sprintf("path doesn't end with 'skills' or 'skill' (got '%s')", base))
	}

	// Check if path exists
	info, statErr := os.Stat(path)
	if statErr != nil {
		if os.IsNotExist(statErr) {
			return nil, fmt.Errorf("path does not exist: %s", path)
		}
		return nil, fmt.Errorf("cannot access path: %w", statErr)
	} else if !info.IsDir() {
		return nil, fmt.Errorf("path exists but is not a directory")
	}

	return warnings, nil
}

// IsLikelySkillsPath checks if a path looks like a skills directory.
// Returns true if the path ends with "skills", "skill", or is a known CLI skills path.
func IsLikelySkillsPath(path string) bool {
	base := filepath.Base(path)
	if base == "skills" || base == "skill" {
		return true
	}

	// Check for known CLI patterns
	knownPatterns := []string{
		".claude/skills",
		".codex/skills",
		".cursor/skills",
		".gemini/antigravity/skills",
		".config/opencode/skills",
	}

	for _, pattern := range knownPatterns {
		if strings.HasSuffix(path, pattern) {
			return true
		}
	}

	return false
}

// FlatSkillName validates a flat skill name (used in target directories).
// Rules:
//   - Can optionally start with _ (for tracked repo skills)
//   - After optional _, must start with a letter or number
//   - Can contain letters, numbers, underscores (including __), and hyphens
//   - Length 1-128 characters (longer to allow nested paths)
//
// Examples: "my-skill", "_team__frontend__ui", "personal__email"
func FlatSkillName(name string) error {
	if name == "" {
		return fmt.Errorf("skill name cannot be empty")
	}

	if len(name) > 128 {
		return fmt.Errorf("skill name too long (max 128 characters)")
	}

	if !validFlatSkillNameRegex.MatchString(name) {
		return fmt.Errorf("skill name must start with a letter, number, or _ followed by alphanumeric, and contain only letters, numbers, underscores, and hyphens")
	}

	return nil
}

// IntoPath validates an --into subdirectory path.
// Rules:
//   - Cannot be empty
//   - Cannot be an absolute path
//   - Split by "/", each segment must pass SkillName() validation
//   - No empty, ".", or ".." segments
//   - Total length max 256 characters
func IntoPath(p string) error {
	if p == "" {
		return fmt.Errorf("--into path cannot be empty")
	}
	if len(p) > 256 {
		return fmt.Errorf("--into path too long (max 256 characters)")
	}
	if filepath.IsAbs(p) || strings.HasPrefix(p, "/") || strings.HasPrefix(p, "\\") {
		return fmt.Errorf("--into path must be relative, not absolute")
	}

	segments := strings.Split(filepath.ToSlash(p), "/")
	for _, seg := range segments {
		if seg == "" || seg == "." || seg == ".." {
			return fmt.Errorf("--into path contains invalid segment: %q", seg)
		}
		if err := SkillName(seg); err != nil {
			return fmt.Errorf("--into path segment %q: %w", seg, err)
		}
	}

	return nil
}

// TrackedRepoName validates a tracked repository name.
// Rules:
//   - Must start with _
//   - After _, must start with a letter or number
//   - Can contain letters, numbers, underscores, and hyphens
//   - Length 2-64 characters (_ + at least 1 char)
//
// Examples: "_team-repo", "_company-skills"
func TrackedRepoName(name string) error {
	if name == "" {
		return fmt.Errorf("tracked repo name cannot be empty")
	}

	if len(name) < 2 {
		return fmt.Errorf("tracked repo name too short (need _ plus at least one character)")
	}

	if len(name) > 64 {
		return fmt.Errorf("tracked repo name too long (max 64 characters)")
	}

	if !validTrackedRepoNameRegex.MatchString(name) {
		return fmt.Errorf("tracked repo name must start with _ followed by a letter or number, and contain only letters, numbers, underscores, and hyphens")
	}

	return nil
}
