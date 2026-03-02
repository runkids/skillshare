package audit

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"
)

// Severity levels for audit findings.
const (
	SeverityCritical = "CRITICAL"
	SeverityHigh     = "HIGH"
	SeverityMedium   = "MEDIUM"
	SeverityLow      = "LOW"
	SeverityInfo     = "INFO"
)

// validSeverities is the set of accepted severity values.
var validSeverities = map[string]bool{
	SeverityCritical: true,
	SeverityHigh:     true,
	SeverityMedium:   true,
	SeverityLow:      true,
	SeverityInfo:     true,
}

var severityRank = map[string]int{
	SeverityCritical: 0,
	SeverityHigh:     1,
	SeverityMedium:   2,
	SeverityLow:      3,
	SeverityInfo:     4,
}

// DefaultThreshold returns the default block threshold.
func DefaultThreshold() string {
	return SeverityCritical
}

// severityAliases maps shorthand forms to canonical severity names.
var severityAliases = map[string]string{
	"CRIT": SeverityCritical,
	"C":    SeverityCritical,
	"H":    SeverityHigh,
	"MED":  SeverityMedium,
	"M":    SeverityMedium,
	"L":    SeverityLow,
	"I":    SeverityInfo,
}

// NormalizeSeverity normalizes and validates a severity-like value.
// Accepts full names (CRITICAL, HIGH, …) and shorthands (CRIT, C, H, MED, M, L, I).
func NormalizeSeverity(v string) (string, error) {
	sev := strings.ToUpper(strings.TrimSpace(v))
	if sev == "" {
		return "", fmt.Errorf("empty severity")
	}
	if validSeverities[sev] {
		return sev, nil
	}
	if canonical, ok := severityAliases[sev]; ok {
		return canonical, nil
	}
	return "", fmt.Errorf("invalid severity %q", v)
}

// NormalizeThreshold normalizes block threshold, defaulting to CRITICAL when empty.
func NormalizeThreshold(v string) (string, error) {
	if strings.TrimSpace(v) == "" {
		return DefaultThreshold(), nil
	}
	return NormalizeSeverity(v)
}

// SeverityRank returns the sort/block rank for a severity.
// Lower rank means higher severity.
func SeverityRank(sev string) int {
	if rank, ok := severityRank[sev]; ok {
		return rank
	}
	return 999
}

// rule defines a single compiled scanning pattern.
type rule struct {
	ID       string
	Severity string
	Pattern  string // rule name
	Message  string
	Regex    *regexp.Regexp
	Exclude  *regexp.Regexp // if non-nil, suppress match when this also matches
}

// yamlRule is the YAML deserialization type for a single rule.
type yamlRule struct {
	ID       string `yaml:"id"`
	Severity string `yaml:"severity"`
	Pattern  string `yaml:"pattern"`
	Message  string `yaml:"message"`
	Regex    string `yaml:"regex"`
	Exclude  string `yaml:"exclude,omitempty"`
	Enabled  *bool  `yaml:"enabled,omitempty"` // nil = true; false = disable
}

type rulesFile struct {
	Rules []yamlRule `yaml:"rules"`
}

//go:embed rules.yaml
var defaultRulesData []byte

var (
	builtinRules    []rule
	builtinRulesErr error
	builtinOnce     sync.Once

	builtinYAMLCache []yamlRule
	builtinYAMLOnce  sync.Once

	globalRules    []rule
	globalRulesErr error
	globalOnce     sync.Once
)

// loadBuiltinRules parses and compiles the embedded rules.yaml + table-driven credential rules.
func loadBuiltinRules() ([]rule, error) {
	builtinOnce.Do(func() {
		yr, err := parseRulesYAML(defaultRulesData)
		if err != nil {
			builtinRulesErr = fmt.Errorf("builtin rules: %w", err)
			return
		}
		yr = append(yr, credentialYAMLRules()...)
		builtinRules, builtinRulesErr = compileRules(yr)
	})
	return builtinRules, builtinRulesErr
}

// Rules returns the compiled rules for global mode (builtin + global user overrides).
func Rules() ([]rule, error) {
	globalOnce.Do(func() {
		builtin, err := loadBuiltinRules()
		if err != nil {
			globalRulesErr = err
			return
		}

		userYAML, err := loadUserRules(globalAuditRulesPath())
		if err != nil {
			globalRulesErr = fmt.Errorf("global user rules: %w", err)
			return
		}

		if userYAML == nil {
			globalRules = builtin
			return
		}

		merged := mergeYAMLRules(builtinYAML(), userYAML)
		globalRules, globalRulesErr = compileRules(merged)
	})
	return globalRules, globalRulesErr
}

