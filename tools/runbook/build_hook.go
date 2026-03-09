package main

import (
	"fmt"
	"os"
	"os/exec"
	"time"
)

// HookResult holds the outcome of a build hook execution.
type HookResult struct {
	OK       bool
	ExitCode int
	Output   string
	Duration time.Duration
}

// runBuildHook executes the build command as a simple shell process.
// Unlike setup/teardown which run inside the session executor,
// build runs once before any runbook and uses os/exec directly.
func runBuildHook(command string) *HookResult {
	start := time.Now()

	cmd := exec.Command("bash", "-c", command)
	cmd.Stdout = os.Stderr // show build output to stderr so it doesn't mix with JSON
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	fmt.Fprintf(os.Stderr, "  Build: running...\n")

	err := cmd.Run()
	dur := time.Since(start)

	result := &HookResult{
		OK:       err == nil,
		Duration: dur,
	}

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			result.ExitCode = exitErr.ExitCode()
		} else {
			result.ExitCode = 1
			result.Output = err.Error()
		}
	}

	if result.OK {
		fmt.Fprintf(os.Stderr, "  Build: passed (%.1fs)\n", dur.Seconds())
	} else {
		fmt.Fprintf(os.Stderr, "  Build: failed (%.1fs)\n", dur.Seconds())
	}

	return result
}
