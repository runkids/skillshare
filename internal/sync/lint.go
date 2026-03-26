package sync

import (
	_ "embed"
	"regexp"
	"strconv"
	"strings"
	gosync "sync"
	"unicode/utf8"

	"gopkg.in/yaml.v3"
)

//go:embed lint_rules.yaml
var lintRulesData []byte

// LintSeverity represents the severity level of a lint issue.
type LintSeverity string

const (
	LintError   LintSeverity = "error"
	LintWarning LintSeverity = "warning"
)

// Lint check types (match YAML check field values).
const (
	checkFieldEmpty      = "field-empty"
	checkBodyEmpty       = "body-empty"
	checkFieldLengthMin  = "field-length-min"
	checkFieldLengthMax  = "field-length-max"
	checkFieldLengthRange = "field-length-range"
	checkFieldNotMatches = "field-not-matches"
	checkFieldMatches    = "field-matches"
)

// LintIssue represents a single lint finding for a skill.
type LintIssue struct {
	Rule     string       `json:"rule"`
	Severity LintSeverity `json:"severity"`
	Category string       `json:"category"`
	Message  string       `json:"message"`
}

// lintRule is the YAML deserialization type for a single lint rule.
type lintRule struct {
	ID        string `yaml:"id"`
	Severity  string `yaml:"severity"`
	Category  string `yaml:"category"`
	Check     string `yaml:"check"`
	Field     string `yaml:"field,omitempty"`
	Threshold int    `yaml:"threshold,omitempty"`
	Min       int    `yaml:"min,omitempty"`
	Max       int    `yaml:"max,omitempty"`
	Pattern   string `yaml:"pattern,omitempty"`
	Message   string `yaml:"message"`
}

type lintRulesFile struct {
	Rules []lintRule `yaml:"rules"`
}

// compiledLintRule is a lint rule with its regex pre-compiled.
type compiledLintRule struct {
	lintRule
	compiledPattern *regexp.Regexp
}

var (
	compiledLintRules []compiledLintRule
	lintOnce          gosync.Once
)

func loadLintRules() []compiledLintRule {
	lintOnce.Do(func() {
		var f lintRulesFile
		if err := yaml.Unmarshal(lintRulesData, &f); err != nil {
			panic("lint: embedded rules YAML is invalid: " + err.Error())
		}
		for _, r := range f.Rules {
			cr := compiledLintRule{lintRule: r}
			if r.Pattern != "" {
				re, err := regexp.Compile(r.Pattern)
				if err != nil {
					panic("lint: invalid regex in rule " + r.ID + ": " + err.Error())
				}
				cr.compiledPattern = re
			}
			compiledLintRules = append(compiledLintRules, cr)
		}
	})
	return compiledLintRules
}

// LintSkill runs all lint rules against a skill's metadata.
// name and description come from SKILL.md frontmatter.
// bodyChars is the rune count of the skill body after frontmatter.
func LintSkill(name, description string, bodyChars int) []LintIssue {
	rules := loadLintRules()

	var issues []LintIssue
	for _, r := range rules {
		if issue, ok := evalLintRule(r, name, description, bodyChars); ok {
			issues = append(issues, issue)
		}
	}
	return issues
}

func evalLintRule(r compiledLintRule, name, description string, bodyChars int) (LintIssue, bool) {
	fieldValue := resolveField(r.Field, name, description)
	charCount := utf8.RuneCountInString(fieldValue)

	var triggered bool
	switch r.Check {
	case checkFieldEmpty:
		triggered = strings.TrimSpace(fieldValue) == ""
	case checkBodyEmpty:
		triggered = bodyChars == 0
	case checkFieldLengthMin:
		triggered = charCount > 0 && charCount < r.Threshold
	case checkFieldLengthMax:
		triggered = charCount > r.Threshold
	case checkFieldLengthRange:
		triggered = charCount >= r.Min && charCount <= r.Max
	case checkFieldNotMatches:
		if r.compiledPattern != nil && strings.TrimSpace(fieldValue) != "" {
			triggered = !r.compiledPattern.MatchString(fieldValue)
		}
	case checkFieldMatches:
		if r.compiledPattern != nil {
			triggered = r.compiledPattern.MatchString(fieldValue)
		}
	}

	if !triggered {
		return LintIssue{}, false
	}

	msg := r.Message
	msg = strings.ReplaceAll(msg, "{chars}", strconv.Itoa(charCount))

	return LintIssue{
		Rule:     r.ID,
		Severity: LintSeverity(r.Severity),
		Category: r.Category,
		Message:  msg,
	}, true
}

func resolveField(field, name, description string) string {
	switch field {
	case "name":
		return name
	case "description":
		return description
	default:
		return ""
	}
}
