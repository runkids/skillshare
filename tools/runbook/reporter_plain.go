package main

import (
	"fmt"
	"io"
	"strings"
)

// WriteSingleReport prints a single runbook result in plain text.
// verbosity: 0=default, 1=show assertions, 2=assertions+stdout/stderr.
func WriteSingleReport(w io.Writer, r Report, verbosity int) {
	icon := "✓"
	if r.Summary.Failed > 0 {
		icon = "✗"
	}

	fmt.Fprintf(w, "\n %s %s\n", icon, r.Runbook)
	fmt.Fprintf(w, " %s\n", strings.Repeat("─", 50))

	// Show hook status if present.
	if status, ok := r.Hooks["setup"]; ok {
		hIcon := plainStatusIcon(status)
		fmt.Fprintf(w, " %s  [setup]\n", hIcon)
		if status == StatusFailed {
			fmt.Fprintf(w, "          └─ setup failed, all steps skipped\n")
		}
	}

	for _, s := range r.Steps {
		sIcon := plainStatusIcon(s.Status)
		fmt.Fprintf(w, " %s  Step %-2d %-38s", sIcon, s.Step.Number, truncateText(s.Step.Title, 38))
		if s.Status == StatusPassed || s.Status == StatusFailed {
			fmt.Fprintf(w, " %s", formatDurationMs(s.DurationMs))
		}
		fmt.Fprintln(w)

		if verbosity == 0 {
			if s.Status == StatusFailed {
				reason := stepFailReason(s)
				if reason != "" {
					fmt.Fprintf(w, "          └─ %s\n", reason)
				}
			} else if s.Status == StatusSkipped && s.Error != "" && s.Error != "manual step" {
				fmt.Fprintf(w, "          └─ %s\n", s.Error)
			}
		}

		// -v: show all assertions
		if verbosity >= 1 {
			writeAssertionDetails(w, s)
		}

		// -vv: show stdout/stderr snippets
		if verbosity >= 2 {
			writeOutputSnippet(w, "stdout", s.Stdout)
			writeOutputSnippet(w, "stderr", s.Stderr)
		}
	}

	// Show teardown status if present.
	if status, ok := r.Hooks["teardown"]; ok {
		hIcon := plainStatusIcon(status)
		fmt.Fprintf(w, " %s  [teardown]\n", hIcon)
	}

	fmt.Fprintf(w, " %s\n", strings.Repeat("─", 50))
	fmt.Fprintf(w, " %d/%d passed", r.Summary.Passed, r.Summary.Total)
	if r.Summary.Failed > 0 {
		fmt.Fprintf(w, "  %d failed", r.Summary.Failed)
	}
	if r.Summary.Skipped > 0 {
		fmt.Fprintf(w, "  %d skipped", r.Summary.Skipped)
	}
	fmt.Fprintf(w, "  %.1fs\n\n", float64(r.DurationMs)/1000)
}

// WritePlainSummary prints a multi-runbook batch summary.
func WritePlainSummary(w io.Writer, reports []Report, verbosity int) {
	fmt.Fprintf(w, "\n Runbook Results (%d files)\n", len(reports))
	fmt.Fprintf(w, " %s\n", strings.Repeat("─", 55))

	for _, r := range reports {
		icon := "✓"
		if r.Summary.Failed > 0 {
			icon = "✗"
		}
		fmt.Fprintf(w, " %s  %-42s %d/%-3d %.1fs\n",
			icon, r.Runbook,
			r.Summary.Passed, r.Summary.Total,
			float64(r.DurationMs)/1000)

		for _, s := range r.Steps {
			if s.Status == StatusFailed {
				fmt.Fprintf(w, "    └─ Step %d: %s\n", s.Step.Number, stepFailReason(s))
			}
		}
	}

	totalP, totalF, totalS := 0, 0, 0
	for _, r := range reports {
		totalP += r.Summary.Passed
		totalF += r.Summary.Failed
		totalS += r.Summary.Skipped
	}

	fmt.Fprintf(w, " %s\n", strings.Repeat("─", 55))
	total := totalP + totalF + totalS
	fmt.Fprintf(w, " %d/%d passed", totalP, total)
	if totalF > 0 {
		fmt.Fprintf(w, "  %d failed", totalF)
	}
	if totalS > 0 {
		fmt.Fprintf(w, "  %d skipped", totalS)
	}
	fmt.Fprintln(w)
}

// writeAssertionDetails prints assertion results for a step (-v).
func writeAssertionDetails(w io.Writer, s StepResult) {
	// Show skip reason.
	if s.Status == StatusSkipped && s.Error != "" && s.Error != "manual step" {
		fmt.Fprintf(w, "          ○ %s\n", s.Error)
		return
	}
	if len(s.Assertions) == 0 && s.Status == StatusFailed {
		reason := stepFailReason(s)
		if reason != "" {
			fmt.Fprintf(w, "          ✗ %s\n", reason)
		}
		return
	}
	for _, a := range s.Assertions {
		icon := "✓"
		if !a.Matched {
			icon = "✗"
		}
		label := a.Pattern
		if a.Type != "" && a.Type != AssertSubstring {
			label = a.Type + ": " + a.Pattern
		}
		fmt.Fprintf(w, "          %s %s\n", icon, label)
	}
}

// writeOutputSnippet prints first/last 5 lines of output (-vv).
func writeOutputSnippet(w io.Writer, label, output string) {
	output = strings.TrimSpace(output)
	if output == "" {
		return
	}
	lines := strings.Split(output, "\n")
	fmt.Fprintf(w, "          %s (%d lines):\n", label, len(lines))
	if len(lines) <= 10 {
		for _, l := range lines {
			fmt.Fprintf(w, "            %s\n", l)
		}
	} else {
		for _, l := range lines[:5] {
			fmt.Fprintf(w, "            %s\n", l)
		}
		fmt.Fprintf(w, "            ... (%d lines omitted)\n", len(lines)-10)
		for _, l := range lines[len(lines)-5:] {
			fmt.Fprintf(w, "            %s\n", l)
		}
	}
}

func plainStatusIcon(status string) string {
	switch status {
	case StatusPassed:
		return "✓"
	case StatusFailed:
		return "✗"
	case StatusSkipped:
		return "○"
	default:
		return "●"
	}
}
