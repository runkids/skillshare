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
	TUI        bool
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
		if opts.DryRun || s.Executor == "manual" {
			results = append(results, StepResult{
				Step:   s,
				Status: "skipped",
			})
			continue
		}

		ctx, cancel := context.WithTimeout(context.Background(), opts.Timeout)
		result := Execute(ctx, s)
		cancel()

		// Run assertions when exit code is 0 and expected patterns exist.
		if result.ExitCode == 0 && len(s.Expected) > 0 {
			combined := result.Stdout + "\n" + result.Stderr
			result.Assertions = MatchAssertions(combined, s.Expected)
			if !AllPassed(result.Assertions) {
				result.Status = "failed"
			}
		}

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
		case "passed":
			s.Passed++
		case "failed":
			s.Failed++
		case "skipped":
			s.Skipped++
		}
	}
	return s
}
