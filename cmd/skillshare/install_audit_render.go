package main

import (
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"skillshare/internal/audit"
	"skillshare/internal/install"
	"skillshare/internal/ui"
)

var installAuditSeverityOrder = []string{
	audit.SeverityCritical,
	audit.SeverityHigh,
	audit.SeverityMedium,
	audit.SeverityLow,
	audit.SeverityInfo,
}

var installAuditThresholdPattern = regexp.MustCompile(`at/above\s+([A-Z]+)\s+detected`)

// skillInstallResult holds the result of installing a single skill
type skillInstallResult struct {
	skill          install.SkillInfo
	success        bool
	message        string
	warnings       []string
	auditRiskLabel string
	auditRiskScore int
	auditSkipped   bool
	result         *install.InstallResult
	err            error
}

type installWarningDigest struct {
	findingCounts   map[string]int
	findingByLevel  map[string][]string
	statusLines     []string
	otherAuditLines []string
	nonAuditLines   []string
}

type batchInstallWarningDigest struct {
	totalFindings            int
	findingCounts            map[string]int
	skillsWithFindings       int
	belowThresholdSkillCount int
	aboveThresholdSkillCount int
	scanErrorSkillCount      int
	skippedAuditSkillCount   int
	highCriticalBySkill      map[string]int
	nonAuditLines            []string
	otherAuditLines          []string
}

// auditFindingGroup groups findings with the same severity and message.
type auditFindingGroup struct {
	severity  string
	message   string
	locations []string // e.g., "SKILL.md:3"
}

type auditBlockedFailureDigest struct {
	threshold    string
	findingCount int
	firstFinding string
}

// normalizeInstallAuditThreshold normalizes install threshold values and
// supports shorthand level aliases for CLI ergonomics.
func normalizeInstallAuditThreshold(raw string) (string, error) {
	v := strings.ToLower(strings.TrimSpace(raw))
	switch v {
	case "c", "crit":
		v = audit.SeverityCritical
	case "h":
		v = audit.SeverityHigh
	case "m", "med":
		v = audit.SeverityMedium
	case "l":
		v = audit.SeverityLow
	case "i":
		v = audit.SeverityInfo
	}

	threshold, err := audit.NormalizeThreshold(v)
	if err != nil {
		return "", fmt.Errorf("invalid audit threshold %q (use: critical|high|medium|low|info or c|h|m|l|i)", raw)
	}
	return threshold, nil
}

func parseInstallAuditSeverity(warning string) (string, bool) {
	for _, severity := range installAuditSeverityOrder {
		if strings.HasPrefix(warning, "audit "+severity+":") {
			return severity, true
		}
	}
	return "", false
}

func firstWarningLine(warning string) string {
	trimmed := strings.TrimSpace(warning)
	if i := strings.IndexByte(trimmed, '\n'); i >= 0 {
		return strings.TrimSpace(trimmed[:i])
	}
	return trimmed
}

func digestInstallWarnings(warnings []string) installWarningDigest {
	digest := installWarningDigest{
		findingCounts:  make(map[string]int, len(installAuditSeverityOrder)),
		findingByLevel: make(map[string][]string, len(installAuditSeverityOrder)),
	}

	for _, warning := range warnings {
		if severity, ok := parseInstallAuditSeverity(warning); ok {
			line := firstWarningLine(warning)
			digest.findingCounts[severity]++
			digest.findingByLevel[severity] = append(digest.findingByLevel[severity], line)
			continue
		}

		switch {
		case strings.HasPrefix(warning, "audit findings "):
			digest.statusLines = append(digest.statusLines, firstWarningLine(warning))
		case strings.HasPrefix(warning, "audit scan error"), strings.HasPrefix(warning, "audit skipped"), strings.HasPrefix(warning, "audit "):
			digest.otherAuditLines = append(digest.otherAuditLines, firstWarningLine(warning))
		default:
			digest.nonAuditLines = append(digest.nonAuditLines, warning)
		}
	}

	return digest
}

