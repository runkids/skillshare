package sync

import (
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"skillshare/internal/config"
	"skillshare/internal/resource"
	"skillshare/internal/utils"
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

// FilterAgents filters discovered agents by include/exclude patterns.
// Agent FlatNames include the .md extension (e.g. "tutor.md"), but filter
// patterns are matched against the name without extension so users can write
// intuitive patterns like "tutor" or "team-*" instead of "tutor.md".
func FilterAgents(agents []resource.DiscoveredResource, include, exclude []string) ([]resource.DiscoveredResource, error) {
	includePatterns, excludePatterns, err := normalizedFilterPatterns(include, exclude)
	if err != nil {
		return nil, err
	}

	filtered := make([]resource.DiscoveredResource, 0, len(agents))
	for _, agent := range agents {
		name := strings.TrimSuffix(agent.FlatName, ".md")
		if shouldSyncFlatName(name, includePatterns, excludePatterns) {
			filtered = append(filtered, agent)
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
	_, matched := firstMatchingPattern(name, patterns)
	return matched
}

// firstMatchingPattern returns the first pattern that matches name, or ("", false).
func firstMatchingPattern(name string, patterns []string) (string, bool) {
	for _, pattern := range patterns {
		matched, err := filepath.Match(pattern, name)
		if err != nil {
			continue
		}
		if matched {
			return pattern, true
		}
	}
	return "", false
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

// UnmatchedInclude is an include pattern that selects no discovered skill,
// together with the source-path names a bare pattern most likely meant.
type UnmatchedInclude struct {
	Pattern     string
	Suggestions []string
}

// FindUnmatchedIncludes returns the include patterns that select no skill.
// Patterns are matched against the source-path flat name (see FilterSkills), so
// a pattern written with the name the target directory shows selects nothing,
// and the entries a previous pattern created get pruned. exclude is not checked:
// a filter that blocks nothing is the normal state of a spare exclusion.
// Callers must have validated the patterns through FilterSkills first, which is
// also why an invalid pattern yields no report here.
func FindUnmatchedIncludes(include []string, skills []DiscoveredSkill) []UnmatchedInclude {
	patterns, err := normalizePatterns(include)
	if err != nil || len(patterns) == 0 {
		return nil
	}

	var unmatched []UnmatchedInclude
	for _, pattern := range patterns {
		selected := false
		for _, skill := range skills {
			if matched, _ := filepath.Match(pattern, skill.FlatName); matched {
				selected = true
				break
			}
		}
		if selected {
			continue
		}
		unmatched = append(unmatched, UnmatchedInclude{
			Pattern:     pattern,
			Suggestions: suggestFlatNames(pattern, skills),
		})
	}
	return unmatched
}

// suggestFlatNames returns the flat names whose last path segment is the
// pattern, which is the name the target shows under standard naming. Glob
// patterns get no suggestion, and the result is capped because this is printed
// per target on every sync.
func suggestFlatNames(pattern string, skills []DiscoveredSkill) []string {
	if strings.ContainsAny(pattern, "*?[") {
		return nil
	}

	seen := make(map[string]bool)
	var names []string
	for _, skill := range skills {
		if skill.FlatName == pattern || seen[skill.FlatName] {
			continue
		}
		flat := skill.FlatName
		last := flat[strings.LastIndex(flat, utils.NestedSeparator)+len(utils.NestedSeparator):]
		if last != pattern {
			continue
		}
		seen[flat] = true
		names = append(names, flat)
	}
	slices.Sort(names)
	if len(names) > 3 {
		names = names[:3]
	}
	return names
}

// FilterSkillsByTarget removes skills whose Targets field does not include
// the given target name.  Skills with nil Targets (no field declared) pass
// through unconditionally.
func FilterSkillsByTarget(skills []DiscoveredSkill, targetName string) []DiscoveredSkill {
	filtered := make([]DiscoveredSkill, 0, len(skills))
	for _, skill := range skills {
		if skill.Targets == nil {
			filtered = append(filtered, skill)
			continue
		}
		for _, t := range skill.Targets {
			if config.MatchesTargetName(t, targetName) {
				filtered = append(filtered, skill)
				break
			}
		}
	}
	return filtered
}

// FilterAgentsByTarget removes agents whose frontmatter targets field does not
// include the given target name. Agents with nil Targets (no field declared)
// pass through unconditionally. Mirrors FilterSkillsByTarget.
func FilterAgentsByTarget(agents []resource.DiscoveredResource, targetName string) []resource.DiscoveredResource {
	filtered := make([]resource.DiscoveredResource, 0, len(agents))
	for _, agent := range agents {
		if agent.Targets == nil {
			filtered = append(filtered, agent)
			continue
		}
		for _, t := range agent.Targets {
			if config.MatchesTargetName(t, targetName) {
				filtered = append(filtered, agent)
				break
			}
		}
	}
	return filtered
}
