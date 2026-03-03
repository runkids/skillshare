package audit

import (
	"crypto/sha256"
	"encoding/hex"
	"regexp"
	"strconv"
	"strings"

	"skillshare/internal/ui"
)

var whitespaceRun = regexp.MustCompile(`\s+`)

func normalizeSnippet(s string) string {
	s = ui.StripANSI(s)
	s = strings.TrimSpace(s)
	s = whitespaceRun.ReplaceAllString(s, " ")
	return s
}

// ComputeFingerprint returns a stable sha256 hex hash for a finding.
// Formula: sha256(lower(ruleId|pattern|analyzer|file|line|normalizedSnippet))
func ComputeFingerprint(f Finding) string {
	raw := strings.Join([]string{
		f.RuleID, f.Pattern, f.Analyzer,
		f.File, strconv.Itoa(f.Line),
		normalizeSnippet(f.Snippet),
	}, "|")
	h := sha256.Sum256([]byte(strings.ToLower(raw)))
	return hex.EncodeToString(h[:])
}

func dedupeKey(f Finding) string {
	if f.Fingerprint != "" {
		return f.Fingerprint
	}
	return f.Pattern + "|" + f.File + "|" + strconv.Itoa(f.Line) + "|" + normalizeSnippet(f.Snippet)
}

// StampFingerprints fills the Fingerprint field for every finding that lacks one.
func StampFingerprints(findings []Finding) {
	for i := range findings {
		if findings[i].Fingerprint == "" {
			findings[i].Fingerprint = ComputeFingerprint(findings[i])
		}
	}
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