func formatWarningWithSkill(skillName, warning string) string {
	if skillName == "" {
		return warning
	}
	return fmt.Sprintf("%s: %s", skillName, warning)
}

// installAuditFindingLinePattern parses "audit HIGH: Sudo escalation (SKILL.md:3)"
var installAuditFindingLinePattern = regexp.MustCompile(
	`^audit\s+([A-Z]+):\s+(.+?)\s+\(([^)]+)\)\s*$`,
)

// groupAuditFindings groups finding lines by (severity, message), collecting locations.
func groupAuditFindings(digest installWarningDigest) []auditFindingGroup {
	type groupKey struct {
		severity string
		message  string
	}
	indexMap := make(map[groupKey]int)
	var groups []auditFindingGroup

	for _, severity := range installAuditSeverityOrder {
		for _, line := range digest.findingByLevel[severity] {
			m := installAuditFindingLinePattern.FindStringSubmatch(line)
			if m == nil {
				// Fallback: ungroupable line as its own group
				groups = append(groups, auditFindingGroup{
					severity: severity, message: stripAuditPrefix(line), locations: nil,
				})
				continue
			}
			sev, msg, loc := m[1], m[2], m[3]
			key := groupKey{severity: strings.ToUpper(sev), message: msg}
			if idx, ok := indexMap[key]; ok {
				groups[idx].locations = append(groups[idx].locations, loc)
			} else {
				indexMap[key] = len(groups)
				groups = append(groups, auditFindingGroup{
					severity: strings.ToUpper(sev), message: msg, locations: []string{loc},
				})
			}
		}
	}
	return groups
}

// stripAuditPrefix removes the "audit " prefix from a finding line.
func stripAuditPrefix(line string) string {
	for _, severity := range installAuditSeverityOrder {
		prefix := "audit " + severity + ": "
		if strings.HasPrefix(line, prefix) {
			return severity + ": " + strings.TrimPrefix(line, prefix)
		}
	}
	// Also strip bare "audit " prefix from status lines
	if strings.HasPrefix(line, "audit ") {
		return strings.TrimPrefix(line, "audit ")
	}
	return line
}

// formatFindingGroup formats a grouped finding with locations on a separate indented line.
func formatFindingGroup(g auditFindingGroup) string {
	var sb strings.Builder
	sb.WriteString(ui.Colorize(ui.SeverityColor(g.severity), g.severity))
	sb.WriteString(": ")
	sb.WriteString(g.message)
	if len(g.locations) > 1 {
		sb.WriteString(fmt.Sprintf(" × %d", len(g.locations)))
	}
	return sb.String()
}

// formatFindingLocations formats the locations as an indented second line.
func formatFindingLocations(g auditFindingGroup) string {
	if len(g.locations) == 0 {
		return ""
	}
	const maxLocs = 3
	var sb strings.Builder
	for i, loc := range g.locations {
		if i >= maxLocs {
			sb.WriteString(fmt.Sprintf(", +%d more", len(g.locations)-maxLocs))
			break
		}
		if i > 0 {
			sb.WriteString(", ")
		}
		sb.WriteString(loc)
	}
	return sb.String()
}

// printFindingGroup prints a finding with severity-colored prefix and locations on a second line.
func printFindingGroup(skillName string, g auditFindingGroup) {
	color := ui.SeverityColor(g.severity)
	prefix := ui.Colorize(color, "!")
	fmt.Printf("%s %s\n", prefix, formatWarningWithSkill(skillName, formatFindingGroup(g)))
	if locs := formatFindingLocations(g); locs != "" {
		fmt.Printf("  %s\n", ui.Colorize(ui.Gray, locs))
	}
}

// renderInstallWarnings renders audit warnings for a single skill.
// When skillName is empty, it prints a SectionLabel header (single-skill mode).
// When skillName is set, each line is prefixed with the skill name (batch mode).
func renderInstallWarnings(skillName string, warnings []string, auditVerbose bool) {
	renderInstallWarningsWithResult(skillName, warnings, auditVerbose, nil)
}

