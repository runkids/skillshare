package main

import (
	"context"
	"io"
	"time"
)

// RunOptions controls runner behavior.
type RunOptions struct {
	DryRun     bool
	JSONOutput io.Writer
	Timeout    time.Duration
}

// RunRunbook parses, classifies, executes, and reports a runbook.
// Non-dry-run execution uses a session executor that preserves shell
// variables across steps (single bash process with env file persistence).
func RunRunbook(r io.Reader, name string, opts RunOptions) (Report, error) {
	if opts.Timeout == 0 {
		opts.Timeout = 2 * time.Minute
	}

	rb, err := ParseRunbook(r)
	if err != nil {
		return Report{}, err
	}

	steps := ClassifyAll(rb.Steps)

	start := time.Now()
	var results []StepResult

	if opts.DryRun {
		// Dry-run: skip all steps.
		results = make([]StepResult, len(steps))
		for i, s := range steps {
			results[i] = StepResult{Step: s, Status: StatusSkipped}
		}
	} else {
		// Session execution: single bash process, variables persist across steps.
		results = ExecuteSession(context.Background(), steps, opts.Timeout)
	}

	report := Report{
		Version:    "1",
		Runbook:    name,
		DurationMs: msDuration(time.Since(start)),
		Summary:    computeSummary(results),
		Steps:      results,
	}

	if opts.JSONOutput != nil {
		if err := WriteJSONReport(opts.JSONOutput, report); err != nil {
			return report, err
		}
	}

	return report, nil
}

// computeSummary tallies step results into a Summary.
func computeSummary(results []StepResult) Summary {
	var s Summary
	s.Total = len(results)
	for _, r := range results {
		switch r.Status {
		case StatusPassed:
			s.Passed++
		case StatusFailed:
			s.Failed++
		case StatusSkipped:
			s.Skipped++
		}
	}
	return s
}
