package main

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"skillshare/internal/audit"
	"skillshare/internal/theme"
	"skillshare/internal/ui"
	"skillshare/internal/utils"
)

// riskColor maps a risk label to an ANSI color, aligned with formatSeverity.
func riskColor(label string) string {
	if c := ui.SeverityColor(label); c != "" {
		return c
	}
	return ui.Dim
}

// auditTUIContext carries info needed to scan the "other" kind for TUI tab switching.
type auditTUIContext struct {
	kind             resourceKindFilter
	sourcePath       string // skills source (always)
	agentsSourcePath string // agents source (always)
	projectRoot      string
	threshold        string
	registry         *audit.Registry
	mode             string
}

// presentAuditResults handles the common output path for audit scans:
// prints per-skill rows only when TUI is unavailable, always prints the
// summary, and launches TUI when conditions are met.
func presentAuditResults(results []*audit.Result, elapsed []time.Duration, scanOutputs []audit.ScanOutput, summary auditRunSummary, jsonOutput bool, opts auditOptions, took time.Duration, tuiCtx *auditTUIContext) error {
	useTUI := !jsonOutput && shouldLaunchTUI(opts.NoTUI, nil) && len(results) > 1

	if !jsonOutput {
		kind := kindSkills
		if tuiCtx != nil {
			kind = tuiCtx.kind
		}
		flagged, blockedFlagged := "", false
		if !useTUI {
			// In batch mode (multiple results), only show skills with findings
			// to avoid flooding the terminal. Use --quiet=false explicitly or
			// single-skill mode to see clean results.
			suppressClean := len(results) > 1
			var shown []*audit.Result
			var shownElapsed []time.Duration
			for i, r := range results {
				if len(r.Findings) > 0 || (!opts.Quiet && !suppressClean) {
					shown = append(shown, r)
					shownElapsed = append(shownElapsed, elapsed[i])
				}
			}
			names := make([]string, len(shown))
			// Point Next at the first blocked skill, else the first with findings.
			for i, r := range shown {
				names[i] = r.SkillName
				if r.SkillName == audit.CrossSkillResultName || len(r.Findings) == 0 {
					continue
				}
				if flagged == "" || (r.IsBlocked && !blockedFlagged) {
					flagged, blockedFlagged = r.SkillName, r.IsBlocked
				}
			}
			width := ui.RowWidth(names...)
			for i, r := range shown {
				printSkillResultLine(r, shownElapsed[i], width)
			}
			if len(shown) > 0 {
				fmt.Println()
			}
		}
		printAuditSummary(summary, kind)
		fmt.Println()
		printAuditDone(summary, kind, "", took)
		if flagged != "" {
			ui.Next("skillshare audit "+flagged, "see its findings")
		}
	}

	if useTUI {
		if tuiCtx != nil {
			return launchAuditTUIWithTabs(results, scanOutputs, summary, tuiCtx)
		}
		return runAuditTUI(results, scanOutputs, summary, nil, nil, auditRunSummary{}, auditTabSkills)
	}
	return nil
}

// printAuditHeader names what the audit scans and the rules it applies.
func printAuditHeader(mode, path, threshold, policyLine string) {
	fmt.Println(theme.Primary().Bold(true).Render("Audit") + "  " + utils.FoldHomePath(path))
	ui.Note(fmt.Sprintf("%s · blocks at %s · policy %s", mode, threshold, policyLine))
	fmt.Println()
}