// renderInstallWarningsWithResult is like renderInstallWarnings but also displays
// the aggregate risk score from the install result when available.
func renderInstallWarningsWithResult(skillName string, warnings []string, auditVerbose bool, result *install.InstallResult) {
	// Visual separator for single-skill output
	if skillName == "" {
		ui.SectionLabel("Audit Findings")
	}

	if len(warnings) == 0 {
		if skillName == "" {
			renderAuditRiskOnly(skillName, result)
		} else {
			// Batch mode: prefix with skill name
			ui.Info("%s: risk CLEAN", skillName)
		}
		return
	}

	digest := digestInstallWarnings(warnings)

	// Non-audit warnings first
	for _, warning := range digest.nonAuditLines {
		ui.Warning("%s", formatWarningWithSkill(skillName, warning))
	}

	// Compute totals
	totalFindings := 0
	for _, severity := range installAuditSeverityOrder {
		totalFindings += digest.findingCounts[severity]
	}

	// Summary-first: counts + status + risk score
	if totalFindings > 0 {
		summary := formatInstallSeverityCounts(digest.findingCounts)
		if len(digest.statusLines) > 0 {
			// Append threshold status (strip "audit " prefix)
			status := stripAuditPrefix(digest.statusLines[0])
			summary += " — " + status
		}
		ui.Info("%s", formatWarningWithSkill(skillName, fmt.Sprintf("%d finding(s): %s", totalFindings, summary)))
	} else {
		for _, line := range digest.statusLines {
			ui.Info("%s", formatWarningWithSkill(skillName, stripAuditPrefix(line)))
		}
	}
	renderAuditRiskOnly(skillName, result)
	for _, line := range digest.otherAuditLines {
		ui.Warning("%s", formatWarningWithSkill(skillName, stripAuditPrefix(line)))
	}

	if totalFindings == 0 {
		return
	}

	// Group findings by message
	groups := groupAuditFindings(digest)

	// Visual gap before finding details (single-skill only)
	if skillName == "" {
		fmt.Println()
	}

	if auditVerbose {
		// Verbose: show all groups with all locations
		for _, g := range groups {
			printFindingGroup(skillName, g)
		}
		return
	}

	// Compact: show top groups (by severity order, already sorted)
	const maxGroups = 5
	shown := 0
	for _, g := range groups {
		if shown >= maxGroups {
			break
		}
		printFindingGroup(skillName, g)
		shown++
	}

	if remaining := len(groups) - shown; remaining > 0 {
		ui.Info("%s", formatWarningWithSkill(skillName,
			fmt.Sprintf("+%d more finding type(s); use --audit-verbose for full details", remaining)))
	}
}

// renderAuditRiskOnly prints the aggregate risk score if available.
func renderAuditRiskOnly(skillName string, result *install.InstallResult) {
	if result == nil || result.AuditSkipped {
		return
	}
	label := strings.ToUpper(result.AuditRiskLabel)
	if label == "" {
		label = "CLEAN"
	}
	coloredLabel := formatSeverity(label)
	if result.AuditRiskScore > 0 {
		ui.Info("%s", formatWarningWithSkill(skillName,
			fmt.Sprintf("risk: %s (%d/100)", coloredLabel, result.AuditRiskScore)))
	} else {
		ui.Info("%s", formatWarningWithSkill(skillName,
			fmt.Sprintf("risk: %s", coloredLabel)))
	}
}

