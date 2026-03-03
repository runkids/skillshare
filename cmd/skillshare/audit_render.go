package main

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"skillshare/internal/audit"
	"skillshare/internal/ui"
)

// riskColor maps a risk label to an ANSI color, aligned with formatSeverity.
func riskColor(label string) string {
	if c := ui.SeverityColor(label); c != "" {
		return c
	}
	return ui.Gray
}

// presentAuditResults handles the common output path for audit scans:
// prints per-skill list only when TUI is unavailable, always prints summary,
// and launches TUI when conditions are met.
func presentAuditResults(results []*audit.Result, elapsed []time.Duration, scanOutputs []audit.ScanOutput, summary auditRunSummary, jsonOutput bool, opts auditOptions, headerMinWidth int) error {
	useTUI := !jsonOutput && shouldLaunchTUI(opts.NoTUI, nil) && len(results) > 1

	if !jsonOutput {
		if !useTUI {
			// In batch mode (multiple results), only show skills with findings
			// to avoid flooding the terminal. Use --quiet=false explicitly or
			// single-skill mode to see clean results.
			suppressClean := len(results) > 1
			for i, r := range results {
				if len(r.Findings) > 0 || (!opts.Quiet && !suppressClean) {
					printSkillResultLine(i+1, len(results), r, elapsed[i])
				}
			}
			fmt.Println()
		}
		summaryLines := buildAuditSummaryLines(summary)
		printAuditSummary(summary, summaryLines, headerMinWidth)
	}

	if useTUI {
		return runAuditTUI(results, scanOutputs, summary)
	}
	return nil
}

// printSkillResultLine prints a single-line result for a skill during batch scan.
func printSkillResultLine(index, total int, result *audit.Result, elapsed time.Duration) {
	prefix := fmt.Sprintf("[%d/%d]", index, total)
	name := result.SkillName
	showTime := elapsed >= time.Second
	timeStr := fmt.Sprintf("%.1fs", elapsed.Seconds())

	if len(result.Findings) == 0 {
		if ui.IsTTY() {
			if showTime {
				fmt.Printf("%s %s✓%s %s %s%s%s\n", prefix, ui.Green, ui.Reset, name, ui.Gray, timeStr, ui.Reset)
			} else {
				fmt.Printf("%s %s✓%s %s\n", prefix, ui.Green, ui.Reset, name)
			}
		} else {
			if showTime {
				fmt.Printf("%s ✓ %s %s\n", prefix, name, timeStr)
			} else {
				fmt.Printf("%s ✓ %s\n", prefix, name)
			}
		}
		return
	}

	color := riskColor(result.RiskLabel)
	symbol := "!"
	if result.IsBlocked {
		symbol = "✗"
	}
	maxSeverity := result.MaxSeverity()
	if maxSeverity == "" {
		maxSeverity = "NONE"
	}
	riskText := fmt.Sprintf("AGG %s %d/100, max %s", strings.ToUpper(result.RiskLabel), result.RiskScore, maxSeverity)

	if ui.IsTTY() {
		if showTime {
			fmt.Printf("%s %s%s%s %s  %s(%s)%s  %s%s%s\n", prefix, color, symbol, ui.Reset, name, color, riskText, ui.Reset, ui.Gray, timeStr, ui.Reset)
		} else {
			fmt.Printf("%s %s%s%s %s  %s(%s)%s\n", prefix, color, symbol, ui.Reset, name, color, riskText, ui.Reset)
		}
	} else {
		if showTime {
			fmt.Printf("%s %s %s  (%s)  %s\n", prefix, symbol, name, riskText, timeStr)
		} else {
			fmt.Printf("%s %s %s  (%s)\n", prefix, symbol, name, riskText)
		}
	}
}

