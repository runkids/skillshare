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
func ExecuteSession(ctx context.Context, steps []Step, timeout time.Duration, failFast bool, envVars map[string]string) []StepResult {
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

	script := buildSessionScript(autoSteps, tmpDir, failFast, envVars)

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
	// Layer 2 fail-fast: if a step fails after assertions, skip remaining steps.
	failFastTriggered := false
	for _, as := range autoSteps {
		r := &results[as.idx]
		if failFast && failFastTriggered && r.Status != StatusSkipped {
			r.Status = StatusSkipped
			r.Error = "skipped: earlier step failed (--fail-fast)"
			continue
		}
		if r.Status == StatusPassed || r.Status == StatusFailed {
			checkAssertions(r, as.step)
		}
		if failFast && r.Status == StatusFailed {
			failFastTriggered = true
		}
	}

	return results
}

// buildSessionScript generates a single bash script that executes all steps
// sequentially, emitting markers to stdout for per-step output parsing.
// When failFast is true, a step failure sets __rb_stop=1 and subsequent steps
// emit skip markers (exit code -1) without executing.
func buildSessionScript(steps []indexedStep, tmpDir string, failFast bool, envVars map[string]string) string {
	envFile := filepath.Join(tmpDir, "env")

	var sb strings.Builder
	sb.WriteString("#!/bin/bash\n")
	sb.WriteString("set -o pipefail\n\n")

	// Seed environment variables from config.
	if len(envVars) > 0 {
		keys := sortedKeys(envVars)
		for _, k := range keys {
			fmt.Fprintf(&sb, "export %s=%q\n", k, envVars[k])
		}
		sb.WriteByte('\n')
	}

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

	if failFast {
		sb.WriteString("__rb_stop=0\n\n")
	}

	for _, as := range steps {
		n := as.step.Number
		errFile := filepath.Join(tmpDir, fmt.Sprintf("step_%d_err", n))

		command := as.step.Command
		command = strings.ReplaceAll(command, "\n---\n", "\n")

		fmt.Fprintf(&sb, "# Step %d: %s\n", n, as.step.Title)

		if failFast {
			fmt.Fprintf(&sb, "if [ \"$__rb_stop\" = \"0\" ]; then\n")
		}

		// depends directive: skip if the depended-on step failed.
		if as.step.DependsOn > 0 {
			fmt.Fprintf(&sb, "if [ \"${__rb_status_%d:-1}\" = \"0\" ]; then\n", as.step.DependsOn)
		}

		fmt.Fprintf(&sb, "echo '@@RB:BEGIN:%d@@'\n", n)
		fmt.Fprintf(&sb, "__rb_t0=$(__rb_now_ms)\n")

		// Build the subshell body.
		subshell := buildStepSubshell(as.step, command, envFile, errFile)

		// retry directive: wrap subshell in a for loop.
		if as.step.Retry > 0 {
			attempts := as.step.Retry + 1
			fmt.Fprintf(&sb, "for __rb_attempt in $(seq 1 %d); do\n", attempts)
			fmt.Fprintf(&sb, "%s", subshell)
			fmt.Fprintf(&sb, "__rb_rc=$?\n")
			fmt.Fprintf(&sb, "[ $__rb_rc -eq 0 ] && break\n")
			if as.step.RetryDelay > 0 {
				fmt.Fprintf(&sb, "[ $__rb_attempt -lt %d ] && sleep %d\n", attempts, int(as.step.RetryDelay.Seconds()))
			}
			sb.WriteString("done\n")
		} else {
			fmt.Fprintf(&sb, "%s", subshell)
			fmt.Fprintf(&sb, "__rb_rc=$?\n")
		}

		fmt.Fprintf(&sb, "__rb_t1=$(__rb_now_ms)\n")
		fmt.Fprintf(&sb, "__rb_dur=$(( __rb_t1 - __rb_t0 ))\n")
		fmt.Fprintf(&sb, "echo \"@@RB:END:%d:${__rb_rc}:${__rb_dur}@@\"\n", n)

		// Track step status for depends directives.
		fmt.Fprintf(&sb, "__rb_status_%d=$__rb_rc\n", n)

		// Close depends block.
		if as.step.DependsOn > 0 {
			fmt.Fprintf(&sb, "else\n")
			fmt.Fprintf(&sb, "echo '@@RB:END:%d:-2:0@@'\n", n)
			fmt.Fprintf(&sb, "__rb_status_%d=-2\n", n)
			fmt.Fprintf(&sb, "__rb_rc=0\n") // prevent stale __rb_rc from triggering fail-fast
			fmt.Fprintf(&sb, "fi\n")
		}

		if failFast {
			fmt.Fprintf(&sb, "[ $__rb_rc -ne 0 ] && __rb_stop=1\n")
			fmt.Fprintf(&sb, "else\n")
			// Emit skip marker: exit code -1 sentinel for skipped.
			fmt.Fprintf(&sb, "echo '@@RB:END:%d:-1:0@@'\n", n)
			fmt.Fprintf(&sb, "fi\n")
		}

		sb.WriteByte('\n')
	}

	return sb.String()
}

// buildStepSubshell generates the subshell block for a single step.
// Returns the subshell string WITHOUT the trailing `__rb_rc=$?`.
func buildStepSubshell(step Step, command, envFile, errFile string) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "(\n")
	fmt.Fprintf(&sb, "  set -o pipefail -a\n")
	fmt.Fprintf(&sb, "  [ -f %q ] && source %q\n", envFile, envFile)
	fmt.Fprintf(&sb, "  __rb_save_env() { export -p > %q 2>/dev/null; }\n", envFile)
	fmt.Fprintf(&sb, "  trap __rb_save_env EXIT\n")

	if step.Timeout > 0 {
		secs := int(step.Timeout.Seconds())
		if secs < 1 {
			secs = 1
		}
		fmt.Fprintf(&sb, "  timeout %d bash <<'__RB_STEP_%d__'\n", secs, step.Number)
		fmt.Fprintf(&sb, "%s\n", command)
		fmt.Fprintf(&sb, "__RB_STEP_%d__\n", step.Number)
	} else {
		fmt.Fprintf(&sb, "  %s\n", command)
	}

	fmt.Fprintf(&sb, ") 2>%q\n", errFile)
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

				// Exit code -1 is a sentinel for fail-fast skipped steps.
				// Exit code -2 is a sentinel for depends-skipped steps.
				if exitCode == -1 {
					r.Status = StatusSkipped
					r.Error = "skipped: earlier step failed (--fail-fast)"
				} else if exitCode == -2 {
					r.Status = StatusSkipped
					r.Error = fmt.Sprintf("skipped: depends on step %d (failed)", as.step.DependsOn)
				} else {
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
