package audit

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

// AcceptKey returns a stable key for acknowledging a finding across updates.
// Unlike Fingerprint it omits the line number, so an accepted finding survives
// edits elsewhere in the file. A different rule, file, or matched text yields
// a new key and blocks again.
func AcceptKey(f Finding) string {
	parts := []string{
		f.RuleID, f.Pattern, f.Analyzer, f.File, normalizeSnippet(f.Snippet),
	}
	if f.acceptanceContext != "" {
		parts = append(parts, f.acceptanceContext)
	}
	raw := strings.Join(parts, "|")
	h := sha256.Sum256([]byte(strings.ToLower(raw)))
	return hex.EncodeToString(h[:])
}

// MarkAcknowledged flags findings whose AcceptKey is in keys. Acknowledged
// findings stay in the result for display but no longer count toward blocking.
// Returns the number of findings marked.
func (r *Result) MarkAcknowledged(keys []string) int {
	if r == nil || len(keys) == 0 {
		return 0
	}
	set := make(map[string]bool, len(keys))
	for _, k := range keys {
		set[k] = true
	}
	n := 0
	for i := range r.Findings {
		if set[AcceptKey(r.Findings[i])] {
			r.Findings[i].Acknowledged = true
			n++
		}
	}
	return n
}

// AcceptKeysAtOrAbove returns keys for unacknowledged findings at or above
// threshold — the set a --force override accepts.
func (r *Result) AcceptKeysAtOrAbove(threshold string) []string {
	normalized, err := NormalizeThreshold(threshold)
	if err != nil {
		normalized = DefaultThreshold()
	}
	cutoff := SeverityRank(normalized)
	var keys []string
	for _, f := range r.Findings {
		if !f.Acknowledged && SeverityRank(f.Severity) <= cutoff {
			keys = append(keys, AcceptKey(f))
		}
	}
	return keys
}