func launchAuditTUIWithTabs(results []*audit.Result, scanOutputs []audit.ScanOutput, summary auditRunSummary, ctx *auditTUIContext) error {
	initialTab := auditTabSkills
	if ctx.kind == kindAgents {
		initialTab = auditTabAgents
	}

	// Scan the "other" kind for the second tab.
	var otherResults []*audit.Result
	var otherOutputs []audit.ScanOutput
	var otherSummary auditRunSummary

	otherKindFilter := kindAgents
	otherSource := ctx.agentsSourcePath
	if ctx.kind == kindAgents {
		otherKindFilter = kindSkills
		otherSource = ctx.sourcePath
	}

	if otherSource != "" {
		otherPaths, err := discoverForKind(otherKindFilter, otherSource)
		otherInputs := toInputsForKind(otherKindFilter, otherPaths)
		if err == nil && len(otherPaths) > 0 {
			otherScanResults := audit.ParallelScan(otherInputs, ctx.projectRoot, nil, ctx.registry)
			for i := range otherPaths {
				if i < len(otherScanResults) {
					sr := otherScanResults[i]
					if sr.Err == nil {
						sr.Result.Threshold = ctx.threshold
						sr.Result.IsBlocked = sr.Result.HasSeverityAtOrAbove(ctx.threshold)
						sr.Result.Kind = otherKindFilter.SingularNoun()
						if rel, relErr := filepath.Rel(otherSource, sr.Result.ScanTarget); relErr == nil {
							sr.Result.SkillName = rel
						}
						otherResults = append(otherResults, sr.Result)
						otherOutputs = append(otherOutputs, sr)
					}
				}
			}
			otherSummary = summarizeAuditResults(len(otherPaths), otherResults, ctx.threshold)
			otherSummary.Mode = ctx.mode
		}
	}

	// Filter out synthetic _cross-skill result — it's not a real resource.
	var filteredResults []*audit.Result
	var filteredOutputs []audit.ScanOutput
	for i, r := range results {
		if r.SkillName != audit.CrossSkillResultName {
			filteredResults = append(filteredResults, r)
			if i < len(scanOutputs) {
				filteredOutputs = append(filteredOutputs, scanOutputs[i])
			}
		}
	}

	// Arrange into skills vs agents.
	var skillResults, agentResults []*audit.Result
	var skillOutputs, agentOutputs []audit.ScanOutput
	var skillSummary, agentSummary auditRunSummary

	if ctx.kind == kindAgents {
		agentResults, agentOutputs, agentSummary = filteredResults, filteredOutputs, summary
		skillResults, skillOutputs, skillSummary = otherResults, otherOutputs, otherSummary
	} else {
		skillResults, skillOutputs, skillSummary = filteredResults, filteredOutputs, summary
		agentResults, agentOutputs, agentSummary = otherResults, otherOutputs, otherSummary
	}

	return runAuditTUI(skillResults, skillOutputs, skillSummary, agentResults, agentOutputs, agentSummary, initialTab)
}

// printSkillResultLine prints a one-row result for a skill during batch scan.
func printSkillResultLine(result *audit.Result, elapsed time.Duration, width int) {
	took := ""
	if elapsed >= time.Second {
		took = ui.DimText(fmt.Sprintf(" · %.1fs", elapsed.Seconds()))
	}
	if len(result.Findings) == 0 {
		ui.Row(ui.MarkOK, result.SkillName, took, width)
		return
	}
	mark := ui.MarkWarn
	if result.IsBlocked {
		mark = ui.MarkFail
	}
	maxSeverity := result.MaxSeverity()
	if maxSeverity == "" {
		maxSeverity = "NONE"
	}
	value := formatSeverity(maxSeverity) + ui.DimText(fmt.Sprintf(" · risk %d/100", result.RiskScore)) + took
	ui.Row(mark, result.SkillName, value, width)
}

// printSkillResult prints the findings of a single-skill audit, then its
// risk, and closes with whether the skill is blocked.
func printSkillResult(result *audit.Result, kind resourceKindFilter, elapsed time.Duration) {
	if len(result.Findings) == 0 {
		printAuditDone(auditRunSummary{Passed: 1, Threshold: result.Threshold}, kind, result.SkillName, elapsed)
		return
	}

	for _, f := range result.Findings {
		pad := strings.Repeat(" ", max(0, len("CRITICAL")-len(f.Severity)))
		fmt.Printf("%s%s  %s  %s\n", formatSeverity(f.Severity), pad, f.Message, ui.DimText(fmt.Sprintf("%s:%d", f.File, f.Line)))
		if meta := findingMetaCLI(f); meta != "" {
			ui.Note(meta)
		}
		ui.Note(fmt.Sprintf("%q", f.Snippet))
		fmt.Println()
	}

	labels := []string{"Risk", "Auditable"}
	if !result.TierProfile.IsEmpty() {
		labels = append(labels, "Commands")
	}
	width := ui.RowWidth(labels...)
	ui.Row(ui.MarkNone, "Risk", ui.Colorize(riskColor(result.RiskLabel), fmt.Sprintf("%s %d/100", strings.ToUpper(result.RiskLabel), result.RiskScore)), width)
	ui.Row(ui.MarkNone, "Auditable", fmt.Sprintf("%.0f%%", result.Analyzability*100), width)
	if !result.TierProfile.IsEmpty() {
		ui.Row(ui.MarkNone, "Commands", result.TierProfile.String(), width)
	}

	summary := auditRunSummary{Threshold: result.Threshold}
	if result.IsBlocked {
		summary.Failed = 1
	} else {
		summary.Warning = 1
	}
	fmt.Println()
	printAuditDone(summary, kind, result.SkillName, elapsed)
}