// renderBlockedAuditError displays a structured audit-blocked error and returns
// a short error for main.go to print as a one-line summary.
func renderBlockedAuditError(err error) error {
	msg := err.Error()

	// Extract finding detail lines (lines starting with a severity level)
	var detailLines []string
	for _, rawLine := range strings.Split(msg, "\n") {
		line := strings.TrimSpace(rawLine)
		if isAuditSeverityLine(line) {
			detailLines = append(detailLines, "  "+line)
		}
	}

	// Build summary from parsed digest
	digest := parseAuditBlockedFailure(msg)
	summaryParts := []string{"security audit"}
	contextSuffix := ""
	if strings.Contains(strings.ToLower(msg), "tracked repository") {
		contextSuffix = " in tracked repository"
	}
	if digest.threshold != "" && digest.findingCount > 0 {
		suffix := "findings"
		if digest.findingCount == 1 {
			suffix = "finding"
		}
		summaryParts = append(summaryParts, fmt.Sprintf("blocked — %d %s at/above %s%s", digest.findingCount, suffix, digest.threshold, contextSuffix))
	} else {
		summaryParts = append(summaryParts, "blocked"+contextSuffix)
	}

	ui.SectionLabel("Audit Findings")
	ui.StepFail(strings.Join(summaryParts, ": "), "")
	for _, line := range detailLines {
		fmt.Println(line)
	}

	// Check for cleanup failure or rollback info
	if strings.Contains(msg, "Automatic cleanup failed") || strings.Contains(msg, "Manual removal is required") {
		fmt.Println()
		ui.Warning("automatic cleanup failed — manual removal may be required")
	}
	if strings.Contains(msg, "rollback also failed") {
		fmt.Println()
		ui.Warning("rollback also failed — malicious content may remain")
	}

	return fmt.Errorf("blocked by security audit (use --force to override)")
}

func appendUniqueLimited(lines []string, line string, limit int) []string {
	for _, existing := range lines {
		if existing == line {
			return lines
		}
	}
	if len(lines) >= limit {
		return lines
	}
	return append(lines, line)
}

func summarizeBatchInstallWarnings(results []skillInstallResult) batchInstallWarningDigest {
	summary := batchInstallWarningDigest{
		findingCounts:       make(map[string]int, len(installAuditSeverityOrder)),
		highCriticalBySkill: make(map[string]int),
	}

	for _, result := range results {
		if len(result.warnings) == 0 {
			continue
		}

		digest := digestInstallWarnings(result.warnings)
		skillHasFindings := false

		for _, severity := range installAuditSeverityOrder {
			count := digest.findingCounts[severity]
			if count == 0 {
				continue
			}
			summary.totalFindings += count
			summary.findingCounts[severity] += count
			skillHasFindings = true
		}

		if skillHasFindings {
			summary.skillsWithFindings++
			highCritical := digest.findingCounts[audit.SeverityCritical] + digest.findingCounts[audit.SeverityHigh]
			if highCritical > 0 {
				summary.highCriticalBySkill[result.skill.Name] = highCritical
			}
		}

		for _, line := range digest.statusLines {
			switch {
			case strings.HasPrefix(line, "audit findings detected, but none at/above block threshold"):
				summary.belowThresholdSkillCount++
			case strings.HasPrefix(line, "audit findings at/above block threshold"):
				summary.aboveThresholdSkillCount++
			default:
				summary.otherAuditLines = appendUniqueLimited(
					summary.otherAuditLines,
					formatWarningWithSkill(result.skill.Name, line),
					5,
				)
			}
		}

		for _, line := range digest.otherAuditLines {
			switch {
			case strings.HasPrefix(line, "audit scan error"):
				summary.scanErrorSkillCount++
			case strings.HasPrefix(line, "audit skipped"):
				summary.skippedAuditSkillCount++
			default:
				summary.otherAuditLines = appendUniqueLimited(
					summary.otherAuditLines,
					formatWarningWithSkill(result.skill.Name, line),
					5,
				)
			}
		}

		for _, line := range digest.nonAuditLines {
			summary.nonAuditLines = append(summary.nonAuditLines, formatWarningWithSkill(result.skill.Name, line))
		}
	}

	return summary
}