// printSkillResult prints detailed results for a single-skill audit.
func printSkillResult(result *audit.Result, elapsed time.Duration) {
	if len(result.Findings) == 0 {
		ui.Success("No issues found in %s (%.1fs)", result.SkillName, elapsed.Seconds())
		return
	}

	for _, f := range result.Findings {
		sevLabel := formatSeverity(f.Severity)
		loc := fmt.Sprintf("%s:%d", f.File, f.Line)
		if ui.IsTTY() {
			fmt.Printf("  %s: %s (%s)\n", sevLabel, f.Message, loc)
			if meta := findingMetaCLI(f); meta != "" {
				fmt.Printf("  %s[%s]%s\n", ui.Gray, meta, ui.Reset)
			}
			fmt.Printf("  %s\"%s\"%s\n\n", ui.Gray, f.Snippet, ui.Reset)
		} else {
			fmt.Printf("  %s: %s (%s)\n", f.Severity, f.Message, loc)
			if meta := findingMetaCLI(f); meta != "" {
				fmt.Printf("  [%s]\n", meta)
			}
			fmt.Printf("  \"%s\"\n\n", f.Snippet)
		}
	}

	color := riskColor(result.RiskLabel)
	threshold := result.Threshold
	if threshold == "" {
		threshold = audit.DefaultThreshold()
	}
	maxSeverity := result.MaxSeverity()
	if maxSeverity == "" {
		maxSeverity = "NONE"
	}
	decision := "ALLOW"
	compare := "<"
	if result.IsBlocked {
		decision = "BLOCK"
		compare = ">="
	}
	if ui.IsTTY() {
		fmt.Printf("%s→%s Aggregate risk: %s%s (%d/100)%s\n", ui.Cyan, ui.Reset, color, strings.ToUpper(result.RiskLabel), result.RiskScore, ui.Reset)
		fmt.Printf("%s→%s Auditable: %.0f%%\n", ui.Cyan, ui.Reset, result.Analyzability*100)
		if !result.TierProfile.IsEmpty() {
			fmt.Printf("%s→%s Commands: %s\n", ui.Cyan, ui.Reset, result.TierProfile.String())
		}
		fmt.Printf("%s→%s Block decision: %s (max severity %s %s threshold %s)\n", ui.Cyan, ui.Reset, decision, maxSeverity, compare, threshold)
	} else {
		fmt.Printf("→ Aggregate risk: %s (%d/100)\n", strings.ToUpper(result.RiskLabel), result.RiskScore)
		fmt.Printf("→ Auditable: %.0f%%\n", result.Analyzability*100)
		if !result.TierProfile.IsEmpty() {
			fmt.Printf("→ Commands: %s\n", result.TierProfile.String())
		}
		fmt.Printf("→ Block decision: %s (max severity %s %s threshold %s)\n", decision, maxSeverity, compare, threshold)
	}
}

// buildAuditSummaryLines builds the summary box lines (without printing).
func buildAuditSummaryLines(summary auditRunSummary) []string {
	var lines []string
	maxSeverity := summary.MaxSeverity
	if maxSeverity == "" {
		maxSeverity = "NONE"
	}
	// -- Policy settings --
	lines = append(lines, fmt.Sprintf("  Block:     severity >= %s", ui.Colorize(ui.SeverityColor(summary.Threshold), summary.Threshold)))
	lines = append(lines, fmt.Sprintf("  Policy:    %s", formatPolicyLine(summary.PolicyProfile, summary.PolicyDedupe, summary.PolicyAnalyzers)))
	lines = append(lines, fmt.Sprintf("  Max sev:   %s", ui.Colorize(ui.SeverityColor(maxSeverity), maxSeverity)))

	// -- Result counts --
	lines = append(lines, "")
	lines = append(lines, fmt.Sprintf("  Scanned:   %d skill(s)", summary.Scanned))
	lines = append(lines, fmt.Sprintf("  Passed:    %s", ui.Colorize(ui.Green, fmt.Sprintf("%d", summary.Passed))))
	if summary.Warning > 0 {
		lines = append(lines, fmt.Sprintf("  Warning:   %s", ui.Colorize(ui.Yellow, fmt.Sprintf("%d", summary.Warning))))
	} else {
		lines = append(lines, fmt.Sprintf("  Warning:   %d", summary.Warning))
	}
	if summary.Failed > 0 {
		lines = append(lines, fmt.Sprintf("  Failed:    %s", ui.Colorize(ui.Red, fmt.Sprintf("%d", summary.Failed))))
	} else {
		lines = append(lines, fmt.Sprintf("  Failed:    %d", summary.Failed))
	}

	// -- Severity & threat breakdown --
	lines = append(lines, "")
	lines = append(lines, fmt.Sprintf("  Severity:  c/h/m/l/i = %s/%s/%s/%s/%s",
		ui.Colorize(ui.SeverityColor("CRITICAL"), fmt.Sprintf("%d", summary.Critical)),
		ui.Colorize(ui.SeverityColor("HIGH"), fmt.Sprintf("%d", summary.High)),
		ui.Colorize(ui.SeverityColor("MEDIUM"), fmt.Sprintf("%d", summary.Medium)),
		ui.Colorize(ui.SeverityColor("LOW"), fmt.Sprintf("%d", summary.Low)),
		ui.Colorize(ui.SeverityColor("INFO"), fmt.Sprintf("%d", summary.Info))))
	if ui.IsTTY() {
		if threatsLine := formatCategoryBreakdownCLI(summary.ByCategory); threatsLine != "" {
			lines = append(lines, fmt.Sprintf("  Threats:   %s", threatsLine))
		}
	} else {
		if threatsLine := formatCategoryBreakdown(summary.ByCategory, false); threatsLine != "" {
			lines = append(lines, fmt.Sprintf("  Threats:   %s", threatsLine))
		}
	}

	// -- Aggregate risk --
	lines = append(lines, "")
	riskLabel := strings.ToUpper(summary.RiskLabel)
	riskText := fmt.Sprintf("%s (%d/100)", riskLabel, summary.RiskScore)
	lines = append(lines, fmt.Sprintf("  Aggregate: %s", ui.Colorize(riskColor(summary.RiskLabel), riskText)))
	lines = append(lines, fmt.Sprintf("  Auditable: %.0f%% avg", summary.AvgAnalyzability*100))

	// -- Note --
	lines = append(lines, "")
	lines = append(lines, "  Note:      Failed uses severity gate; aggregate is informational")
	if summary.ScanErrors > 0 {
		lines = append(lines, fmt.Sprintf("  Scan errs: %d", summary.ScanErrors))
	}
	return lines
}

