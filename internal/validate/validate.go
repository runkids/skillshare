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
// Warnings are returned when:
//   - The path doesn't end with "skills"
//   - The path doesn't exist yet
//   - The parent directory doesn't exist
func TargetPath(path string) (warnings []string, err error) {
	// Basic path validation
	if err := Path(path); err != nil {
		return nil, err
	}

	// Check if path ends with "skills" (common convention)
	base := filepath.Base(path)
	if base != "skills" {
		warnings = append(warnings, fmt.Sprintf("path doesn't end with 'skills' (got '%s')", base))
	}

	// Check if path exists
	info, statErr := os.Stat(path)
	if statErr != nil {
		if os.IsNotExist(statErr) {
			// Path doesn't exist - check parent
			parent := filepath.Dir(path)
			if _, parentErr := os.Stat(parent); os.IsNotExist(parentErr) {
				warnings = append(warnings, fmt.Sprintf("parent directory doesn't exist: %s", parent))
			} else {
				warnings = append(warnings, "target directory doesn't exist yet (will be created on sync)")
			}
		} else {
			return nil, fmt.Errorf("cannot access path: %w", statErr)
		}
	} else if !info.IsDir() {
		return nil, fmt.Errorf("path exists but is not a directory")
	}

	return warnings, nil
}

// IsLikelySkillsPath checks if a path looks like a skills directory.
// Returns true if the path ends with "skills" or is a known CLI skills path.
func IsLikelySkillsPath(path string) bool {
	base := filepath.Base(path)
	if base == "skills" {
		return true
	}

	// Check for known CLI patterns
	knownPatterns := []string{
		".claude/skills",
		".codex/skills",
		".cursor/skills",
		".gemini/antigravity/skills",
		".config/opencode/skill",
	}

	for _, pattern := range knownPatterns {
		if strings.HasSuffix(path, pattern) {
			return true
		}
	}

	return false
}
