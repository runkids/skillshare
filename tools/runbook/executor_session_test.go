package main

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestExecuteSession_SimpleEcho(t *testing.T) {
	steps := []Step{
		{Number: 1, Title: "echo", Command: "echo hello", Executor: ExecutorAuto},
	}
	results := ExecuteSession(context.Background(), steps, 30*time.Second)

	if results[0].Status != StatusPassed {
		t.Fatalf("expected passed, got %s (err=%s stderr=%q)", results[0].Status, results[0].Error, results[0].Stderr)
	}
	if got := strings.TrimSpace(results[0].Stdout); got != "hello" {
		t.Fatalf("expected stdout 'hello', got %q", got)
	}
}

func TestExecuteSession_VariablePersistence(t *testing.T) {
	steps := []Step{
		{Number: 1, Title: "set var", Command: "MY_VAR=fromstep1\necho \"set MY_VAR=$MY_VAR\"", Executor: ExecutorAuto},
		{Number: 2, Title: "read var", Command: "echo \"got MY_VAR=$MY_VAR\"", Executor: ExecutorAuto},
	}
	results := ExecuteSession(context.Background(), steps, 30*time.Second)

	if results[0].Status != StatusPassed {
		t.Fatalf("step 1: expected passed, got %s (err=%s stderr=%q)", results[0].Status, results[0].Error, results[0].Stderr)
	}
	if results[1].Status != StatusPassed {
		t.Fatalf("step 2: expected passed, got %s (err=%s stderr=%q)", results[1].Status, results[1].Error, results[1].Stderr)
	}
	if !strings.Contains(results[1].Stdout, "got MY_VAR=fromstep1") {
		t.Fatalf("step 2: expected variable from step 1, got stdout=%q", results[1].Stdout)
	}
}

func TestExecuteSession_StepFailureContinues(t *testing.T) {
	steps := []Step{
		{Number: 1, Title: "fail", Command: "echo before_fail && exit 1", Executor: ExecutorAuto},
		{Number: 2, Title: "still runs", Command: "echo after_fail", Executor: ExecutorAuto},
	}
	results := ExecuteSession(context.Background(), steps, 30*time.Second)

	if results[0].Status != StatusFailed {
		t.Fatalf("step 1: expected failed, got %s", results[0].Status)
	}
	if results[0].ExitCode != 1 {
		t.Fatalf("step 1: expected exit code 1, got %d", results[0].ExitCode)
	}
	if results[1].Status != StatusPassed {
		t.Fatalf("step 2: expected passed, got %s (err=%s stderr=%q)", results[1].Status, results[1].Error, results[1].Stderr)
	}
	if !strings.Contains(results[1].Stdout, "after_fail") {
		t.Fatalf("step 2: expected 'after_fail', got %q", results[1].Stdout)
	}
}

func TestExecuteSession_SkipsManual(t *testing.T) {
	steps := []Step{
		{Number: 1, Title: "auto", Command: "echo ok", Executor: ExecutorAuto},
		{Number: 2, Title: "manual", Command: "echo skip", Executor: ExecutorManual},
	}
	results := ExecuteSession(context.Background(), steps, 30*time.Second)

	if results[0].Status != StatusPassed {
		t.Fatalf("step 1: expected passed, got %s", results[0].Status)
	}
	if results[1].Status != StatusSkipped {
		t.Fatalf("step 2: expected skipped, got %s", results[1].Status)
	}
}

func TestExecuteSession_CapturesStderr(t *testing.T) {
	steps := []Step{
		{Number: 1, Title: "stderr", Command: "echo out && echo err >&2", Executor: ExecutorAuto},
	}
	results := ExecuteSession(context.Background(), steps, 30*time.Second)

	if results[0].Status != StatusPassed {
		t.Fatalf("expected passed, got %s (err=%s)", results[0].Status, results[0].Error)
	}
	if !strings.Contains(results[0].Stdout, "out") {
		t.Errorf("expected stdout to contain 'out', got %q", results[0].Stdout)
	}
	if !strings.Contains(results[0].Stderr, "err") {
		t.Errorf("expected stderr to contain 'err', got %q", results[0].Stderr)
	}
}

func TestExecuteSession_VariableSurvivedFailedStep(t *testing.T) {
	steps := []Step{
		{Number: 1, Title: "set and fail", Command: "SURV=yes\necho set_surv\nfalse", Executor: ExecutorAuto},
		{Number: 2, Title: "check surv", Command: "echo \"SURV=$SURV\"", Executor: ExecutorAuto},
	}
	results := ExecuteSession(context.Background(), steps, 30*time.Second)

	if results[0].Status != StatusFailed {
		t.Fatalf("step 1: expected failed, got %s", results[0].Status)
	}
	// EXIT trap should have saved SURV even though step failed.
	if results[1].Status != StatusPassed {
		t.Fatalf("step 2: expected passed, got %s (err=%s stderr=%q)", results[1].Status, results[1].Error, results[1].Stderr)
	}
	if !strings.Contains(results[1].Stdout, "SURV=yes") {
		t.Fatalf("step 2: expected SURV=yes, got %q", results[1].Stdout)
	}
}

func TestExecuteSession_Assertions(t *testing.T) {
	steps := []Step{
		{
			Number:   1,
			Title:    "with expected",
			Command:  "echo apple banana",
			Expected: []string{"apple", "cherry"},
			Executor: ExecutorAuto,
		},
	}
	results := ExecuteSession(context.Background(), steps, 30*time.Second)

	// Command succeeds but assertion for "cherry" fails.
	if results[0].Status != StatusFailed {
		t.Fatalf("expected failed due to assertion, got %s", results[0].Status)
	}
	if len(results[0].Assertions) != 2 {
		t.Fatalf("expected 2 assertions, got %d", len(results[0].Assertions))
	}
	if !results[0].Assertions[0].Matched {
		t.Error("'apple' assertion should have matched")
	}
	if results[0].Assertions[1].Matched {
		t.Error("'cherry' assertion should NOT have matched")
	}
}

func TestExecuteSession_EmptySteps(t *testing.T) {
	results := ExecuteSession(context.Background(), nil, 30*time.Second)
	if len(results) != 0 {
		t.Fatalf("expected 0 results, got %d", len(results))
	}
}

func TestExecuteSession_MergedCodeBlocks(t *testing.T) {
	steps := []Step{
		{Number: 1, Title: "merged", Command: "echo first\n---\necho second", Executor: ExecutorAuto},
	}
	results := ExecuteSession(context.Background(), steps, 30*time.Second)

	if results[0].Status != StatusPassed {
		t.Fatalf("expected passed, got %s (err=%s stderr=%q)", results[0].Status, results[0].Error, results[0].Stderr)
	}
	if !strings.Contains(results[0].Stdout, "first") || !strings.Contains(results[0].Stdout, "second") {
		t.Fatalf("expected both 'first' and 'second', got %q", results[0].Stdout)
	}
}
