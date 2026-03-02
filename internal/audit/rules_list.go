package audit

import (
	"path/filepath"
	"sort"
)

// CompiledRule is the public view of a rule for listing/display.
type CompiledRule struct {
	ID       string `json:"id"`
	Severity string `json:"severity"`
	Pattern  string `json:"pattern"`
	Message  string `json:"message"`
	Regex    string `json:"regex"`
	Exclude  string `json:"exclude,omitempty"`
	Enabled  bool   `json:"enabled"`
	Source   string `json:"source"` // "builtin", "global", "project"
}

// PatternGroup summarizes rules grouped by pattern name.
type PatternGroup struct {
	Pattern     string `json:"pattern"`
	Total       int    `json:"total"`
	Enabled     int    `json:"enabled"`
	Disabled    int    `json:"disabled"`
	MaxSeverity string `json:"maxSeverity"`
}

// ListRules returns all rules (builtin + global overrides) with enable/disable status.
// Disabled rules are included with Enabled=false.
func ListRules() ([]CompiledRule, error) {
	base := builtinYAML()
	globalUser, err := loadUserRules(globalAuditRulesPath())
	if err != nil {
		return nil, err
	}
	return buildCompiledList(base, globalUser, nil)
}

// ListRulesWithProject returns all rules (builtin + global + project overrides)
// with enable/disable status.
func ListRulesWithProject(projectRoot string) ([]CompiledRule, error) {
	base := builtinYAML()
	globalUser, err := loadUserRules(globalAuditRulesPath())
	if err != nil {
		return nil, err
	}
	projectPath := filepath.Join(projectRoot, ".skillshare", "audit-rules.yaml")
	projectUser, err := loadUserRules(projectPath)
	if err != nil {
		return nil, err
	}
	return buildCompiledList(base, globalUser, projectUser)
}

// buildCompiledList merges base, global user, and project user rule layers
// and produces the full listing. Disabled rules are included with Enabled=false.
func buildCompiledList(base, globalUser, projectUser []yamlRule) ([]CompiledRule, error) {
	// Track which IDs are overridden at each layer.
	globalOverrides := indexByID(globalUser)
	projectOverrides := indexByID(projectUser)

	// Track pattern-level directives at each layer.
	globalPatternOverrides := patternLevelSet(globalUser)
	projectPatternOverrides := patternLevelSet(projectUser)

	// Merge layers: base → global → project
	merged := base
	if globalUser != nil {
		merged = mergeYAMLRules(merged, globalUser)
	}
	if projectUser != nil {
		merged = mergeYAMLRules(merged, projectUser)
	}

	var result []CompiledRule
	for _, yr := range merged {
		// Skip pattern-level directives — they're not real rules.
		if isPatternLevel(yr) {
			continue
		}

		enabled := yr.Enabled == nil || *yr.Enabled
		sev := yr.Severity
		if s, err := NormalizeSeverity(sev); err == nil {
			sev = s
		}

		source := determineSource(yr.ID, yr.Pattern, globalOverrides, projectOverrides,
			globalPatternOverrides, projectPatternOverrides)

		result = append(result, CompiledRule{
			ID:       yr.ID,
			Severity: sev,
			Pattern:  yr.Pattern,
			Message:  yr.Message,
			Regex:    yr.Regex,
			Exclude:  yr.Exclude,
			Enabled:  enabled,
			Source:   source,
		})
	}
	return result, nil
}

// determineSource returns the source layer for a rule.
func determineSource(id, pattern string,
	globalIDs, projectIDs map[string]bool,
	globalPatterns, projectPatterns map[string]bool,
) string {
	// Project-level ID override takes precedence.
	if projectIDs[id] {
		return "project"
	}
	// Project-level pattern override.
	if projectPatterns[pattern] {
		return "project"
	}
	// Global-level ID override.
	if globalIDs[id] {
		return "global"
	}
	// Global-level pattern override.
	if globalPatterns[pattern] {
		return "global"
	}
	return "builtin"
}

// indexByID returns a set of rule IDs present in a yamlRule slice (non-pattern-level only).
func indexByID(rules []yamlRule) map[string]bool {
	m := make(map[string]bool)
	for _, r := range rules {
		if !isPatternLevel(r) && r.ID != "" {
			m[r.ID] = true
		}
	}
	return m
}

// patternLevelSet returns the set of pattern names targeted by pattern-level directives.
func patternLevelSet(rules []yamlRule) map[string]bool {
	m := make(map[string]bool)
	for _, r := range rules {
		if isPatternLevel(r) {
			m[r.Pattern] = true
		}
	}
	return m
}

// PatternSummary groups rules by pattern and returns summary stats.
// Sorted by severity (most severe first), then by pattern name.
func PatternSummary(rules []CompiledRule) []PatternGroup {
	groups := make(map[string]*PatternGroup)
	for _, r := range rules {
		g, ok := groups[r.Pattern]
		if !ok {
			g = &PatternGroup{Pattern: r.Pattern, MaxSeverity: r.Severity}
			groups[r.Pattern] = g
		}
		g.Total++
		if r.Enabled {
			g.Enabled++
		} else {
			g.Disabled++
		}
		if SeverityRank(r.Severity) < SeverityRank(g.MaxSeverity) {
			g.MaxSeverity = r.Severity
		}
	}

	result := make([]PatternGroup, 0, len(groups))
	for _, g := range groups {
		result = append(result, *g)
	}
	sort.Slice(result, func(i, j int) bool {
		ri := SeverityRank(result[i].MaxSeverity)
		rj := SeverityRank(result[j].MaxSeverity)
		if ri != rj {
			return ri < rj
		}
		return result[i].Pattern < result[j].Pattern
	})
	return result
}

// UniquePatterns returns a sorted list of distinct pattern names from builtin rules.
func UniquePatterns() []string {
	seen := make(map[string]bool)
	for _, yr := range builtinYAML() {
		if yr.Pattern != "" {
			seen[yr.Pattern] = true
		}
	}
	result := make([]string, 0, len(seen))
	for p := range seen {
		result = append(result, p)
	}
	sort.Strings(result)
	return result
}
