package main

import (
	"fmt"
	"strings"

	"skillshare/internal/audit"
	"skillshare/internal/install"
	"skillshare/internal/ui"
)

// updateResult tracks the result of an update operation
type updateResult struct {
	updated        int
	skipped        int
	securityFailed int
	pruned         int
}

// batchBlockedEntry records a skill that was blocked by security audit during batch update.
type batchBlockedEntry struct {
	name   string
	errMsg string
}

// batchAuditEntry holds per-item audit info for post-batch summary.
type batchAuditEntry struct {
	name      string
	risk      string // e.g. "CLEAN", "MEDIUM (42/100)"
	warnings  []string
	riskScore int
	skipped   bool
	result    *install.InstallResult // nil for tracked repos
}

// batchAuditEntryFromAuditResult builds a batchAuditEntry from an audit.Result
// (used for tracked repos where we only have the raw audit scan result).
func batchAuditEntryFromAuditResult(name string, ar *audit.Result, skipAudit bool) batchAuditEntry {
	entry := batchAuditEntry{
		name:      name,
		riskScore: ar.RiskScore,
		skipped:   skipAudit,
	}
	label := strings.ToUpper(ar.RiskLabel)
	if label == "" && len(ar.Findings) == 0 {
		label = "CLEAN"
	}
	if ar.RiskScore > 0 {
		entry.risk = fmt.Sprintf("%s (%d/100)", label, ar.RiskScore)
	} else {
		entry.risk = label
	}
	// Convert findings to warning strings (same format as install_audit.go)
	for _, f := range ar.Findings {
		msg := fmt.Sprintf("audit %s: %s (%s:%d)", f.Severity, f.Message, f.File, f.Line)
		if f.Snippet != "" {
			msg += fmt.Sprintf("\n       %q", f.Snippet)
		}
		entry.warnings = append(entry.warnings, msg)
	}
	return entry
}

// batchAuditEntryFromInstallResult builds a batchAuditEntry from an InstallResult
// (used for grouped and standalone skills).
func batchAuditEntryFromInstallResult(name string, res *install.InstallResult) batchAuditEntry {
	return batchAuditEntry{
		name:      name,
		risk:      formatRiskLabel(res),
		warnings:  res.Warnings,
		riskScore: res.AuditRiskScore,
		skipped:   res.AuditSkipped,
		result:    res,
	}
}

// displayUpdateAuditResults renders the audit findings section for batch updates.
// Non-verbose: CLEAN count + compact severity breakdown (matches install --all output).
// Verbose: adds per-skill risk lines and detailed findings.
func displayUpdateAuditResults(entries []batchAuditEntry, auditVerbose bool) {
	if len(entries) == 0 {
		return
	}

	// Convert batchAuditEntry → skillInstallResult for reuse of install rendering
	results := make([]skillInstallResult, 0, len(entries))
	for _, e := range entries {
		sir := skillInstallResult{
			skill:          install.SkillInfo{Name: e.name},
			success:        true,
			warnings:       e.warnings,
			auditRiskLabel: e.risk,
			auditRiskScore: e.riskScore,
			auditSkipped:   e.skipped,
			result:         e.result,
		}
		results = append(results, sir)
	}

	// Categorize entries
	clean := 0
	var notable []batchAuditEntry
	for _, e := range entries {
		if e.risk == "CLEAN" || e.risk == "" {
			clean++
		} else {
			notable = append(notable, e)
		}
	}

	// Count total warnings across all entries
	totalWarnings := 0
	for _, r := range results {
		totalWarnings += len(r.warnings)
	}

	ui.SectionLabel("Audit Findings")

	if auditVerbose {
		// Verbose: per-skill risk lines + detailed findings
		for _, e := range notable {
			ui.Warning("risk: %s — %s", e.risk, e.name)
		}
		if clean > 0 {
			ui.Info("Audit: %d skill(s) CLEAN", clean)
		}
		if totalWarnings > 0 {
			skillsWithWarnings := countSkillsWithWarnings(results)
			if skillsWithWarnings <= 20 {
				for _, e := range entries {
					if len(e.warnings) > 0 {
						renderInstallWarningsWithResult(e.name, e.warnings, true, e.result)
					}
				}
			} else {
				// Large batch verbose: compact summary + top HIGH/CRITICAL detail
				renderBatchInstallWarningsCompact(results, totalWarnings,
					"%d audit finding line(s) across all skills; HIGH/CRITICAL detail expanded below")
				fmt.Println()
				ui.Warning("HIGH/CRITICAL detail (top skills):")
				shown := 0
				for _, r := range sortResultsByHighCritical(results) {
					if shown >= 20 || !hasHighCriticalWarnings(r) {
						break
					}
					renderInstallWarningsHighCriticalOnly(r.skill.Name, r.warnings)
					shown++
				}
			}
		}
	} else {
		// Non-verbose: compact summary only (matches install --all output)
		if clean > 0 {
			ui.Info("Audit: %d skill(s) CLEAN", clean)
		}
		if totalWarnings > 0 {
			if len(entries) > 100 {
				renderUltraCompactAuditSummary(results, totalWarnings)
			} else {
				renderBatchInstallWarningsCompact(results, totalWarnings,
					"suppressed %d audit finding line(s); re-run with --audit-verbose for full details")
			}
		}
	}
}

// displayUpdateBlockedSection renders the "Blocked / Rolled Back" section
// for skills that were blocked by security audit during batch update.
func displayUpdateBlockedSection(blocked []batchBlockedEntry) {
	if len(blocked) == 0 {
		return
	}
	ui.SectionLabel("Blocked / Rolled Back")
	ui.Warning("%d skill(s) blocked by security audit", len(blocked))
	ui.Info("Use --force or --skip-audit to bypass")
	for _, b := range blocked {
		digest := parseAuditBlockedFailure(b.errMsg)
		label := blockedSkillLabel(b.name, digest.threshold)
		ui.StepFail(label, compactBlockedUpdateMessage(b.errMsg))
	}
}

// compactBlockedUpdateMessage extracts a compact message from a blocked update error.
func compactBlockedUpdateMessage(errMsg string) string {
	digest := parseAuditBlockedFailure(errMsg)
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
	return strings.Join(parts, " ")
}

// formatRiskLabel builds a display string from install result audit info.
func formatRiskLabel(result *install.InstallResult) string {
	if result == nil || result.AuditSkipped || result.AuditRiskLabel == "" {
		return ""
	}
	label := strings.ToUpper(result.AuditRiskLabel)
	if result.AuditRiskScore > 0 {
		return fmt.Sprintf("%s (%d/100)", label, result.AuditRiskScore)
	}
	return label
}

// displayPrunedSection shows the list of pruned (stale) skills after batch update.
func displayPrunedSection(pruned []string) {
	if len(pruned) == 0 {
		return
	}
	ui.SectionLabel("Pruned (Stale)")
	for _, name := range pruned {
		ui.ListItem("warning", name, "removed (deleted upstream)")
	}
}

// displayStaleWarning shows a warning for stale skills when --prune is not used.
func displayStaleWarning(stale []string) {
	if len(stale) == 0 {
		return
	}
	fmt.Println()
	ui.Warning("%d skill(s) no longer found in upstream repository:", len(stale))
	for _, name := range stale {
		ui.ListItem("warning", name, "stale (deleted upstream)")
	}
	ui.Info("Run with --prune to remove stale skills")
}
