package audit

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// rulesFileHeader is prepended when writing audit-rules.yaml.
const rulesFileHeader = "# Custom audit rules for skillshare.\n# See: https://skillshare.runkids.cc/docs/reference/commands/audit#custom-rules\n"

// ToggleRule enables or disables a single rule by ID in the given audit-rules.yaml.
// If enabled=false, adds an `enabled: false` entry.
// If enabled=true, removes any existing disable entry for that ID.
// Creates the file (with parent dirs) if it doesn't exist.
func ToggleRule(path, id string, enabled bool) error {
	return ToggleRules(path, []string{id}, enabled)
}

// ToggleRules enables or disables multiple rules by ID in a single read/write cycle.
func ToggleRules(path string, ids []string, enabled bool) error {
	rules, err := loadUserRules(path)
	if err != nil {
		return err
	}

	for _, id := range ids {
		if enabled {
			rules = removeEntryByID(rules, id)
		} else {
			rules = upsertDisableByID(rules, id)
		}
	}

	return writeRulesFile(path, rules)
}

// TogglePattern enables or disables all rules matching a pattern.
// If enabled=false, adds a pattern-level `enabled: false` entry.
// If enabled=true, removes any pattern-level disable entry for that pattern.
// Creates the file (with parent dirs) if it doesn't exist.
func TogglePattern(path, pattern string, enabled bool) error {
	rules, err := loadUserRules(path)
	if err != nil {
		return err
	}

	if enabled {
		rules = removePatternEntry(rules, pattern)
		rules = clearDisableForPattern(rules, pattern)
	} else {
		rules = upsertPatternDisable(rules, pattern)
	}

	return writeRulesFile(path, rules)
}

// writeRulesFile writes rules back to YAML file with a header comment.
// Creates parent directories if they don't exist.
func writeRulesFile(path string, rules []yamlRule) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("create directory: %w", err)
	}

	f := rulesFile{Rules: rules}
	data, err := yaml.Marshal(&f)
	if err != nil {
		return fmt.Errorf("marshal YAML: %w", err)
	}

	content := rulesFileHeader + string(data)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}

// removeEntryByID removes all entries with the given ID.
func removeEntryByID(rules []yamlRule, id string) []yamlRule {
	result := make([]yamlRule, 0, len(rules))
	for _, r := range rules {
		if r.ID != id {
			result = append(result, r)
		}
	}
	return result
}

// upsertDisableByID adds an enabled:false entry, or updates existing.
func upsertDisableByID(rules []yamlRule, id string) []yamlRule {
	disabled := false
	for i, r := range rules {
		if r.ID == id {
			rules[i].Enabled = &disabled
			return rules
		}
	}
	return append(rules, yamlRule{ID: id, Enabled: &disabled})
}

// removePatternEntry removes pattern-level entries matching the pattern.
func removePatternEntry(rules []yamlRule, pattern string) []yamlRule {
	result := make([]yamlRule, 0, len(rules))
	for _, r := range rules {
		if isPatternLevel(r) && r.Pattern == pattern {
			continue
		}
		result = append(result, r)
	}
	return result
}

// upsertPatternDisable adds or updates a pattern-level disable entry.
func upsertPatternDisable(rules []yamlRule, pattern string) []yamlRule {
	disabled := false
	for i, r := range rules {
		if isPatternLevel(r) && r.Pattern == pattern {
			rules[i].Enabled = &disabled
			return rules
		}
	}
	return append(rules, yamlRule{Pattern: pattern, Enabled: &disabled})
}

// SetSeverity overrides the severity of a single rule by ID.
// Writes an entry with just id + severity (no enabled field) to the audit-rules.yaml.
func SetSeverity(path, id, severity string) error {
	return SetSeverityBatch(path, []string{id}, severity)
}

// SetSeverityBatch overrides the severity of multiple rules by ID in a single read/write cycle.
func SetSeverityBatch(path string, ids []string, severity string) error {
	sev, err := NormalizeSeverity(severity)
	if err != nil {
		return fmt.Errorf("invalid severity %q (use CRITICAL, HIGH, MEDIUM, LOW, INFO)", severity)
	}

	rules, err := loadUserRules(path)
	if err != nil {
		return err
	}

	for _, id := range ids {
		rules = upsertSeverityByID(rules, id, sev)
	}
	return writeRulesFile(path, rules)
}

// SetPatternSeverity overrides the severity for all rules matching a pattern.
func SetPatternSeverity(path, pattern, severity string) error {
	sev, err := NormalizeSeverity(severity)
	if err != nil {
		return fmt.Errorf("invalid severity %q (use CRITICAL, HIGH, MEDIUM, LOW, INFO)", severity)
	}

	rules, err := loadUserRules(path)
	if err != nil {
		return err
	}

	rules = upsertPatternSeverity(rules, pattern, sev)
	return writeRulesFile(path, rules)
}

// ResetRules deletes the audit-rules.yaml file, restoring all rules to built-in defaults.
// Returns nil if the file doesn't exist.
func ResetRules(path string) error {
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove %s: %w", path, err)
	}
	return nil
}

// upsertSeverityByID adds or updates a severity override entry for a rule ID.
func upsertSeverityByID(rules []yamlRule, id, severity string) []yamlRule {
	for i, r := range rules {
		if r.ID == id {
			rules[i].Severity = severity
			return rules
		}
	}
	return append(rules, yamlRule{ID: id, Severity: severity})
}

// upsertPatternSeverity adds or updates a pattern-level severity entry.
func upsertPatternSeverity(rules []yamlRule, pattern, severity string) []yamlRule {
	for i, r := range rules {
		if isPatternLevel(r) && r.Pattern == pattern {
			rules[i].Severity = severity
			return rules
		}
	}
	return append(rules, yamlRule{Pattern: pattern, Severity: severity})
}

// builtinIDsForPattern returns all builtin rule IDs belonging to a pattern.
func builtinIDsForPattern(pattern string) map[string]bool {
	ids := make(map[string]bool)
	for _, r := range builtinYAML() {
		if r.Pattern == pattern {
			ids[r.ID] = true
		}
	}
	return ids
}

// clearDisableForPattern clears enabled:false on ID-level entries
// belonging to the given pattern — both builtin and custom rules.
// Entries that only had enabled:false (no other overrides) are removed entirely.
func clearDisableForPattern(rules []yamlRule, pattern string) []yamlRule {
	ids := builtinIDsForPattern(pattern)
	result := make([]yamlRule, 0, len(rules))
	for _, r := range rules {
		belongsToPattern := ids[r.ID] || (r.Pattern == pattern && r.ID != "")
		if belongsToPattern && r.Enabled != nil && !*r.Enabled {
			r.Enabled = nil
			// Drop entry if it has no other overrides.
			if r.Severity == "" && r.Regex == "" {
				continue
			}
		}
		result = append(result, r)
	}
	return result
}
