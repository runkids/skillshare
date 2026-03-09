package main

import (
	"context"
	"fmt"
	"io"
	"slices"
	"time"
)

// RunOptions controls runner behavior.
type RunOptions struct {
	DryRun     bool
	JSONOutput io.Writer
	Timeout    time.Duration
	Setup      string // command to run before the runbook
	Teardown   string // command to run after the runbook
	Steps      []int  // only run these step numbers (empty = all)
	From       int    // run from this step number onwards (0 = disabled)
	FailFast   bool   // stop after first failed step
}

// shouldRun reports whether stepNum should execute given the filter flags.
func (o RunOptions) shouldRun(stepNum int) bool {
	if len(o.Steps) > 0 {
		return slices.Contains(o.Steps, stepNum)
	}
	if o.From > 0 {
		return stepNum >= o.From
	}
	return true
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

	// Apply step filter: mark filtered-out auto steps as manual so they get skipped.
	for i, s := range steps {
		if s.Executor == ExecutorAuto && !opts.shouldRun(s.Number) {
			steps[i].Executor = ExecutorManual
		}
	}

	start := time.Now()
	var results []StepResult
	var setupResult, teardownResult *StepResult

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

		allResults := ExecuteSession(context.Background(), execSteps, opts.Timeout, opts.FailFast)

		// Extract setup result — if setup failed, mark all runbook steps skipped.
		if setupIdx >= 0 {
			sr := allResults[setupIdx]
			setupResult = &sr
		}

		// Extract teardown result.
		if teardownIdx >= 0 {
			tr := allResults[teardownIdx]
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
			reason := setupResult.Error
			if reason == "" && setupResult.ExitCode != 0 {
				reason = fmt.Sprintf("exit code %d", setupResult.ExitCode)
			}
			for i := range results {
				if results[i].Status != StatusSkipped {
					results[i].Status = StatusSkipped
					results[i].Error = "setup failed: " + reason
				}
			}
		}

		// Include setup/teardown in report metadata (not in steps).
		_ = teardownResult // teardown failure is informational only
	}

	// Build hooks metadata for the report.
	hooks := make(map[string]string)
	if setupResult != nil {
		hooks["setup"] = setupResult.Status
	}
	if teardownResult != nil {
		hooks["teardown"] = teardownResult.Status
	}

	report := Report{
		Version:    "1",
		Runbook:    name,
		DurationMs: msDuration(time.Since(start)),
		Summary:    computeSummary(results),
		Steps:      results,
	}
	if len(hooks) > 0 {
		report.Hooks = hooks
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