// RulesWithProject returns compiled rules for project mode
// (builtin + global user + project user overrides).
func RulesWithProject(projectRoot string) ([]rule, error) {
	// Start from global rules' YAML (builtin + global user merged)
	builtin, err := loadBuiltinRules()
	if err != nil {
		return nil, err
	}

	globalUserYAML, err := loadUserRules(globalAuditRulesPath())
	if err != nil {
		return nil, fmt.Errorf("global user rules: %w", err)
	}

	baseYAML := builtinYAML()
	if globalUserYAML != nil {
		baseYAML = mergeYAMLRules(baseYAML, globalUserYAML)
	}

	projectPath := filepath.Join(projectRoot, ".skillshare", "audit-rules.yaml")
	projectYAML, err := loadUserRules(projectPath)
	if err != nil {
		return nil, fmt.Errorf("project user rules: %w", err)
	}

	if projectYAML == nil && globalUserYAML == nil {
		return builtin, nil
	}
	if projectYAML == nil {
		// Only global overrides, no project overrides
		rules, err := Rules()
		if err != nil {
			return nil, err
		}
		return rules, nil
	}

	merged := mergeYAMLRules(baseYAML, projectYAML)
	return compileRules(merged)
}

// builtinYAML returns the parsed (not compiled) builtin rules for merging.
// Includes both rules.yaml entries and table-driven credential rules.
// Result is cached; returns a copy to prevent mutation of the cache.
func builtinYAML() []yamlRule {
	builtinYAMLOnce.Do(func() {
		var f rulesFile
		// Already validated in loadBuiltinRules, safe to ignore error
		yaml.Unmarshal(defaultRulesData, &f) //nolint:errcheck
		builtinYAMLCache = append(f.Rules, credentialYAMLRules()...)
	})
	result := make([]yamlRule, len(builtinYAMLCache))
	copy(result, builtinYAMLCache)
	return result
}

// parseRulesYAML parses YAML bytes into yamlRule slice.
func parseRulesYAML(data []byte) ([]yamlRule, error) {
	var f rulesFile
	if err := yaml.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("parse YAML: %w", err)
	}
	return f.Rules, nil
}

// compileRules validates and compiles yamlRule slice into rule slice.
func compileRules(yr []yamlRule) ([]rule, error) {
	var rules []rule
	for _, y := range yr {
		if y.Enabled != nil && !*y.Enabled {
			continue // disabled rule
		}
		sev, err := NormalizeSeverity(y.Severity)
		if err != nil {
			return nil, fmt.Errorf("rule %q: %w", y.ID, err)
		}
		if y.Regex == "" {
			return nil, fmt.Errorf("rule %q: empty regex", y.ID)
		}
		re, err := regexp.Compile(y.Regex)
		if err != nil {
			return nil, fmt.Errorf("rule %q: invalid regex: %w", y.ID, err)
		}

		r := rule{
			ID:       y.ID,
			Severity: sev,
			Pattern:  y.Pattern,
			Message:  y.Message,
			Regex:    re,
		}
		if y.Exclude != "" {
			excl, err := regexp.Compile(y.Exclude)
			if err != nil {
				return nil, fmt.Errorf("rule %q: invalid exclude regex: %w", y.ID, err)
			}
			r.Exclude = excl
		}
		rules = append(rules, r)
	}
	return rules, nil
}

// loadUserRules reads an optional user audit-rules.yaml file.
// Returns nil, nil if the file does not exist.
func loadUserRules(path string) ([]yamlRule, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	return parseRulesYAML(data)
}

// isPatternLevel returns true when an overlay entry targets all rules
// sharing a pattern name rather than a single rule by ID.
func isPatternLevel(r yamlRule) bool {
	return r.ID == "" && r.Pattern != "" && r.Regex == ""
}

// mergeYAMLRules merges overlay rules into base rules.
//
// Overlay entries come in two flavours:
//  1. Pattern-level (no id, has pattern, no regex): applied to every base
//     rule whose Pattern field matches.
//  2. ID-level (has id): matched to a single base rule, or appended as new.
//
// Processing order:
//   - Phase 1: pattern-level overlays (bulk disable / severity override)
//   - Phase 2: id-level overlays (can override pattern-level, e.g. re-enable
//     an individual rule after a pattern-level disable)
func mergeYAMLRules(base, overlay []yamlRule) []yamlRule {
	// Separate overlay into pattern-level and id-level entries.
	var patternOverlays, idOverlays []yamlRule
	for _, o := range overlay {
		if isPatternLevel(o) {
			patternOverlays = append(patternOverlays, o)
		} else {
			idOverlays = append(idOverlays, o)
		}
	}

	// Index base rules by ID for fast lookup.
	idx := make(map[string]int, len(base))
	result := make([]yamlRule, len(base))
	copy(result, base)
	for i, r := range result {
		idx[r.ID] = i
	}

	// Phase 1: apply pattern-level overlays to every matching base rule.
	for _, po := range patternOverlays {
		for i := range result {
			if result[i].Pattern != po.Pattern {
				continue
			}
			if po.Enabled != nil && !*po.Enabled {
				result[i].Enabled = po.Enabled
			}
			if po.Severity != "" {
				result[i].Severity = po.Severity
			}
		}
	}

	// Phase 2: apply id-level overlays (same logic as before).
	for _, o := range idOverlays {
		if pos, exists := idx[o.ID]; exists {
			if o.Enabled != nil && !*o.Enabled {
				result[pos].Enabled = o.Enabled
			} else if o.Enabled != nil && *o.Enabled {
				// Explicit enabled:true clears a disabled flag (e.g. from pattern-level).
				result[pos].Enabled = nil
			} else {
				result[pos] = o
			}
		} else {
			// New rule — append.
			result = append(result, o)
		}
	}

	return result
}

