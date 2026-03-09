package main

import (
	"bytes"
	"context"
	"errors"
	"os/exec"
	"strings"
	"syscall"
	"time"
)

// Execute runs a bash command from a Step and returns the result.
func Execute(ctx context.Context, s Step) StepResult {
	start := time.Now()

	command := s.Command
	// Merge consecutive code blocks separated by "---".
	command = strings.ReplaceAll(command, "\n---\n", "\n")

	cmd := exec.CommandContext(ctx, "bash", "-eo", "pipefail", "-c", command)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	elapsed := time.Since(start)

	result := StepResult{
		Step:       s,
		DurationMs: msDuration(elapsed),
		Stdout:     stdout.String(),
		Stderr:     stderr.String(),
	}

	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			if ws, ok := exitErr.Sys().(syscall.WaitStatus); ok {
				result.ExitCode = ws.ExitStatus()
			} else {
				result.ExitCode = exitErr.ExitCode()
			}
			result.Status = "failed"
		} else if ctx.Err() != nil {
			// Context timeout or cancellation.
			result.Status = "failed"
			result.ExitCode = -1
			result.Error = ctx.Err().Error()
		} else {
			result.Status = "failed"
			result.ExitCode = -1
			result.Error = err.Error()
		}
		return result
	}

	result.Status = "passed"
	result.ExitCode = 0
	return result
}
