package main

import (
	"strings"

	"skillshare/internal/ui"
)

// ── Policy label helpers (shared between CLI and TUI) ──

// policyProfileLabel returns the UPPERCASE display label for a profile.
func policyProfileLabel(profile string) string {
	if upper := strings.ToUpper(profile); upper != "" {
		return upper
	}
	return "DEFAULT"
}

// policyDedupeLabel returns the UPPERCASE display label for a dedupe mode.
func policyDedupeLabel(dedupe string) string {
	if upper := strings.ToUpper(dedupe); upper != "" {
		return upper
	}
	return "GLOBAL"
}

// policyAnalyzersLabel returns the UPPERCASE display label for enabled analyzers.
func policyAnalyzersLabel(analyzers []string) string {
	if len(analyzers) == 0 {
		return "ALL"
	}
	upper := make([]string, len(analyzers))
	for i, a := range analyzers {
		upper[i] = strings.ToUpper(a)
	}
	return strings.Join(upper, ", ")
}

// ── ANSI color helpers for audit policy values ──

// colorizeProfile returns an ANSI-colored UPPERCASE profile name.
// Only STRICT gets a warning color; everything else is dim metadata.
func colorizeProfile(profile string) string {
	label := policyProfileLabel(profile)
	if label == "STRICT" {
		return ui.Colorize(ui.Yellow, label)
	}
	return ui.Colorize(ui.Gray, label)
}

// colorizeDedupe returns an ANSI-colored UPPERCASE dedupe mode.
func colorizeDedupe(dedupe string) string {
	label := policyDedupeLabel(dedupe)
	if label == "LEGACY" {
		return ui.Colorize(ui.Yellow, label)
	}
	return ui.Colorize(ui.Gray, label)
}

// colorizeAnalyzers returns an ANSI-colored UPPERCASE analyzer list.
func colorizeAnalyzers(analyzers []string) string {
	return ui.Colorize(ui.Gray, policyAnalyzersLabel(analyzers))
}

// formatPolicyLine returns a compact one-line policy description.
// Uses dim/gray for metadata; only non-default values get attention color.
func formatPolicyLine(profile, dedupe string, analyzers []string) string {
	sep := ui.Colorize(ui.Gray, " / ")
	return colorizeProfile(profile) +
		sep + ui.Colorize(ui.Gray, "dedupe:") + colorizeDedupe(dedupe) +
		sep + ui.Colorize(ui.Gray, "analyzers:") + colorizeAnalyzers(analyzers)
}

// applyPolicyToSummary copies resolved policy fields from opts to summary
// so that TUI and text output can display them.
func applyPolicyToSummary(s *auditRunSummary, opts auditOptions) {
	s.PolicyProfile = opts.PolicyProfile
	s.PolicyDedupe = opts.PolicyDedupe
	s.PolicyAnalyzers = opts.PolicyAnalyzers
}
