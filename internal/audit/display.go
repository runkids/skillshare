package audit

// DisplayFindings selects findings at or above a normalized display floor.
// An empty floor preserves all findings. Callers must retain the original
// results for scoring, blocking, machine output and completeness reporting.
func DisplayFindings(findings []Finding, minSeverity string) []Finding {
	if minSeverity == "" {
		return findings
	}
	visible := make([]Finding, 0, len(findings))
	for _, f := range findings {
		if SeverityRank(f.Severity) <= SeverityRank(minSeverity) {
			visible = append(visible, f)
		}
	}
	return visible
}

// HiddenByFloor reports whether a display floor hides r entirely: it has
// findings, all below the floor, and is not blocked. Clean and blocked
// results stay visible.
func HiddenByFloor(r *Result, minSeverity string) bool {
	return minSeverity != "" && !r.IsBlocked && len(r.Findings) > 0 && len(DisplayFindings(r.Findings, minSeverity)) == 0
}

// DisplayCounts reports finding counts before and after applying a display floor.
func DisplayCounts(results []*Result, minSeverity string) (visible, hidden int) {
	for _, r := range results {
		n := len(DisplayFindings(r.Findings, minSeverity))
		visible += n
		hidden += len(r.Findings) - n
	}
	return visible, hidden
}
