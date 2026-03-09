package main

import (
	"fmt"
	"strconv"
)

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
// If assertions are defined, they always run (regardless of exit code)
// and determine the final pass/fail status. If no assertions are defined,
// exit code alone determines the result (0=pass, non-zero=fail).
func checkAssertions(result *StepResult, step Step) {
	if len(step.Expected) == 0 {
		return
	}

	result.Assertions = RunAssertions(result, step.Expected)
	if AllPassed(result.Assertions) {
		result.Status = StatusPassed
	} else {
		result.Status = StatusFailed
	}
}

// countFlag implements flag.Value for counting repeated -v flags.
// Supports: -v (=1), -v -v (=2), -v=2 (=2).
type countFlag int

func (c *countFlag) String() string { return strconv.Itoa(int(*c)) }
func (c *countFlag) Set(s string) error {
	if s == "true" || s == "" {
		// Called as a boolean-style flag: -v
		*c++
		return nil
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return fmt.Errorf("invalid verbosity %q", s)
	}
	*c = countFlag(n)
	return nil
}
func (c *countFlag) IsBoolFlag() bool { return true }
