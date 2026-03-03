package audit

import (
	"regexp"
	"strconv"
	"strings"
)

var ansiPattern = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)

var whitespaceRun = regexp.MustCompile(`\s+`)

func normalizeSnippet(s string) string {
	s = ansiPattern.ReplaceAllString(s, "")
	s = strings.TrimSpace(s)
	s = whitespaceRun.ReplaceAllString(s, " ")
	return s
}

func dedupeKey(f Finding) string {
	return f.Pattern + "|" + f.File + "|" + strconv.Itoa(f.Line) + "|" + normalizeSnippet(f.Snippet)
}

// DeduplicateGlobal removes duplicate findings based on a composite key
// (pattern|file|line|normalizedSnippet). When duplicates exist, the finding
// with the highest severity is kept; ties keep the first encountered.
func DeduplicateGlobal(findings []Finding) []Finding {
	if len(findings) == 0 {
		return nil
	}

	type entry struct {
		index int
		f     Finding
	}
	seen := make(map[string]*entry)
	var order []string

	for _, f := range findings {
		key := dedupeKey(f)
		if existing, ok := seen[key]; ok {
			if SeverityRank(f.Severity) < SeverityRank(existing.f.Severity) {
				existing.f = f
			}
		} else {
			seen[key] = &entry{index: len(order), f: f}
			order = append(order, key)
		}
	}

	result := make([]Finding, len(order))
	for i, key := range order {
		result[i] = seen[key].f
	}
	return result
}
