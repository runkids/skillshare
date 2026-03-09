package main

import (
	"context"
	"fmt"
	"io"
	"time"
)

// RunOptions controls runner behavior.
type RunOptions struct {
	DryRun     bool
	JSONOutput io.Writer
	Timeout    time.Duration
	Setup      string // command to run before the runbook
	Teardown   string // command to run after the runbook
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
		// Inject setup/teardown as synthetic steps.
		execSteps := steps
		setupIdx, teardownIdx := -1, -1
		if opts.Setup != "" {
			setupStep := Step{
				Number:   -1,
				Title:    "[setup]",
				Command:  opts.Setup,
				Lang:     "bash",
				Executor: ExecutorAuto,
			}
			setupIdx = 0
			execSteps = append([]Step{setupStep}, execSteps...)
		}
		if opts.Teardown != "" {
			teardownStep := Step{
				Number:   -2,
				Title:    "[teardown]",
				Command:  opts.Teardown,
				Lang:     "bash",
				Executor: ExecutorAuto,
			}
			teardownIdx = len(execSteps)
			execSteps = append(execSteps, teardownStep)
		}

		allResults := ExecuteSession(context.Background(), execSteps, opts.Timeout)

		// Extract setup result — if setup failed, mark all runbook steps skipped.
		var setupResult *StepResult
		if setupIdx >= 0 {
			sr := allResults[setupIdx]
			setupResult = &sr
		}

		// Extract teardown result.
		var teardownResult *StepResult
		if teardownIdx >= 0 {
			// teardownIdx was computed before prepend; adjust if setup was added.
			actualIdx := teardownIdx
			tr := allResults[actualIdx]
			teardownResult = &tr
		}

		// Collect runbook step results (skip synthetic steps).
		startIdx := 0
		if setupIdx >= 0 {
			startIdx = 1
		}
		endIdx := len(allResults)
		if teardownIdx >= 0 {
			endIdx = len(allResults) - 1
		}
		results = allResults[startIdx:endIdx]

		// If setup failed, mark all runbook steps as skipped.
		if setupResult != nil && setupResult.Status == StatusFailed {
			for i := range results {
				if results[i].Status != StatusSkipped {
					results[i].Status = StatusSkipped
					results[i].Error = "setup failed: " + setupResult.Error
					if setupResult.ExitCode != 0 && results[i].Error == "setup failed: " {
						results[i].Error = fmt.Sprintf("setup failed (exit code %d)", setupResult.ExitCode)
					}
				}
			}
		}

		// Include setup/teardown in report metadata (not in steps).
		_ = teardownResult // teardown failure is informational only
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