// globalAuditRulesPath returns the path to the global user audit-rules.yaml,
// next to config.yaml, respecting SKILLSHARE_CONFIG.
func globalAuditRulesPath() string {
	return GlobalAuditRulesPath()
}

// GlobalAuditRulesPath returns the path to the global user audit-rules.yaml.
func GlobalAuditRulesPath() string {
	return filepath.Join(configDir(), "audit-rules.yaml")
}

// ProjectAuditRulesPath returns the path to a project's audit-rules.yaml.
func ProjectAuditRulesPath(projectRoot string) string {
	return filepath.Join(projectRoot, ".skillshare", "audit-rules.yaml")
}

// configDir returns the skillshare config directory without importing
// internal/config (which would create an import cycle).
func configDir() string {
	if envPath := os.Getenv("SKILLSHARE_CONFIG"); envPath != "" {
		return filepath.Dir(envPath)
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "skillshare")
}

// DefaultRulesTemplate returns the scaffold YAML content for a new audit-rules.yaml.
func DefaultRulesTemplate() string {
	return `# Custom audit rules for skillshare.
# Rules are merged on top of built-in rules in order:
#   built-in → global (~/.config/skillshare/audit-rules.yaml) → project (.skillshare/audit-rules.yaml)
#
# Each rule needs: id, severity (CRITICAL/HIGH/MEDIUM/LOW/INFO), pattern, message, regex.
# Optional: exclude (suppress match when line also matches), enabled (false to disable).

rules:
  # Example: flag TODO comments as informational
  # - id: flag-todo
  #   severity: MEDIUM
  #   pattern: todo-comment
  #   message: "TODO comment found"
  #   regex: '(?i)\bTODO\b'

  # Example: disable a built-in rule by id
  # - id: insecure-http-0
  #   enabled: false

  # Example: disable the dangling-link structural check
  # - id: dangling-link
  #   enabled: false

  # Example: override a built-in rule (match by id, change severity)
  # - id: destructive-commands-2
  #   severity: MEDIUM
  #   pattern: destructive-commands
  #   message: "Sudo usage (downgraded)"
  #   regex: '(?i)\bsudo\s+'
`
}

// InitRulesFile creates a starter audit-rules.yaml at the given path.
// Returns an error if the file already exists.
func InitRulesFile(path string) error {
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("file already exists: %s", path)
	}

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("create directory: %w", err)
	}

	if err := os.WriteFile(path, []byte(DefaultRulesTemplate()), 0644); err != nil {
		return fmt.Errorf("write file: %w", err)
	}
	return nil
}

// ValidateRulesYAML parses raw YAML and compiles all regex patterns.
// Returns the first error encountered, or nil if valid.
func ValidateRulesYAML(raw string) error {
	yr, err := parseRulesYAML([]byte(raw))
	if err != nil {
		return err
	}
	_, err = compileRules(yr)
	return err
}

// extractDisabledIDs returns the set of rule IDs explicitly disabled
// (enabled: false) in a merged yamlRule slice.
func extractDisabledIDs(yr []yamlRule) map[string]bool {
	m := make(map[string]bool)
	for _, r := range yr {
		if r.Enabled != nil && !*r.Enabled {
			m[r.ID] = true
		}
	}
	return m
}

// disabledIDsGlobal returns IDs disabled in global mode (builtin + global user overrides).
func disabledIDsGlobal() map[string]bool {
	base := builtinYAML()
	user, err := loadUserRules(globalAuditRulesPath())
	if err != nil || user == nil {
		return extractDisabledIDs(base)
	}
	return extractDisabledIDs(mergeYAMLRules(base, user))
}

// disabledIDsForProject returns IDs disabled in project mode
// (builtin + global user + project user overrides).
func disabledIDsForProject(projectRoot string) map[string]bool {
	base := builtinYAML()
	globalUser, _ := loadUserRules(globalAuditRulesPath())
	if globalUser != nil {
		base = mergeYAMLRules(base, globalUser)
	}
	projectUser, _ := loadUserRules(filepath.Join(projectRoot, ".skillshare", "audit-rules.yaml"))
	if projectUser != nil {
		base = mergeYAMLRules(base, projectUser)
	}
	return extractDisabledIDs(base)
}

// ResetGlobalCache resets cached global/builtin rules so the next call to
// Rules() or ListRules() re-reads from disk. Useful after ToggleRule/TogglePattern
// mutations that change audit-rules.yaml.
func ResetGlobalCache() {
	globalOnce = sync.Once{}
	globalRules = nil
	globalRulesErr = nil
}

// resetForTest resets cached state for testing.
func resetForTest() {
	builtinOnce = sync.Once{}
	builtinRules = nil
	builtinRulesErr = nil
	builtinYAMLOnce = sync.Once{}
	builtinYAMLCache = nil
	globalOnce = sync.Once{}
	globalRules = nil
	globalRulesErr = nil
}