func formatInstallSeverityCounts(counts map[string]int) string {
	parts := make([]string, 0, len(installAuditSeverityOrder))
	for _, severity := range installAuditSeverityOrder {
		if count := counts[severity]; count > 0 {
			label := ui.Colorize(ui.SeverityColor(severity), severity)
			parts = append(parts, fmt.Sprintf("%s=%d", label, count))
		}
	}
	return strings.Join(parts, ", ")
}

func topHighCriticalSkillsByCount(scoreBySkill map[string]int, limit int) []string {
	type skillScore struct {
		name  string
		score int
	}

	if limit <= 0 || len(scoreBySkill) == 0 {
		return nil
	}

	scores := make([]skillScore, 0, len(scoreBySkill))
	for name, score := range scoreBySkill {
		scores = append(scores, skillScore{name: name, score: score})
	}

	sort.Slice(scores, func(i, j int) bool {
		if scores[i].score == scores[j].score {
			return scores[i].name < scores[j].name
		}
		return scores[i].score > scores[j].score
	})

	if len(scores) > limit {
		scores = scores[:limit]
	}

	top := make([]string, 0, len(scores))
	for _, item := range scores {
		top = append(top, fmt.Sprintf("%s(%d)", item.name, item.score))
	}
	return top
}

func renderBatchInstallWarningsCompact(results []skillInstallResult, totalWarnings int, hints ...string) {
	ui.Warning("%d warning(s) detected during install (compact batch view)", totalWarnings)

	summary := summarizeBatchInstallWarnings(results)

	const maxNonAuditLines = 5
	for i, line := range summary.nonAuditLines {
		if i >= maxNonAuditLines {
			ui.Warning("+%d more non-audit warning(s)", len(summary.nonAuditLines)-maxNonAuditLines)
			break
		}
		ui.Warning("%s", line)
	}

	if summary.totalFindings > 0 {
		ui.Warning("audit findings across %d skill(s): %s",
			summary.skillsWithFindings,
			formatInstallSeverityCounts(summary.findingCounts),
		)
	}
	if summary.belowThresholdSkillCount > 0 {
		ui.Warning("%d skill(s) had findings below the active block threshold", summary.belowThresholdSkillCount)
	}
	if summary.aboveThresholdSkillCount > 0 {
		ui.Warning("%d skill(s) had findings at/above threshold and continued due to --force", summary.aboveThresholdSkillCount)
	}
	if summary.scanErrorSkillCount > 0 {
		ui.Warning("%d skill(s) had audit scan errors", summary.scanErrorSkillCount)
	}
	if summary.skippedAuditSkillCount > 0 {
		ui.Warning("%d skill(s) skipped audit (--skip-audit)", summary.skippedAuditSkillCount)
	}

	if skillsWithHighCritical := len(summary.highCriticalBySkill); skillsWithHighCritical > 0 {
		ui.Warning("skills with HIGH/CRITICAL findings: %d", skillsWithHighCritical)
		top := topHighCriticalSkillsByCount(summary.highCriticalBySkill, 5)
		if len(top) > 0 {
			extra := ""
			if skillsWithHighCritical > len(top) {
				extra = fmt.Sprintf(" +%d more", skillsWithHighCritical-len(top))
			}
			ui.Info("top HIGH/CRITICAL: %s%s", strings.Join(top, ", "), extra)
		}
	}

	for _, line := range summary.otherAuditLines {
		ui.Warning("%s", line)
	}

	if summary.totalFindings > 0 {
		hint := "suppressed %d audit finding line(s); re-run with --audit-verbose for full details"
		if len(hints) > 0 {
			hint = hints[0]
		}
		ui.Info(hint, summary.totalFindings)
	}
}

