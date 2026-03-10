package main

import "time"

// Step represents a single test step in a runbook.
type Step struct {
	Number      int           `json:"number"`
	Title       string        `json:"title"`
	Description string        `json:"description,omitempty"`
	Command     string        `json:"command,omitempty"`
	Lang        string        `json:"lang,omitempty"`
	Expected    []string      `json:"expected,omitempty"`
	Executor    string        `json:"executor,omitempty"` // "auto", "ai-delegate", "manual"
	Timeout     time.Duration `json:"timeout,omitempty"`  // per-step timeout override (0 = use global)
	Retry       int           `json:"retry,omitempty"`    // retry count on failure (0 = no retry)
	RetryDelay  time.Duration `json:"retry_delay,omitempty"`
	DependsOn   int           `json:"depends_on,omitempty"` // skip if this step number failed (0 = none)
}

// StepResult represents the execution result of a single step.
type StepResult struct {
	Step       Step              `json:"step"`
	Status     string            `json:"status"` // "passed", "failed", "skipped", "running"
	DurationMs int64             `json:"duration_ms"`
	Stdout     string            `json:"stdout,omitempty"`
	Stderr     string            `json:"stderr,omitempty"`
	ExitCode   int               `json:"exit_code"`
	Assertions []AssertionResult `json:"assertions,omitempty"`
	Error      string            `json:"error,omitempty"`
}

// Assertion type constants.
const (
	AssertSubstring = "substring"
	AssertExitCode  = "exit_code"
	AssertRegex     = "regex"
	AssertJQ        = "jq"
)

// AssertionResult represents the result of a single assertion check.
type AssertionResult struct {
	Pattern string `json:"pattern"`
	Type    string `json:"type,omitempty"` // "substring", "exit_code", "regex", "jq"
	Matched bool   `json:"matched"`
	Negated bool   `json:"negated,omitempty"`
	Detail  string `json:"detail,omitempty"` // extra info on failure (e.g., "got exit_code=1")
}

// Report represents the full execution report for a runbook.
type Report struct {
	Version     string            `json:"version"`
	Runbook     string            `json:"runbook"`
	Environment map[string]string `json:"environment,omitempty"`
	Hooks       map[string]string `json:"hooks,omitempty"` // setup/teardown status
	DurationMs  int64             `json:"duration_ms"`
	Summary     Summary           `json:"summary"`
	Steps       []StepResult      `json:"steps"`
}

// Summary represents execution summary counts.
type Summary struct {
	Total   int `json:"total"`
	Passed  int `json:"passed"`
	Failed  int `json:"failed"`
	Skipped int `json:"skipped"`
}

// RunbookMeta represents metadata extracted from runbook headings.
type RunbookMeta struct {
	Title string
	Scope string
	Env   string
}

// msDuration converts time.Duration to milliseconds.
func msDuration(d time.Duration) int64 {
	return d.Milliseconds()
}
