// Package pattern implements the include and exclude patterns used by filters.
package pattern

import (
	"fmt"
	"path/filepath"
	"strings"
)

// Normalize trims and validates include and exclude patterns, ignoring blanks.
func Normalize(include, exclude []string) ([]string, []string, error) {
	includePatterns, err := NormalizePatterns(include)
	if err != nil {
		return nil, nil, fmt.Errorf("invalid include pattern: %w", err)
	}
	excludePatterns, err := NormalizePatterns(exclude)
	if err != nil {
		return nil, nil, fmt.Errorf("invalid exclude pattern: %w", err)
	}
	return includePatterns, excludePatterns, nil
}

// NormalizePatterns trims and validates patterns, ignoring blanks.
func NormalizePatterns(patterns []string) ([]string, error) {
	if len(patterns) == 0 {
		return nil, nil
	}
	result := make([]string, 0, len(patterns))
	for _, pattern := range patterns {
		p := strings.TrimSpace(pattern)
		if p == "" {
			continue
		}
		if _, err := filepath.Match(p, ""); err != nil {
			return nil, fmt.Errorf("%q: %w", p, err)
		}
		result = append(result, p)
	}
	return result, nil
}

// FirstMatch returns the first pattern that matches name, or ("", false).
func FirstMatch(name string, patterns []string) (string, bool) {
	for _, pattern := range patterns {
		matched, err := filepath.Match(pattern, name)
		if err == nil && matched {
			return pattern, true
		}
	}
	return "", false
}

// Match applies normalized include and exclude patterns to a flat name.
// An empty include matches all names; excludes take precedence.
func Match(name string, include, exclude []string) bool {
	if _, matched := FirstMatch(name, include); len(include) > 0 && !matched {
		return false
	}
	_, excluded := FirstMatch(name, exclude)
	return !excluded
}