// auditSummaryNoteLine is the longest fixed-content line in the summary box.
const auditSummaryNoteLine = "  Note:      Failed uses severity gate; aggregate is informational"

// auditHeaderMinWidth computes a minimum content width for the header box,
// ensuring it is at least as wide as the summary box's fixed-content lines.
func auditHeaderMinWidth(subtitle string) int {
	minW := len(auditSummaryNoteLine)
	for _, line := range strings.Split(subtitle, "\n") {
		if w := ui.DisplayWidth(line); w > minW {
			minW = w
		}
	}
	return minW
}

// printAuditSummary prints the summary box with a shared minimum width.
func printAuditSummary(_ auditRunSummary, lines []string, minWidth int) {
	ui.BoxWithMinWidth("Summary", minWidth, lines...)
	fmt.Println()
}

// formatSeverity returns an ANSI-colored uppercase severity label.
func formatSeverity(sev string) string {
	return ui.Colorize(ui.SeverityColor(sev), strings.ToUpper(sev))
}

// findingMetaCLI builds a compact "ruleID / analyzer" metadata string for CLI output.
// Returns "" if neither field is set.
func findingMetaCLI(f audit.Finding) string {
	var parts []string
	if f.RuleID != "" {
		parts = append(parts, f.RuleID)
	}
	if f.Analyzer != "" {
		parts = append(parts, f.Analyzer)
	}
	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, " / ")
}

// categoryShortNames maps full category names to compact abbreviations for TUI footer.
var categoryShortNames = map[string]string{
	"injection":    "inj",
	"exfiltration": "exfil",
	"credential":   "cred",
	"obfuscation":  "obfusc",
	"privilege":    "priv",
	"integrity":    "integ",
	"structure":    "struct",
	"risk":         "risk",
}

// formatCategoryBreakdown formats a category count map as "cat:N cat:N ..."
// sorted by count descending. If compact is true, uses short names.
// Returns "" if the map is empty.
func formatCategoryBreakdown(cats map[string]int, compact bool) string {
	if len(cats) == 0 {
		return ""
	}
	type catCount struct {
		name  string
		count int
	}
	sorted := make([]catCount, 0, len(cats))
	for name, count := range cats {
		sorted = append(sorted, catCount{name, count})
	}
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].count != sorted[j].count {
			return sorted[i].count > sorted[j].count
		}
		return sorted[i].name < sorted[j].name
	})

	parts := make([]string, len(sorted))
	for i, cc := range sorted {
		label := cc.name
		if compact {
			if short, ok := categoryShortNames[cc.name]; ok {
				label = short
			}
		}
		parts[i] = fmt.Sprintf("%s:%d", label, cc.count)
	}
	return strings.Join(parts, " ")
}

// categoryColorCLI returns an ANSI color code for a threat category,
// matching the semantic palette used in the TUI (categoryStyle).
func categoryColorCLI(cat string) string {
	switch strings.ToLower(cat) {
	case "injection":
		return ui.BrightRed
	case "exfiltration":
		return ui.Orange
	case "credential":
		return ui.Magenta
	case "obfuscation":
		return ui.Purple
	case "privilege":
		return ui.Yellow
	case "integrity":
		return ui.Cyan
	case "structure":
		return ui.Blue
	case "risk":
		return ui.OrangeAlt
	default:
		return ui.Gray
	}
}

