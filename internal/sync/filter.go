package sync

import (
	"fmt"
	"path/filepath"
	"strings"
)

// FilterSkills filters discovered skills by include/exclude patterns.
// Matching uses filepath.Match against DiscoveredSkill.FlatName.
func FilterSkills(skills []DiscoveredSkill, include, exclude []string) ([]DiscoveredSkill, error) {
	includePatterns, excludePatterns, err := normalizedFilterPatterns(include, exclude)
	if err != nil {
		return nil, err
	}

	filtered := make([]DiscoveredSkill, 0, len(skills))
	for _, skill := range skills {
		if shouldSyncFlatName(skill.FlatName, includePatterns, excludePatterns) {
			filtered = append(filtered, skill)
		}
	}

	return filtered, nil
}

// ShouldSyncFlatName returns whether a single flat skill name should be managed
// by the given include/exclude filters.
func ShouldSyncFlatName(flatName string, include, exclude []string) (bool, error) {
	includePatterns, excludePatterns, err := normalizedFilterPatterns(include, exclude)
	if err != nil {
		return false, err
	}
	return shouldSyncFlatName(flatName, includePatterns, excludePatterns), nil
}

func normalizedFilterPatterns(include, exclude []string) ([]string, []string, error) {
	includePatterns, err := normalizePatterns(include)
	if err != nil {
		return nil, nil, fmt.Errorf("invalid include pattern: %w", err)
	}
	excludePatterns, err := normalizePatterns(exclude)
	if err != nil {
		return nil, nil, fmt.Errorf("invalid exclude pattern: %w", err)
	}
	return includePatterns, excludePatterns, nil
}

func normalizePatterns(patterns []string) ([]string, error) {
	if len(patterns) == 0 {
		return nil, nil
	}

	normalized := make([]string, 0, len(patterns))
	for _, pattern := range patterns {
		p := strings.TrimSpace(pattern)
		if p == "" {
			continue
		}
		if _, err := filepath.Match(p, ""); err != nil {
			return nil, fmt.Errorf("%q: %w", p, err)
		}
		normalized = append(normalized, p)
	}

	return normalized, nil
}

func matchesAnyPattern(name string, patterns []string) bool {
	for _, pattern := range patterns {
		matched, err := filepath.Match(pattern, name)
		if err != nil {
			continue
		}
		if matched {
			return true
		}
	}
	return false
}

func shouldSyncFlatName(name string, includePatterns, excludePatterns []string) bool {
	if len(includePatterns) > 0 && !matchesAnyPattern(name, includePatterns) {
		return false
	}
	if len(excludePatterns) > 0 && matchesAnyPattern(name, excludePatterns) {
		return false
	}
	return true
}
