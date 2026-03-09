package main

import (
	"fmt"
	"io"
	"strings"
)

// WriteSingleReport prints a single runbook result in plain text.
func WriteSingleReport(w io.Writer, r Report) {
	icon := "✓"
	if r.Summary.Failed > 0 {
		icon = "✗"
	}

	fmt.Fprintf(w, "\n %s %s\n", icon, r.Runbook)
	fmt.Fprintf(w, " %s\n", strings.Repeat("─", 50))

	for _, s := range r.Steps {
		sIcon := stepStatusIcon(s.Status)
		fmt.Fprintf(w, " %s  Step %-2d %-38s", sIcon, s.Step.Number, truncTitle(s.Step.Title))
		if s.Status == "passed" || s.Status == "failed" {
			fmt.Fprintf(w, " %s", fmtDur(s.DurationMs))
		}
		fmt.Fprintln(w)

		if s.Status == "failed" {
			reason := plainFailReason(s)
			if reason != "" {
				fmt.Fprintf(w, "          └─ %s\n", reason)
			}
		}
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
func WritePlainSummary(w io.Writer, reports []Report) {
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
			if s.Status == "failed" {
				fmt.Fprintf(w, "    └─ Step %d: %s\n", s.Step.Number, plainFailReason(s))
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

func stepStatusIcon(status string) string {
	switch status {
	case "passed":
		return "✓"
	case "failed":
		return "✗"
	case "skipped":
		return "○"
	default:
		return "●"
	}
}

func truncTitle(s string) string {
	if len(s) > 38 {
		return s[:35] + "..."
	}
	return s
}

func fmtDur(ms int64) string {
	if ms < 1000 {
		return fmt.Sprintf("%dms", ms)
	}
	return fmt.Sprintf("%.1fs", float64(ms)/1000)
}

func plainFailReason(r StepResult) string {
	for _, a := range r.Assertions {
		if !a.Matched {
			return fmt.Sprintf("expected %q not found", a.Pattern)
		}
	}
	if r.Error != "" {
		return r.Error
	}
	if r.ExitCode != 0 {
		return fmt.Sprintf("exit code %d", r.ExitCode)
	}
	return "unknown"
}
