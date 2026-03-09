package main

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// stepEndPattern matches the end-of-step marker emitted by the session script.
// Format: @@RB:END:<step_number>:<exit_code>:<duration_ms>@@
// Step number can be negative (synthetic setup/teardown steps use -1, -2).
var stepEndPattern = regexp.MustCompile(`^@@RB:END:(-?\d+):(-?\d+):(\d+)@@$`)

// indexedStep pairs a Step with its position in the original steps slice.
type indexedStep struct {
	idx  int
	step Step
}

// ExecuteSession runs all auto steps in a single bash session, preserving
// shell variables across steps via an env file. Each step runs in a subshell
// with pipefail; an EXIT trap saves exported variables for the next step.
// The step's exit code is the exit code of the last command in the subshell.
func ExecuteSession(ctx context.Context, steps []Step, timeout time.Duration) []StepResult {
	if timeout == 0 {
		timeout = 10 * time.Minute
	}

	results := make([]StepResult, len(steps))

	// Defense in depth: refuse to execute outside a container.
	if !IsContainerEnv() {
		for i, s := range steps {
			results[i] = StepResult{
				Step:   s,
				Status: StatusFailed,
				Error:  ErrNotInContainer.Error(),
			}
		}
		return results
	}

	// Collect auto steps with their original indices.
	var autoSteps []indexedStep
	for i, s := range steps {
		if s.Executor == ExecutorAuto {
			autoSteps = append(autoSteps, indexedStep{idx: i, step: s})
		} else {
			results[i] = StepResult{Step: s, Status: StatusSkipped}
		}
	}
	if len(autoSteps) == 0 {
		return results
	}

	// Create temp dir for stderr files and env persistence.
	tmpDir, err := os.MkdirTemp("", "runbook-session-*")
	if err != nil {
		for _, as := range autoSteps {
			results[as.idx] = StepResult{
				Step:   as.step,
				Status: StatusFailed,
				Error:  fmt.Sprintf("create temp dir: %v", err),
			}
		}
		return results
	}
	defer os.RemoveAll(tmpDir)

	script := buildSessionScript(autoSteps, tmpDir)

	scriptFile := filepath.Join(tmpDir, "session.sh")
	if err := os.WriteFile(scriptFile, []byte(script), 0700); err != nil {
		for _, as := range autoSteps {
			results[as.idx] = StepResult{
				Step:   as.step,
				Status: StatusFailed,
				Error:  fmt.Sprintf("write script: %v", err),
			}
		}
		return results
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "bash", scriptFile)
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = nil // step-level stderr goes to temp files

	_ = cmd.Run() // exit code is per-step, not global

	parseSessionResults(&stdout, autoSteps, results, tmpDir)

	// Run assertions on each completed step.
	// If assertions are defined, they always run (regardless of exit code)
	// and determine the final pass/fail status.
	for _, as := range autoSteps {
		r := &results[as.idx]
		if r.Status == StatusPassed || r.Status == StatusFailed {
			checkAssertions(r, as.step)
		}
	}

	return results
}