// renderUltraCompactAuditSummary prints a 3-4 line audit summary for very large
// batches (>100 results). Designed to keep terminal output manageable.
func renderUltraCompactAuditSummary(results []skillInstallResult, _ int) {
	summary := summarizeBatchInstallWarnings(results)

	// Line 1: total findings with severity breakdown
	if summary.totalFindings > 0 {
		ui.Warning("%d finding(s) across %d skill(s): %s",
			summary.totalFindings,
			summary.skillsWithFindings,
			formatInstallSeverityCounts(summary.findingCounts),
		)
	}

	// Line 2: status line (only if nonzero)
	var statusParts []string
	if summary.belowThresholdSkillCount > 0 {
		statusParts = append(statusParts, fmt.Sprintf("%d below threshold", summary.belowThresholdSkillCount))
	}
	if summary.aboveThresholdSkillCount > 0 {
		statusParts = append(statusParts, fmt.Sprintf("%d above threshold (--force)", summary.aboveThresholdSkillCount))
	}
	if summary.scanErrorSkillCount > 0 {
		statusParts = append(statusParts, fmt.Sprintf("%d scan errors", summary.scanErrorSkillCount))
	}
	if len(statusParts) > 0 {
		ui.Warning("%s", strings.Join(statusParts, ", "))
	}

	// Line 3: top HIGH/CRITICAL (max 3 skills)
	if len(summary.highCriticalBySkill) > 0 {
		top := topHighCriticalSkillsByCount(summary.highCriticalBySkill, 3)
		extra := ""
		if len(summary.highCriticalBySkill) > len(top) {
			extra = fmt.Sprintf(" +%d more", len(summary.highCriticalBySkill)-len(top))
		}
		ui.Info("top HIGH/CRITICAL: %s%s", strings.Join(top, ", "), extra)
	}

	// Line 4: suppressed hint
	if summary.totalFindings > 0 {
		ui.Info("suppressed %d audit finding line(s); re-run with --audit-verbose for full details", summary.totalFindings)
	}
}

func isAuditSeverityLine(line string) bool {
	for _, severity := range installAuditSeverityOrder {
		if strings.HasPrefix(line, severity+":") {
			return true
		}
	}
	return false
}

func parseAuditBlockedFailure(message string) auditBlockedFailureDigest {
	digest := auditBlockedFailureDigest{}
	if matches := installAuditThresholdPattern.FindStringSubmatch(message); len(matches) == 2 {
		digest.threshold = matches[1]
	}

	for _, rawLine := range strings.Split(message, "\n") {
		line := strings.TrimSpace(rawLine)
		if !isAuditSeverityLine(line) {
			continue
		}
		digest.findingCount++
		if digest.firstFinding == "" {
			digest.firstFinding = line
		}
	}

	return digest
}

func summarizeBlockedThreshold(failures []skillInstallResult) string {
	thresholds := map[string]bool{}
	for _, failure := range failures {
		digest := parseAuditBlockedFailure(failure.message)
		if digest.threshold != "" {
			thresholds[digest.threshold] = true
		}
	}

	if len(thresholds) == 0 {
		return "configured"
	}
	if len(thresholds) == 1 {
		for threshold := range thresholds {
			return threshold
		}
	}
	return "mixed"
}

func truncateForInstallSummary(text string, max int) string {
	if max <= 0 || len(text) <= max {
		return text
	}
	if max <= 3 {
		return text[:max]
	}
	return text[:max-3] + "..."
}

func blockedSkillLabel(name, threshold string) string {
	if !ui.IsTTY() {
		return name
	}
	color := riskColor(strings.ToLower(strings.TrimSpace(threshold)))
	if color == "" {
		color = ui.Red
	}
	return ui.Bold + color + name + ui.Reset
}

func formatBlockedThresholdLabel(threshold string) string {
	threshold = strings.TrimSpace(threshold)
	if threshold == "" {
		return "configured"
	}
	if threshold == "mixed" {
		return threshold
	}
	return formatSeverity(threshold)
}