// printAuditSummary prints the counts, severities, threats and aggregate
// risk of a batch scan. Zero counts are left out.
func printAuditSummary(summary auditRunSummary, kind resourceKindFilter) {
	// The audit header already leaves a blank line above.
	fmt.Println(theme.Primary().Bold(true).Render("Summary"))
	width := ui.RowWidth("Scanned", "Severity", "Scan errors")
	ui.Row(ui.MarkNone, "Scanned", fmt.Sprintf("%d %s", summary.Scanned, kind.Noun(summary.Scanned)), width)
	for _, c := range []struct {
		mark, label string
		n           int
	}{
		{ui.MarkOK, "Passed", summary.Passed},
		{ui.MarkWarn, "Warning", summary.Warning},
		{ui.MarkFail, "Failed", summary.Failed},
		{ui.MarkFail, "Scan errors", summary.ScanErrors},
	} {
		if c.n > 0 {
			ui.Row(c.mark, c.label, fmt.Sprintf("%d", c.n), width)
		}
	}
	var severities []string
	for _, s := range []struct {
		label string
		n     int
	}{
		{"CRITICAL", summary.Critical},
		{"HIGH", summary.High},
		{"MEDIUM", summary.Medium},
		{"LOW", summary.Low},
		{"INFO", summary.Info},
	} {
		if s.n > 0 {
			severities = append(severities, ui.Colorize(ui.SeverityColor(s.label), fmt.Sprintf("%d %s", s.n, strings.ToLower(s.label))))
		}
	}
	if len(severities) > 0 {
		ui.Row(ui.MarkNone, "Severity", strings.Join(severities, ", "), width)
	}
	if threats := formatCategoryBreakdown(summary.ByCategory, false); threats != "" {
		ui.Row(ui.MarkNone, "Threats", threats, width)
	}
	risk := ui.Colorize(riskColor(summary.RiskLabel), fmt.Sprintf("%s %d/100", strings.ToUpper(summary.RiskLabel), summary.RiskScore))
	ui.Row(ui.MarkNone, "Risk", risk+ui.DimText(fmt.Sprintf(" · %.0f%% auditable", summary.AvgAnalyzability*100)), width)
}

// printAuditDone closes an audit: blocked when a finding reaches the
// threshold, a warning when findings stay below it, clean otherwise. name
// is set for a single-skill audit.
func printAuditDone(summary auditRunSummary, kind resourceKindFilter, name string, took time.Duration) {
	scanned := plural(summary.Scanned, kind.SingularNoun())
	if name != "" {
		scanned = name
	}
	switch {
	case summary.Failed > 0:
		text := fmt.Sprintf("%s blocked: findings at %s or above", scanned, summary.Threshold)
		if name == "" {
			text = fmt.Sprintf("Blocked %d of %s: findings at %s or above", summary.Failed, scanned, summary.Threshold)
		}
		ui.Done(ui.MarkFail, text, took)
		ui.Note("The risk score is informational; only the severity blocks")
	case summary.Warning > 0:
		text := fmt.Sprintf("%s has findings below %s", scanned, summary.Threshold)
		if name == "" {
			text = fmt.Sprintf("%d of %s with findings below %s", summary.Warning, scanned, summary.Threshold)
		}
		ui.Done(ui.MarkWarn, text, took)
	default:
		ui.Done(ui.MarkOK, "No issues found in "+scanned, took)
	}
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

// formatCategoryBreakdownTUI formats a category count map as lipgloss-styled
// "cat:N cat:N ..." using dim/emphasis (no per-category colors).
// Uses compact short names. Returns "" if the map is empty.
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
		if cc.count > 50 {
			parts[i] = theme.Primary().Bold(true).Render(label+":") + theme.Primary().Bold(true).Render(fmt.Sprintf("%d", cc.count))
		} else {
			parts[i] = theme.Dim().Render(label + ":" + fmt.Sprintf("%d", cc.count))
		}
	}
	return strings.Join(parts, " ")
}

func printAuditHelp() {
	fmt.Println(`Usage: skillshare audit [agents] [name...] [options]
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
  skillshare audit -p --init-rules           # Create project custom rules file
  skillshare audit agents                    # Scan agents only`)
}