// buildSessionScript generates a single bash script that executes all steps
// sequentially, emitting markers to stdout for per-step output parsing.
func buildSessionScript(steps []indexedStep, tmpDir string) string {
	envFile := filepath.Join(tmpDir, "env")

	var sb strings.Builder
	sb.WriteString("#!/bin/bash\n")
	sb.WriteString("set -o pipefail\n\n")

	// Timing helper: works on Linux (date +%s%N) and macOS (date +%s).
	sb.WriteString("__rb_now_ms() {\n")
	sb.WriteString("  local ns\n")
	sb.WriteString("  ns=$(date +%s%N 2>/dev/null)\n")
	sb.WriteString("  if [ ${#ns} -gt 10 ]; then\n")
	sb.WriteString("    echo $(( ns / 1000000 ))\n")
	sb.WriteString("  else\n")
	sb.WriteString("    echo $(( $(date +%s) * 1000 ))\n")
	sb.WriteString("  fi\n")
	sb.WriteString("}\n\n")

	for _, as := range steps {
		n := as.step.Number
		errFile := filepath.Join(tmpDir, fmt.Sprintf("step_%d_err", n))

		command := as.step.Command
		command = strings.ReplaceAll(command, "\n---\n", "\n")

		fmt.Fprintf(&sb, "# Step %d: %s\n", n, as.step.Title)
		fmt.Fprintf(&sb, "echo '@@RB:BEGIN:%d@@'\n", n)
		fmt.Fprintf(&sb, "__rb_t0=$(__rb_now_ms)\n")

		// Subshell: isolates failures while env file + trap persist variables.
		// No set -e: the step's exit code is the last command's exit code,
		// so authors don't need workarounds like `cmd || EXIT=$?`.
		fmt.Fprintf(&sb, "(\n")
		fmt.Fprintf(&sb, "  set -o pipefail -a\n")
		fmt.Fprintf(&sb, "  [ -f %q ] && source %q\n", envFile, envFile)
		fmt.Fprintf(&sb, "  __rb_save_env() { export -p > %q 2>/dev/null; }\n", envFile)
		fmt.Fprintf(&sb, "  trap __rb_save_env EXIT\n")
		fmt.Fprintf(&sb, "  %s\n", command)
		fmt.Fprintf(&sb, ") 2>%q\n", errFile)

		fmt.Fprintf(&sb, "__rb_rc=$?\n")
		fmt.Fprintf(&sb, "__rb_t1=$(__rb_now_ms)\n")
		fmt.Fprintf(&sb, "__rb_dur=$(( __rb_t1 - __rb_t0 ))\n")
		fmt.Fprintf(&sb, "echo \"@@RB:END:%d:${__rb_rc}:${__rb_dur}@@\"\n\n", n)
	}

	return sb.String()
}

// parseSessionResults reads the combined stdout and splits it into per-step
// results using the @@RB:BEGIN/END markers.
func parseSessionResults(stdout *bytes.Buffer, autoSteps []indexedStep, results []StepResult, tmpDir string) {
	// Build a map from step number to autoSteps index.
	stepMap := make(map[int]int, len(autoSteps))
	for i, as := range autoSteps {
		stepMap[as.step.Number] = i
	}

	scanner := bufio.NewScanner(stdout)
	var currentBuf strings.Builder
	inStep := false

	for scanner.Scan() {
		line := scanner.Text()

		if strings.HasPrefix(line, "@@RB:BEGIN:") && strings.HasSuffix(line, "@@") {
			numStr := line[len("@@RB:BEGIN:") : len(line)-2]
			if _, err := strconv.Atoi(numStr); err == nil {
				currentBuf.Reset()
				inStep = true
			}
			continue
		}

		if m := stepEndPattern.FindStringSubmatch(line); m != nil {
			stepNum, _ := strconv.Atoi(m[1])
			exitCode, _ := strconv.Atoi(m[2])
			durationMs, _ := strconv.ParseInt(m[3], 10, 64)

			if asIdx, ok := stepMap[stepNum]; ok {
				as := autoSteps[asIdx]
				r := &results[as.idx]
				r.Step = as.step
				r.ExitCode = exitCode
				r.DurationMs = durationMs
				r.Stdout = currentBuf.String()

				// Read stderr from temp file.
				errFile := filepath.Join(tmpDir, fmt.Sprintf("step_%d_err", stepNum))
				if data, err := os.ReadFile(errFile); err == nil {
					r.Stderr = string(data)
				}

				if exitCode == 0 {
					r.Status = StatusPassed
				} else {
					r.Status = StatusFailed
				}
			}
			inStep = false
			continue
		}

		if inStep {
			if currentBuf.Len() > 0 {
				currentBuf.WriteByte('\n')
			}
			currentBuf.WriteString(line)
		}
	}

	// Mark any auto steps without results as failed (e.g., script aborted).
	for _, as := range autoSteps {
		if results[as.idx].Status == "" {
			results[as.idx] = StepResult{
				Step:   as.step,
				Status: StatusFailed,
				Error:  "step did not complete (session aborted)",
			}
		}
	}
}