// formatCategoryBreakdownCLI formats a category count map as ANSI-colored
// "cat:N cat:N ..." for CLI summary box output. Returns "" if empty.
func formatCategoryBreakdownCLI(cats map[string]int) string {
	if len(cats) == 0 {
		return ""
	}
	type catCount struct {
		name  string
		count int
	}
	sorted := make([]catCount, 0, len(cats))
	for name, count := range cats {
		sorted = append(sorted, catCount{name, count})
	}
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].count != sorted[j].count {
			return sorted[i].count > sorted[j].count
		}
		return sorted[i].name < sorted[j].name
	})

	parts := make([]string, len(sorted))
	for i, cc := range sorted {
		color := categoryColorCLI(cc.name)
		parts[i] = fmt.Sprintf("%s%s:%s%d%s", color, cc.name, ui.Bold, cc.count, ui.Reset)
	}
	return strings.Join(parts, " ")
}

// formatCategoryBreakdownTUI formats a category count map as lipgloss-styled
// "cat:N cat:N ..." using semantic category colors. Uses compact short names.
// Returns "" if the map is empty.
func formatCategoryBreakdownTUI(cats map[string]int) string {
	if len(cats) == 0 {
		return ""
	}
	type catCount struct {
		name  string
		count int
	}
	sorted := make([]catCount, 0, len(cats))
	for name, count := range cats {
		sorted = append(sorted, catCount{name, count})
	}
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].count != sorted[j].count {
			return sorted[i].count > sorted[j].count
		}
		return sorted[i].name < sorted[j].name
	})

	parts := make([]string, len(sorted))
	for i, cc := range sorted {
		label := cc.name
		if short, ok := categoryShortNames[cc.name]; ok {
			label = short
		}
		style := categoryStyle(cc.name)
		parts[i] = style.Render(label) + tc.Dim.Render(":") + style.Bold(true).Render(fmt.Sprintf("%d", cc.count))
	}
	return strings.Join(parts, " ")
}

func printAuditHelp() {
	fmt.Println(`Usage: skillshare audit [name...] [options]
       skillshare audit --group <group> [options]
       skillshare audit <path> [options]

Scan installed skills (or a specific skill/path) for security threats.

If no names or groups are specified, all installed skills are scanned.
Block decisions use severity threshold; aggregate risk score is reported separately.

Arguments:
  name...              Skill name(s) to scan (optional)
  path                 Existing file/directory path to scan (optional)

Options:
  --group, -G <name>   Scan all skills in a group (repeatable)
  -p, --project        Use project-level skills
  -g, --global         Use global skills
  --threshold, -T <t>  Block by severity at/above: critical|high|medium|low|info
                       (also supports c|h|m|l|i)
  --profile <p>        Audit profile preset: default, strict, permissive
  --dedupe <mode>      Dedup mode: legacy, global (default)
  --analyzer <id>      Only run specified analyzer (repeatable)
                       IDs: static, dataflow, tier, integrity, structure, cross-skill
  --format <f>         Output format: text (default), json, sarif, markdown
  --json               Output JSON (deprecated: use --format json)
  --quiet, -q          Only show skills with findings + summary (skip clean ✓ lines)
  --yes, -y            Skip large-audit confirmation prompt
  --no-tui             Disable interactive TUI, use plain text output
  --init-rules         Create a starter audit-rules.yaml
  -h, --help           Show this help

Subcommands:
  rules                Browse, enable/disable rules (see: audit rules --help)

Examples:
  skillshare audit                           # Scan all installed skills
  skillshare audit react-patterns            # Scan a specific skill
  skillshare audit a b c                     # Scan multiple skills
  skillshare audit --group frontend          # Scan all skills in frontend/
  skillshare audit x -G backend              # Mix names and groups
  skillshare audit ./skills/my-skill         # Scan a directory path
  skillshare audit ./skills/foo/SKILL.md     # Scan a single file
  skillshare audit --threshold high          # Block on HIGH+ findings
  skillshare audit -T h                      # Same, with shorthand alias
  skillshare audit --format json              # Output machine-readable JSON
  skillshare audit --format sarif            # Output SARIF 2.1.0 for GitHub Code Scanning
  skillshare audit --format markdown         # Output Markdown report (for GitHub Issues/PRs)
  skillshare audit --json                    # Same as --format json (deprecated)
  skillshare audit -p --init-rules           # Create project custom rules file`)
}