func compactInstallFailureMessage(result skillInstallResult) string {
	if result.err != nil && errors.Is(result.err, audit.ErrBlocked) {
		digest := parseAuditBlockedFailure(result.message)

		parts := []string{"blocked by security audit"}
		if digest.threshold != "" && digest.findingCount > 0 {
			suffix := "findings"
			if digest.findingCount == 1 {
				suffix = "finding"
			}
			parts = append(parts, fmt.Sprintf("(%s, %d %s)", digest.threshold, digest.findingCount, suffix))
		} else if digest.threshold != "" {
			parts = append(parts, "("+digest.threshold+")")
		}

		if digest.firstFinding != "" {
			first := digest.firstFinding
			for _, severity := range installAuditSeverityOrder {
				prefix := severity + ": "
				if strings.HasPrefix(first, prefix) {
					first = strings.TrimPrefix(first, prefix)
					break
				}
			}
			parts = append(parts, truncateForInstallSummary(first, 110))
		}

		return strings.Join(parts, " ")
	}

	return firstWarningLine(result.message)
}

// renderInstallWarningsHighCriticalOnly prints only HIGH/CRITICAL findings
// verbosely, and summarizes remaining findings in one line.
func renderInstallWarningsHighCriticalOnly(skillName string, warnings []string) {
	digest := digestInstallWarnings(warnings)

	// Print non-audit warnings as-is
	for _, warning := range digest.nonAuditLines {
		ui.Warning("%s", formatWarningWithSkill(skillName, warning))
	}

	// Print HIGH and CRITICAL findings verbosely
	highCritCount := 0
	for _, severity := range []string{audit.SeverityCritical, audit.SeverityHigh} {
		for _, line := range digest.findingByLevel[severity] {
			ui.Warning("%s", formatWarningWithSkill(skillName, line))
			highCritCount++
		}
	}

	// Summarize remaining findings (MEDIUM/LOW/INFO) in one line
	otherParts := make([]string, 0, 3)
	for _, severity := range []string{audit.SeverityMedium, audit.SeverityLow, audit.SeverityInfo} {
		if count := digest.findingCounts[severity]; count > 0 {
			otherParts = append(otherParts, fmt.Sprintf("%s=%d", severity, count))
		}
	}
	if len(otherParts) > 0 {
		ui.Info("%s", formatWarningWithSkill(skillName,
			fmt.Sprintf("also: %s (use 'skillshare check %s' for details)", strings.Join(otherParts, ", "), skillName)))
	}

	// Print status/other audit lines
	for _, line := range digest.statusLines {
		ui.Warning("%s", formatWarningWithSkill(skillName, line))
	}
	for _, line := range digest.otherAuditLines {
		ui.Warning("%s", formatWarningWithSkill(skillName, line))
	}
}

// countSkillsWithWarnings counts how many results have at least one warning.
func countSkillsWithWarnings(results []skillInstallResult) int {
	n := 0
	for _, r := range results {
		if len(r.warnings) > 0 {
			n++
		}
	}
	return n
}

// hasHighCriticalWarnings checks if a result contains HIGH or CRITICAL audit findings.
func hasHighCriticalWarnings(r skillInstallResult) bool {
	for _, w := range r.warnings {
		if strings.HasPrefix(w, "audit CRITICAL:") || strings.HasPrefix(w, "audit HIGH:") {
			return true
		}
	}
	return false
}

// sortResultsByHighCritical returns a copy sorted by HIGH/CRITICAL finding count descending.
func sortResultsByHighCritical(results []skillInstallResult) []skillInstallResult {
	sorted := make([]skillInstallResult, len(results))
	copy(sorted, results)
	sort.Slice(sorted, func(i, j int) bool {
		ci := countHighCritical(sorted[i])
		cj := countHighCritical(sorted[j])
		if ci != cj {
			return ci > cj
		}
		return sorted[i].skill.Name < sorted[j].skill.Name
	})
	return sorted
}

func countHighCritical(r skillInstallResult) int {
	n := 0
	for _, w := range r.warnings {
		if strings.HasPrefix(w, "audit CRITICAL:") || strings.HasPrefix(w, "audit HIGH:") {
			n++
		}
	}
	return n
}
