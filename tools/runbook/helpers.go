package main

import "fmt"

// Status constants
const (
	StatusPassed  = "passed"
	StatusFailed  = "failed"
	StatusSkipped = "skipped"
	StatusRunning = "running"
)

// Executor mode constants
const (
	ExecutorAuto   = "auto"
	ExecutorManual = "manual"
)

// truncateText shortens s to max characters, adding ellipsis if needed.
func truncateText(s string, max int) string {
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	if max <= 1 {
		return "…"
	}
	return string(runes[:max-1]) + "…"
}

// formatDurationMs formats milliseconds into a human-readable string.
func formatDurationMs(ms int64) string {
	if ms < 1000 {
		return fmt.Sprintf("%dms", ms)
	}
	return fmt.Sprintf("%.1fs", float64(ms)/1000)
}

// stepFailReason extracts a concise failure reason from a StepResult.
func stepFailReason(r StepResult) string {
	for _, a := range r.Assertions {
		if !a.Matched {
			if a.Negated {
				return fmt.Sprintf("unexpected match: %s", a.Pattern)
			}
			return fmt.Sprintf("expected: %s", a.Pattern)
		}
	}
	if r.Error != "" {
		return r.Error
	}
	if r.ExitCode != 0 {
		return fmt.Sprintf("exit code %d", r.ExitCode)
	}
	return ""
}

// checkAssertions runs assertion matching on a step result.
func checkAssertions(result *StepResult, step Step) {
	if result.ExitCode == 0 && len(step.Expected) > 0 {
		combined := result.Stdout + "\n" + result.Stderr
		result.Assertions = MatchAssertions(combined, step.Expected)
		if !AllPassed(result.Assertions) {
			result.Status = StatusFailed
		}
	}
}
