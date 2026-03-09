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
	results := make([]StepResult, 0, len(steps))

	for _, s := range steps {
		if opts.DryRun || s.Executor == ExecutorManual {
			results = append(results, StepResult{
				Step:   s,
				Status: StatusSkipped,
			})
			continue
		}

		ctx, cancel := context.WithTimeout(context.Background(), opts.Timeout)
		result := Execute(ctx, s)
		cancel()

		checkAssertions(&result, s)

		results = append(results, result)
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
